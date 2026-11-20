package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type simModelStore interface {
	ListEnabled(ctx context.Context) ([]models.Model, error)
}

type simProviderStore interface {
	ListByUserID(ctx context.Context, userID string) ([]models.Provider, error)
}

type simCredStore interface {
	ListActiveProviderIDs(ctx context.Context, userID ...string) (map[string]bool, error)
	FindAllActiveByProviderID(ctx context.Context, providerID string, excludeIDs []string, userOrOrgID ...string) ([]models.Credential, error)
}

type simPolicyStore interface {
	FindByID(ctx context.Context, id, userID string) (*models.RoutingPolicy, error)
	FindByDefault(ctx context.Context, userID string) (*models.RoutingPolicy, error)
	FindByName(ctx context.Context, name, userID string) (*models.RoutingPolicy, error)
}

type simRuleStore interface {
	ListEnabledByUserID(ctx context.Context, userID string) ([]models.RoutingRule, error)
}

type simAgentStore interface {
	FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error)
}

type simDecisionStore interface {
	ListWithFilter(ctx context.Context, userID string, limit, offset int) ([]repository.RoutingDecisionLog, int64, error)
}

type simTelemetryStore interface {
	GetMultipleModelMetrics(ctx context.Context, slugs []string) (map[string]*goredis.ModelMetrics, error)
}

type simHealthStore interface {
	ScoresFor(ctx context.Context, orgID string) map[string]float64
}

type simCooldownStore interface {
	CoolingSet(ctx context.Context, credIDs []string) (map[string]bool, error)
}

type SimulateHandler struct {
	models    simModelStore
	providers simProviderStore
	creds     simCredStore
	policies  simPolicyStore
	rules     simRuleStore
	agents    simAgentStore
	decisions simDecisionStore
	telemetry simTelemetryStore
	health    simHealthStore
	cooldown  simCooldownStore
}

func NewSimulateHandler(
	models *repository.ModelRepository,
	providers *repository.ProviderRepository,
	creds *repository.CredentialRepository,
	policies *repository.RoutingPolicyRepository,
	telemetry *goredis.ModelTelemetryStore,
	health *proxy.ProviderHealthStore,
) *SimulateHandler {
	h := &SimulateHandler{}
	if models != nil {
		h.models = models
	}
	if providers != nil {
		h.providers = providers
	}
	if creds != nil {
		h.creds = creds
	}
	if policies != nil {
		h.policies = policies
	}
	if telemetry != nil {
		h.telemetry = telemetry
	}
	if health != nil {
		h.health = health
	}
	return h
}

// SetRoutingDependencies wires live routing rules, Redis cooldown checks,
// agent allowlist enforcement, and historical routing decisions for simulation and replay.
func (h *SimulateHandler) SetRoutingDependencies(
	rules *repository.RoutingRuleRepository,
	cooldown *goredis.CooldownStore,
	agents *repository.AgentRepository,
	decisions *repository.RoutingDecisionRepository,
) {
	if h == nil {
		return
	}
	if rules != nil {
		h.rules = rules
	}
	if cooldown != nil {
		h.cooldown = cooldown
	}
	if agents != nil {
		h.agents = agents
	}
	if decisions != nil {
		h.decisions = decisions
	}
}

type SimulateRequest struct {
	Prompt           string             `json:"prompt"`
	TargetModel      string             `json:"targetModel"`
	PolicyID         string             `json:"policyId"`
	AgentID          string             `json:"agentId"`
	CustomWeights    map[string]float64 `json:"customWeights"`
	WeightLatency    *float64           `json:"weightLatency"`
	WeightCost       *float64           `json:"weightCost"`
	WeightThroughput *float64           `json:"weightThroughput"`
	WeightErrorRate  *float64           `json:"weightErrorRate"`
	BudgetStatus     string             `json:"budgetStatus"`
	ProviderID       string             `json:"providerId"`
}

type ModelScoreDetail struct {
	ModelID            string   `json:"modelId"`
	ModelSlug          string   `json:"modelSlug"`
	Slug               string   `json:"slug"`
	DisplayName        string   `json:"displayName"`
	ProviderName       string   `json:"providerName"`
	Score              float64  `json:"score"`
	TotalScore         float64  `json:"totalScore"`
	QualityScore       float64  `json:"qualityScore"`
	CostScore          float64  `json:"costScore"`
	SpeedScore         float64  `json:"speedScore"`
	LatencyScore       float64  `json:"latencyScore"`
	HealthScore        float64  `json:"healthScore"`
	Reasons            []string `json:"reasons"`
	InputPrice1M       float64  `json:"inputPrice1M"`
	OutputPrice1M      float64  `json:"outputPrice1M"`
	EstimatedLatencyMs int      `json:"estimatedLatencyMs"`
	EstimatedCostUSD   float64  `json:"estimatedCostUSD"`
	IsHealthy          bool     `json:"isHealthy"`
}

type SimulateResponse struct {
	PromptPreview      string             `json:"promptPreview"`
	TaskType           string             `json:"taskType"`
	Complexity         string             `json:"complexity"`
	PolicyName         string             `json:"policyName"`
	Strategy           string             `json:"strategy"`
	WeightsUsed        map[string]float64 `json:"weightsUsed"`
	BudgetStatus       string             `json:"budgetStatus"`
	SelectedModel      string             `json:"selectedModel"`
	SelectedProvider   string             `json:"selectedProvider"`
	ConfidenceScore    float64            `json:"confidenceScore"`
	SimulatedLatencyMs int                `json:"simulatedLatencyMs"`
	SimulatedCostUSD   float64            `json:"simulatedCostUSD"`
	Reasoning          string             `json:"reasoning"`
	Candidates         []ModelScoreDetail `json:"candidates"`
	DowngradeReason    string             `json:"downgradeReason"`
}

type ReplayRequest struct {
	PolicyID      string             `json:"policyId"`
	CustomWeights map[string]float64 `json:"customWeights"`
	BudgetStatus  string             `json:"budgetStatus"`
	Limit         int                `json:"limit"`
}

type ReplayItem struct {
	RequestID          string  `json:"requestId"`
	PromptPreview      string  `json:"promptPreview,omitempty"`
	TaskType           string  `json:"taskType,omitempty"`
	Complexity         string  `json:"complexity,omitempty"`
	OriginalModel      string  `json:"originalModel"`
	OriginalProvider   string  `json:"originalProvider"`
	OriginalCost       float64 `json:"originalCost"`
	OriginalStatusCode *int    `json:"originalStatusCode,omitempty"`
	OriginalLatencyMs  *int    `json:"originalLatencyMs,omitempty"`
	OriginalError      string  `json:"originalError,omitempty"`
	SimulatedModel     string  `json:"simulatedModel"`
	SimulatedProvider  string  `json:"simulatedProvider"`
	SimulatedCost      float64 `json:"simulatedCost"`
	Changed            bool    `json:"changed"`
	CostDelta          float64 `json:"costDelta"`
}

type ReplayResponse struct {
	PolicyName          string       `json:"policyName"`
	EvaluatedCount      int          `json:"evaluatedCount"`
	ChangedCount        int          `json:"changedCount"`
	ChangeRate          float64      `json:"changeRate"`
	HistoricalTotalCost float64      `json:"historicalTotalCost"`
	SimulatedTotalCost  float64      `json:"simulatedTotalCost"`
	EstimatedCostDelta  float64      `json:"estimatedCostDelta"`
	Items               []ReplayItem `json:"items"`
}

func (h *SimulateHandler) resolvePolicy(ctx context.Context, userID, policyID string, customWeights map[string]float64, req *SimulateRequest) *proxy.RoutingPolicy {
	policy := proxy.DefaultRoutingPolicy()
	applyDBPolicy := func(dbPolicy *models.RoutingPolicy) {
		policy.Name = dbPolicy.Name
		if len(dbPolicy.Weights) > 0 {
			policy.Weights = dbPolicy.Weights
		}
		policy.Constraints = dbPolicy.Constraints
	}

	if policyID != "" && h.policies != nil {
		if dbPolicy, err := h.policies.FindByID(ctx, policyID, userID); err == nil && dbPolicy != nil {
			applyDBPolicy(dbPolicy)
			return policy
		}
	}
	if len(customWeights) > 0 {
		policy.Name = "custom"
		policy.Weights = customWeights
		policy.Constraints = map[string]float64{}
		return policy
	}
	if req != nil && (req.WeightLatency != nil || req.WeightCost != nil || req.WeightThroughput != nil || req.WeightErrorRate != nil) {
		policy.Name = "custom"
		w := map[string]float64{}
		if req.WeightLatency != nil {
			w["speed"] = *req.WeightLatency
		}
		if req.WeightCost != nil {
			w["cost"] = *req.WeightCost
		}
		if req.WeightThroughput != nil {
			w["task_match"] = *req.WeightThroughput
		}
		if req.WeightErrorRate != nil {
			w["quality"] = *req.WeightErrorRate
		}
		policy.Weights = proxy.NormalizePolicyWeights(w)
		policy.Constraints = map[string]float64{}
		return policy
	}
	if h.policies != nil {
		if dbPolicy, err := h.policies.FindByDefault(ctx, userID); err == nil && dbPolicy != nil {
			applyDBPolicy(dbPolicy)
		} else if dbPolicy, err := h.policies.FindByName(ctx, "balanced", userID); err == nil && dbPolicy != nil {
			applyDBPolicy(dbPolicy)
		}
	}
	return policy
}

func (h *SimulateHandler) coolingProviders(ctx context.Context, userID string, providerIDs map[string]bool) map[string]bool {
	out := make(map[string]bool)
	if h.cooldown == nil || h.creds == nil {
		return out
	}
	for pid, active := range providerIDs {
		if !active {
			continue
		}
		creds, err := h.creds.FindAllActiveByProviderID(ctx, pid, nil, userID)
		if err != nil || len(creds) == 0 {
			continue
		}
		ids := make([]string, len(creds))
		for i, cred := range creds {
			ids[i] = cred.ID
		}
		cooling, err := h.cooldown.CoolingSet(ctx, ids)
		if err != nil {
			continue
		}
		allCooling := true
		for _, id := range ids {
			if !cooling[id] {
				allCooling = false
				break
			}
		}
		if allCooling {
			out[pid] = true
		}
	}
	return out
}

func estimateModelPer1KCost(m *models.Model) float64 {
	if m == nil {
		return 0
	}
	return (m.InputPricePer1M*0.5 + m.OutputPricePer1M*0.5) / 1000.0
}

func estimateModelLatencyMs(_ *models.Model, speedScore float64, metrics *goredis.ModelMetrics) int {
	if metrics != nil && metrics.SampleCount > 0 && metrics.P95TTFTMs > 0 {
		return int(math.Round(metrics.P95TTFTMs))
	}
	ms := int(math.Round((1.15 - speedScore) * 1200))
	if ms < 120 {
		ms = 120
	}
	return ms
}

func (h *SimulateHandler) Simulate(c *gin.Context) {
	userID := c.GetString("userId")

	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.Prompt == "" {
		req.Prompt = "Explain Server-Sent Events (SSE) streaming in 2 paragraphs."
	}
	if req.BudgetStatus == "" {
		req.BudgetStatus = "healthy"
	}

	// 1. Classify prompt characteristics
	messages := []map[string]interface{}{
		{"role": "user", "content": req.Prompt},
	}
	chars := proxy.ClassifyRequest(messages)

	promptPreview := req.Prompt
	if len(promptPreview) > 250 {
		promptPreview = promptPreview[:250] + "..."
	}

	ctx := middleware.ContextWithTenant(c)
	policy := h.resolvePolicy(ctx, userID, req.PolicyID, req.CustomWeights, &req)

	// 2. Load enabled models, providers, active credentials, rules, cooldown, and agent
	allModels, err := h.models.ListEnabled(ctx)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load models", err, "MODELS_LOAD_FAILED")
		return
	}

	providerMap := make(map[string]string)
	if h.providers != nil {
		allProviders, _ := h.providers.ListByUserID(ctx, userID)
		for _, p := range allProviders {
			providerMap[p.ID] = p.Name
		}
	}

	activeProviderIDs := make(map[string]bool)
	if h.creds != nil {
		activeProviderIDs, _ = h.creds.ListActiveProviderIDs(ctx, userID)
	}
	allCredsCooling := h.coolingProviders(ctx, userID, activeProviderIDs)

	var agent *models.Agent
	if strings.TrimSpace(req.AgentID) != "" && h.agents != nil {
		agent, _ = h.agents.FindForUser(ctx, userID, strings.TrimSpace(req.AgentID))
	}

	var pinnedProviderID string
	targetModel := strings.TrimSpace(req.TargetModel)
	isExplicitTarget := targetModel != "" && targetModel != "prism-auto" && targetModel != "roozy-auto"
	if isExplicitTarget && h.rules != nil {
		if rules, rErr := h.rules.ListEnabledByUserID(ctx, userID); rErr == nil {
			if rule := proxy.MatchRoutingRule(rules, targetModel); rule != nil && rule.ProviderID != nil {
				pinnedProviderID = *rule.ProviderID
			}
		}
	}

	var candidates []*models.Model
	for i := range allModels {
		m := allModels[i]
		if !activeProviderIDs[m.ProviderID] {
			continue
		}
		if req.ProviderID != "" && m.ProviderID != req.ProviderID {
			continue
		}
		if isExplicitTarget && m.Slug != targetModel {
			continue
		}
		if pinnedProviderID != "" && m.ProviderID != pinnedProviderID {
			continue
		}
		if agent != nil && !proxy.CheckAgentResolvedModel(agent, m.Slug) {
			continue
		}
		candidates = append(candidates, &m)
	}

	var telemetryMap map[string]*goredis.ModelMetrics
	if h.telemetry != nil && len(candidates) > 0 {
		candidateSlugs := make([]string, len(candidates))
		for i, m := range candidates {
			candidateSlugs[i] = m.Slug
		}
		telemetryMap, _ = h.telemetry.GetMultipleModelMetrics(c.Request.Context(), candidateSlugs)
	}

	var healthScores map[string]float64
	if h.health != nil {
		healthScores = h.health.ScoresFor(c.Request.Context(), middleware.GetTenantContext(c).OrgID)
	}

	scores := proxy.ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, req.BudgetStatus, telemetryMap, healthScores)

	// Reorder healthy providers ahead of cooling providers just like live routing.
	sort.SliceStable(scores, func(i, j int) bool {
		ci := allCredsCooling[scores[i].Model.ProviderID]
		cj := allCredsCooling[scores[j].Model.ProviderID]
		return !ci && cj
	})

	scoreDetails := make([]ModelScoreDetail, 0, len(scores))
	selectedModel := ""
	selectedProvider := ""
	downgradeReason := ""
	confidenceScore := 0.0
	simulatedLatencyMs := 0
	simulatedCostUSD := 0.0
	reasoning := ""

	if len(scores) > 0 {
		winner := scores[0]
		selectedModel = winner.Model.Slug
		selectedProvider = providerMap[winner.Model.ProviderID]
		confidenceScore = winner.Score
		if winner.Score < 0.50 {
			downgradeReason = "low_score_under_budget_pressure"
		}
		if allCredsCooling[winner.Model.ProviderID] {
			downgradeReason = "all_credentials_cooling_down"
		}
		var winMetrics *goredis.ModelMetrics
		if telemetryMap != nil {
			winMetrics = telemetryMap[winner.Model.Slug]
		}
		simulatedLatencyMs = estimateModelLatencyMs(winner.Model, winner.SpeedScore, winMetrics)
		simulatedCostUSD = estimateModelPer1KCost(winner.Model)

		for _, s := range scores {
			cooling := allCredsCooling[s.Model.ProviderID]
			reasons := append([]string{}, s.Reason...)
			if cooling {
				reasons = append(reasons, "provider_credentials_in_cooldown")
			}
			var mMetrics *goredis.ModelMetrics
			if telemetryMap != nil {
				mMetrics = telemetryMap[s.Model.Slug]
			}
			estLatency := estimateModelLatencyMs(s.Model, s.SpeedScore, mMetrics)
			estCost := estimateModelPer1KCost(s.Model)
			isHealthy := !cooling && (s.HealthScore == 0 || s.HealthScore >= 0.5)

			scoreDetails = append(scoreDetails, ModelScoreDetail{
				ModelID:            s.Model.ID,
				ModelSlug:          s.Model.Slug,
				Slug:               s.Model.Slug,
				DisplayName:        s.Model.DisplayName,
				ProviderName:       providerMap[s.Model.ProviderID],
				Score:              s.Score,
				TotalScore:         s.Score,
				QualityScore:       s.QualityScore,
				CostScore:          s.CostScore,
				SpeedScore:         s.SpeedScore,
				LatencyScore:       s.SpeedScore,
				HealthScore:        s.HealthScore,
				Reasons:            reasons,
				InputPrice1M:       s.Model.InputPricePer1M,
				OutputPrice1M:      s.Model.OutputPricePer1M,
				EstimatedLatencyMs: estLatency,
				EstimatedCostUSD:   estCost,
				IsHealthy:          isHealthy,
			})
		}
		reasoning = strings.Join(scoreDetails[0].Reasons, ", ")
	}

	c.JSON(http.StatusOK, SimulateResponse{
		PromptPreview:      promptPreview,
		TaskType:           string(chars.Task),
		Complexity:         string(chars.Complexity),
		PolicyName:         policy.Name,
		Strategy:           policy.Name,
		WeightsUsed:        policy.Weights,
		BudgetStatus:       req.BudgetStatus,
		SelectedModel:      selectedModel,
		SelectedProvider:   selectedProvider,
		ConfidenceScore:    confidenceScore,
		SimulatedLatencyMs: simulatedLatencyMs,
		SimulatedCostUSD:   simulatedCostUSD,
		Reasoning:          reasoning,
		Candidates:         scoreDetails,
		DowngradeReason:    downgradeReason,
	})
}

// Replay re-evaluates recent persisted routing decisions against a candidate routing policy
// and summarizes model selection diffs and estimated cost delta.
func (h *SimulateHandler) Replay(c *gin.Context) {
	if h.decisions == nil {
		httputil.RespondInternalError(c, "Routing decision store unavailable", fmt.Errorf("decision repository not wired"), "DECISIONS_UNAVAILABLE")
		return
	}

	userID := c.GetString("userId")
	var req ReplayRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
			return
		}
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 25
	}

	ctx := middleware.ContextWithTenant(c)
	policy := h.resolvePolicy(ctx, userID, req.PolicyID, req.CustomWeights, nil)

	logs, _, err := h.decisions.ListWithFilter(ctx, userID, limit, 0)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load historical routing decisions", err, "ROUTING_DECISIONS_LOAD_FAILED")
		return
	}

	allModels, err := h.models.ListEnabled(ctx)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load models", err, "MODELS_LOAD_FAILED")
		return
	}

	providerMap := make(map[string]string)
	if h.providers != nil {
		allProviders, _ := h.providers.ListByUserID(ctx, userID)
		for _, p := range allProviders {
			providerMap[p.ID] = p.Name
		}
	}

	activeProviderIDs := make(map[string]bool)
	if h.creds != nil {
		activeProviderIDs, _ = h.creds.ListActiveProviderIDs(ctx, userID)
	}

	modelBySlug := make(map[string]*models.Model)
	var candidates []*models.Model
	var candidateSlugs []string
	for i := range allModels {
		m := &allModels[i]
		modelBySlug[m.Slug] = m
		if activeProviderIDs[m.ProviderID] {
			candidates = append(candidates, m)
			candidateSlugs = append(candidateSlugs, m.Slug)
		}
	}

	var telemetryMap map[string]*goredis.ModelMetrics
	if h.telemetry != nil && len(candidateSlugs) > 0 {
		telemetryMap, _ = h.telemetry.GetMultipleModelMetrics(c.Request.Context(), candidateSlugs)
	}
	var healthScores map[string]float64
	if h.health != nil {
		healthScores = h.health.ScoresFor(c.Request.Context(), middleware.GetTenantContext(c).OrgID)
	}

	items := make([]ReplayItem, 0, len(logs))
	changedCount := 0
	var historicalTotal, simulatedTotal float64

	for _, d := range logs {
		chars := proxy.ClassifyRequest([]map[string]interface{}{{"role": "user", "content": d.PromptPreview}})
		if d.TaskType != "" {
			chars.Task = proxy.TaskType(d.TaskType)
		}
		if d.Complexity != "" {
			chars.Complexity = proxy.Complexity(d.Complexity)
		}
		budgetStatus := req.BudgetStatus
		if budgetStatus == "" {
			budgetStatus = d.BudgetStatus
		}
		if budgetStatus == "" {
			budgetStatus = "healthy"
		}

		origCost := d.ActualCost
		if origCost == 0 {
			origCost = d.EstimatedCost
		}
		historicalTotal += origCost

		simModel := d.SelectedModel
		simProvider := d.SelectedProvider
		simCost := origCost

		scores := proxy.ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, budgetStatus, telemetryMap, healthScores)
		if len(scores) > 0 {
			w := scores[0].Model
			simModel = w.Slug
			simProvider = providerMap[w.ProviderID]
			if simModel == d.SelectedModel {
				simCost = origCost
			} else if origM := modelBySlug[d.SelectedModel]; origM != nil && estimateModelPer1KCost(origM) > 0 && origCost > 0 {
				simCost = origCost * (estimateModelPer1KCost(w) / estimateModelPer1KCost(origM))
			} else {
				simCost = estimateModelPer1KCost(w)
			}
		}

		changed := simModel != d.SelectedModel && simModel != ""
		if changed {
			changedCount++
		}
		simulatedTotal += simCost

		items = append(items, ReplayItem{
			RequestID:          d.RequestID,
			PromptPreview:      d.PromptPreview,
			TaskType:           d.TaskType,
			Complexity:         d.Complexity,
			OriginalModel:      d.SelectedModel,
			OriginalProvider:   d.SelectedProvider,
			OriginalCost:       origCost,
			OriginalStatusCode: d.StatusCode,
			OriginalLatencyMs:  d.LatencyMs,
			OriginalError:      d.ErrorMessage,
			SimulatedModel:     simModel,
			SimulatedProvider:  simProvider,
			SimulatedCost:      simCost,
			Changed:            changed,
			CostDelta:          simCost - origCost,
		})
	}

	changeRate := 0.0
	if len(items) > 0 {
		changeRate = float64(changedCount) / float64(len(items))
	}

	c.JSON(http.StatusOK, ReplayResponse{
		PolicyName:          policy.Name,
		EvaluatedCount:      len(items),
		ChangedCount:        changedCount,
		ChangeRate:          changeRate,
		HistoricalTotalCost: historicalTotal,
		SimulatedTotalCost:  simulatedTotal,
		EstimatedCostDelta:  simulatedTotal - historicalTotal,
		Items:               items,
	})
}
