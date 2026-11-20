package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type routingPolicyStore interface {
	ListByUserID(ctx context.Context, userID string) ([]models.RoutingPolicy, error)
	FindByID(ctx context.Context, id, userID string) (*models.RoutingPolicy, error)
	Create(ctx context.Context, p *models.RoutingPolicy) error
	Update(ctx context.Context, p *models.RoutingPolicy) error
	Delete(ctx context.Context, id, userID string) error
	SetDefault(ctx context.Context, id, userID string) error
	FindByName(ctx context.Context, name, userID string) (*models.RoutingPolicy, error)
}

var _ routingPolicyStore = (*repository.RoutingPolicyRepository)(nil)

func validateAndNormalizePolicyWeights(weights map[string]float64) (map[string]float64, error) {
	for k, v := range weights {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return nil, fmt.Errorf("weight %q must be finite and non-negative", k)
		}
	}
	return proxy.NormalizePolicyWeights(weights), nil
}

type RoutingPolicyHandler struct {
	policyRepo routingPolicyStore
	audit      *AdminAudit
}

func NewRoutingPolicyHandler(policyRepo routingPolicyStore) *RoutingPolicyHandler {
	return &RoutingPolicyHandler{policyRepo: policyRepo}
}

func (h *RoutingPolicyHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

func (h *RoutingPolicyHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	policies, err := h.policyRepo.ListByUserID(middleware.ContextWithTenant(c), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list routing policies", err, "POLICIES_LIST_FAILED")
		return
	}
	if policies == nil {
		policies = []models.RoutingPolicy{}
	}
	c.JSON(http.StatusOK, policies)
}

func (h *RoutingPolicyHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	policy, err := h.policyRepo.FindByID(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, policy)
}

type CreatePolicyRequest struct {
	Name        string             `json:"name" binding:"required"`
	Weights     map[string]float64 `json:"weights"`
	Constraints map[string]float64 `json:"constraints"`
}

func (h *RoutingPolicyHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Weights == nil {
		req.Weights = map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15}
	} else {
		normalized, err := validateAndNormalizePolicyWeights(req.Weights)
		if err != nil {
			httputil.RespondBadRequest(c, "Invalid policy weights", err, "INVALID_WEIGHTS")
			return
		}
		req.Weights = normalized
	}
	if req.Constraints == nil {
		req.Constraints = map[string]float64{"max_cost_per_request": 0.05}
	}
	policy := &models.RoutingPolicy{
		UserID:      userID,
		Name:        req.Name,
		Weights:     req.Weights,
		Constraints: req.Constraints,
		Enabled:     true,
	}
	if err := h.policyRepo.Create(middleware.ContextWithTenant(c), policy); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_CREATE")
		return
	}
	h.audit.Record(c, "routing_policy.create", "routing_policy", policy.ID, map[string]any{"name": policy.Name})
	c.JSON(http.StatusCreated, policy)
}

type UpdatePolicyRequest struct {
	Name        *string            `json:"name"`
	Weights     map[string]float64 `json:"weights"`
	Constraints map[string]float64 `json:"constraints"`
	Enabled     *bool              `json:"enabled"`
}

func (h *RoutingPolicyHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.policyRepo.FindByID(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		return
	}
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Weights != nil {
		normalized, err := validateAndNormalizePolicyWeights(req.Weights)
		if err != nil {
			httputil.RespondBadRequest(c, "Invalid policy weights", err, "INVALID_WEIGHTS")
			return
		}
		existing.Weights = normalized
	}
	if req.Constraints != nil {
		existing.Constraints = req.Constraints
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.policyRepo.Update(middleware.ContextWithTenant(c), existing); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_UPDATE")
		return
	}
	h.audit.Record(c, "routing_policy.update", "routing_policy", existing.ID, map[string]any{"name": existing.Name, "enabled": existing.Enabled})
	c.JSON(http.StatusOK, existing)
}

func (h *RoutingPolicyHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.policyRepo.Delete(middleware.ContextWithTenant(c), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_DELETE")
		return
	}
	h.audit.Record(c, "routing_policy.delete", "routing_policy", id, nil)
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

func (h *RoutingPolicyHandler) SetDefault(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.policyRepo.SetDefault(middleware.ContextWithTenant(c), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_SET_DEFAULT")
		return
	}
	h.audit.Record(c, "routing_policy.set_default", "routing_policy", id, nil)
	policy, err := h.policyRepo.FindByID(middleware.ContextWithTenant(c), id, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "policy set as default"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *RoutingPolicyHandler) FindByName(c *gin.Context) {
	userID := c.GetString("userId")
	name := c.Param("name")
	policy, err := h.policyRepo.FindByName(middleware.ContextWithTenant(c), name, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		} else {
			httputil.RespondInternalError(c, "Failed to find policy", err, "POLICY_FIND_FAILED")
		}
		return
	}
	c.JSON(http.StatusOK, policy)
}
