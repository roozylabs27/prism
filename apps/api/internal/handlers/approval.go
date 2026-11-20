package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
)

type approvalStore interface {
	Create(ctx context.Context, req *models.ApprovalRequest) error
	FindByID(ctx context.Context, id, userID string) (*models.ApprovalRequest, error)
	List(ctx context.Context, userID, status string, limit, offset int) ([]models.ApprovalRequest, int64, error)
	Decide(ctx context.Context, id, userID, actorID, decisionStatus, note string) (*models.ApprovalRequest, error)
	VerifyAndConsume(ctx context.Context, id, userID, toolName, mcpServerName, argumentsHash string) (*models.ApprovalRequest, error)
}

var _ approvalStore = (*repository.ApprovalRepository)(nil)

type ApprovalHandler struct {
	approvals approvalStore
	eventPub  *goredis.EventPublisher
	audit     *AdminAudit
}

func NewApprovalHandler(approvals approvalStore, eventPub *goredis.EventPublisher, audit *AdminAudit) *ApprovalHandler {
	return &ApprovalHandler{
		approvals: approvals,
		eventPub:  eventPub,
		audit:     audit,
	}
}

type approvalDecisionRequest struct {
	Note string `json:"note"`
}

func (h *ApprovalHandler) List(c *gin.Context) {
	if h == nil || h.approvals == nil {
		httputil.RespondInternalError(c, "Approvals store unavailable", errors.New("approvals nil"), "APPROVALS_UNAVAILABLE")
		return
	}
	userID := c.GetString("userId")
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	} else if l := c.Query("pageSize"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	items, total, err := h.approvals.List(middleware.ContextWithTenant(c), userID, status, limit, offset)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list approval requests", err, "APPROVALS_LIST_FAILED")
		return
	}
	if items == nil {
		items = []models.ApprovalRequest{}
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     items,
		"total":    total,
		"page":     page,
		"pageSize": limit,
	})
}

func (h *ApprovalHandler) Get(c *gin.Context) {
	if h == nil || h.approvals == nil {
		httputil.RespondInternalError(c, "Approvals store unavailable", errors.New("approvals nil"), "APPROVALS_UNAVAILABLE")
		return
	}
	userID := c.GetString("userId")
	item, err := h.approvals.FindByID(middleware.ContextWithTenant(c), c.Param("id"), userID)
	if err != nil {
		httputil.RespondNotFound(c, "Approval request not found", err, "APPROVAL_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, item)
}

// GetForGateway allows gateway API key callers (agents) to poll whether a pending
// approval hold has been approved or denied by a human operator.
func (h *ApprovalHandler) GetForGateway(c *gin.Context) {
	if h == nil || h.approvals == nil {
		httputil.RespondInternalError(c, "Approvals store unavailable", errors.New("approvals nil"), "APPROVALS_UNAVAILABLE")
		return
	}
	gatewayKey, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	if gatewayKey == nil {
		httputil.RespondUnauthorized(c, "Gateway key required", nil, "GATEWAY_KEY_REQUIRED")
		return
	}
	if _, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey); err != nil {
		httputil.RespondForbidden(c, err.Error(), err, "TENANT_SECURITY_ERROR")
		return
	}

	item, err := h.approvals.FindByID(middleware.ContextWithTenant(c), c.Param("approvalId"), gatewayKey.UserID)
	if err != nil {
		httputil.RespondNotFound(c, "Approval request not found", err, "APPROVAL_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	h.decide(c, models.ApprovalStatusApproved)
}

func (h *ApprovalHandler) Deny(c *gin.Context) {
	h.decide(c, models.ApprovalStatusDenied)
}

func (h *ApprovalHandler) decide(c *gin.Context, status string) {
	if h == nil || h.approvals == nil {
		httputil.RespondInternalError(c, "Approvals store unavailable", errors.New("approvals nil"), "APPROVALS_UNAVAILABLE")
		return
	}
	userID := c.GetString("userId")
	id := c.Param("id")
	var req approvalDecisionRequest
	if c.Request.ContentLength > 0 {
		_ = c.ShouldBindJSON(&req)
	}

	updated, err := h.approvals.Decide(middleware.ContextWithTenant(c), id, userID, userID, status, strings.TrimSpace(req.Note))
	if err != nil {
		if errors.Is(err, repository.ErrApprovalNotDecidable) {
			httputil.RespondError(c, http.StatusConflict, err.Error(), err, "APPROVAL_NOT_PENDING")
			return
		}
		httputil.RespondStoreError(c, err, "Approval request", "APPROVAL_DECIDE")
		return
	}

	if h.audit != nil {
		action := "approval.approve"
		if status == models.ApprovalStatusDenied {
			action = "approval.deny"
		}
		h.audit.Record(c, action, "approval_request", updated.ID, map[string]any{
			"toolName":      updated.ToolName,
			"mcpServerName": updated.MCPServerName,
			"status":        updated.Status,
		})
	}

	if h.eventPub != nil {
		_ = h.eventPub.Publish(c.Request.Context(), updated.UserID, "APPROVAL_DECIDED", map[string]interface{}{
			"approvalId":    updated.ID,
			"toolName":      updated.ToolName,
			"mcpServerName": updated.MCPServerName,
			"status":        updated.Status,
			"decidedBy":     updated.DecidedBy,
		})
	}

	c.JSON(http.StatusOK, updated)
}

func notifyApprovalRequired(ctx context.Context, pub *goredis.EventPublisher, approval *models.ApprovalRequest) {
	if approval == nil {
		return
	}
	if pub != nil {
		_ = pub.Publish(ctx, approval.UserID, "APPROVAL_REQUIRED", map[string]interface{}{
			"approvalId":    approval.ID,
			"toolName":      approval.ToolName,
			"mcpServerName": approval.MCPServerName,
			"agentName":     approval.AgentName,
			"reason":        approval.Reason,
			"expiresAt":     approval.ExpiresAt,
		})
	}
	webhookURL := strings.TrimSpace(os.Getenv("APPROVAL_WEBHOOK_URL"))
	if webhookURL != "" {
		payload, _ := json.Marshal(map[string]interface{}{
			"event":    "APPROVAL_REQUIRED",
			"approval": approval,
		})
		go func(target string, body []byte) {
			client := security.NewSafeHTTPClient(5 * time.Second)
			reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, target, bytes.NewReader(body))
			if err != nil {
				return
			}
			httpReq.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(httpReq)
			if err == nil && resp != nil {
				_ = resp.Body.Close()
			}
		}(webhookURL, payload)
	}
}
