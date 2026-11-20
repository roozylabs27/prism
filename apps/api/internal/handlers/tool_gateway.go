package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type toolGatewayExecutor interface {
	Execute(ctx context.Context, userID, toolName string, args map[string]interface{}, encKey string) (*proxy.ToolExecutionResult, error)
}

type toolDefinitionFinder interface {
	FindByUserAndName(ctx context.Context, userID, name string) (*models.Tool, error)
}

type ToolGatewayHandler struct {
	gateway       toolGatewayExecutor
	toolInv       *repository.ToolInvocationRepository
	eventPub      *goredis.EventPublisher
	encKey        string
	admission     *proxy.AdmissionController
	auditRecorder *proxy.AuditRecorder
	roleResolver  MemberRoleResolver
	approvals     approvalStore
	tools         toolDefinitionFinder
}

func NewToolGatewayHandler(gw *proxy.ToolGateway, toolInv *repository.ToolInvocationRepository, eventPub *goredis.EventPublisher, encKey string) *ToolGatewayHandler {
	var exec toolGatewayExecutor
	if gw != nil {
		exec = gw
	}
	return &ToolGatewayHandler{gateway: exec, toolInv: toolInv, eventPub: eventPub, encKey: encKey}
}

// SetGovernance wires admission control, audit trail recording, and role
// resolution onto tool execution so RBAC, quotas, budgets, and deny rules
// are enforced at the execution boundary.
func (h *ToolGatewayHandler) SetGovernance(admission *proxy.AdmissionController, auditRecorder *proxy.AuditRecorder, roleResolver MemberRoleResolver) {
	if h == nil {
		return
	}
	h.admission = admission
	h.auditRecorder = auditRecorder
	h.roleResolver = roleResolver
}

// SetApprovals wires persistent HITL approval storage and tool require_approval lookup.
func (h *ToolGatewayHandler) SetApprovals(approvals approvalStore, tools toolDefinitionFinder) {
	if h == nil {
		return
	}
	h.approvals = approvals
	h.tools = tools
}

type ExecuteToolRequest struct {
	Args       map[string]interface{} `json:"args" binding:"required"`
	ApprovalID string                 `json:"approvalId"`
}

func (h *ToolGatewayHandler) ExecuteTool(c *gin.Context) {
	toolName := c.Param("toolName")
	gatewayKey, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	if gatewayKey == nil {
		httputil.RespondUnauthorized(c, "Gateway key required", nil, "GATEWAY_KEY_REQUIRED")
		return
	}

	tenantCtx, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey)
	if err != nil {
		httputil.RespondForbidden(c, err.Error(), err, "TENANT_SECURITY_ERROR")
		return
	}

	agent := middleware.AgentFromContext(c)
	var agentID, agentName string
	if agent != nil {
		agentID, agentName = agent.ID, agent.Name
	}

	userRole := "developer"
	if h.roleResolver != nil && gatewayKey.UserID != "" && tenantCtx.OrgID != "" {
		resolvedRole, rErr := h.roleResolver.GetOrganizationMemberRole(c.Request.Context(), tenantCtx.OrgID, gatewayKey.UserID)
		if rErr != nil || resolvedRole == "" {
			httputil.RespondForbidden(c, ErrMemberRoleUnresolved.Error(), rErr, "TENANT_SECURITY_ERROR")
			return
		}
		userRole = resolvedRole
	}

	var req ExecuteToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	argsJSON, argsHash := repository.HashToolArguments(req.Args)
	approvalID := strings.TrimSpace(c.GetHeader("X-Prism-Approval-ID"))
	if approvalID == "" {
		approvalID = strings.TrimSpace(req.ApprovalID)
	}

	executionID := uuid.New().String()
	tenantScopedCtx := middleware.ContextWithTenant(c)

	var approvalRequired bool
	var approvalPolicyID, approvalPolicyName, approvalReason string

	if h.admission != nil {
		admRes, admErr := h.admission.Evaluate(tenantScopedCtx, proxy.AdmissionRequest{
			UserID:    gatewayKey.UserID,
			Role:      userRole,
			AgentID:   agentID,
			AgentName: agentName,
			Agent:     agent,
			ToolName:  toolName,
			TenantCtx: tenantCtx,
		})
		if admErr != nil {
			httputil.RespondInternalError(c, "Governance check failed", admErr, "GOVERNANCE_CHECK_FAILED")
			return
		}
		if admRes.Decision == proxy.AdmissionDeny {
			h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, admRes.HTTPStatus, 0, "denied", admRes.Reason)
			utils.RespondWithError(c, admRes.HTTPStatus, admRes.ErrorCode, admRes.Reason, "FORBIDDEN", admRes.PolicyID, admRes.PolicyName)
			return
		}
		if admRes.Decision == proxy.AdmissionRequireApproval {
			approvalRequired = true
			approvalPolicyID = admRes.PolicyID
			approvalPolicyName = admRes.PolicyName
			approvalReason = admRes.Reason
		}
	} else if agent != nil {
		if res := proxy.CheckAgentToolAccess(agent, toolName); !res.ToolAllowed {
			h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, http.StatusForbidden, 0, "denied", res.Reason)
			httputil.RespondForbidden(c, res.Reason, nil, "TOOL_PERMISSION_DENIED")
			return
		}
	}

	if !approvalRequired && h.tools != nil {
		if toolDef, err := h.tools.FindByUserAndName(tenantScopedCtx, gatewayKey.UserID, toolName); err == nil && toolDef != nil && toolDef.RequireApproval {
			approvalRequired = true
			approvalReason = "tool requires human approval before execution"
		}
	}

	if approvalID != "" {
		if h.approvals == nil {
			httputil.RespondForbidden(c, "Approval verification store is unavailable", nil, "APPROVAL_STORE_UNAVAILABLE")
			return
		}
		if _, err := h.approvals.VerifyAndConsume(tenantScopedCtx, approvalID, gatewayKey.UserID, toolName, "", argsHash); err != nil {
			h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, http.StatusForbidden, 0, "denied", err.Error())
			httputil.RespondForbidden(c, "Approval request is not valid, approved, matching arguments, or unexpired", err, "APPROVAL_INVALID")
			return
		}
	} else if approvalRequired {
		if h.approvals == nil {
			httputil.RespondForbidden(c, "Approval workflow unavailable", nil, "APPROVAL_WORKFLOW_UNAVAILABLE")
			return
		}
		approval := &models.ApprovalRequest{
			OrgID:         tenantCtx.OrgID,
			UserID:        gatewayKey.UserID,
			AgentID:       agentID,
			AgentName:     agentName,
			ToolName:      toolName,
			ArgumentsJSON: argsJSON,
			ArgumentsHash: argsHash,
			PolicyID:      approvalPolicyID,
			PolicyName:    approvalPolicyName,
			Reason:        approvalReason,
			Status:        models.ApprovalStatusPending,
		}
		if err := h.approvals.Create(tenantScopedCtx, approval); err != nil {
			httputil.RespondInternalError(c, "Failed to create approval request", err, "APPROVAL_CREATE_FAILED")
			return
		}
		notifyApprovalRequired(c.Request.Context(), h.eventPub, approval)
		h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, http.StatusAccepted, 0, "approval_required", approval.Reason)
		c.JSON(http.StatusAccepted, gin.H{
			"status":     "approval_required",
			"code":       "APPROVAL_REQUIRED",
			"approvalId": approval.ID,
			"expiresAt":  approval.ExpiresAt,
			"pollUrl":    "/v1/approvals/" + approval.ID,
			"policyId":   approval.PolicyID,
			"policyName": approval.PolicyName,
			"reason":     approval.Reason,
		})
		return
	}

	ctx, cancel := context.WithTimeout(tenantScopedCtx, 60*time.Second)
	defer cancel()

	result, err := h.gateway.Execute(ctx, gatewayKey.UserID, toolName, req.Args, h.encKey)
	if err != nil {
		h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, http.StatusBadGateway, 0, "error", err.Error())
		httputil.RespondError(c, http.StatusBadGateway, "Tool execution failed: "+err.Error(), err, "TOOL_EXECUTION_FAILED")
		return
	}

	if h.toolInv != nil {
		_ = h.toolInv.CreateBatch(tenantScopedCtx, executionID, []models.ToolCallRecord{
			{Name: toolName, Arguments: argsJSON},
		})
	}

	if h.eventPub != nil {
		_ = h.eventPub.Publish(c.Request.Context(), gatewayKey.UserID, "TOOL_EXECUTED", map[string]interface{}{
			"executionId": executionID,
			"tool":        toolName,
			"backend":     result.Backend,
			"statusCode":  result.StatusCode,
			"latencyMs":   result.LatencyMs,
		})
	}

	h.recordAudit(tenantScopedCtx, executionID, gatewayKey, tenantCtx.OrgID, agentID, agentName, userRole, toolName, result.StatusCode, result.LatencyMs, "compliant", string(argsJSON))
	c.JSON(http.StatusOK, result)
}

func (h *ToolGatewayHandler) recordAudit(ctx context.Context, reqID string, key *models.GatewayAPIKey, orgID, agentID, agentName, userRole, toolName string, statusCode, latencyMs int, compliance, payload string) {
	if h.auditRecorder == nil || key == nil {
		return
	}
	trail := &models.AIAuditTrail{
		RequestID:         reqID,
		UserID:            key.UserID,
		OrgID:             orgID,
		GatewayKeyID:      &key.ID,
		UserRole:          userRole,
		ModelSlug:         "tool:" + toolName,
		FailoverChain:     []string{},
		ToolsInvoked:      []string{toolName},
		ResourcesAccessed: []string{},
		MCPServersCalled:  []string{},
		StatusCode:        statusCode,
		LatencyMS:         latencyMs,
		PromptHash:        utils.HashSHA256(toolName + ":" + payload),
		ResponseHash:      utils.HashSHA256(compliance + ":" + reqID),
		ComplianceStatus:  compliance,
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}
	_ = h.auditRecorder.Record(ctx, trail)
}
