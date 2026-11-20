package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

// GatewayKeyInvalidator drops a key from the gateway's lookup cache, so a
// deleted key stops working at once instead of when the cache entry expires.
type GatewayKeyInvalidator interface {
	Invalidate(keyHash string)
}

type gatewayKeyStore interface {
	Create(ctx context.Context, k *models.GatewayAPIKey) error
	ListByUserIDWithFilter(ctx context.Context, userID, search string, limit, offset int, orgID ...string) ([]models.GatewayAPIKey, int64, error)
	Delete(ctx context.Context, id string, userID ...string) (string, error)
}

type gatewayKeyCredentialCounter interface {
	CountActiveByProviderID(ctx context.Context, providerID string, userID ...string) (int64, error)
}

type gatewayKeyScopeValidator interface {
	FindWorkspaceInOrg(ctx context.Context, orgID, workspaceID string) (*models.Workspace, error)
	FindProjectInOrg(ctx context.Context, orgID, projectID string) (*models.Project, error)
}

type gatewayKeyAgentValidator interface {
	FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error)
}

type GatewayKeyHandler struct {
	keys           gatewayKeyStore
	credentials    gatewayKeyCredentialCounter
	cache          GatewayKeyInvalidator
	audit          *AdminAudit
	scopeValidator gatewayKeyScopeValidator
	agentValidator gatewayKeyAgentValidator
}

func NewGatewayKeyHandler(keys *repository.GatewayKeyRepository, credentials *repository.CredentialRepository, cache GatewayKeyInvalidator) *GatewayKeyHandler {
	var keyStore gatewayKeyStore
	if keys != nil {
		keyStore = keys
	}
	var credCounter gatewayKeyCredentialCounter
	if credentials != nil {
		credCounter = credentials
	}
	return &GatewayKeyHandler{keys: keyStore, credentials: credCounter, cache: cache}
}

func (h *GatewayKeyHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

func (h *GatewayKeyHandler) SetScopeValidator(v gatewayKeyScopeValidator) {
	h.scopeValidator = v
}

func (h *GatewayKeyHandler) SetAgentValidator(v gatewayKeyAgentValidator) {
	h.agentValidator = v
}

type CreateGatewayKeyRequest struct {
	Name          string   `json:"name" binding:"required"`
	ProviderID    string   `json:"providerId"`
	WorkspaceID   string   `json:"workspaceId"`
	ProjectID     string   `json:"projectId"`
	AgentID       string   `json:"agentId"`
	RateLimit     int      `json:"rateLimit"`
	AllowedModels []string `json:"allowedModels"`
	ExpiresInDays int      `json:"expiresInDays"`
}

type CreateGatewayKeyResponse struct {
	models.GatewayAPIKey
	RawKey string `json:"key"`
}

func (h *GatewayKeyHandler) Create(c *gin.Context) {
	var req CreateGatewayKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request: name is required", err, "NAME_REQUIRED")
		return
	}

	tc := middleware.GetTenantContext(c)
	orgID := strings.TrimSpace(tc.OrgID)
	wsID := strings.TrimSpace(req.WorkspaceID)
	if wsID == "" {
		wsID = strings.TrimSpace(tc.WorkspaceID)
	}
	projID := strings.TrimSpace(req.ProjectID)
	if projID == "" {
		projID = strings.TrimSpace(tc.ProjectID)
	}

	ctx := middleware.ContextWithTenant(c)
	if h.scopeValidator != nil && (wsID != "" || projID != "") {
		if orgID == "" {
			httputil.RespondForbidden(c, "Organization context is required to scope gateway key", nil, "TENANT_REQUIRED")
			return
		}
		if projID != "" {
			proj, err := h.scopeValidator.FindProjectInOrg(ctx, orgID, projID)
			if err != nil {
				httputil.RespondNotFound(c, "Project not found in this organization", err, "PROJECT_NOT_FOUND")
				return
			}
			if wsID != "" && wsID != proj.WorkspaceID {
				httputil.RespondBadRequest(c, "Project does not belong to the specified workspace", nil, "PROJECT_WORKSPACE_MISMATCH")
				return
			}
			if wsID == "" {
				wsID = proj.WorkspaceID
			}
		}
		if wsID != "" {
			if _, err := h.scopeValidator.FindWorkspaceInOrg(ctx, orgID, wsID); err != nil {
				httputil.RespondNotFound(c, "Workspace not found in this organization", err, "WORKSPACE_NOT_FOUND")
				return
			}
		}
	}

	userID := c.GetString("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}

	var agentIDPtr *string
	if agentRef := strings.TrimSpace(req.AgentID); agentRef != "" {
		if h.agentValidator != nil {
			ag, err := h.agentValidator.FindForUser(ctx, userID, agentRef)
			if err != nil || ag == nil {
				httputil.RespondNotFound(c, "Agent not found in this tenant", err, "AGENT_NOT_FOUND")
				return
			}
			if !ag.Enabled {
				httputil.RespondBadRequest(c, "Cannot create Gateway Key bound to a disabled agent", nil, "AGENT_DISABLED")
				return
			}
			idCopy := ag.ID
			agentIDPtr = &idCopy
		} else {
			agentIDPtr = &agentRef
		}
	}

	var providerIDPtr *string
	if req.ProviderID != "" && req.ProviderID != "global" {
		pID := req.ProviderID
		providerIDPtr = &pID

		if h.credentials != nil {
			count, err := h.credentials.CountActiveByProviderID(ctx, req.ProviderID, userID)
			if err != nil || count == 0 {
				httputil.RespondBadRequest(c, "Cannot create Gateway Key: the selected provider has no active credentials", err, "PROVIDER_NO_CREDENTIALS")
				return
			}
		}
	}

	rawKey, err := utils.NewGatewayKey()
	if err != nil {
		httputil.RespondInternalError(c, "Failed to generate key random bytes", err, "KEY_GENERATION_FAILED")
		return
	}
	keyHash := utils.HashSHA256(rawKey)
	keyPrefix := utils.GatewayKeyDisplayPrefix(rawKey)

	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = 100
	}

	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &t
	}

	var orgIDPtr, wsIDPtr, projIDPtr *string
	if orgID != "" {
		orgIDPtr = &orgID
	}
	if wsID != "" {
		wsIDPtr = &wsID
	}
	if projID != "" {
		projIDPtr = &projID
	}

	key := &models.GatewayAPIKey{
		UserID:        userID,
		OrgID:         orgIDPtr,
		WorkspaceID:   wsIDPtr,
		ProjectID:     projIDPtr,
		ProviderID:    providerIDPtr,
		AgentID:       agentIDPtr,
		Name:          req.Name,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		Enabled:       true,
		RateLimit:     rateLimit,
		AllowedModels: req.AllowedModels,
		ExpiresAt:     expiresAt,
	}

	if err := h.keys.Create(ctx, key); err != nil {
		httputil.RespondInternalError(c, "Failed to create gateway key", err, "GATEWAY_KEY_CREATE_FAILED")
		return
	}
	auditDetails := map[string]any{"name": key.Name, "keyPrefix": key.KeyPrefix, "rateLimit": key.RateLimit}
	if wsID != "" {
		auditDetails["workspaceId"] = wsID
	}
	if projID != "" {
		auditDetails["projectId"] = projID
	}
	if agentIDPtr != nil {
		auditDetails["agentId"] = *agentIDPtr
	}
	h.audit.Record(c, "gateway_key.create", "gateway_key", key.ID, auditDetails)

	c.JSON(http.StatusCreated, CreateGatewayKeyResponse{
		GatewayAPIKey: *key,
		RawKey:        rawKey,
	})
}

func (h *GatewayKeyHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	search := c.Query("search")

	pg := httputil.ParsePage(c)
	orgID := middleware.GetTenantContext(c).OrgID

	keys, total, err := h.keys.ListByUserIDWithFilter(middleware.ContextWithTenant(c), userID, search, pg.Limit, pg.Offset, orgID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list gateway keys", err, "GATEWAY_KEYS_LIST_FAILED")
		return
	}
	httputil.RespondPage(c, pg, keys, total)
}

func (h *GatewayKeyHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userId")
	keyHash, err := h.keys.Delete(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Gateway key not found", err, "GATEWAY_KEY_NOT_FOUND")
		return
	}
	if h.cache != nil {
		h.cache.Invalidate(keyHash)
	}
	h.audit.Record(c, "gateway_key.delete", "gateway_key", id, nil)
	c.Status(http.StatusNoContent)
}
