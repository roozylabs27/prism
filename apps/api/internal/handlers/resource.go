package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
	"github.com/roozylabs/prism/internal/utils"
)

type resourceStore interface {
	ListByUserID(ctx context.Context, userID, workspaceID string) ([]models.Resource, error)
	FindByID(ctx context.Context, id, userID string) (*models.Resource, error)
	CreateWithBackendsTx(ctx context.Context, res *models.Resource, backends []models.ResourceBackend) error
	Update(ctx context.Context, res *models.Resource) error
	Delete(ctx context.Context, id, userID string) error
	GetResourceWithBackendsByID(ctx context.Context, id, userID string) (*models.ResourceWithBackends, error)
}

type resourceBackendStore interface {
	ListByResourceID(ctx context.Context, resourceID string) ([]models.ResourceBackend, error)
	DeleteByResourceID(ctx context.Context, resourceID string) error
	Create(ctx context.Context, b *models.ResourceBackend) error
}

var (
	_ resourceStore        = (*repository.ResourceRepository)(nil)
	_ resourceBackendStore = (*repository.ResourceBackendRepository)(nil)
)

type ResourceHandler struct {
	resources       resourceStore
	backends        resourceBackendStore
	gateway         proxy.ResourceGatewayExecutor
	encKey          string
	postgresEnabled bool
}

func NewResourceHandler(resources *repository.ResourceRepository, backends *repository.ResourceBackendRepository, gateway proxy.ResourceGatewayExecutor, encKey string) *ResourceHandler {
	var rStore resourceStore
	if resources != nil {
		rStore = resources
	}
	var bStore resourceBackendStore
	if backends != nil {
		bStore = backends
	}
	return &ResourceHandler{resources: rStore, backends: bStore, gateway: gateway, encKey: encKey}
}

// SetPostgresBackends allows backends of type "postgres". They open a
// connection to a host the user chooses, so they are off unless the
// operator turns them on (RESOURCE_POSTGRES_ENABLED).
func (h *ResourceHandler) SetPostgresBackends(enabled bool) {
	h.postgresEnabled = enabled
}

var errPostgresBackendsDisabled = errors.New("postgres backends are disabled on this gateway (RESOURCE_POSTGRES_ENABLED)")

// validateBackend refuses a backend that would make the gateway connect to
// an internal host: REST and GraphQL endpoints must be public URLs, and a
// postgres connection string must name a public host.
func (h *ResourceHandler) validateBackend(br CreateResourceBackendRequest) error {
	switch strings.ToLower(strings.TrimSpace(br.BackendType)) {
	case "postgres":
		if !h.postgresEnabled {
			return errPostgresBackendsDisabled
		}
		if br.ConnectionString == nil || strings.TrimSpace(*br.ConnectionString) == "" {
			return errors.New("postgres backend needs a connectionString")
		}
		return security.ValidatePostgresDSN(*br.ConnectionString)
	default:
		if br.EndpointURL == nil || strings.TrimSpace(*br.EndpointURL) == "" {
			return errors.New("backend needs an endpointUrl")
		}
		return security.ValidateOutboundURL(*br.EndpointURL)
	}
}

// backendsAllowed validates every backend of a request before anything is
// written, answering 400 (and returning false) on the first problem.
func (h *ResourceHandler) backendsAllowed(c *gin.Context, backends []CreateResourceBackendRequest) bool {
	for _, br := range backends {
		if err := h.validateBackend(br); err != nil {
			code := "RESOURCE_BACKEND_BLOCKED"
			if errors.Is(err, errPostgresBackendsDisabled) {
				code = "RESOURCE_POSTGRES_DISABLED"
			}
			httputil.RespondBadRequest(c, "Backend "+br.Name+": "+err.Error(), err, code)
			return false
		}
	}
	return true
}

func getTenantContext(c *gin.Context) (orgID, wsID, userID string) {
	orgID = c.GetString("organization_id")
	if orgID == "" {
		orgID = c.GetHeader("X-Prism-Org-ID")
	}
	wsID = c.GetString("workspace_id")
	if wsID == "" {
		wsID = c.GetHeader("X-Prism-Workspace-ID")
	}
	userID = c.GetString("userId")
	if userID == "" {
		userID = c.GetString("userID")
	}
	return
}

type CreateResourceBackendRequest struct {
	ID               string   `json:"id"`
	Name             string   `json:"name" binding:"required"`
	BackendType      string   `json:"backendType" binding:"required"`
	EndpointURL      *string  `json:"endpointUrl"`
	HTTPMethod       string   `json:"httpMethod"`
	AuthToken        string   `json:"authToken"`
	QueryTemplate    *string  `json:"queryTemplate"`
	ConnectionString *string  `json:"connectionString"`
	SQLQuery         *string  `json:"sqlQuery"`
	ParamNames       []string `json:"paramNames"`
	TimeoutMs        int      `json:"timeoutMs"`
	Priority         int      `json:"priority"`
}

type CreateResourceRequest struct {
	Name             string                         `json:"name" binding:"required"`
	DisplayName      string                         `json:"displayName"`
	Description      string                         `json:"description"`
	ParametersSchema json.RawMessage                `json:"parametersSchema"`
	Enabled          *bool                          `json:"enabled"`
	Backends         []CreateResourceBackendRequest `json:"backends"`
}

type UpdateResourceRequest struct {
	DisplayName      *string                         `json:"displayName"`
	Description      *string                         `json:"description"`
	ParametersSchema json.RawMessage                 `json:"parametersSchema"`
	Enabled          *bool                           `json:"enabled"`
	Backends         *[]CreateResourceBackendRequest `json:"backends"`
}

func (h *ResourceHandler) List(c *gin.Context) {
	_, wsID, userID := getTenantContext(c)
	resources, err := h.resources.ListByUserID(middleware.ContextWithTenant(c), userID, wsID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list resources", err, "RESOURCES_LIST_FAILED")
		return
	}
	if resources == nil {
		resources = []models.Resource{}
	}
	c.JSON(http.StatusOK, resources)
}

func (h *ResourceHandler) Get(c *gin.Context) {
	_, _, userID := getTenantContext(c)
	rwb, err := h.resources.GetResourceWithBackendsByID(middleware.ContextWithTenant(c), c.Param("id"), userID)
	if err != nil {
		httputil.RespondNotFound(c, "Resource not found", err, "RESOURCE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, rwb)
}

func (h *ResourceHandler) Create(c *gin.Context) {
	orgID, wsID, userID := getTenantContext(c)
	var req CreateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if !h.backendsAllowed(c, req.Backends) {
		return
	}

	schema := req.ParametersSchema
	if len(schema) == 0 {
		schema = json.RawMessage(`{}`)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	res := &models.Resource{
		ID:               uuid.New().String(),
		OrgID:            orgID,
		WorkspaceID:      wsID,
		UserID:           userID,
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		ParametersSchema: schema,
		Enabled:          enabled,
	}

	var backends []models.ResourceBackend
	for _, br := range req.Backends {
		b := models.ResourceBackend{
			Name:             br.Name,
			BackendType:      br.BackendType,
			EndpointURL:      br.EndpointURL,
			HTTPMethod:       br.HTTPMethod,
			QueryTemplate:    br.QueryTemplate,
			SQLQuery:         br.SQLQuery,
			ParamNames:       br.ParamNames,
			TimeoutMs:        br.TimeoutMs,
			Priority:         br.Priority,
			Enabled:          true,
			AuthHeaderName:   "Authorization",
			AuthHeaderPrefix: "Bearer ",
		}
		if b.TimeoutMs <= 0 {
			b.TimeoutMs = 30000
		}
		if b.Priority <= 0 {
			b.Priority = 1
		}
		if b.HTTPMethod == "" {
			b.HTTPMethod = "POST"
		}
		if br.AuthToken != "" && h.encKey != "" {
			enc, err := utils.EncryptAES256GCM(br.AuthToken, h.encKey)
			if err != nil {
				httputil.RespondInternalError(c, "Failed to encrypt auth token for backend: "+br.Name, err, "ENCRYPTION_FAILED")
				return
			}
			b.AuthTokenEncrypted = &enc
		}
		if br.ConnectionString != nil && *br.ConnectionString != "" && h.encKey != "" {
			enc, err := utils.EncryptAES256GCM(*br.ConnectionString, h.encKey)
			if err != nil {
				httputil.RespondInternalError(c, "Failed to encrypt connection string for backend: "+br.Name, err, "ENCRYPTION_FAILED")
				return
			}
			b.ConnectionStringEncrypted = &enc
		}
		backends = append(backends, b)
	}

	if err := h.resources.CreateWithBackendsTx(middleware.ContextWithTenant(c), res, backends); err != nil {
		httputil.RespondStoreError(c, err, "Resource", "RESOURCE_CREATE")
		return
	}

	rwb, _ := h.resources.GetResourceWithBackendsByID(middleware.ContextWithTenant(c), res.ID, userID)
	c.JSON(http.StatusCreated, rwb)
}

func (h *ResourceHandler) Update(c *gin.Context) {
	_, _, userID := getTenantContext(c)
	id := c.Param("id")

	existing, err := h.resources.FindByID(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Resource not found", err, "RESOURCE_NOT_FOUND")
		return
	}

	var req UpdateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Backends != nil && !h.backendsAllowed(c, *req.Backends) {
		return
	}

	if req.DisplayName != nil {
		existing.DisplayName = *req.DisplayName
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.ParametersSchema != nil {
		existing.ParametersSchema = req.ParametersSchema
	}
	if len(existing.ParametersSchema) == 0 {
		existing.ParametersSchema = json.RawMessage(`{}`)
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.resources.Update(middleware.ContextWithTenant(c), existing); err != nil {
		httputil.RespondStoreError(c, err, "Resource", "RESOURCE_UPDATE")
		return
	}

	if req.Backends != nil {
		priorBackends, err := h.backends.ListByResourceID(c.Request.Context(), id)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to load existing backends", err, "BACKENDS_LOAD_FAILED")
			return
		}
		if err := h.backends.DeleteByResourceID(c.Request.Context(), id); err != nil {
			httputil.RespondInternalError(c, "Failed to replace backends", err, "BACKENDS_REPLACE_FAILED")
			return
		}
		for _, br := range *req.Backends {
			prior := matchPriorResourceBackend(br, priorBackends)
			if err := h.createBackendWithPrior(c.Request.Context(), id, br, prior); err != nil {
				httputil.RespondInternalError(c, "Backend failed: "+br.Name, err, "RESOURCE_BACKEND_FAILED")
				return
			}
		}
	}

	rwb, _ := h.resources.GetResourceWithBackendsByID(middleware.ContextWithTenant(c), id, userID)
	c.JSON(http.StatusOK, rwb)
}

func (h *ResourceHandler) Delete(c *gin.Context) {
	_, _, userID := getTenantContext(c)
	if err := h.resources.Delete(middleware.ContextWithTenant(c), c.Param("id"), userID); err != nil {
		httputil.RespondStoreError(c, err, "Resource", "RESOURCE_DELETE")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "resource deleted"})
}

type TestResourceRequest struct {
	Args map[string]interface{} `json:"args" binding:"required"`
}

func (h *ResourceHandler) TestResource(c *gin.Context) {
	_, _, userID := getTenantContext(c)
	id := c.Param("id")

	res, err := h.resources.FindByID(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Resource not found", err, "RESOURCE_NOT_FOUND")
		return
	}
	var req TestResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	result, err := h.gateway.Execute(ctx, userID, res.Name, req.Args, h.encKey)
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "Resource execution failed: "+err.Error(), err, "RESOURCE_EXECUTION_FAILED")
		return
	}
	c.JSON(http.StatusOK, result)
}

func matchPriorResourceBackend(br CreateResourceBackendRequest, prior []models.ResourceBackend) *models.ResourceBackend {
	if br.ID != "" {
		for i := range prior {
			if prior[i].ID == br.ID {
				return &prior[i]
			}
		}
	}
	lowerName := strings.ToLower(strings.TrimSpace(br.Name))
	var uniqueMatch *models.ResourceBackend
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

func (h *ResourceHandler) createBackendWithPrior(ctx context.Context, resourceID string, br CreateResourceBackendRequest, prior *models.ResourceBackend) error {
	b := &models.ResourceBackend{
		ResourceID:       resourceID,
		Name:             br.Name,
		BackendType:      br.BackendType,
		EndpointURL:      br.EndpointURL,
		HTTPMethod:       br.HTTPMethod,
		QueryTemplate:    br.QueryTemplate,
		SQLQuery:         br.SQLQuery,
		ParamNames:       br.ParamNames,
		TimeoutMs:        br.TimeoutMs,
		Priority:         br.Priority,
		Enabled:          true,
		AuthHeaderName:   "Authorization",
		AuthHeaderPrefix: "Bearer ",
	}
	if br.TimeoutMs <= 0 {
		b.TimeoutMs = 30000
	}
	if br.Priority <= 0 {
		b.Priority = 1
	}
	if b.HTTPMethod == "" {
		b.HTTPMethod = "POST"
	}
	if br.AuthToken != "" && h.encKey != "" {
		enc, err := utils.EncryptAES256GCM(br.AuthToken, h.encKey)
		if err != nil {
			return err
		}
		b.AuthTokenEncrypted = &enc
	} else if prior != nil && prior.AuthTokenEncrypted != nil {
		encCopy := *prior.AuthTokenEncrypted
		b.AuthTokenEncrypted = &encCopy
		b.AuthHeaderName = prior.AuthHeaderName
		b.AuthHeaderPrefix = prior.AuthHeaderPrefix
	}
	if br.ConnectionString != nil && *br.ConnectionString != "" && h.encKey != "" {
		enc, err := utils.EncryptAES256GCM(*br.ConnectionString, h.encKey)
		if err != nil {
			return err
		}
		b.ConnectionStringEncrypted = &enc
	} else if prior != nil && prior.ConnectionStringEncrypted != nil {
		encCopy := *prior.ConnectionStringEncrypted
		b.ConnectionStringEncrypted = &encCopy
	}
	return h.backends.Create(ctx, b)
}
