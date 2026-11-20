package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
	"github.com/roozylabs/prism/internal/utils"
)

type toolStore interface {
	ListByUserID(ctx context.Context, userID string) ([]models.Tool, error)
	FindByID(ctx context.Context, id, userID string) (*models.Tool, error)
	CreateWithBackendsTx(ctx context.Context, t *models.Tool, backends []models.ToolBackend) error
	Update(ctx context.Context, t *models.Tool) error
	Delete(ctx context.Context, id, userID string) error
	GetToolWithBackendsByID(ctx context.Context, id, userID string) (*models.ToolWithBackends, error)
}

type toolBackendStore interface {
	ListByToolID(ctx context.Context, toolID string) ([]models.ToolBackend, error)
	DeleteByToolID(ctx context.Context, toolID string) error
	Create(ctx context.Context, b *models.ToolBackend) error
}

var (
	_ toolStore        = (*repository.ToolRepository)(nil)
	_ toolBackendStore = (*repository.ToolBackendRepository)(nil)
)

type ToolHandler struct {
	tools    toolStore
	backends toolBackendStore
	gateway  *proxy.ToolGateway
	encKey   string
}

func NewToolHandler(tools toolStore, backends toolBackendStore, gateway *proxy.ToolGateway, encKey string) *ToolHandler {
	return &ToolHandler{tools: tools, backends: backends, gateway: gateway, encKey: encKey}
}

type BackendRequest struct {
	ID          string `json:"id"`
	Name        string `json:"name" binding:"required"`
	EndpointURL string `json:"endpointUrl" binding:"required"`
	AuthToken   string `json:"authToken"`
	TimeoutMs   int    `json:"timeoutMs"`
	Priority    int    `json:"priority"`
}

type CreateToolRequest struct {
	Name        string           `json:"name" binding:"required"`
	DisplayName string           `json:"displayName"`
	Description string           `json:"description"`
	InputSchema json.RawMessage  `json:"inputSchema"`
	Enabled     *bool            `json:"enabled"`
	Backends    []BackendRequest `json:"backends"`
}

type UpdateToolRequest struct {
	DisplayName *string           `json:"displayName"`
	Description *string           `json:"description"`
	InputSchema json.RawMessage   `json:"inputSchema"`
	Enabled     *bool             `json:"enabled"`
	Backends    *[]BackendRequest `json:"backends"`
}

func (h *ToolHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	tools, err := h.tools.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list tools", err, "TOOLS_LIST_FAILED")
		return
	}
	if tools == nil {
		tools = []models.Tool{}
	}
	c.JSON(http.StatusOK, tools)
}

func (h *ToolHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	twb, err := h.tools.GetToolWithBackendsByID(c.Request.Context(), c.Param("id"), userID)
	if err != nil {
		httputil.RespondNotFound(c, "Tool not found", err, "TOOL_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, twb)
}

func (h *ToolHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	schema := req.InputSchema
	if len(schema) == 0 {
		schema = json.RawMessage(`{}`)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	tool := &models.Tool{
		UserID:      userID,
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		InputSchema: schema,
		Enabled:     enabled,
	}

	var backends []models.ToolBackend
	for _, br := range req.Backends {
		if err := security.ValidateOutboundURL(br.EndpointURL); err != nil {
			httputil.RespondBadRequest(c, "Tool backend URL is blocked or invalid: "+br.Name, err, "TOOL_BACKEND_URL_BLOCKED")
			return
		}
		b := models.ToolBackend{
			Name:        br.Name,
			BackendType: "http",
			EndpointURL: br.EndpointURL,
			TimeoutMs:   br.TimeoutMs,
			Priority:    br.Priority,
			Enabled:     true,
		}
		if b.TimeoutMs <= 0 {
			b.TimeoutMs = 30000
		}
		if b.Priority <= 0 {
			b.Priority = 1
		}
		if strings.TrimSpace(br.AuthToken) != "" && h.encKey != "" {
			enc, err := utils.EncryptAES256GCM(br.AuthToken, h.encKey)
			if err != nil {
				httputil.RespondInternalError(c, "Failed to encrypt auth token for backend: "+br.Name, err, "ENCRYPTION_FAILED")
				return
			}
			b.AuthTokenEncrypted = &enc
		}
		backends = append(backends, b)
	}

	if err := h.tools.CreateWithBackendsTx(c.Request.Context(), tool, backends); err != nil {
		httputil.RespondStoreError(c, err, "Tool", "TOOL_CREATE")
		return
	}

	twb, _ := h.tools.GetToolWithBackendsByID(c.Request.Context(), tool.ID, userID)
	c.JSON(http.StatusCreated, twb)
}

func (h *ToolHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	existing, err := h.tools.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Tool not found", err, "TOOL_NOT_FOUND")
		return
	}

	var req UpdateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.Backends != nil {
		for _, br := range *req.Backends {
			if err := security.ValidateOutboundURL(br.EndpointURL); err != nil {
				httputil.RespondBadRequest(c, "Tool backend URL is blocked or invalid: "+br.Name, err, "TOOL_BACKEND_URL_BLOCKED")
				return
			}
		}
	}

	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.InputSchema != nil {
		existing.InputSchema = req.InputSchema
	}
	if len(existing.InputSchema) == 0 {
		existing.InputSchema = json.RawMessage(`{}`)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.tools.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Tool", "TOOL_UPDATE")
		return
	}

	if req.Backends != nil {
		priorBackends, err := h.backends.ListByToolID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to load existing backends", err, "BACKENDS_LOAD_FAILED")
			return
		}
		if err := h.backends.DeleteByToolID(c.Request.Context(), id); err != nil {
			httputil.RespondInternalError(c, "Failed to replace backends", err, "BACKENDS_REPLACE_FAILED")
			return
		}
		for _, br := range *req.Backends {
			prior := matchPriorToolBackend(br, priorBackends)
			if err := h.createBackendWithPrior(c.Request.Context(), id, br, prior); err != nil {
				httputil.RespondInternalError(c, "Backend failed: "+br.Name, err, "TOOL_BACKEND_FAILED")
				return
			}
		}
	}

	twb, _ := h.tools.GetToolWithBackendsByID(c.Request.Context(), id, userID)
	c.JSON(http.StatusOK, twb)
}

func (h *ToolHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	if err := h.tools.Delete(c.Request.Context(), c.Param("id"), userID); err != nil {
		httputil.RespondStoreError(c, err, "Tool", "TOOL_DELETE")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "tool deleted"})
}

type TestToolRequest struct {
	Args map[string]interface{} `json:"args" binding:"required"`
}

func (h *ToolHandler) TestTool(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	tool, err := h.tools.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Tool not found", err, "TOOL_NOT_FOUND")
		return
	}

	var req TestToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	result, err := h.gateway.Execute(ctx, userID, tool.Name, req.Args, h.encKey)
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "Tool execution failed: "+err.Error(), err, "TOOL_EXECUTION_FAILED")
		return
	}
	c.JSON(http.StatusOK, result)
}

func matchPriorToolBackend(br BackendRequest, prior []models.ToolBackend) *models.ToolBackend {
	if br.ID != "" {
		for i := range prior {
			if prior[i].ID == br.ID {
				return &prior[i]
			}
		}
	}
	lowerName := strings.ToLower(strings.TrimSpace(br.Name))
	for i := range prior {
		if strings.ToLower(strings.TrimSpace(prior[i].Name)) == lowerName && prior[i].EndpointURL == br.EndpointURL {
			return &prior[i]
		}
	}
	var uniqueMatch *models.ToolBackend
	for i := range prior {
		if strings.ToLower(strings.TrimSpace(prior[i].Name)) == lowerName {
			if uniqueMatch != nil {
				return nil
			}
			uniqueMatch = &prior[i]
		}
	}
	return uniqueMatch
}

func (h *ToolHandler) createBackendWithPrior(ctx context.Context, toolID string, br BackendRequest, prior *models.ToolBackend) error {
	backend := &models.ToolBackend{
		ToolID:      toolID,
		Name:        br.Name,
		BackendType: "http",
		EndpointURL: br.EndpointURL,
		TimeoutMs:   br.TimeoutMs,
		Priority:    br.Priority,
		Enabled:     true,
	}
	if br.TimeoutMs <= 0 {
		backend.TimeoutMs = 30000
	}
	if br.Priority <= 0 {
		backend.Priority = 1
	}
	if strings.TrimSpace(br.AuthToken) != "" && h.encKey != "" {
		enc, err := utils.EncryptAES256GCM(br.AuthToken, h.encKey)
		if err != nil {
			return err
		}
		backend.AuthTokenEncrypted = &enc
	} else if prior != nil && prior.AuthTokenEncrypted != nil {
		encCopy := *prior.AuthTokenEncrypted
		backend.AuthTokenEncrypted = &encCopy
		backend.AuthHeaderName = prior.AuthHeaderName
		backend.AuthHeaderPrefix = prior.AuthHeaderPrefix
	}
	return h.backends.Create(ctx, backend)
}
