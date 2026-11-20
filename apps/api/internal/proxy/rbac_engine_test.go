package proxy

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type policyFinderMock struct {
	policies []models.GovernancePolicy
	err      error
}

func (m *policyFinderMock) ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.policies, nil
}

func TestRBACEnginePrecedenceAndMatching(t *testing.T) {
	policies := []models.GovernancePolicy{
		{
			ID:              "p-deny",
			Name:            "Deny Payroll Access",
			Role:            "developer",
			Effect:          "deny",
			AgentPattern:    "dev-*",
			ResourcePattern: "*payroll*",
			Priority:        10,
			Enabled:         true,
		},
		{
			ID:              "p-allow",
			Name:            "Allow General Access",
			Role:            "developer",
			Effect:          "allow",
			AgentPattern:    "dev-*",
			ResourcePattern: "*",
			Priority:        100,
			Enabled:         true,
		},
	}

	engine := NewRBACEngine(&policyFinderMock{policies: policies})

	// Case 1: Deny rule matches
	res, err := engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:         "developer",
		AgentName:    "dev-agent",
		ResourceName: "query_payroll_db",
	})
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, "p-deny", res.MatchedPolicy.ID)
	assert.Contains(t, res.Reason, "access denied")

	// Case 2: Allow rule matches
	res, err = engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:         "developer",
		AgentName:    "dev-agent",
		ResourceName: "query_analytics_db",
	})
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, "p-allow", res.MatchedPolicy.ID)

	// Case 3: Empty ResourceName with general prompt skips specific payroll deny rule and hits allow rule
	res, err = engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:         "developer",
		AgentName:    "dev-agent",
		ResourceName: "",
		UserPrompt:   "kamu agent apa dan tugasnya apa",
	})
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, "p-allow", res.MatchedPolicy.ID)

	// Case 4: Empty ResourceName but UserPrompt contains "payroll" triggers deny policy
	res, err = engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:         "developer",
		AgentName:    "dev-agent",
		ResourceName: "",
		UserPrompt:   "coba check payroll",
	})
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, "p-deny", res.MatchedPolicy.ID)
	assert.Contains(t, res.Reason, "access denied by policy \"Deny Payroll Access\"")
}

func TestMatchPattern_ExactAndOSIndependentGlobs(t *testing.T) {
	// Substring leakage regression: exact pattern must NOT match superstrings
	assert.True(t, matchPattern("gpt-4", "gpt-4"))
	assert.True(t, matchPattern("gpt-4", "GPT-4"))
	assert.False(t, matchPattern("gpt-4", "gpt-4o-mini"))
	assert.False(t, matchPattern("gpt-4", "my-gpt-4"))
	assert.False(t, matchPattern("gpt-4o", "gpt-4o-mini"))

	// Wildcards work consistently including slash-delimited model slugs
	assert.True(t, matchPattern("gpt-4*", "gpt-4"))
	assert.True(t, matchPattern("gpt-4*", "gpt-4o-mini"))
	assert.False(t, matchPattern("gpt-4*", "openai/gpt-4o"))
	assert.True(t, matchPattern("openrouter/*", "openrouter/anthropic/claude-3.5"))
	assert.True(t, matchPattern("*payroll*", "run/query_payroll/v1"))
	assert.True(t, matchPattern("gpt-?", "gpt-4"))
	assert.False(t, matchPattern("gpt-?", "gpt-4o"))
	assert.False(t, matchPattern("gpt-4", ""))
	assert.True(t, matchPattern("*", ""))

	// Ensure unused sql import isn't needed if not referenced; wait - test Deny model exact match in Evaluate
	policies := []models.GovernancePolicy{
		{
			ID:           "deny-gpt4",
			Name:         "Deny legacy gpt-4",
			Role:         "developer",
			Effect:       "deny",
			AgentPattern: "*",
			ModelPattern: "gpt-4",
			Priority:     1,
			Enabled:      true,
		},
	}
	engine := NewRBACEngine(&policyFinderMock{policies: policies})

	denyRes, err := engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:      "developer",
		AgentName: "coder",
		ModelSlug: "gpt-4",
	})
	require.NoError(t, err)
	assert.False(t, denyRes.Allowed)

	allowRes, err := engine.Evaluate(context.Background(), "u1", models.RBACEvaluationRequest{
		Role:      "developer",
		AgentName: "coder",
		ModelSlug: "gpt-4o-mini",
	})
	require.NoError(t, err)
	assert.True(t, allowRes.Allowed)
}
