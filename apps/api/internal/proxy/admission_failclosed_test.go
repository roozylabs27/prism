package proxy

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scriptedQuotaEvaluator struct {
	results map[string]*models.QuotaCheckResult
	errs    map[string]error
}

func (s *scriptedQuotaEvaluator) EvaluateQuota(_ context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	key := targetType + "/" + targetID
	if err, ok := s.errs[key]; ok {
		return nil, err
	}
	if res, ok := s.results[key]; ok {
		return res, nil
	}
	return &models.QuotaCheckResult{Allowed: true, TargetType: targetType, TargetID: targetID}, nil
}

type fakeAgentSpendFinder struct {
	spent float64
	err   error
}

func (f fakeAgentSpendFinder) GetAgentTotalSpendUSD(_ context.Context, _ string) (float64, error) {
	return f.spent, f.err
}

func TestAdmissionController_WorkspaceAndProjectQuotasAndFailClosed(t *testing.T) {
	ctx := context.Background()

	// 1. Workspace quota exceeded while org is healthy -> denied 429.
	q := &scriptedQuotaEvaluator{
		results: map[string]*models.QuotaCheckResult{
			"organization/org1": {Allowed: true},
			"workspace/ws_client": {Allowed: false, Reason: "Workspace daily spend exceeded"},
		},
	}
	adm := NewAdmissionController(nil, nil, q, nil)
	res, err := adm.Evaluate(ctx, AdmissionRequest{
		UserID:    "u1",
		TenantCtx: models.TenantContext{OrgID: "org1", WorkspaceID: "ws_client", ProjectID: "proj_1"},
	})
	require.NoError(t, err)
	assert.Equal(t, AdmissionDeny, res.Decision)
	assert.Equal(t, http.StatusTooManyRequests, res.HTTPStatus)
	assert.Equal(t, "quota_exceeded_error", res.ErrorCode)
	assert.Contains(t, res.Reason, "Workspace")

	// 2. Project quota exceeded while org and workspace are healthy -> denied 429.
	q.results["workspace/ws_client"] = &models.QuotaCheckResult{Allowed: true}
	q.results["project/proj_1"] = &models.QuotaCheckResult{Allowed: false, Reason: "Project daily request limit exceeded"}
	res, err = adm.Evaluate(ctx, AdmissionRequest{
		UserID:    "u1",
		TenantCtx: models.TenantContext{OrgID: "org1", WorkspaceID: "ws_client", ProjectID: "proj_1"},
	})
	require.NoError(t, err)
	assert.Equal(t, AdmissionDeny, res.Decision)
	assert.Contains(t, res.Reason, "Project")

	// 3. Quota evaluator error must fail closed (return error, never AdmissionAllow).
	qErr := &scriptedQuotaEvaluator{
		errs: map[string]error{"organization/org1": errors.New("postgres timeout")},
	}
	admErr := NewAdmissionController(nil, nil, qErr, nil)
	_, err = admErr.Evaluate(ctx, AdmissionRequest{
		UserID:    "u1",
		TenantCtx: models.TenantContext{OrgID: "org1"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "quota evaluate")

	// 4. BudgetManager error must fail closed.
	bmErr := &BudgetManager{budgetRepo: &fakeBudgetStore{err: errors.New("budget store down")}, inFlight: make(map[string]float64)}
	admBudgetErr := NewAdmissionController(nil, nil, nil, bmErr)
	_, err = admBudgetErr.Evaluate(ctx, AdmissionRequest{UserID: "u1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "budget evaluate")
}

func TestAdmissionController_AgentResourceMCPAndBudgetEnforcement(t *testing.T) {
	ctx := context.Background()
	ag := &models.Agent{
		ID:                "agent_1",
		Name:              "support-bot",
		Enabled:           true,
		AllowedModels:     []string{"gpt-4o-mini"},
		AllowedTools:      []string{"search", "!drop_table"},
		AllowedResources:  []string{"docs", "!secrets_db"},
		AllowedMCPServers: []string{"github"},
		MaxBudgetCents:    500, // $5.00
	}

	adm := NewAdmissionController(nil, nil, nil, nil)
	adm.SetAgentSpendFinder(fakeAgentSpendFinder{spent: 1.25})

	// Resource explicit deny
	res, err := adm.Evaluate(ctx, AdmissionRequest{UserID: "u1", Agent: ag, ResourceName: "secrets_db"})
	require.NoError(t, err)
	assert.Equal(t, AdmissionDeny, res.Decision)
	assert.Equal(t, "agent_resource_denied", res.ErrorCode)

	// Disallowed MCP server
	res, err = adm.Evaluate(ctx, AdmissionRequest{UserID: "u1", Agent: ag, MCPServerName: "stripe"})
	require.NoError(t, err)
	assert.Equal(t, AdmissionDeny, res.Decision)
	assert.Equal(t, "agent_mcp_server_denied", res.ErrorCode)

	// Agent MaxBudgetCents exceeded ($5.50 >= $5.00) -> 429 agent_budget_exceeded
	admOver := NewAdmissionController(nil, nil, nil, nil)
	admOver.SetAgentSpendFinder(fakeAgentSpendFinder{spent: 5.50})
	res, err = admOver.Evaluate(ctx, AdmissionRequest{UserID: "u1", Agent: ag, ModelSlug: "gpt-4o-mini"})
	require.NoError(t, err)
	assert.Equal(t, AdmissionDeny, res.Decision)
	assert.Equal(t, http.StatusTooManyRequests, res.HTTPStatus)
	assert.Equal(t, "agent_budget_exceeded", res.ErrorCode)
}
