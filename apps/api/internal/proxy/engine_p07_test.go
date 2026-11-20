package proxy

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveEngineOrgID(t *testing.T) {
	orgFromKey := "org-from-key"
	keyWithOrg := &models.GatewayAPIKey{ID: "gk-1", UserID: "u1", OrgID: &orgFromKey}
	keyNoOrg := &models.GatewayAPIKey{ID: "gk-2", UserID: "u1"}

	t.Run("prefers tenant DB context OrgID over gateway key", func(t *testing.T) {
		ctx := database.WithTenantContext(context.Background(), database.TenantDBContext{
			UserID: "u1",
			OrgID:  "org-from-ctx",
		})
		assert.Equal(t, "org-from-ctx", resolveEngineOrgID(ctx, keyWithOrg))
	})

	t.Run("falls back to gateway key OrgID when tenant ctx empty", func(t *testing.T) {
		assert.Equal(t, "org-from-key", resolveEngineOrgID(context.Background(), keyWithOrg))
	})

	t.Run("returns empty when neither carries OrgID", func(t *testing.T) {
		assert.Equal(t, "", resolveEngineOrgID(context.Background(), keyNoOrg))
		assert.Equal(t, "", resolveEngineOrgID(context.Background(), nil))
	})
}

func TestSummarizeToolCalls(t *testing.T) {
	tools, servers := summarizeToolCalls([]ToolCallRecord{
		{Name: "linear__create_issue"},
		{Name: "linear__update_issue"},
		{Name: "github__open_pr"},
		{Name: "get_weather"},
		{Name: "get_weather"}, // duplicate tool must be deduped
		{Name: ""},
		{Name: "__leading_double_underscore_only"},
	})

	assert.Equal(t, []string{
		"linear__create_issue",
		"linear__update_issue",
		"github__open_pr",
		"get_weather",
		"__leading_double_underscore_only",
	}, tools)
	assert.Equal(t, []string{"linear", "github"}, servers)

	nilTools, nilServers := summarizeToolCalls(nil)
	assert.Nil(t, nilTools)
	assert.Nil(t, nilServers)
}

func TestEngine_SetMCPRepositoriesIsDeprecatedNoOp(t *testing.T) {
	e := &Engine{}
	assert.NotPanics(t, func() {
		e.SetMCPRepositories(nil, nil)
		e.persistToolCalls("req-1", "user-1", []ToolCallRecord{{Name: "linear__list_issues"}})
	})
}

func TestEngine_ProxyStream_RecordsTTFTAndOrgID(t *testing.T) {
	handler, _ := sseServer(func(w http.ResponseWriter, _ int32) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(25 * time.Millisecond)
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi \",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"linear__get_issue\",\"arguments\":\"{}\"}}]}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	f := newFailoverFixture(t, 1, 0, handler)
	orgID := "org-stream-test"
	f.key.OrgID = &orgID

	c, _ := f.context(t, 10*time.Second)
	c.Request = c.Request.WithContext(database.WithTenantContext(c.Request.Context(), database.TenantDBContext{
		UserID: "u1",
		OrgID:  orgID,
	}))

	logEntry, err := f.engine.ProxyStream(c, streamRequest(), f.key)
	require.NoError(t, err)
	require.NotNil(t, logEntry)
	assert.Equal(t, orgID, logEntry.OrgID)
	assert.GreaterOrEqual(t, logEntry.TTFTMs, 20)
	assert.Equal(t, []string{"linear__get_issue"}, logEntry.ToolsInvoked)
	assert.Equal(t, []string{"linear"}, logEntry.MCPServersCalled)
}
