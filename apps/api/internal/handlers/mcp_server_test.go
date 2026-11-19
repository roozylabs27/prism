package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMCPServerCatalog struct {
	srvs []models.MCPServer
}

func (f fakeMCPServerCatalog) ListByUserID(_ context.Context, _ string) ([]models.MCPServer, error) {
	return f.srvs, nil
}

type fakeMCPToolCatalog struct {
	byServer map[string][]models.MCPTool
}

func (f fakeMCPToolCatalog) ListByServerID(_ context.Context, serverID string) ([]models.MCPTool, error) {
	return f.byServer[serverID], nil
}

type fakeMCPServerFinder struct {
	byName map[string]*models.MCPServer
}

func (f fakeMCPServerFinder) FindByUserAndName(_ context.Context, _, name string) (*models.MCPServer, error) {
	if s, ok := f.byName[name]; ok {
		return s, nil
	}
	return nil, context.DeadlineExceeded
}
func (f fakeMCPServerFinder) FindByID(_ context.Context, id, _ string) (*models.MCPServer, error) {
	for _, s := range f.byName {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, context.DeadlineExceeded
}
func (f fakeMCPServerFinder) UpdateStatus(_ context.Context, _, _ string) error { return nil }
func (f fakeMCPServerFinder) GetServerWithTools(_ context.Context, s *models.MCPServer) (*models.MCPServerWithTools, error) {
	return &models.MCPServerWithTools{Server: *s}, nil
}

type stubDelegatedOrchestrator struct {
	res *service.OrchestrationResult
	err error
	req *proxy.ProxyRequest
}

func (s *stubDelegatedOrchestrator) ExecuteChatCompletions(
	_ *gin.Context,
	_ context.Context,
	req *proxy.ProxyRequest,
	_ *models.GatewayAPIKey,
	_ models.TenantContext,
	_, _, _, _, _, _, _ string,
) (*service.OrchestrationResult, error) {
	s.req = req
	return s.res, s.err
}

func callMCPJSONRPC(r *gin.Engine, payload map[string]interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMCPServerHandler_InitializeToolsListAndExecutionGovernance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")

	var createIssueCalls atomic.Int32
	var deleteRepoCalls atomic.Int32
	upstream := server.NewMCPServer("github-mcp", "1.0.0")
	upstream.AddTool(mcp.NewTool("create_issue", mcp.WithDescription("Create issue")), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		createIssueCalls.Add(1)
		return mcp.NewToolResultText(`{"issue_id":42,"status":"open"}`), nil
	})
	upstream.AddTool(mcp.NewTool("delete_repo", mcp.WithDescription("Danger delete repo")), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		deleteRepoCalls.Add(1)
		return mcp.NewToolResultText(`{"deleted":true}`), nil
	})
	ts := server.NewTestStreamableHTTPServer(upstream)
	defer ts.Close()

	orgID := "org_agency"
	gk := &models.GatewayAPIKey{ID: "gk_mcp", UserID: "alice", OrgID: &orgID}
	agent := &models.Agent{
		ID:                "ag_mcp",
		Name:              "triage-agent",
		Enabled:           true,
		AllowedMCPServers: []string{"github"},
		AllowedTools:      []string{"*", "!delete_repo"},
	}

	githubSrv := models.MCPServer{ID: "s_gh", UserID: "alice", Name: "github", DisplayName: "GitHub", EndpointURL: ts.URL, TransportType: "http", Enabled: true}
	stripeSrv := models.MCPServer{ID: "s_st", UserID: "alice", Name: "stripe", DisplayName: "Stripe", EndpointURL: ts.URL, TransportType: "http", Enabled: true}

	mcpGW := proxy.NewMCPGateway(fakeMCPServerFinder{byName: map[string]*models.MCPServer{"github": &githubSrv, "stripe": &stripeSrv}}, nil)
	adm := proxy.NewAdmissionController(nil, nil, nil, nil)
	orch := &stubDelegatedOrchestrator{
		res: &service.OrchestrationResult{
			HTTPStatus: http.StatusOK,
			Response: &proxy.ProviderResponse{
				Model:   "gpt-4o-mini",
				Choices: []proxy.Choice{{Index: 0, Message: map[string]interface{}{"role": "assistant", "content": `{"category":"billing"}`}}},
				Usage:   proxy.Usage{PromptTokens: 120, CompletionTokens: 18, TotalTokens: 138},
			},
			RequestLog: &models.RequestLog{
				Model:        "gpt-4o-mini",
				ProviderType: "openai",
				InputTokens:  120,
				OutputTokens: 18,
				TotalTokens:  138,
				CostUSD:      0.00042,
				LatencyMs:    84,
			},
		},
	}

	h := NewMCPServerHandler(
		fakeMCPServerCatalog{srvs: []models.MCPServer{githubSrv, stripeSrv}},
		fakeMCPToolCatalog{byServer: map[string][]models.MCPTool{
			"s_gh": {
				{ID: "t1", MCPServerID: "s_gh", Name: "create_issue", Description: "Create issue", Enabled: true},
				{ID: "t2", MCPServerID: "s_gh", Name: "delete_repo", Description: "Danger", Enabled: true},
			},
			"s_st": {
				{ID: "t3", MCPServerID: "s_st", Name: "refund_charge", Description: "Refund", Enabled: true},
			},
		}},
		mcpGW,
		orch,
		adm,
		nil,
		fixedRoleResolver("developer"),
		nil,
		"",
	)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("gatewayKey", gk)
		c.Set("agentObject", agent)
		c.Next()
	})
	r.POST("/v1/mcp", h.Handle)

	// 1. initialize
	wInit := callMCPJSONRPC(r, map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "initialize"})
	require.Equal(t, http.StatusOK, wInit.Code)
	assert.Contains(t, wInit.Body.String(), "prism-mcp-gateway")

	// 2. tools/list: exposes delegate_ai_task and github__create_issue ONLY (filters !delete_repo & disallowed stripe)
	wList := callMCPJSONRPC(r, map[string]interface{}{"jsonrpc": "2.0", "id": 2, "method": "tools/list"})
	require.Equal(t, http.StatusOK, wList.Code)
	var listResp struct {
		Result struct {
			Tools []mcpToolDefinition `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(wList.Body.Bytes(), &listResp))
	var names []string
	for _, tl := range listResp.Result.Tools {
		names = append(names, tl.Name)
	}
	assert.Contains(t, names, "delegate_ai_task")
	assert.Contains(t, names, "github__create_issue")
	assert.NotContains(t, names, "github__delete_repo")
	assert.NotContains(t, names, "stripe__refund_charge")

	// 3. Authorized upstream MCP tool call succeeds
	wOK := callMCPJSONRPC(r, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "github__create_issue",
			"arguments": map[string]interface{}{"title": "hello"},
		},
	})
	require.Equal(t, http.StatusOK, wOK.Code)
	assert.Contains(t, wOK.Body.String(), "issue_id")
	assert.Equal(t, int32(1), createIssueCalls.Load())

	// 4. Unauthorized tool call (!delete_repo) & unauthorized MCP server (stripe) rejected without reaching upstream
	wDenyTool := callMCPJSONRPC(r, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      4,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": "github__delete_repo", "arguments": map[string]interface{}{}},
	})
	assert.Contains(t, wDenyTool.Body.String(), "-32001")
	assert.Equal(t, int32(0), deleteRepoCalls.Load())

	wDenySrv := callMCPJSONRPC(r, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      5,
		"method":  "tools/call",
		"params":  map[string]interface{}{"name": "stripe__refund_charge", "arguments": map[string]interface{}{}},
	})
	assert.Contains(t, wDenySrv.Body.String(), "-32001")

	// 5. delegate_ai_task invokes Smart Router & returns explicit model disclosure
	wDel := callMCPJSONRPC(r, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      6,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "delegate_ai_task",
			"arguments": map[string]interface{}{
				"task_type":     "classification",
				"prompt":        "Invoice #104 payment question",
				"output_format": "json",
			},
		},
	})
	require.Equal(t, http.StatusOK, wDel.Code)
	require.NotNil(t, orch.req)
	assert.Equal(t, models.SmartRouterModel, orch.req.Model)
	assert.Contains(t, wDel.Body.String(), "gpt-4o-mini")
	assert.Contains(t, wDel.Body.String(), "0.00042")

	// 6. delegate_ai_task denied when orchestrator enforces hard budget cap -> -32002
	orch.res = &service.OrchestrationResult{
		Denied:       true,
		HTTPStatus:   http.StatusTooManyRequests,
		ErrorCode:    "budget_exceeded_error",
		ErrorMessage: "monthly budget exceeded",
	}
	wBudget := callMCPJSONRPC(r, map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      7,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "delegate_ai_task",
			"arguments": map[string]interface{}{"task_type": "summarization", "prompt": "Summarize"},
		},
	})
	assert.Contains(t, wBudget.Body.String(), "-32002")
	assert.Contains(t, wBudget.Body.String(), "budget_exceeded_error")
}
