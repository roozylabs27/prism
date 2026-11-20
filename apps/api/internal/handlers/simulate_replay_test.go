package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSimModels struct{ items []models.Model }

func (f *fakeSimModels) ListEnabled(context.Context) ([]models.Model, error) { return f.items, nil }

type fakeSimProviders struct{ items []models.Provider }

func (f *fakeSimProviders) ListByUserID(context.Context, string) ([]models.Provider, error) {
	return f.items, nil
}

type fakeSimCreds struct {
	activeProviders map[string]bool
	credsByProv     map[string][]models.Credential
}

func (f *fakeSimCreds) ListActiveProviderIDs(context.Context, ...string) (map[string]bool, error) {
	return f.activeProviders, nil
}
func (f *fakeSimCreds) FindAllActiveByProviderID(_ context.Context, pid string, _ []string, _ ...string) ([]models.Credential, error) {
	return f.credsByProv[pid], nil
}

type fakeSimPolicies struct{ items map[string]*models.RoutingPolicy }

func (f *fakeSimPolicies) FindByID(_ context.Context, id, _ string) (*models.RoutingPolicy, error) {
	return f.items[id], nil
}
func (f *fakeSimPolicies) FindByDefault(context.Context, string) (*models.RoutingPolicy, error) {
	return nil, nil
}
func (f *fakeSimPolicies) FindByName(_ context.Context, name, _ string) (*models.RoutingPolicy, error) {
	return f.items[name], nil
}

type fakeSimRules struct{ items []models.RoutingRule }

func (f *fakeSimRules) ListEnabledByUserID(context.Context, string) ([]models.RoutingRule, error) {
	return f.items, nil
}

type fakeSimAgents struct{ items map[string]*models.Agent }

func (f *fakeSimAgents) FindForUser(_ context.Context, _, ref string) (*models.Agent, error) {
	return f.items[ref], nil
}

type fakeSimDecisions struct{ logs []repository.RoutingDecisionLog }

func (f *fakeSimDecisions) ListWithFilter(context.Context, string, int, int) ([]repository.RoutingDecisionLog, int64, error) {
	return f.logs, int64(len(f.logs)), nil
}

type fakeSimCooldown struct{ cooling map[string]bool }

func (f *fakeSimCooldown) CoolingSet(_ context.Context, ids []string) (map[string]bool, error) {
	out := make(map[string]bool)
	for _, id := range ids {
		if f.cooling[id] {
			out[id] = true
		}
	}
	return out, nil
}

func TestSimulateHandler_SimulateContractAndFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	provCheap := "prov-openai"
	provRule := "prov-openrouter"
	h := &SimulateHandler{
		models: &fakeSimModels{items: []models.Model{
			{ID: "m1", Slug: "gpt-4o-mini", DisplayName: "GPT-4o Mini", ProviderID: provCheap, Enabled: true, QualityScore: 0.72, SpeedScore: 0.90, InputPricePer1M: 0.15, OutputPricePer1M: 0.60},
			{ID: "m2", Slug: "claude-3-5-sonnet", DisplayName: "Claude 3.5 Sonnet", ProviderID: provRule, Enabled: true, QualityScore: 0.95, SpeedScore: 0.75, InputPricePer1M: 3.00, OutputPricePer1M: 15.0},
		}},
		providers: &fakeSimProviders{items: []models.Provider{
			{ID: provCheap, Name: "OpenAI", Enabled: true},
			{ID: provRule, Name: "OpenRouter", Enabled: true},
		}},
		creds: &fakeSimCreds{
			activeProviders: map[string]bool{provCheap: true, provRule: true},
			credsByProv: map[string][]models.Credential{
				provCheap: {{ID: "c1"}},
				provRule:  {{ID: "c2"}},
			},
		},
		policies: &fakeSimPolicies{items: map[string]*models.RoutingPolicy{
			"cost-first": {ID: "cost-first", Name: "cost-first", Weights: map[string]float64{"cost": 0.9, "quality": 0.1}},
		}},
		rules:  &fakeSimRules{items: []models.RoutingRule{{Enabled: true, ModelPattern: "claude-*", ProviderID: &provRule}}},
		agents: &fakeSimAgents{items: map[string]*models.Agent{"agent-mini": {ID: "agent-mini", Enabled: true, AllowedModels: []string{"gpt-4o-mini"}}}},
		cooldown: &fakeSimCooldown{cooling: map[string]bool{}},
	}

	// 1. Auto simulation emits dual Playground + API fields
	body, _ := json.Marshal(SimulateRequest{Prompt: "Summarize a SQL table", PolicyID: "cost-first"})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/routing/simulate", bytes.NewReader(body))
	c.Set("userId", "u1")
	h.Simulate(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp SimulateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "gpt-4o-mini", resp.SelectedModel)
	assert.Equal(t, "cost-first", resp.PolicyName)
	assert.Equal(t, "cost-first", resp.Strategy)
	require.NotEmpty(t, resp.Candidates)
	assert.InDelta(t, resp.Candidates[0].Score, resp.Candidates[0].TotalScore, 1e-9)
	assert.Greater(t, resp.Candidates[0].EstimatedLatencyMs, 0)
	assert.Greater(t, resp.Candidates[0].EstimatedCostUSD, 0.0)
	assert.True(t, resp.Candidates[0].IsHealthy)

	// 2. Explicit targetModel pinned via routing rule + cooldown demotion
	body2, _ := json.Marshal(SimulateRequest{Prompt: "Write Go tests", TargetModel: "claude-3-5-sonnet"})
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/api/routing/simulate", bytes.NewReader(body2))
	c2.Set("userId", "u1")
	h.Simulate(c2)

	require.Equal(t, http.StatusOK, w2.Code)
	var resp2 SimulateResponse
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp2))
	assert.Equal(t, "claude-3-5-sonnet", resp2.SelectedModel)
	assert.Equal(t, "OpenRouter", resp2.SelectedProvider)

	// 3. Agent allowlist restricts candidates to gpt-4o-mini even under quality weights
	body3, _ := json.Marshal(SimulateRequest{Prompt: "Complex architecture proof", AgentID: "agent-mini"})
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodPost, "/api/routing/simulate", bytes.NewReader(body3))
	c3.Set("userId", "u1")
	h.Simulate(c3)

	require.Equal(t, http.StatusOK, w3.Code)
	var resp3 SimulateResponse
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &resp3))
	require.Len(t, resp3.Candidates, 1)
	assert.Equal(t, "gpt-4o-mini", resp3.SelectedModel)
}

func TestSimulateHandler_ReplayHistoricalDecisions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	status200 := 200
	lat450 := 450

	h := &SimulateHandler{
		models: &fakeSimModels{items: []models.Model{
			{ID: "m1", Slug: "gpt-4o-mini", ProviderID: "p1", Enabled: true, QualityScore: 0.70, SpeedScore: 0.90, InputPricePer1M: 0.15, OutputPricePer1M: 0.60},
			{ID: "m2", Slug: "gpt-4o", ProviderID: "p1", Enabled: true, QualityScore: 0.96, SpeedScore: 0.75, InputPricePer1M: 5.00, OutputPricePer1M: 15.00},
		}},
		providers: &fakeSimProviders{items: []models.Provider{{ID: "p1", Name: "OpenAI", Enabled: true}}},
		creds:     &fakeSimCreds{activeProviders: map[string]bool{"p1": true}},
		policies: &fakeSimPolicies{items: map[string]*models.RoutingPolicy{
			"cheapest": {ID: "cheapest", Name: "cheapest", Weights: map[string]float64{"cost": 0.95, "quality": 0.05}},
		}},
		decisions: &fakeSimDecisions{logs: []repository.RoutingDecisionLog{
			{
				RequestID:        "req-1",
				PromptPreview:    "Quick summary",
				TaskType:         "general",
				Complexity:       "low",
				SelectedModel:    "gpt-4o",
				SelectedProvider: "OpenAI",
				ActualCost:       0.020,
				StatusCode:       &status200,
				LatencyMs:        &lat450,
			},
			{
				RequestID:        "req-2",
				PromptPreview:    "Hello",
				SelectedModel:    "gpt-4o-mini",
				SelectedProvider: "OpenAI",
				ActualCost:       0.001,
			},
		}},
	}

	body, _ := json.Marshal(ReplayRequest{PolicyID: "cheapest", Limit: 10})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/routing/replay", bytes.NewReader(body))
	c.Set("userId", "u1")
	h.Replay(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp ReplayResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "cheapest", resp.PolicyName)
	assert.Equal(t, 2, resp.EvaluatedCount)
	assert.Equal(t, 1, resp.ChangedCount)
	assert.Less(t, resp.EstimatedCostDelta, 0.0, "switching from gpt-4o to gpt-4o-mini should save cost")
	assert.Equal(t, "gpt-4o-mini", resp.Items[0].SimulatedModel)
	assert.True(t, resp.Items[0].Changed)
	assert.False(t, resp.Items[1].Changed)
}
