package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRequestLogCreator struct {
	err       error
	logs      []*models.RequestLog
	tenantCtx []database.TenantDBContext
}

func (s *stubRequestLogCreator) Create(ctx context.Context, log *models.RequestLog) error {
	s.tenantCtx = append(s.tenantCtx, database.TenantContextFrom(ctx))
	if s.err != nil {
		return s.err
	}
	s.logs = append(s.logs, log)
	return nil
}

type stubOutboxEnqueuer struct {
	items []*models.DeliveryOutboxItem
}

func (s *stubOutboxEnqueuer) Enqueue(_ context.Context, item *models.DeliveryOutboxItem) error {
	s.items = append(s.items, item)
	return nil
}

func TestHandleProxyError_TenantAgentLatencyAndOutboxFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	orgID := "org-acme"
	wsID := "ws-prod"
	projID := "proj-1"
	key := &models.GatewayAPIKey{
		ID:          "gk-1",
		UserID:      "user-alice",
		OrgID:       &orgID,
		WorkspaceID: &wsID,
		ProjectID:   &projID,
	}
	tenantCtx := models.TenantContext{
		OrgID:       orgID,
		WorkspaceID: wsID,
		ProjectID:   projID,
	}

	t.Run("persists org, workspace, project, agent, and non-zero latency on failure", func(t *testing.T) {
		logStore := &stubRequestLogCreator{}
		outbox := &stubOutboxEnqueuer{}
		h := &GatewayHandler{requestLogStore: logStore, outbox: outbox}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		c.Set(middleware.TenantContextKey, tenantCtx)
		c.Set(proxy.CtxFailoverInfo, &proxy.FailoverInfo{
			Attempts:   []byte(`[{"credential_id":"c1","status_code":503}]`),
			LastStatus: 503,
			Retries:    1,
		})

		h.handleProxyError(
			c,
			errors.New("all credentials exhausted after 1 retries: upstream returned 503"),
			key,
			tenantCtx,
			"agent-sec",
			&proxy.ProxyRequest{Model: "claude-3-7-sonnet"},
			"req-err-1",
			"203.0.113.5",
			"ClaudeCode/1.0",
			"Claude Code",
			false,
			47,
		)

		require.Len(t, logStore.logs, 1)
		rl := logStore.logs[0]
		assert.Equal(t, orgID, rl.OrgID)
		assert.Equal(t, wsID, rl.WorkspaceID)
		assert.Equal(t, projID, rl.ProjectID)
		require.NotNil(t, rl.AgentID)
		assert.Equal(t, "agent-sec", *rl.AgentID)
		assert.Equal(t, 47, rl.LatencyMs)
		assert.Equal(t, 503, rl.StatusCode)
		assert.Equal(t, 1, rl.RetryCount)
		assert.Equal(t, "org-acme", logStore.tenantCtx[0].OrgID)
		assert.Equal(t, "user-alice", logStore.tenantCtx[0].UserID)
		assert.Empty(t, outbox.items)
	})

	t.Run("enqueues to outbox when primary RequestLog insert fails", func(t *testing.T) {
		logStore := &stubRequestLogCreator{err: errors.New("postgres read-only")}
		outbox := &stubOutboxEnqueuer{}
		h := &GatewayHandler{requestLogStore: logStore, outbox: outbox}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		c.Set(middleware.TenantContextKey, tenantCtx)

		h.handleProxyError(
			c,
			errors.New("upstream returned 502"),
			key,
			tenantCtx,
			"agent-fin",
			&proxy.ProxyRequest{Model: "gpt-4o"},
			"req-err-2",
			"198.51.100.1",
			"OpenCode/2.0",
			"OpenCode",
			true,
			125,
		)

		require.Len(t, outbox.items, 1)
		item := outbox.items[0]
		assert.Equal(t, models.OutboxEventRequestLog, item.EventType)
		assert.Equal(t, orgID, item.OrgID)
		assert.Equal(t, "user-alice", item.UserID)
		assert.Contains(t, item.LastError, "postgres read-only")

		decoded, err := models.DecodeOutboxRequestLog(item.Payload)
		require.NoError(t, err)
		assert.Equal(t, orgID, decoded.OrgID)
		require.NotNil(t, decoded.AgentID)
		assert.Equal(t, "agent-fin", *decoded.AgentID)
		assert.Equal(t, 125, decoded.LatencyMs)
		assert.True(t, decoded.IsStream)
		assert.True(t, decoded.ErrorMessage.Valid)
	})
}
