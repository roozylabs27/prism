package proxy

import (
	"context"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type agentFinderMock struct {
	agent *models.Agent
	err   error
}

func (m *agentFinderMock) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.agent == nil {
		return nil, errors.New("agent not found")
	}
	return m.agent, nil
}

func TestAgentGovernanceValidateModelAccess(t *testing.T) {
	ag := &models.Agent{
		Name:          "dev-agent",
		AllowedModels: []string{"gpt-4o", "claude-sonnet"},
		AllowedTools:  []string{"search_web"},
		Enabled:       true,
	}

	engine := NewAgentGovernanceEngine(&agentFinderMock{agent: ag})

	// Test allowed model
	res, err := engine.ValidateAgentModelAccess(context.Background(), "u1", "dev-agent", "gpt-4o")
	require.NoError(t, err)
	assert.True(t, res.ModelAllowed)

	// Test denied model
	res, err = engine.ValidateAgentModelAccess(context.Background(), "u1", "dev-agent", "deepseek-coder")
	require.NoError(t, err)
	assert.False(t, res.ModelAllowed)
	assert.Contains(t, res.Reason, "not permitted")
}

func TestAgentGovernanceValidateToolAccess(t *testing.T) {
	ag := &models.Agent{
		Name:         "dev-agent",
		AllowedTools: []string{"search_web"},
		Enabled:      true,
	}

	engine := NewAgentGovernanceEngine(&agentFinderMock{agent: ag})

	// Test allowed tool
	res, err := engine.ValidateAgentToolAccess(context.Background(), "u1", "dev-agent", "search_web")
	require.NoError(t, err)
	assert.True(t, res.ToolAllowed)

	// Test denied tool
	res, err = engine.ValidateAgentToolAccess(context.Background(), "u1", "dev-agent", "execute_sql")
	require.NoError(t, err)
	assert.False(t, res.ToolAllowed)
	assert.Contains(t, res.Reason, "not permitted")
}

func TestAgentGovernance_ResourceMCPAndResolvedModelChecks(t *testing.T) {
	ag := &models.Agent{
		Name:              "client-agent",
		Enabled:           true,
		AllowedModels:     []string{"gpt-4o-mini", "!gpt-4o"},
		AllowedResources:  []string{"*", "!prod_db"},
		AllowedMCPServers: []string{"context7", "!local_shell"},
	}

	assert.True(t, CheckAgentResourceAccess(ag, "kb_docs").ResourceAllowed)
	assert.False(t, CheckAgentResourceAccess(ag, "prod_db").ResourceAllowed)

	assert.True(t, CheckAgentMCPServerAccess(ag, "context7", "Context7", "srv1").ToolAllowed)
	assert.False(t, CheckAgentMCPServerAccess(ag, "local_shell", "Local", "srv2").ToolAllowed)
	assert.False(t, CheckAgentMCPServerAccess(ag, "github", "GitHub", "srv3").ToolAllowed)

	assert.True(t, CheckAgentResolvedModel(ag, "gpt-4o-mini"))
	assert.False(t, CheckAgentResolvedModel(ag, "gpt-4o"))
	assert.False(t, CheckAgentResolvedModel(ag, "claude-3-5-sonnet"), "concrete resolved check must not allow unlisted model")
}
