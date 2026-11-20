package proxy

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type AdmissionDecision string

const (
	AdmissionAllow           AdmissionDecision = "ALLOW"
	AdmissionDeny            AdmissionDecision = "DENY"
	AdmissionDowngrade       AdmissionDecision = "DOWNGRADE"
	AdmissionWarn            AdmissionDecision = "WARN"
	AdmissionRequireApproval AdmissionDecision = "REQUIRE_APPROVAL"
)

type AdmissionResult struct {
	Decision     AdmissionDecision
	HTTPStatus   int
	ErrorCode    string
	Reason       string
	PolicyID     string
	PolicyName   string
	BudgetStatus string
}

type AdmissionRequest struct {
	UserID    string
	Role      string
	AgentID   string
	AgentName string
	// Agent is the agent the gateway already resolved for this request. When
	// it is nil but AgentID or AgentName is set, admission resolves it.
	Agent         *models.Agent
	ModelSlug     string
	ToolName      string
	ResourceName  string
	MCPServerName string
	PromptPayload string
	TenantCtx     models.TenantContext
}

// AgentSpendFinder returns an agent's month-to-date spend in USD for enforcing
// Agent.MaxBudgetCents.
type AgentSpendFinder interface {
	GetAgentTotalSpendUSD(ctx context.Context, agentID string) (float64, error)
}

// StreamConcurrencyLimiter tracks distributed active streaming slots per quota scope.
type StreamConcurrencyLimiter interface {
	AcquireScopeStream(ctx context.Context, scopeKey, reqID string, limit int, ttl time.Duration) (bool, int64, error)
	ReleaseScopeStream(ctx context.Context, scopeKey, reqID string) error
}

type AdmissionController struct {
	rbacEngine       *RBACEngine
	agentGovernance  *AgentGovernanceEngine
	quotaRepo        QuotaEvaluator
	budgetManager    *BudgetManager
	agentSpend       AgentSpendFinder
	streamLimiter    StreamConcurrencyLimiter
	streamFailClosed bool
	streamMu         sync.Mutex
	localStreams     map[string]map[string]struct{}
}

func NewAdmissionController(
	rbacEngine *RBACEngine,
	agentGovernance *AgentGovernanceEngine,
	quotaRepo QuotaEvaluator,
	budgetManager *BudgetManager,
) *AdmissionController {
	return &AdmissionController{
		rbacEngine:      rbacEngine,
		agentGovernance: agentGovernance,
		quotaRepo:       quotaRepo,
		budgetManager:   budgetManager,
	}
}

// SetAgentSpendFinder wires month-to-date per-agent spend lookup used to
// enforce Agent.MaxBudgetCents during admission.
func (a *AdmissionController) SetAgentSpendFinder(finder AgentSpendFinder) {
	if a != nil {
		a.agentSpend = finder
	}
}

// SetStreamConcurrencyLimiter configures distributed concurrent stream enforcement
// across organization, workspace, and project quota scopes.
func (a *AdmissionController) SetStreamConcurrencyLimiter(limiter StreamConcurrencyLimiter, failClosed bool) {
	if a != nil {
		a.streamLimiter = limiter
		a.streamFailClosed = failClosed
	}
}

// AcquireStreamSlots acquires concurrent streaming leases across all non-empty
// tenant scopes (organization, workspace, project) that configure MaxConcurrentStreams > 0.
// The returned release closure is idempotent and automatically rolls back partial acquisitions on denial.
func (a *AdmissionController) AcquireStreamSlots(ctx context.Context, reqID string, tenantCtx models.TenantContext) (func(), *AdmissionResult, error) {
	if a == nil || a.quotaRepo == nil || reqID == "" {
		return nil, nil, nil
	}

	scopes := []struct {
		kind string
		id   string
	}{
		{"organization", tenantCtx.OrgID},
		{"workspace", tenantCtx.WorkspaceID},
		{"project", tenantCtx.ProjectID},
	}

	type acquiredSlot struct {
		scopeKey    string
		distributed bool
	}
	var acquired []acquiredSlot

	rollback := func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		for _, slot := range acquired {
			if slot.distributed && a.streamLimiter != nil {
				_ = a.streamLimiter.ReleaseScopeStream(releaseCtx, slot.scopeKey, reqID)
			} else {
				a.releaseLocalStream(slot.scopeKey, reqID)
			}
		}
	}

	for _, sc := range scopes {
		if sc.id == "" {
			continue
		}
		quotaRes, err := a.quotaRepo.EvaluateQuota(ctx, sc.kind, sc.id)
		if err != nil {
			rollback()
			return nil, nil, fmt.Errorf("quota evaluate (%s %s): %w", sc.kind, sc.id, err)
		}
		if quotaRes == nil || quotaRes.MaxConcurrentStreams <= 0 {
			continue
		}

		scopeKey := sc.kind + ":" + sc.id
		if a.streamLimiter != nil {
			allowed, active, err := a.streamLimiter.AcquireScopeStream(ctx, scopeKey, reqID, quotaRes.MaxConcurrentStreams, 10*time.Minute)
			if err == nil {
				if !allowed {
					rollback()
					return nil, &AdmissionResult{
						Decision:   AdmissionDeny,
						HTTPStatus: http.StatusTooManyRequests,
						ErrorCode:  "max_concurrent_streams_exceeded",
						Reason:     fmt.Sprintf("%s concurrent stream limit (%d) exceeded (active: %d)", sc.kind, quotaRes.MaxConcurrentStreams, active),
					}, nil
				}
				acquired = append(acquired, acquiredSlot{scopeKey: scopeKey, distributed: true})
				continue
			}
			if a.streamFailClosed {
				rollback()
				return nil, nil, fmt.Errorf("acquire distributed stream slot (%s): %w", scopeKey, err)
			}
		}

		allowed, active := a.acquireLocalStream(scopeKey, reqID, quotaRes.MaxConcurrentStreams)
		if !allowed {
			rollback()
			return nil, &AdmissionResult{
				Decision:   AdmissionDeny,
				HTTPStatus: http.StatusTooManyRequests,
				ErrorCode:  "max_concurrent_streams_exceeded",
				Reason:     fmt.Sprintf("%s concurrent stream limit (%d) exceeded (active: %d)", sc.kind, quotaRes.MaxConcurrentStreams, active),
			}, nil
		}
		acquired = append(acquired, acquiredSlot{scopeKey: scopeKey, distributed: false})
	}

	if len(acquired) == 0 {
		return nil, nil, nil
	}
	var once sync.Once
	return func() {
		once.Do(rollback)
	}, nil, nil
}

func (a *AdmissionController) acquireLocalStream(scopeKey, reqID string, limit int) (bool, int64) {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()
	if a.localStreams == nil {
		a.localStreams = make(map[string]map[string]struct{})
	}
	active := a.localStreams[scopeKey]
	if active == nil {
		active = make(map[string]struct{})
		a.localStreams[scopeKey] = active
	}
	if _, exists := active[reqID]; exists {
		return true, int64(len(active))
	}
	if limit > 0 && len(active) >= limit {
		return false, int64(len(active))
	}
	active[reqID] = struct{}{}
	return true, int64(len(active))
}

func (a *AdmissionController) releaseLocalStream(scopeKey, reqID string) {
	a.streamMu.Lock()
	defer a.streamMu.Unlock()
	if a.localStreams == nil {
		return
	}
	if active := a.localStreams[scopeKey]; active != nil {
		delete(active, reqID)
		if len(active) == 0 {
			delete(a.localStreams, scopeKey)
		}
	}
}

// BudgetManager returns the controller's BudgetManager, if configured.
func (a *AdmissionController) BudgetManager() *BudgetManager {
	if a == nil {
		return nil
	}
	return a.budgetManager
}

func (a *AdmissionController) Evaluate(ctx context.Context, req AdmissionRequest) (*AdmissionResult, error) {
	// 0. Prompt Safety & System Boundary Leak Check
	if req.PromptPayload != "" {
		lowerPrompt := strings.ToLower(req.PromptPayload)
		restrictedKeywords := []string{
			"dump database password",
			"show env jwt_secret",
			"reveal backend encryption_key",
			"explain internal gateway architecture behind the scene",
			"show system environment variables",
			"reveal database connection string",
		}
		for _, kw := range restrictedKeywords {
			if strings.Contains(lowerPrompt, kw) {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusBadRequest,
					ErrorCode:  "security_prompt_denied",
					Reason:     "prompt payload contains restricted system boundary probe query",
				}, nil
			}
		}
	}

	// 1. RBAC Governance Check
	var pendingApproval *AdmissionResult
	if a.rbacEngine != nil {
		evalReq := models.RBACEvaluationRequest{
			Role:         req.Role,
			AgentName:    req.AgentName,
			ModelSlug:    req.ModelSlug,
			ToolName:     req.ToolName,
			ResourceName: req.ResourceName,
			UserPrompt:   req.PromptPayload,
		}
		rbacRes, err := a.rbacEngine.Evaluate(ctx, req.UserID, evalReq)
		if err != nil {
			return nil, fmt.Errorf("rbac evaluate: %w", err)
		}
		if rbacRes.RequiresApproval {
			policyID := ""
			policyName := ""
			if rbacRes.MatchedPolicy != nil {
				policyID = rbacRes.MatchedPolicy.ID
				policyName = rbacRes.MatchedPolicy.Name
			}
			pendingApproval = &AdmissionResult{
				Decision:   AdmissionRequireApproval,
				HTTPStatus: http.StatusAccepted,
				ErrorCode:  "approval_required",
				Reason:     rbacRes.Reason,
				PolicyID:   policyID,
				PolicyName: policyName,
			}
		} else if !rbacRes.Allowed {
			policyID := ""
			policyName := ""
			if rbacRes.MatchedPolicy != nil {
				policyID = rbacRes.MatchedPolicy.ID
				policyName = rbacRes.MatchedPolicy.Name
			}
			return &AdmissionResult{
				Decision:   AdmissionDeny,
				HTTPStatus: http.StatusForbidden,
				ErrorCode:  "governance_policy_denied",
				Reason:     rbacRes.Reason,
				PolicyID:   policyID,
				PolicyName: policyName,
			}, nil
		}
	}

	// 2. Agent Governance Check (Model & Tool access)
	if res, err := a.evaluateAgent(ctx, req); err != nil || res != nil {
		return res, err
	}

	// 3. Tenant Quota Check (Organization, Workspace, and Project quotas - fail closed)
	if a.quotaRepo != nil {
		scopes := []struct {
			kind string
			id   string
		}{
			{"organization", req.TenantCtx.OrgID},
			{"workspace", req.TenantCtx.WorkspaceID},
			{"project", req.TenantCtx.ProjectID},
		}
		for _, sc := range scopes {
			if sc.id == "" {
				continue
			}
			quotaRes, err := a.quotaRepo.EvaluateQuota(ctx, sc.kind, sc.id)
			if err != nil {
				return nil, fmt.Errorf("quota evaluate (%s %s): %w", sc.kind, sc.id, err)
			}
			if quotaRes != nil && !quotaRes.Allowed {
				return &AdmissionResult{
					Decision:   AdmissionDeny,
					HTTPStatus: http.StatusTooManyRequests,
					ErrorCode:  "quota_exceeded_error",
					Reason:     quotaRes.Reason,
				}, nil
			}
		}
	}

	// 4. Multi-Level Budget Check (fail closed on store error)
	var budgetStatusStr string
	if a.budgetManager != nil {
		budgetStatus, err := a.budgetManager.GetStatus(ctx, req.UserID)
		if err != nil {
			return nil, fmt.Errorf("budget evaluate: %w", err)
		}
		if budgetStatus != nil {
			budgetStatusStr = budgetStatus.Status
			if budgetStatus.Status == "exceeded" && budgetStatus.Budget != nil && budgetStatus.Budget.HardLimit {
				return &AdmissionResult{
					Decision:     AdmissionDeny,
					HTTPStatus:   http.StatusTooManyRequests,
					ErrorCode:    "budget_exceeded_error",
					Reason:       budgetExceededReason(budgetStatus),
					BudgetStatus: budgetStatus.Status,
				}, nil
			}

			if budgetStatus.Status == "critical" {
				return &AdmissionResult{
					Decision:     AdmissionDowngrade,
					HTTPStatus:   http.StatusOK,
					Reason:       "Budget usage critical; model downgrade penalty applied",
					BudgetStatus: budgetStatus.Status,
				}, nil
			}

			if budgetStatus.Status == "warning" {
				if pendingApproval != nil {
					pendingApproval.BudgetStatus = budgetStatus.Status
					return pendingApproval, nil
				}
				return &AdmissionResult{
					Decision:     AdmissionWarn,
					HTTPStatus:   http.StatusOK,
					Reason:       "Budget usage approaching limit threshold",
					BudgetStatus: budgetStatus.Status,
				}, nil
			}
		}
	}

	if pendingApproval != nil {
		pendingApproval.BudgetStatus = budgetStatusStr
		return pendingApproval, nil
	}

	return &AdmissionResult{
		Decision:     AdmissionAllow,
		HTTPStatus:   http.StatusOK,
		BudgetStatus: budgetStatusStr,
	}, nil
}

// evaluateAgent denies a request whose agent may not use the model or tool,
// or whose named agent does not exist. It returns nil when the request may
// continue.
func (a *AdmissionController) evaluateAgent(ctx context.Context, req AdmissionRequest) (*AdmissionResult, error) {
	ref := req.AgentID
	if ref == "" {
		ref = req.AgentName
	}
	if req.Agent == nil && ref == "" {
		return nil, nil
	}

	agent := req.Agent
	if agent == nil {
		if a.agentGovernance == nil {
			return nil, nil
		}
		found, reason, err := a.agentGovernance.resolve(ctx, req.UserID, ref)
		if err != nil {
			return nil, fmt.Errorf("agent governance: %w", err)
		}
		if found == nil {
			return agentDenied("agent_not_found", reason), nil
		}
		agent = found
	}

	if req.ModelSlug != "" {
		if res := CheckAgentModelAccess(agent, req.ModelSlug); !res.ModelAllowed {
			return agentDenied("agent_model_denied", res.Reason), nil
		}
	}
	if req.ToolName != "" {
		if res := CheckAgentToolAccess(agent, req.ToolName); !res.ToolAllowed {
			return agentDenied("agent_tool_denied", res.Reason), nil
		}
	}
	if req.ResourceName != "" {
		if res := CheckAgentResourceAccess(agent, req.ResourceName); !res.ResourceAllowed {
			return agentDenied("agent_resource_denied", res.Reason), nil
		}
	}
	if req.MCPServerName != "" {
		if res := CheckAgentMCPServerAccess(agent, req.MCPServerName, "", ""); !res.ToolAllowed {
			return agentDenied("agent_mcp_server_denied", res.Reason), nil
		}
	}
	if agent.MaxBudgetCents > 0 && a.agentSpend != nil && agent.ID != "" {
		spentUSD, err := a.agentSpend.GetAgentTotalSpendUSD(ctx, agent.ID)
		if err != nil {
			return nil, fmt.Errorf("agent budget check (%s): %w", agent.ID, err)
		}
		if spentUSD*100.0 >= float64(agent.MaxBudgetCents) {
			return &AdmissionResult{
				Decision:   AdmissionDeny,
				HTTPStatus: http.StatusTooManyRequests,
				ErrorCode:  "agent_budget_exceeded",
				Reason:     fmt.Sprintf("agent %q monthly budget limit ($%.2f) exceeded (spent: $%.2f)", agent.Name, float64(agent.MaxBudgetCents)/100.0, spentUSD),
			}, nil
		}
	}
	return nil, nil
}

func agentDenied(code, reason string) *AdmissionResult {
	return &AdmissionResult{
		Decision:   AdmissionDeny,
		HTTPStatus: http.StatusForbidden,
		ErrorCode:  code,
		Reason:     reason,
	}
}

func budgetExceededReason(st *models.BudgetStatus) string {
	limit := st.Budget.MonthlyLimit
	if st.LimitPeriod == "daily" {
		limit = st.Budget.DailyLimit
	}
	return fmt.Sprintf("%s budget limit ($%.2f) of %q exceeded", st.LimitPeriod, limit, st.Budget.Name)
}
