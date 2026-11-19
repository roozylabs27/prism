package handlers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticKeyLookup map[string]*models.GatewayAPIKey

func (s staticKeyLookup) FindByKeyHash(_ context.Context, keyHash string) (*models.GatewayAPIKey, error) {
	if k, ok := s[keyHash]; ok {
		return k, nil
	}
	return nil, errors.New("key not found")
}

type staticAgentLookup map[string]*models.Agent

func (s staticAgentLookup) FindForUser(_ context.Context, userID, ref string) (*models.Agent, error) {
	if ag, ok := s[userID+":"+ref]; ok {
		return ag, nil
	}
	return nil, errors.New("not found")
}

type failingRateLimiter struct{}

func (failingRateLimiter) Allow(_ context.Context, _ string, _ int) (goredis.Decision, error) {
	return goredis.Decision{}, errors.New("redis connection refused")
}

// TestControlPlane_ZeroExternalDeps_E2EJourneys exercises the full middleware
// and handler boundary chain without requiring Postgres or Redis, verifying
// workspace/project key pinning, fail-closed Redis rate limiting, tool/resource
// deny rules, governed MCP execution, and delegated task disclosure.
func TestControlPlane_ZeroExternalDeps_E2EJourneys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")

	orgID := "org_agency"
	wsAlpha := "ws_client_alpha"
	projAlpha := "proj_alpha_1"
	rawKey, err := utils.NewGatewayKey()
	require.NoError(t, err)
	keyPrefix := utils.GatewayKeyDisplayPrefix(rawKey)
	keyHash := utils.HashSHA256(rawKey)
	assert.NotEmpty(t, keyPrefix)

	gk := &models.GatewayAPIKey{
		ID:          "gk_alpha",
		UserID:      "agency_owner",
		OrgID:       &orgID,
		WorkspaceID: &wsAlpha,
		ProjectID:   &projAlpha,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Enabled:     true,
		RateLimit:   100,
	}

	restrictedAgent := &models.Agent{
		ID:                "ag_support",
		UserID:            "agency_owner",
		Name:              "client-alpha-bot",
		Enabled:           true,
		AllowedModels:     []string{"gpt-4o-mini", "!gpt-4o"},
		AllowedMCPServers: []string{"crm"},
		AllowedTools:      []string{"*", "!crm__drop_records", "!drop_records"},
		AllowedResources:  []string{"*", "!payroll_db"},
	}

	var dropCalls atomic.Int32
	mcpUpstream := server.NewMCPServer("crm-mcp", "1.0.0")
	mcpUpstream.AddTool(mcp.NewTool("lookup_ticket"), func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText(`{"ticket":"T-100","status":"open"}`), nil
	})
	mcpUpstream.AddTool(mcp.NewTool("drop_records"), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		dropCalls.Add(1)
		return mcp.NewToolResultText(`{"dropped":true}`), nil
	})
	mcpTS := server.NewTestStreamableHTTPServer(mcpUpstream)
	defer mcpTS.Close()

	crmSrv := models.MCPServer{ID: "srv_crm", UserID: "agency_owner", Name: "crm", EndpointURL: mcpTS.URL, TransportType: "http", Enabled: true}
	mcpGW := proxy.NewMCPGateway(fakeMCPServerFinder{byName: map[string]*models.MCPServer{"crm": &crmSrv}}, nil)
	toolExec := &fakeToolExecutor{}
	resExec := &fakeResourceExecutor{}

	adm := proxy.NewAdmissionController(nil, nil, nil, nil)
	toolGWHandler := &ToolGatewayHandler{gateway: toolExec}
	toolGWHandler.SetGovernance(adm, nil, fixedRoleResolver("developer"))
	resGWHandler := NewResourceGatewayHandler(resExec, nil, nil, "")
	resGWHandler.SetGovernance(adm, nil, fixedRoleResolver("developer"))

	orch := &stubDelegatedOrchestrator{
		res: &service.OrchestrationResult{
			HTTPStatus: http.StatusOK,
			Response:   &proxy.ProviderResponse{Model: "gpt-4o-mini", Choices: []proxy.Choice{{Index: 0, Message: map[string]interface{}{"role": "assistant", "content": "billing"}}}},
			RequestLog: &models.RequestLog{Model: "gpt-4o-mini", ProviderType: "openai", InputTokens: 50, OutputTokens: 5, TotalTokens: 55, CostUSD: 0.00012, LatencyMs: 42},
		},
	}
	mcpSrvHandler := NewMCPServerHandler(
		fakeMCPServerCatalog{srvs: []models.MCPServer{crmSrv}},
		fakeMCPToolCatalog{byServer: map[string][]models.MCPTool{
			"srv_crm": {
				{ID: "t1", MCPServerID: "srv_crm", Name: "lookup_ticket", Enabled: true},
				{ID: "t2", MCPServerID: "srv_crm", Name: "drop_records", Enabled: true},
			},
		}},
		mcpGW, orch, adm, nil, fixedRoleResolver("developer"), nil, "",
	)

	r := gin.New()
	r.Use(middleware.CorrelationMiddleware())
	v1 := r.Group("/v1")
	v1.Use(middleware.GatewayAuthMiddleware(staticKeyLookup{keyHash: gk}))
	v1.Use(middleware.TenantMiddleware(nil))
	v1.Use(middleware.AgentPolicyMiddleware(staticAgentLookup{"agency_owner:ag_support": restrictedAgent, "agency_owner:client-alpha-bot": restrictedAgent}))
	v1.POST("/tools/:toolName/execute", toolGWHandler.ExecuteTool)
	v1.POST("/resources/:resourceName/query", resGWHandler.ExecuteQuery)
	v1.POST("/mcp", mcpSrvHandler.Handle)

	callWith := func(method, path string, headers map[string]string, body interface{}) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+rawKey)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. Spoofed X-Prism-Workspace-ID against workspace-pinned key must return 403
	wSpoofWS := callWith(http.MethodPost, "/v1/tools/search/execute", map[string]string{"X-Prism-Workspace-ID": "ws_client_beta"}, map[string]interface{}{"args": map[string]interface{}{}})
	assert.Equal(t, http.StatusForbidden, wSpoofWS.Code)
	assert.Contains(t, wSpoofWS.Body.String(), "TENANT_SECURITY_ERROR")

	// 2. Spoofed X-Prism-Project-ID against project-pinned key must return 403
	wSpoofProj := callWith(http.MethodPost, "/v1/tools/search/execute", map[string]string{"X-Prism-Workspace-ID": wsAlpha, "X-Prism-Project-ID": "proj_beta"}, map[string]interface{}{"args": map[string]interface{}{}})
	assert.Equal(t, http.StatusForbidden, wSpoofProj.Code)

	// 3. Matching headers + agent permission enforcement on Tool & Resource gateways
	wResDeny := callWith(http.MethodPost, "/v1/resources/payroll_db/query", map[string]string{"X-Prism-Agent-ID": "ag_support"}, map[string]interface{}{"args": map[string]interface{}{}})
	assert.Equal(t, http.StatusForbidden, wResDeny.Code)
	assert.Equal(t, 0, resExec.calls)

	wToolOK := callWith(http.MethodPost, "/v1/tools/search_kb/execute", map[string]string{"X-Prism-Agent-ID": "ag_support"}, map[string]interface{}{"args": map[string]interface{}{}})
	assert.Equal(t, http.StatusOK, wToolOK.Code)
	assert.Equal(t, 1, toolExec.calls)

	// 4. Governed MCP calls & delegate_ai_task
	wMCPOK := callWith(http.MethodPost, "/v1/mcp", map[string]string{"X-Prism-Agent-ID": "ag_support"}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]interface{}{"name": "crm__lookup_ticket", "arguments": map[string]interface{}{"id": "T-100"}},
	})
	assert.Equal(t, http.StatusOK, wMCPOK.Code)
	assert.Contains(t, wMCPOK.Body.String(), "T-100")

	wMCPDeny := callWith(http.MethodPost, "/v1/mcp", map[string]string{"X-Prism-Agent-ID": "ag_support"}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]interface{}{"name": "crm__drop_records", "arguments": map[string]interface{}{}},
	})
	assert.Contains(t, wMCPDeny.Body.String(), "-32001")
	assert.Equal(t, int32(0), dropCalls.Load())

	wDel := callWith(http.MethodPost, "/v1/mcp", map[string]string{"X-Prism-Agent-ID": "ag_support"}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]interface{}{"name": "delegate_ai_task", "arguments": map[string]interface{}{"task_type": "classification", "prompt": "Need invoice help"}},
	})
	assert.Equal(t, http.StatusOK, wDel.Code)
	assert.Contains(t, wDel.Body.String(), "gpt-4o-mini")

	// 5. Fail-closed rate limiter rejects with 503 when failClosed=true
	rFail := gin.New()
	vFail := rFail.Group("/v1")
	vFail.Use(middleware.GatewayAuthMiddleware(staticKeyLookup{keyHash: gk}))
	vFail.Use(middleware.TenantMiddleware(nil))
	vFail.Use(middleware.GatewayRateLimitMiddlewareWithPolicy(failingRateLimiter{}, 100, true))
	vFail.POST("/mcp", mcpSrvHandler.Handle)
	reqFail := httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	reqFail.Header.Set("Authorization", "Bearer "+rawKey)
	wFail := httptest.NewRecorder()
	rFail.ServeHTTP(wFail, reqFail)
	assert.Equal(t, http.StatusServiceUnavailable, wFail.Code)
	assert.Contains(t, wFail.Body.String(), "RATE_LIMITER_UNAVAILABLE")
}

// TestControlPlane_PostgresRedis_FullStackE2EJourneys exercises real Postgres +
// Redis + Engine + Router + AdmissionController + BudgetManager + Gateway + MCP
// against live in-process fake AI providers & MCP servers. Skips cleanly when
// DATABASE_URL or REDIS_URL is not set.
func TestControlPlane_PostgresRedis_FullStackE2EJourneys(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL not set")
	}
	db := migratedDB(t)

	opts, err := goredislib.ParseURL(redisURL)
	require.NoError(t, err)
	rdb := goredislib.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")

	const encKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	ctx := context.Background()

	// Seed users, orgs, workspaces, projects, memberships & roles.
	_, err = db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('alice', 'Alice Agency', 'alice@agency.local'), ('bob', 'Bob Other', 'bob@other.local');
		INSERT INTO organizations (id, name, slug, plan_tier) VALUES ('org_a', 'Agency Org', 'agency-org', 'enterprise'), ('org_b', 'Other Org', 'other-org', 'free');
		INSERT INTO workspaces (id, org_id, name, slug) VALUES ('ws_client1', 'org_a', 'Client 1', 'client-1');
		INSERT INTO projects (id, workspace_id, name, slug) VALUES ('proj_c1', 'ws_client1', 'Bot Project', 'bot-proj');
		INSERT INTO organization_members (id, org_id, user_id, role_id)
		SELECT 'om_a', 'org_a', 'alice', id FROM roles WHERE slug = 'owner' AND is_system LIMIT 1;
		INSERT INTO organization_members (id, org_id, user_id, role_id)
		SELECT 'om_b', 'org_b', 'bob', id FROM roles WHERE slug = 'owner' AND is_system LIMIT 1;
	`)
	require.NoError(t, err)

	// Fake OpenAI-compatible upstream supporting primary 500 -> secondary failover,
	// SSE streaming, and distinct gpt-4o vs gpt-4o-mini responses.
	var failoverAttempts atomic.Int32
	aiUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		authHeader := r.Header.Get("Authorization")

		// Simulate primary credential failing on fail-model so secondary credential succeeds.
		if body.Model == "fail-model" && authHeader == "Bearer sk-alice-primary" {
			failoverAttempts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"upstream crash","type":"server_error"}}`))
			return
		}

		if body.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(fmt.Sprintf("data: {\"id\":\"cmpl-s\",\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"stream \"}}]}\n\n", body.Model)))
			_, _ = w.Write([]byte(fmt.Sprintf("data: {\"id\":\"cmpl-s\",\"model\":%q,\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n", body.Model)))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-e2e",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   body.Model,
			"choices": []map[string]interface{}{{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": "answer from " + body.Model},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{"prompt_tokens": 100, "completion_tokens": 25, "total_tokens": 125},
		})
	}))
	defer aiUpstream.Close()

	// Providers + BYOK credentials + Models
	var provAliceID, provBobID string
	require.NoError(t, db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy) VALUES ('alice', 'AliceOpenAI', 'alice-openai', $1, 'openai', true, 'fallback') RETURNING id`, aiUpstream.URL).Scan(&provAliceID))
	require.NoError(t, db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy) VALUES ('bob', 'BobOpenAI', 'bob-openai', $1, 'openai', true, 'round_robin') RETURNING id`, aiUpstream.URL).Scan(&provBobID))

	encPrimary, _ := utils.EncryptAES256GCM("sk-alice-primary", encKey)
	encSecondary, _ := utils.EncryptAES256GCM("sk-alice-secondary", encKey)
	encBob, _ := utils.EncryptAES256GCM("sk-bob-secret", encKey)
	var credPrimaryID, credSecondaryID string
	require.NoError(t, db.QueryRow(`INSERT INTO credentials (provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, priority, enabled, status) VALUES ($1, 'alice', 'org_a', 'primary', $2, 'sk-', 'sk-***1', 'api_key', 1, true, 'active') RETURNING id`, provAliceID, encPrimary).Scan(&credPrimaryID))
	require.NoError(t, db.QueryRow(`INSERT INTO credentials (provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, priority, enabled, status) VALUES ($1, 'alice', 'org_a', 'secondary', $2, 'sk-', 'sk-***2', 'api_key', 2, true, 'active') RETURNING id`, provAliceID, encSecondary).Scan(&credSecondaryID))
	_, err = db.Exec(`INSERT INTO credentials (provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, priority, enabled, status) VALUES ($1, 'bob', 'org_b', 'bob-key', $2, 'sk-', 'sk-***b', 'api_key', 1, true, 'active')`, provBobID, encBob)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = rdb.Del(ctx, "credential:"+credPrimaryID+":50x_count", "credential:"+credPrimaryID+":cooldown", "credential:"+credSecondaryID+":50x_count", "credential:"+credSecondaryID+":cooldown").Err()
	})

	_, err = db.Exec(`
		INSERT INTO models (provider_id, name, slug, display_name, enabled, context_window, coding_score, reasoning_score, writing_score, speed_score, quality_score, input_price_per_1m, output_price_per_1m, supports_tools)
		VALUES
			($1, 'gpt-4o', 'gpt-4o', 'GPT-4o', true, 128000, 0.95, 0.95, 0.95, 0.75, 0.98, 2.50, 10.00, true),
			($1, 'gpt-4o-mini', 'gpt-4o-mini', 'GPT-4o Mini', true, 128000, 0.80, 0.78, 0.82, 0.96, 0.82, 0.15, 0.60, true),
			($1, 'fail-model', 'fail-model', 'Fail Model', true, 128000, 0.50, 0.50, 0.50, 0.50, 0.50, 0.10, 0.20, false),
			($2, 'bob-private-model', 'bob-private-model', 'Bob Private', true, 32000, 0.9, 0.9, 0.9, 0.9, 0.9, 1.0, 2.0, false);
	`, provAliceID, provBobID)
	require.NoError(t, err)

	// Routing policy (cost-first) for Alice
	_, err = db.Exec(`INSERT INTO routing_policies (user_id, name, weights, constraints, enabled, is_default) VALUES ('alice', 'cost-saver', '{"task_match":0.2,"quality":0.1,"cost":0.6,"speed":0.1}', '{}', true, false)`)
	require.NoError(t, err)

	// Agent for Alice (allows only gpt-4o-mini + !gpt-4o, allows mcp server "crm", blocks !drop_records)
	agentRepo := repository.NewAgentRepository(db)
	aliceAgent := &models.Agent{
		UserID:            "alice",
		Name:              "agency-worker",
		DisplayName:       "Agency Worker",
		AgentType:         "automation",
		AllowedModels:     []string{"gpt-4o-mini", "fail-model", "!gpt-4o"},
		AllowedMCPServers: []string{"crm"},
		AllowedTools:      []string{"*", "!drop_records"},
		Status:            "active",
		Enabled:           true,
	}
	require.NoError(t, agentRepo.Create(ctx, aliceAgent))

	// Upstream MCP Server for Alice
	var mcpAllowedCalls, mcpDeniedCalls atomic.Int32
	upstreamMCP := server.NewMCPServer("crm-live", "1.0.0")
	upstreamMCP.AddTool(mcp.NewTool("get_lead"), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mcpAllowedCalls.Add(1)
		return mcp.NewToolResultText(`{"lead":"Acme Corp","stage":"qualified"}`), nil
	})
	upstreamMCP.AddTool(mcp.NewTool("drop_records"), func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mcpDeniedCalls.Add(1)
		return mcp.NewToolResultText(`{"dropped":true}`), nil
	})
	mcpServerHTTP := server.NewTestStreamableHTTPServer(upstreamMCP)
	defer mcpServerHTTP.Close()

	mcpServerRepo := repository.NewMCPServerRepository(db)
	mcpToolRepo := repository.NewMCPToolRepository(db)
	mcpSrv := &models.MCPServer{UserID: "alice", Name: "crm", DisplayName: "CRM", Type: "remote", TransportType: "http", EndpointURL: mcpServerHTTP.URL, Args: []string{}, Enabled: true}
	require.NoError(t, mcpServerRepo.Create(ctx, mcpSrv))
	require.NoError(t, mcpToolRepo.BatchUpsert(ctx, mcpSrv.ID, []models.MCPTool{
		{MCPServerID: mcpSrv.ID, Name: "get_lead", Description: "Fetch CRM lead", Enabled: true},
		{MCPServerID: mcpSrv.ID, Name: "drop_records", Description: "Drop CRM table", Enabled: true},
	}))

	// Gateway key for Alice
	aliceRawKey, err := utils.NewGatewayKey()
	require.NoError(t, err)
	alicePrefix := utils.GatewayKeyDisplayPrefix(aliceRawKey)
	aliceHash := utils.HashSHA256(aliceRawKey)
	orgA := "org_a"
	ws1 := "ws_client1"
	proj1 := "proj_c1"
	gkAlice := &models.GatewayAPIKey{
		UserID:      "alice",
		OrgID:       &orgA,
		WorkspaceID: &ws1,
		ProjectID:   &proj1,
		Name:        "Client 1 Key",
		KeyHash:     aliceHash,
		KeyPrefix:   alicePrefix,
		Enabled:     true,
		RateLimit:   100,
	}
	gkRepo := repository.NewGatewayKeyRepository(db)
	require.NoError(t, gkRepo.Create(ctx, gkAlice))

	// Repositories & Engine wiring (matching main.go production wiring)
	modelRepo := repository.NewModelRepository(db)
	providerRepo := repository.NewProviderRepository(db)
	credRepo := repository.NewCredentialRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	requestLogRepo := repository.NewRequestLogRepository(db)
	budgetRepo := repository.NewBudgetRepository(db)
	routingPolicyRepo := repository.NewRoutingPolicyRepository(db)
	decisionRepo := repository.NewRoutingDecisionRepository(db)
	payloadRepo := repository.NewPayloadRepository(db)
	toolInvRepo := repository.NewToolInvocationRepository(db)
	govRepo := repository.NewGovernancePolicyRepository(db)
	auditRepo := repository.NewAuditTrailRepository(db)
	quotaRepo := repository.NewQuotaRepository(db)
	rbacRepo := repository.NewRBACRepository(db)
	accountRepo := repository.NewAccountRepository(db)
	pricingRepo := repository.NewModelPricingRepository(db)

	cooldown := goredis.NewCooldownStore(rdb)
	telemetryStore := goredis.NewModelTelemetryStore(rdb)
	budgetMgr := proxy.NewBudgetManager(budgetRepo)
	healthStore := proxy.NewProviderHealthStore(requestLogRepo, time.Minute)
	router := proxy.NewRouter(modelRepo, providerRepo, credRepo, settingRepo, healthStore)
	ruleRepo := repository.NewRoutingRuleRepository(db)
	require.NoError(t, ruleRepo.Create(ctx, &models.RoutingRule{
		UserID: "alice", ModelPattern: "gpt-4o*", ProviderID: &provAliceID, Priority: 1, Enabled: true,
	}))
	router.SetRuleSource(ruleRepo)
	engine := proxy.NewEngine(router, credRepo, cooldown, telemetryStore, nil, encKey, 2, 60, budgetMgr, routingPolicyRepo, decisionRepo, payloadRepo, toolInvRepo)
	defer func() { _ = engine.Shutdown(context.Background()) }()

	admissionCtrl := proxy.NewAdmissionController(proxy.NewRBACEngine(govRepo), proxy.NewAgentGovernanceEngine(agentRepo), proxy.NewCachedQuotaEvaluator(quotaRepo, proxy.DefaultQuotaCacheTTL), budgetMgr)
	admissionCtrl.SetAgentSpendFinder(requestLogRepo)
	auditRec := proxy.NewAuditRecorder(auditRepo)
	orchestrator := service.NewExecutionOrchestrator(engine, admissionCtrl, gkRepo, requestLogRepo, nil, pricingRepo, auditRec)
	orchestrator.SetBudgetManager(budgetMgr)

	gwHandler := NewGatewayHandler(engine, gkRepo, requestLogRepo, nil, pricingRepo, nil, nil, nil, auditRec, modelRepo, agentRepo, orchestrator)
	gwHandler.SetMemberRoleResolver(rbacRepo)
	gwHandler.SetRoutingPolicyFinder(routingPolicyRepo)
	mcpGW := proxy.NewMCPGateway(mcpServerRepo, mcpToolRepo)
	mcpSrvHandler := NewMCPServerHandler(mcpServerRepo, mcpToolRepo, mcpGW, orchestrator, admissionCtrl, auditRec, rbacRepo, routingPolicyRepo, encKey)

	app := gin.New()
	app.Use(middleware.CorrelationMiddleware())
	v1 := app.Group("/v1")
	v1.Use(middleware.GatewayAuthMiddleware(gkRepo))
	v1.Use(middleware.TenantMiddleware(accountRepo))
	v1.Use(middleware.AgentPolicyMiddleware(agentRepo))
	v1.POST("/chat/completions", gwHandler.ChatCompletions)
	v1.POST("/mcp", mcpSrvHandler.Handle)

	doReq := func(path string, headers map[string]string, body interface{}) *httptest.ResponseRecorder {
		b, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+aliceRawKey)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		app.ServeHTTP(w, req)
		return w
	}

	// 1. BYOK isolation: Alice cannot route to Bob's private model
	wBYOK := doReq("/v1/chat/completions", nil, map[string]interface{}{
		"model":    "bob-private-model",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})
	assert.NotEqual(t, http.StatusOK, wBYOK.Code, "Alice must not use Bob's credentials")

	// 2. Smart Router (prism-auto) + X-Routing-Policy + Agent AllowedModels enforcement
	wSmart := doReq("/v1/chat/completions", map[string]string{
		"X-Prism-Agent-ID":  aliceAgent.ID,
		"X-Routing-Policy":  "cost-saver",
	}, map[string]interface{}{
		"model":    "prism-auto",
		"messages": []map[string]string{{"role": "user", "content": "Summarize quarterly numbers"}},
	})
	require.Equal(t, http.StatusOK, wSmart.Code, wSmart.Body.String())
	assert.Equal(t, "gpt-4o-mini", wSmart.Header().Get("X-Prism-Model"))
	assert.Equal(t, "cost-saver", wSmart.Header().Get("X-Prism-Routing-Policy"))

	// 3. Streaming SSE + [DONE]
	wStream := doReq("/v1/chat/completions", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"model":    "gpt-4o-mini",
		"stream":   true,
		"messages": []map[string]string{{"role": "user", "content": "stream hello"}},
	})
	require.Equal(t, http.StatusOK, wStream.Code, wStream.Body.String())
	var sseText strings.Builder
	scanner := bufio.NewScanner(bytes.NewReader(wStream.Body.Bytes()))
	for scanner.Scan() {
		sseText.WriteString(scanner.Text())
	}
	assert.Contains(t, sseText.String(), "stream ")
	assert.Contains(t, sseText.String(), "[DONE]")

	// 4. Primary credential failure (500) -> automatic failover to secondary credential
	wFailover := doReq("/v1/chat/completions", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"model":    "fail-model",
		"messages": []map[string]string{{"role": "user", "content": "test failover"}},
	})
	require.Equal(t, http.StatusOK, wFailover.Code, wFailover.Body.String())
	assert.Equal(t, int32(1), failoverAttempts.Load())
	assert.Equal(t, "1", wFailover.Header().Get("X-Prism-Failovers"))

	// 5. Authorized & Unauthorized MCP calls via /v1/mcp
	wMCPOK := doReq("/v1/mcp", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 10, "method": "tools/call",
		"params": map[string]interface{}{"name": "crm__get_lead", "arguments": map[string]interface{}{}},
	})
	require.Equal(t, http.StatusOK, wMCPOK.Code)
	assert.Contains(t, wMCPOK.Body.String(), "Acme Corp")
	assert.Equal(t, int32(1), mcpAllowedCalls.Load())

	wMCPDeny := doReq("/v1/mcp", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 11, "method": "tools/call",
		"params": map[string]interface{}{"name": "crm__drop_records", "arguments": map[string]interface{}{}},
	})
	assert.Contains(t, wMCPDeny.Body.String(), "-32001")
	assert.Equal(t, int32(0), mcpDeniedCalls.Load())

	// 6. Delegated AI task via /v1/mcp -> Smart Router -> real fake provider -> model disclosure
	wDel := doReq("/v1/mcp", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 12, "method": "tools/call",
		"params": map[string]interface{}{
			"name":      "delegate_ai_task",
			"arguments": map[string]interface{}{"task_type": "extraction", "prompt": "Extract total=$49", "routing_policy": "cost-saver"},
		},
	})
	require.Equal(t, http.StatusOK, wDel.Code, wDel.Body.String())
	assert.Equal(t, "gpt-4o-mini", wDel.Header().Get("X-Prism-Model"))
	var rpcDel struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(wDel.Body.Bytes(), &rpcDel))
	require.NotEmpty(t, rpcDel.Result.Content)
	var delPayload struct {
		Output     string `json:"output"`
		Disclosure struct {
			ExecutedModel string `json:"executed_model"`
		} `json:"disclosure"`
	}
	require.NoError(t, json.Unmarshal([]byte(rpcDel.Result.Content[0].Text), &delPayload))
	assert.Equal(t, "answer from gpt-4o-mini", delPayload.Output)
	assert.Equal(t, "gpt-4o-mini", delPayload.Disclosure.ExecutedModel)

	// 7. Workspace spoofing rejected on pinned key
	wCrossWS := doReq("/v1/mcp", map[string]string{"X-Prism-Workspace-ID": "ws_client2"}, map[string]interface{}{
		"jsonrpc": "2.0", "id": 13, "method": "tools/list",
	})
	assert.Equal(t, http.StatusForbidden, wCrossWS.Code)

	// 8. Hard budget limit enforced -> 429 denial + denied audit record
	require.NoError(t, requestLogRepo.Create(ctx, &models.RequestLog{
		RequestID:       "req-prior-spend",
		OrgID:           "org_a",
		WorkspaceID:     "ws_client1",
		ProjectID:       "proj_c1",
		GatewayAPIKeyID: &gkAlice.ID,
		Model:           "gpt-4o-mini",
		StatusCode:      200,
		LatencyMs:       25,
		InputTokens:     5000,
		OutputTokens:    1000,
		TotalTokens:     6000,
		CostUSD:         0.05,
	}))
	budgetMgr.InvalidateSpendCache("alice")
	require.NoError(t, budgetRepo.Create(ctx, &models.Budget{
		UserID: "alice", Name: "Tight Hard Cap", MonthlyLimit: 1.0, DailyLimit: 0.01,
		HardLimit: true, WarningThreshold: 0.8, CriticalThreshold: 0.9, Enabled: true,
	}))
	wBudgetDeny := doReq("/v1/chat/completions", map[string]string{"X-Prism-Agent-ID": aliceAgent.ID}, map[string]interface{}{
		"model":    "gpt-4o-mini",
		"messages": []map[string]string{{"role": "user", "content": "over budget"}},
	})
	assert.Equal(t, http.StatusTooManyRequests, wBudgetDeny.Code, wBudgetDeny.Body.String())

	// 9. Usage reporting & Audit trail strict tenant isolation verification
	logs, totalLogs, err := requestLogRepo.ListWithFilter(ctx, repository.LogFilter{UserID: "alice", Limit: 20})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, totalLogs, 3)
	assert.GreaterOrEqual(t, len(logs), 3)
	analytics, err := requestLogRepo.GetLogAnalytics(ctx, "alice", 30)
	require.NoError(t, err)
	assert.Greater(t, analytics.TotalSpendUSD, 0.0)
	assert.NotEmpty(t, analytics.Models)

	aliceTrails, aliceTotal, err := auditRepo.ListWithFilter(ctx, models.AuditTrailFilter{UserID: "alice", Limit: 50})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, aliceTotal, 5)
	var sawDenied bool
	for _, tr := range aliceTrails {
		if tr.ComplianceStatus == "denied" {
			sawDenied = true
			break
		}
	}
	assert.True(t, sawDenied, "expected both compliant and denied audit trail records")

	// Bob must never see Alice's trails
	_, bobTotal, err := auditRepo.ListWithFilter(ctx, models.AuditTrailFilter{UserID: "bob", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, 0, bobTotal)
}
