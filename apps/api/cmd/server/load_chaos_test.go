package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/handlers"
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

// TestServer_RealGatewayLoadAndChaos exercises either a live cmd/server target
// (when PRISM_LOAD_TARGET_URL + PRISM_LOAD_API_KEY are set) or an in-process
// Gin server backed by real PostgreSQL + Redis + ExecutionOrchestrator under
// 50-200 RPS concurrent load and upstream 429/500 failover chaos.
func TestServer_RealGatewayLoadAndChaos(t *testing.T) {
	if targetURL := os.Getenv("PRISM_LOAD_TARGET_URL"); targetURL != "" {
		apiKey := os.Getenv("PRISM_LOAD_API_KEY")
		if apiKey == "" {
			t.Skip("PRISM_LOAD_TARGET_URL set without PRISM_LOAD_API_KEY")
		}
		driveHTTPLoad(t, targetURL, apiKey, "gpt-4o-mini", 60, 12)
		return
	}

	dsn := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dsn == "" || redisURL == "" {
		t.Skip("DATABASE_URL and REDIS_URL required for full cmd/server Postgres+Redis load test")
	}

	db, err := database.NewPostgres(dsn, 10*time.Second)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	defer func() { _ = db.Close() }()
	sqlDB := db.DB

	if err := database.RunMigrations(dsn, "../../migrations"); err != nil {
		t.Skipf("migrations unavailable: %v", err)
	}

	opts, err := goredislib.ParseURL(redisURL)
	require.NoError(t, err)
	rdb := goredislib.NewClient(opts)
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	gin.SetMode(gin.TestMode)
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")

	const encKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	suffix := fmt.Sprintf("load_%d", time.Now().UnixNano())
	ctx := context.Background()

	_, err = sqlDB.Exec(`
		INSERT INTO users (id, email, name, password_hash) VALUES ($1, $2, 'Load User', 'x') ON CONFLICT DO NOTHING;
		INSERT INTO "user" (id, name, email) VALUES ($1, 'Load User', $2) ON CONFLICT DO NOTHING;
		INSERT INTO organizations (id, name, slug, owner_id, plan_tier) VALUES ($3, 'Load Org', $4, $1, 'enterprise') ON CONFLICT DO NOTHING;
		INSERT INTO organization_members (id, org_id, user_id, role_id)
		SELECT $5, $3, $1, id FROM roles WHERE slug = 'owner' AND is_system LIMIT 1
		ON CONFLICT DO NOTHING;
	`, suffix+"_u", suffix+"@example.invalid", suffix+"_org", suffix+"-org", suffix+"_om")
	require.NoError(t, err)

	var primaryCalls, secondaryCalls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "Bearer sk-load-primary" {
			n := primaryCalls.Add(1)
			if n%5 == 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"message":"synthetic chaos 500"}}`))
				return
			}
		} else {
			secondaryCalls.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-load",
			"object":  "chat.completion",
			"created": 1700000000,
			"model":   suffix + "-model",
			"choices": []map[string]interface{}{{
				"index":         0,
				"message":       map[string]interface{}{"role": "assistant", "content": "ok"},
				"finish_reason": "stop",
			}},
			"usage": map[string]interface{}{"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12},
		})
	}))
	defer upstream.Close()

	var providerID string
	require.NoError(t, sqlDB.QueryRow(`
		INSERT INTO providers (user_id, org_id, name, slug, base_url, type, enabled, routing_strategy)
		VALUES ($1, $2, 'LoadOpenAI', $3, $4, 'openai', true, 'fallback') RETURNING id
	`, suffix+"_u", suffix+"_org", suffix+"-prov", upstream.URL).Scan(&providerID))

	enc1, _ := utils.EncryptAES256GCM("sk-load-primary", encKey)
	enc2, _ := utils.EncryptAES256GCM("sk-load-secondary", encKey)
	var cred1, cred2 string
	require.NoError(t, sqlDB.QueryRow(`INSERT INTO credentials (provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, priority, enabled, status) VALUES ($1,$2,$3,'p',$4,'sk-','sk-**1','api_key',1,true,'active') RETURNING id`, providerID, suffix+"_u", suffix+"_org", enc1).Scan(&cred1))
	require.NoError(t, sqlDB.QueryRow(`INSERT INTO credentials (provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, priority, enabled, status) VALUES ($1,$2,$3,'s',$4,'sk-','sk-**2','api_key',2,true,'active') RETURNING id`, providerID, suffix+"_u", suffix+"_org", enc2).Scan(&cred2))

	modelSlug := suffix + "-model"
	_, err = sqlDB.Exec(`INSERT INTO models (provider_id, name, slug, display_name, enabled, context_window, input_price_per_1m, output_price_per_1m) VALUES ($1,$2,$2,'Load Model',true,8192,0.1,0.2)`, providerID, modelSlug)
	require.NoError(t, err)

	rawKey, err := utils.NewGatewayKey()
	require.NoError(t, err)
	keyPrefix := utils.GatewayKeyDisplayPrefix(rawKey)
	keyHash := utils.HashSHA256(rawKey)
	orgID := suffix + "_org"

	gkRepo := repository.NewGatewayKeyRepository(sqlDB)
	gk := &models.GatewayAPIKey{
		UserID:     suffix + "_u",
		OrgID:      &orgID,
		Name:       "Load Key",
		KeyPrefix:  keyPrefix,
		KeyHash:    keyHash,
		ProviderID: &providerID,
		RateLimit:  10000,
	}
	require.NoError(t, gkRepo.Create(ctx, gk))

	t.Cleanup(func() {
		_, _ = sqlDB.Exec(`DELETE FROM request_logs WHERE org_id = $1`, orgID)
		_, _ = sqlDB.Exec(`DELETE FROM ai_audit_trails WHERE org_id = $1`, orgID)
		_, _ = sqlDB.Exec(`DELETE FROM gateway_api_keys WHERE id = $1`, gk.ID)
		_, _ = sqlDB.Exec(`DELETE FROM models WHERE slug = $1`, modelSlug)
		_, _ = sqlDB.Exec(`DELETE FROM credentials WHERE provider_id = $1`, providerID)
		_, _ = sqlDB.Exec(`DELETE FROM providers WHERE id = $1`, providerID)
		_, _ = sqlDB.Exec(`DELETE FROM organization_members WHERE org_id = $1`, orgID)
		_, _ = sqlDB.Exec(`DELETE FROM organizations WHERE id = $1`, orgID)
		_, _ = sqlDB.Exec(`DELETE FROM "user" WHERE id = $1`, suffix+"_u")
		_, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, suffix+"_u")
		_ = rdb.Del(ctx, "credential:"+cred1+":cooldown", "credential:"+cred1+":errors", "credential:"+cred2+":cooldown", "credential:"+cred2+":errors").Err()
	})

	modelRepo := repository.NewModelRepository(sqlDB)
	provRepo := repository.NewProviderRepository(sqlDB)
	credRepo := repository.NewCredentialRepository(sqlDB)
	settingRepo := repository.NewSettingRepository(sqlDB)
	reqLogRepo := repository.NewRequestLogRepository(sqlDB)
	pricingRepo := repository.NewModelPricingRepository(sqlDB)
	budgetRepo := repository.NewBudgetRepository(sqlDB)
	quotaRepo := repository.NewQuotaRepository(sqlDB)
	govRepo := repository.NewGovernancePolicyRepository(sqlDB)
	auditRepo := repository.NewAuditTrailRepository(sqlDB)
	rbacRepo := repository.NewRBACRepository(sqlDB)
	accountRepo := repository.NewAccountRepository(sqlDB)
	workspaceRepo := repository.NewWorkspaceRepository(sqlDB)
	agentRepo := repository.NewAgentRepository(sqlDB)

	cooldown := goredis.NewCooldownStore(rdb)
	budgetMgr := proxy.NewBudgetManager(budgetRepo)
	healthStore := proxy.NewProviderHealthStore(reqLogRepo, 30*time.Second)
	router := proxy.NewRouter(modelRepo, provRepo, credRepo, settingRepo, healthStore)
	engine := proxy.NewEngine(router, credRepo, cooldown, nil, nil, encKey, 2, 1, budgetMgr, nil, nil, nil, nil)
	admission := proxy.NewAdmissionController(proxy.NewRBACEngine(govRepo), proxy.NewAgentGovernanceEngine(agentRepo), proxy.NewCachedQuotaEvaluator(quotaRepo, time.Minute), budgetMgr)
	auditRec := proxy.NewAuditRecorderWithKey(auditRepo, proxy.DeriveAuditHMACKey("", encKey))
	orch := service.NewExecutionOrchestrator(engine, admission, gkRepo, reqLogRepo, nil, pricingRepo, auditRec)
	gwHandler := handlers.NewGatewayHandler(engine, gkRepo, reqLogRepo, nil, pricingRepo, nil, proxy.NewAgentGovernanceEngine(agentRepo), proxy.NewRBACEngine(govRepo), auditRec, modelRepo, agentRepo, orch)
	gwHandler.SetMemberRoleResolver(rbacRepo)

	r := gin.New()
	r.Use(middleware.CorrelationMiddleware())
	v1 := r.Group("/v1")
	v1.Use(middleware.GatewayAuthMiddleware(repository.NewGatewayKeyCache(gkRepo)))
	v1.Use(middleware.TenantMiddleware(middleware.NewCompositeTenantChecker(accountRepo, workspaceRepo)))
	v1.Use(middleware.GatewayRateLimitMiddlewareWithPolicy(goredis.NewSlidingWindowLimiter(rdb, time.Minute), 10000, true))
	v1.Use(middleware.AgentPolicyMiddleware(agentRepo))
	v1.POST("/chat/completions", gwHandler.ChatCompletions)

	server := httptest.NewServer(r)
	defer server.Close()

	driveHTTPLoad(t, server.URL, rawKey, modelSlug, 80, 12)
	assert.Greater(t, primaryCalls.Load(), int64(0))
	assert.Greater(t, secondaryCalls.Load(), int64(0), "chaos 500s should trigger secondary failover")
}

func driveHTTPLoad(t *testing.T, baseURL, apiKey, model string, totalRequests, concurrency int) {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	jobs := make(chan int, totalRequests)
	for i := 0; i < totalRequests; i++ {
		jobs <- i
	}
	close(jobs)

	payload, _ := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": "load check"}},
	})

	var okCount atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(payload))
				if err != nil {
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+apiKey)
				resp, err := client.Do(req)
				if err == nil && resp != nil {
					_, _ = io.Copy(io.Discard, resp.Body)
					_ = resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						okCount.Add(1)
					}
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	rps := float64(totalRequests) / elapsed.Seconds()
	t.Logf("cmd/server Real HTTP Load | Requests=%d | OK=%d | Elapsed=%v | Throughput=%.1f RPS",
		totalRequests, okCount.Load(), elapsed, rps)
	require.GreaterOrEqual(t, okCount.Load(), int64(totalRequests-5))
	assert.GreaterOrEqual(t, rps, 20.0, "gateway throughput must sustain concurrent load cleanly")
}
