package handlers

import (
	"context"
	"encoding/json"
	"net/http"
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

type ResourceGatewayHandler struct {
	gateway       proxy.ResourceGatewayExecutor
	toolInv       *repository.ToolInvocationRepository
	eventPub      *goredis.EventPublisher
	encKey        string
	admission     *proxy.AdmissionController
	auditRecorder *proxy.AuditRecorder
	roleResolver  MemberRoleResolver
}

func NewResourceGatewayHandler(gw proxy.ResourceGatewayExecutor, toolInv *repository.ToolInvocationRepository, eventPub *goredis.EventPublisher, encKey string) *ResourceGatewayHandler {
	return &ResourceGatewayHandler{gateway: gw, toolInv: toolInv, eventPub: eventPub, encKey: encKey}
}

// SetGovernance wires admission control, audit trail recording, and role
// resolution onto resource queries so RBAC, quotas, budgets, and deny rules
// are enforced at the execution boundary.
func (h *ResourceGatewayHandler) SetGovernance(admission *proxy.AdmissionController, auditRecorder *proxy.AuditRecorder, roleResolver MemberRoleResolver) {
	if h == nil {
		return
	}
	h.admission = admission
	h.auditRecorder = auditRecorder
	h.roleResolver = roleResolver
}

type QueryResourceRequest struct {
	Args map[string]interface{} `json:"args" binding:"required"`
}

func (h *ResourceGatewayHandler) ExecuteQuery(c *gin.Context) {
	resourceName := c.Param("resourceName")
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

	executionID := uuid.New().String()

	if h.admission != nil {
		admRes, admErr := h.admission.Evaluate(middleware.ContextWithTenant(c), proxy.AdmissionRequest{
			UserID:       gatewayKey.UserID,
			Role:         userRole,
			AgentID:      agentID,
			AgentName:    agentName,
			Agent:        agent,
			ResourceName: resourceName,
			TenantCtx:    tenantCtx,
		})
		if admErr != nil {
			httputil.RespondInternalError(c, "Governance check failed", admErr, "GOVERNANCE_CHECK_FAILED")
			return
		}
		if admRes.Decision == proxy.AdmissionDeny {
			h.recordAudit(c.Request.Context(), executionID, gatewayKey, agentID, agentName, userRole, resourceName, admRes.HTTPStatus, 0, "denied", admRes.Reason)
			utils.RespondWithError(c, admRes.HTTPStatus, admRes.ErrorCode, admRes.Reason, "FORBIDDEN", admRes.PolicyID, admRes.PolicyName)
			return
		}
	} else if agent != nil {
		if res := proxy.CheckAgentResourceAccess(agent, resourceName); !res.ResourceAllowed {
			h.recordAudit(c.Request.Context(), executionID, gatewayKey, agentID, agentName, userRole, resourceName, http.StatusForbidden, 0, "denied", res.Reason)
			httputil.RespondForbidden(c, res.Reason, nil, "RESOURCE_PERMISSION_DENIED")
			return
		}
	}

	var req QueryResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	ctx, cancel := context.WithTimeout(middleware.ContextWithTenant(c), 60*time.Second)
	defer cancel()

	result, err := h.gateway.Execute(ctx, gatewayKey.UserID, resourceName, req.Args, h.encKey)
	if err != nil {
		h.recordAudit(c.Request.Context(), executionID, gatewayKey, agentID, agentName, userRole, resourceName, http.StatusBadGateway, 0, "error", err.Error())
		httputil.RespondError(c, http.StatusBadGateway, "Resource execution failed: "+err.Error(), err, "RESOURCE_EXECUTION_FAILED")
		return
	}

	argsJSON, _ := json.Marshal(req.Args)
	if h.toolInv != nil {
		_ = h.toolInv.CreateBatch(c.Request.Context(), executionID, []models.ToolCallRecord{
			{Name: "resource:" + resourceName, Arguments: argsJSON},
		})
	}

	if h.eventPub != nil {
		_ = h.eventPub.Publish(c.Request.Context(), gatewayKey.UserID, "RESOURCE_EXECUTED", map[string]interface{}{
			"executionId": executionID,
			"resource":    resourceName,
			"backend":     result.Backend,
			"backendType": result.BackendType,
			"statusCode":  result.StatusCode,
			"rowCount":    result.RowCount,
			"latencyMs":   result.LatencyMs,
		})
	}

	h.recordAudit(c.Request.Context(), executionID, gatewayKey, agentID, agentName, userRole, resourceName, result.StatusCode, result.LatencyMs, "compliant", string(argsJSON))
	c.JSON(http.StatusOK, result)
}

func (h *ResourceGatewayHandler) recordAudit(ctx context.Context, reqID string, key *models.GatewayAPIKey, agentID, agentName, userRole, resourceName string, statusCode, latencyMs int, compliance, payload string) {
	if h.auditRecorder == nil || key == nil {
		return
	}
	trail := &models.AIAuditTrail{
		RequestID:         reqID,
		UserID:            key.UserID,
		GatewayKeyID:      &key.ID,
		UserRole:          userRole,
		ModelSlug:         "resource:" + resourceName,
		FailoverChain:     []string{},
		ToolsInvoked:      []string{},
		ResourcesAccessed: []string{resourceName},
		MCPServersCalled:  []string{},
		StatusCode:        statusCode,
		LatencyMS:         latencyMs,
		PromptHash:        utils.HashSHA256(resourceName + ":" + payload),
		ResponseHash:      utils.HashSHA256(compliance + ":" + reqID),
		ComplianceStatus:  compliance,
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}
	_ = h.auditRecorder.Record(ctx, trail)
}
