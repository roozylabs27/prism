package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

type mockAgentFinder struct {
	agent *models.Agent
	err   error
}

func (m *mockAgentFinder) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.agent, nil
}

func TestAgentPolicyMiddleware_NoHeaderPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestAgentPolicyMiddleware_AgentDisabledAborts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &models.Agent{
		ID:      "agent-1",
		Name:    "Disabled Agent",
		Enabled: false,
	}

	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: agent}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Prism-Agent-ID", "agent-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "disabled")
}

func TestAgentPolicyMiddleware_ValidAgentSetsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &models.Agent{
		ID:            "agent-1",
		Name:          "Gopher Agent",
		Enabled:       true,
		AllowedModels: []string{"prism-auto", "claude-sonnet-3-5"},
	}

	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: agent}))
	var setID, setName string
	var retrievedAgent *models.Agent
	r.GET("/test", func(c *gin.Context) {
		setID = c.GetString("agentID")
		setName = c.GetString("agentName")
		if val, ok := c.Get("agentObject"); ok {
			retrievedAgent = val.(*models.Agent)
		}
		c.String(http.StatusOK, "ok")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Prism-Agent-ID", "agent-1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "agent-1", setID)
	assert.Equal(t, "Gopher Agent", setName)
	assert.NotNil(t, retrievedAgent)
	assert.Equal(t, []string{"prism-auto", "claude-sonnet-3-5"}, retrievedAgent.AllowedModels)
}

func TestAgentPolicyMiddleware_EveryAgentHeaderIsResolved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, header := range []string{"X-Prism-Agent-ID", "X-Paperclip-Agent-ID", "X-Prism-Agent-Name", "X-Agent-Name"} {
		r := gin.New()
		// The agent does not exist, so naming it in any header must stop the
		// request rather than reach the handler unchecked.
		r.Use(AgentPolicyMiddleware(&mockAgentFinder{}))
		r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

		req, _ := http.NewRequest("GET", "/test", nil)
		req.Header.Set(header, "ghost-agent")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code, header)
	}
}

func TestAgentPolicyMiddleware_PaperclipHeaderResolvesValidAgent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	agent := &models.Agent{ID: "ag_pc", Name: "paperclip-coder", Enabled: true}
	r := gin.New()
	r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: agent}))
	var gotID string
	r.GET("/test", func(c *gin.Context) {
		gotID = c.GetString("agentID")
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Paperclip-Agent-ID", "ag_pc")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ag_pc", gotID)
}

func TestAgentPolicyMiddleware_KeyBoundAgentEnforced(t *testing.T) {
	gin.SetMode(gin.TestMode)
	boundID := "ag_pinned"
	pinnedAgent := &models.Agent{ID: boundID, Name: "pinned-agent", Enabled: true}

	t.Run("missing header still binds key-pinned agent", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", AgentID: &boundID})
			c.Next()
		})
		r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: pinnedAgent}))
		var got *models.Agent
		r.GET("/test", func(c *gin.Context) {
			got = AgentFromContext(c)
			c.String(http.StatusOK, "ok")
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		if assert.NotNil(t, got) {
			assert.Equal(t, boundID, got.ID)
		}
	})

	t.Run("matching ID or name header is allowed", func(t *testing.T) {
		for _, val := range []string{boundID, "PINNED-AGENT"} {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", AgentID: &boundID})
				c.Next()
			})
			r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: pinnedAgent}))
			r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-Prism-Agent-ID", val)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("conflicting header override is rejected with 403", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", AgentID: &boundID})
			c.Next()
		})
		r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: pinnedAgent}))
		r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Paperclip-Agent-ID", "other-agent")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "AGENT_BINDING_MISMATCH")
	})

	t.Run("disabled pinned agent is rejected with 403", func(t *testing.T) {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", AgentID: &boundID})
			c.Next()
		})
		r.Use(AgentPolicyMiddleware(&mockAgentFinder{agent: &models.Agent{ID: boundID, Name: "pinned-agent", Enabled: false}}))
		r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "disabled")
	})
}
