package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
)

// AgentFinder resolves an agent id or name to one the caller may use: their
// own, or a system agent.
type AgentFinder interface {
	FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error)
}

// agentHeaders are the request headers that name an agent, in order of
// precedence.
var agentHeaders = []string{"X-Prism-Agent-ID", "X-Paperclip-Agent-ID", "X-Prism-Agent-Name", "X-Agent-Name"}

func AgentPolicyMiddleware(agents AgentFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !ResolveAgent(c, agents) {
			return
		}
		c.Next()
	}
}

// ResolveAgent looks up the agent pinned on the gateway key or named by the
// request headers for the gateway key's owner and stores it in the context.
// Key-bound requests enforce the pinned agent and reject overriding headers.
// A named or pinned agent that does not exist or is disabled ends the request
// with 403 and returns false.
func ResolveAgent(c *gin.Context, agents AgentFinder) bool {
	var headerRef string
	for _, h := range agentHeaders {
		if headerRef = strings.TrimSpace(c.GetHeader(h)); headerRef != "" {
			break
		}
	}

	var ownerID string
	var boundAgentID string
	if key, ok := c.Get("gatewayKey"); ok {
		if gk, isKey := key.(*models.GatewayAPIKey); isKey && gk != nil {
			ownerID = gk.UserID
			if gk.AgentID != nil {
				boundAgentID = strings.TrimSpace(*gk.AgentID)
			}
		}
	}

	if boundAgentID != "" {
		if agents == nil {
			httputil.RespondForbidden(c, "Agent identity bound to gateway key could not be resolved", nil, "AGENT_NOT_FOUND")
			return false
		}
		agent, err := agents.FindForUser(c.Request.Context(), ownerID, boundAgentID)
		if err != nil || agent == nil {
			httputil.RespondForbidden(c, "Agent identity bound to gateway key was not found", err, "AGENT_NOT_FOUND")
			return false
		}
		if !agent.Enabled {
			httputil.RespondForbidden(c, "Agent bound to gateway key is disabled", nil, "AGENT_DISABLED")
			return false
		}
		if headerRef != "" && headerRef != agent.ID && !strings.EqualFold(headerRef, agent.Name) {
			httputil.RespondForbidden(c, "Gateway key is bound to a different agent and cannot be overridden", nil, "AGENT_BINDING_MISMATCH")
			return false
		}
		c.Set("agentID", agent.ID)
		c.Set("agentName", agent.Name)
		c.Set("agentObject", agent)
		return true
	}

	if headerRef == "" {
		return true
	}

	if agents == nil {
		httputil.RespondForbidden(c, "Agent identity specified in request headers was not found", nil, "AGENT_NOT_FOUND")
		return false
	}
	agent, err := agents.FindForUser(c.Request.Context(), ownerID, headerRef)
	if err != nil || agent == nil {
		httputil.RespondForbidden(c, "Agent identity specified in request headers was not found", err, "AGENT_NOT_FOUND")
		return false
	}

	if !agent.Enabled {
		httputil.RespondForbidden(c, "Agent specified in request headers is disabled", nil, "AGENT_DISABLED")
		return false
	}

	c.Set("agentID", agent.ID)
	c.Set("agentName", agent.Name)
	c.Set("agentObject", agent)
	return true
}

// AgentFromContext returns the agent ResolveAgent stored, or nil when the
// request named none.
func AgentFromContext(c *gin.Context) *models.Agent {
	if v, ok := c.Get("agentObject"); ok {
		if ag, isAgent := v.(*models.Agent); isAgent {
			return ag
		}
	}
	return nil
}
