package proxy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

// AgentFinder resolves an agent id or name to one the caller may use: their
// own, or a system agent.
type AgentFinder interface {
	FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error)
}

type AgentGovernanceEngine struct {
	agents AgentFinder
}

func NewAgentGovernanceEngine(agents AgentFinder) *AgentGovernanceEngine {
	return &AgentGovernanceEngine{agents: agents}
}

// resolve looks the agent up for the user. A reference that matches no agent
// the user may use yields a denial reason instead of an error.
func (g *AgentGovernanceEngine) resolve(ctx context.Context, userID, ref string) (*models.Agent, string, error) {
	ag, err := g.agents.FindForUser(ctx, userID, ref)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && ag == nil) {
		return nil, fmt.Sprintf("agent %q not found", ref), nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("resolve agent %q: %w", ref, err)
	}
	return ag, "", nil
}

// ValidateAgentModelAccess resolves the agent by id or name and checks it may
// use the model. An unknown agent is denied.
func (g *AgentGovernanceEngine) ValidateAgentModelAccess(ctx context.Context, userID, agentRef, modelSlug string) (*models.AgentGovernanceCheckResult, error) {
	if agentRef == "" {
		return &models.AgentGovernanceCheckResult{ModelAllowed: true}, nil
	}
	ag, reason, err := g.resolve(ctx, userID, agentRef)
	if err != nil {
		return nil, err
	}
	if ag == nil {
		return &models.AgentGovernanceCheckResult{AgentName: agentRef, Reason: reason}, nil
	}
	return CheckAgentModelAccess(ag, modelSlug), nil
}

// ValidateAgentToolAccess resolves the agent by id or name and checks it may
// use the tool. An unknown agent is denied.
func (g *AgentGovernanceEngine) ValidateAgentToolAccess(ctx context.Context, userID, agentRef, toolName string) (*models.AgentGovernanceCheckResult, error) {
	if agentRef == "" {
		return &models.AgentGovernanceCheckResult{ToolAllowed: true}, nil
	}
	ag, reason, err := g.resolve(ctx, userID, agentRef)
	if err != nil {
		return nil, err
	}
	if ag == nil {
		return &models.AgentGovernanceCheckResult{AgentName: agentRef, Reason: reason}, nil
	}
	return CheckAgentToolAccess(ag, toolName), nil
}

// CheckAgentModelAccess applies the agent's allowed_models rules: deny rules
// win, an empty list or an auto-routed model allows, otherwise the model must
// be listed or the list must contain "*".
func CheckAgentModelAccess(ag *models.Agent, modelSlug string) *models.AgentGovernanceCheckResult {
	if !ag.Enabled {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, Reason: fmt.Sprintf("agent %q is disabled", ag.Name)}
	}
	if isDenied(ag.AllowedModels, modelSlug) {
		return &models.AgentGovernanceCheckResult{
			AgentName: ag.Name,
			Reason:    fmt.Sprintf("model %q explicitly denied for agent %q", modelSlug, ag.Name),
		}
	}
	if len(ag.AllowedModels) == 0 || strings.EqualFold(modelSlug, "prism-auto") || strings.EqualFold(modelSlug, "auto") || isAllowed(ag.AllowedModels, modelSlug) {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, ModelAllowed: true}
	}
	return &models.AgentGovernanceCheckResult{
		AgentName: ag.Name,
		Reason:    fmt.Sprintf("model %q is not permitted for agent %q", modelSlug, ag.Name),
	}
}

// CheckAgentToolAccess applies the agent's allowed_tools rules the same way.
func CheckAgentToolAccess(ag *models.Agent, toolName string) *models.AgentGovernanceCheckResult {
	if !ag.Enabled {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, Reason: fmt.Sprintf("agent %q is disabled", ag.Name)}
	}
	if isDenied(ag.AllowedTools, toolName) {
		return &models.AgentGovernanceCheckResult{
			AgentName: ag.Name,
			Reason:    fmt.Sprintf("tool %q explicitly denied for agent %q", toolName, ag.Name),
		}
	}
	if len(ag.AllowedTools) == 0 || isAllowed(ag.AllowedTools, toolName) {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, ToolAllowed: true}
	}
	return &models.AgentGovernanceCheckResult{
		AgentName: ag.Name,
		Reason:    fmt.Sprintf("tool %q is not permitted for agent %q", toolName, ag.Name),
	}
}

// CheckAgentResourceAccess applies the agent's allowed_resources rules: deny
// rules win, an empty list allows, otherwise the resource must be listed or "*".
func CheckAgentResourceAccess(ag *models.Agent, resourceName string) *models.AgentGovernanceCheckResult {
	if !ag.Enabled {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, Reason: fmt.Sprintf("agent %q is disabled", ag.Name)}
	}
	if isDenied(ag.AllowedResources, resourceName) {
		return &models.AgentGovernanceCheckResult{
			AgentName: ag.Name,
			Reason:    fmt.Sprintf("resource %q explicitly denied for agent %q", resourceName, ag.Name),
		}
	}
	if len(ag.AllowedResources) == 0 || isAllowed(ag.AllowedResources, resourceName) {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, ResourceAllowed: true}
	}
	return &models.AgentGovernanceCheckResult{
		AgentName: ag.Name,
		Reason:    fmt.Sprintf("resource %q is not permitted for agent %q", resourceName, ag.Name),
	}
}

// CheckAgentMCPServerAccess verifies whether an agent may call tools exposed
// by the named MCP server (matching name, displayName, or serverID).
func CheckAgentMCPServerAccess(ag *models.Agent, serverName, serverDisplayName, serverID string) *models.AgentGovernanceCheckResult {
	if !ag.Enabled {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, Reason: fmt.Sprintf("agent %q is disabled", ag.Name)}
	}
	candidates := []string{serverName}
	if serverDisplayName != "" {
		candidates = append(candidates, serverDisplayName)
	}
	if serverID != "" {
		candidates = append(candidates, serverID)
	}
	for _, cand := range candidates {
		if cand != "" && isDenied(ag.AllowedMCPServers, cand) {
			return &models.AgentGovernanceCheckResult{
				AgentName: ag.Name,
				Reason:    fmt.Sprintf("mcp server %q explicitly denied for agent %q", serverName, ag.Name),
			}
		}
	}
	if len(ag.AllowedMCPServers) == 0 {
		return &models.AgentGovernanceCheckResult{AgentName: ag.Name, ToolAllowed: true}
	}
	for _, cand := range candidates {
		if cand != "" && isAllowed(ag.AllowedMCPServers, cand) {
			return &models.AgentGovernanceCheckResult{AgentName: ag.Name, ToolAllowed: true}
		}
	}
	return &models.AgentGovernanceCheckResult{
		AgentName: ag.Name,
		Reason:    fmt.Sprintf("mcp server %q is not permitted for agent %q", serverName, ag.Name),
	}
}

// CheckAgentResolvedModel checks whether a concrete model candidate (after
// smart routing) is permitted by the agent's allowed_models rules, without the
// "prism-auto" pass-through.
func CheckAgentResolvedModel(ag *models.Agent, concreteModelSlug string) bool {
	if ag == nil {
		return true
	}
	if !ag.Enabled {
		return false
	}
	if isDenied(ag.AllowedModels, concreteModelSlug) {
		return false
	}
	if len(ag.AllowedModels) == 0 {
		return true
	}
	return isAllowed(ag.AllowedModels, concreteModelSlug)
}

// isDenied reports whether a "!name", "-name" or "deny:name" rule matches.
func isDenied(rules []string, target string) bool {
	for _, rule := range rules {
		if strings.HasPrefix(rule, "!") || strings.HasPrefix(rule, "-") || strings.HasPrefix(strings.ToLower(rule), "deny:") {
			denied := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(rule, "!"), "-"), "deny:")
			if strings.EqualFold(denied, target) {
				return true
			}
		}
	}
	return false
}

func isAllowed(rules []string, target string) bool {
	for _, rule := range rules {
		if rule == "*" || strings.EqualFold(rule, target) {
			return true
		}
	}
	return false
}
