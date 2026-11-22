package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
)

type providerStore interface {
	ListByUserID(ctx context.Context, userID string) ([]models.Provider, error)
	FindByID(ctx context.Context, id string) (*models.Provider, error)
	Create(ctx context.Context, p *models.Provider) error
	Update(ctx context.Context, p *models.Provider) error
	Delete(ctx context.Context, id, userID string) error
}

// ProviderHandler serves providers. Everyone sees their own providers and
// the shared system ones (no owner); only an owner can change or delete a
// provider, so system providers are changed by operators alone.
type ProviderHandler struct {
	providers   providerStore
	gatewayKeys *repository.GatewayKeyRepository
}

func NewProviderHandler(providers providerStore, gatewayKeys *repository.GatewayKeyRepository) *ProviderHandler {
	return &ProviderHandler{
		providers:   providers,
		gatewayKeys: gatewayKeys,
	}
}

// List godoc
// @Summary      List providers
// @Description  Get all providers for the current user
// @Tags         providers
// @Security     BearerAuth
// @Success      200 {array} models.Provider
// @Failure      500 {object} map[string]string
// @Router       /api/providers [get]
func (h *ProviderHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	providers, err := h.providers.ListByUserID(middleware.ContextWithTenant(c), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list providers", err, "PROVIDERS_LIST_FAILED")
		return
	}
	if providers == nil {
		providers = []models.Provider{}
	}
	c.JSON(http.StatusOK, providers)
}

// Get godoc
// @Summary      Get provider
// @Description  Get a provider by ID
// @Tags         providers
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Success      200 {object} models.Provider
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id} [get]
func (h *ProviderHandler) Get(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userId")
	provider, err := h.providers.FindByID(middleware.ContextWithTenant(c), id)
	if err != nil || (userID != "" && !callerCanAccessProvider(c, provider, userID)) {
		httputil.RespondNotFound(c, "Provider not found", err, "PROVIDER_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, provider)
}

// Create godoc
// @Summary      Create provider
// @Description  Create a new provider
// @Tags         providers
// @Security     BearerAuth
// @Param        request body models.Provider true "Provider data"
// @Success      201 {object} models.Provider
// @Failure      400 {object} map[string]string
// @Router       /api/providers [post]
func (h *ProviderHandler) Create(c *gin.Context) {
	var req createProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	p := buildProviderFromCreate(req, c.GetString("userId"))
	p.OrgID = middleware.GetTenantContext(c).OrgID
	if !isValidProviderType(p.Type) {
		httputil.RespondBadRequest(c, "Unsupported provider type", nil, "INVALID_PROVIDER_TYPE")
		return
	}
	if !baseURLAllowed(c, p.BaseURL) {
		return
	}

	if err := h.providers.Create(middleware.ContextWithTenant(c), &p); err != nil {
		httputil.RespondStoreError(c, err, "Provider", "PROVIDER_CREATE")
		return
	}
	c.JSON(http.StatusCreated, p)
}

// Update godoc
// @Summary      Update provider
// @Description  Update a provider
// @Tags         providers
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        request body models.Provider true "Provider data"
// @Success      200 {object} models.Provider
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id} [put]
func (h *ProviderHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userId")
	existing, ok := h.ownedProvider(c, id, userID)
	if !ok {
		return
	}

	var req updateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.BaseURL != nil && !baseURLAllowed(c, trimString(*req.BaseURL)) {
		return
	}
	if req.Type != nil && strings.TrimSpace(*req.Type) != "" && !isValidProviderType(*req.Type) {
		httputil.RespondBadRequest(c, "Unsupported provider type", nil, "INVALID_PROVIDER_TYPE")
		return
	}
	priorOwner := existing.UserID
	priorOrg := existing.OrgID
	applyProviderUpdate(existing, req)
	// The body may not move the provider to another id or owner/org.
	existing.ID = id
	existing.UserID = priorOwner
	existing.OrgID = priorOrg

	if err := h.providers.Update(middleware.ContextWithTenant(c), existing); err != nil {
		httputil.RespondStoreError(c, err, "Provider", "PROVIDER_UPDATE")
		return
	}
	c.JSON(http.StatusOK, existing)
}

// Delete godoc
// @Summary      Delete provider
// @Description  Delete a provider
// @Tags         providers
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Success      204
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /api/providers/{id} [delete]
func (h *ProviderHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userId")
	if _, ok := h.ownedProvider(c, id, userID); !ok {
		return
	}

	ctx := middleware.ContextWithTenant(c)

	// Guard against deleting provider that has active Gateway API Keys
	if h.gatewayKeys != nil {
		count, err := h.gatewayKeys.CountByProviderID(ctx, id)
		if err == nil && count > 0 {
			httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete provider: %d active Gateway API Key(s) are bound to it. Please revoke or reassign the keys first.", count), nil, "PROVIDER_IN_USE")
			return
		}
	}

	if err := h.providers.Delete(ctx, id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Provider", "PROVIDER_DELETE")
		return
	}
	c.Status(http.StatusNoContent)
}

// ownedProvider returns the provider when userID (or caller org) owns it, and
// otherwise writes the response: 404 for a foreign provider, 403 for a shared
// system provider.
func (h *ProviderHandler) ownedProvider(c *gin.Context, id, userID string) (*models.Provider, bool) {
	p, err := h.providers.FindByID(middleware.ContextWithTenant(c), id)
	if err != nil {
		httputil.RespondNotFound(c, "Provider not found", err, "PROVIDER_NOT_FOUND")
		return nil, false
	}
	if callerOwnsProvider(c, p, userID) {
		return p, true
	}
	if isSharedProvider(p) {
		httputil.RespondForbidden(c, "System providers can only be changed by operators", nil, "SYSTEM_PROVIDER_READ_ONLY")
		return nil, false
	}
	httputil.RespondNotFound(c, "Provider not found", nil, "PROVIDER_NOT_FOUND")
	return nil, false
}

// isSharedProvider reports whether p is a system provider every user can use.
func isSharedProvider(p *models.Provider) bool {
	return isSystemProvider(p)
}

// baseURLAllowed answers 400 (and returns false) when a provider's base URL
// is not a public http(s) endpoint. Gateway traffic, headers and all, goes
// to this URL, so a user must not be able to point it at the API, Redis,
// the database or a cloud metadata service.
func baseURLAllowed(c *gin.Context, baseURL string) bool {
	if err := security.ValidateOutboundURL(baseURL); err != nil {
		httputil.RespondBadRequest(c, "baseUrl must be a public http(s) endpoint: "+err.Error(), err, "PROVIDER_BASE_URL_BLOCKED")
		return false
	}
	return true
}
