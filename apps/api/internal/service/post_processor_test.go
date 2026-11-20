package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsyncPostProcessor_EnqueueAndProcess(t *testing.T) {
	processor := NewAsyncPostProcessor(nil, nil, nil, nil, 100, 2)
	defer processor.Stop()

	task := &PostProcessTask{
		Ctx:       context.Background(),
		RequestID: "req_test_123",
		Log: &models.RequestLog{
			ID:        "log_123",
			Model:     "gemini-3.6-flash",
			CostUSD:   0.001,
			CreatedAt: time.Now(),
		},
		GatewayKey: &models.GatewayAPIKey{
			ID: "key_123",
		},
	}

	enqueued := processor.Enqueue(task)
	assert.True(t, enqueued)
}

func TestAsyncPostProcessor_NilTaskHandling(t *testing.T) {
	processor := NewAsyncPostProcessor(nil, nil, nil, nil, 10, 1)
	defer processor.Stop()

	assert.False(t, processor.Enqueue(nil))
	assert.False(t, processor.Enqueue(&PostProcessTask{}))
}

func TestPostProcessorStopWhileEnqueueing(t *testing.T) {
	p := NewAsyncPostProcessor(nil, nil, nil, nil, 1, 1)

	// Enqueue used to send on the channel Stop had closed and panic.
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				p.Enqueue(&PostProcessTask{Log: &models.RequestLog{}})
			}
		}()
	}
	p.Stop()
	wg.Wait()
	p.Stop()
}

func TestPostProcessorSurvivesAPanickingTask(t *testing.T) {
	// A publisher without a Redis client panics on Publish.
	p := NewAsyncPostProcessor(nil, nil, &goredis.EventPublisher{}, nil, 4, 1)
	key := &models.GatewayAPIKey{ID: "k", UserID: "u1"}
	for i := 0; i < 3; i++ {
		if !p.Enqueue(&PostProcessTask{Log: &models.RequestLog{}, GatewayKey: key}) {
			t.Fatal("task not buffered")
		}
	}
	// A panic in a worker used to take the whole server down; here it would
	// end the test binary before Stop returns.
	p.Stop()
}

type mockLogStore struct {
	err     error
	created []*models.RequestLog
}

func (m *mockLogStore) Create(_ context.Context, log *models.RequestLog) error {
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, log)
	return nil
}

type mockKeyStore struct {
	err   error
	calls []string
}

func (m *mockKeyStore) IncrementUsage(_ context.Context, keyID string) error {
	if m.err != nil {
		return m.err
	}
	m.calls = append(m.calls, keyID)
	return nil
}

type mockAuditStore struct {
	err      error
	recorder *proxy.AuditRecorder
	trails   []*models.AIAuditTrail
}

func (m *mockAuditStore) Record(_ context.Context, trail *models.AIAuditTrail) error {
	if m.recorder != nil && trail.SignatureHash == "" {
		m.recorder.Sign(trail)
	}
	if m.err != nil {
		return m.err
	}
	m.trails = append(m.trails, trail)
	return nil
}

type mockOutboxStore struct {
	mu    sync.Mutex
	items []*models.DeliveryOutboxItem
}

func (m *mockOutboxStore) Enqueue(_ context.Context, item *models.DeliveryOutboxItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items = append(m.items, item)
	return nil
}

func TestAsyncPostProcessor_PopulatesTrueAuditFields(t *testing.T) {
	rec := proxy.NewAuditRecorderWithKey(nil, []byte("test-hmac-key"))
	audits := &mockAuditStore{recorder: rec}
	logs := &mockLogStore{}
	keys := &mockKeyStore{}

	p := &AsyncPostProcessor{
		taskChan:      make(chan *PostProcessTask, 8),
		requestLogs:   logs,
		gatewayKeys:   keys,
		auditRecorder: audits,
		workersCount:  1,
		overflow:      make(chan struct{}, 2),
	}
	p.start()

	promptHash := proxy.HashPayload(`[{"role":"user","content":"Check revenue"}]`)
	respHash := proxy.HashPayload(`{"id":"chatcmpl-1","choices":[{"message":{"content":"$10M"}}]}`)
	attemptsJSON, err := json.Marshal([]map[string]interface{}{
		{"credential_id": "cred-1", "provider_id": "openai", "status_code": 429},
		{"credential_id": "cred-2", "provider_name": "anthropic", "status_code": 200},
	})
	require.NoError(t, err)

	task := &PostProcessTask{
		Ctx:        context.Background(),
		RequestID:  "req-audit-1",
		AgentID:    "agent-fin",
		AgentName:  "FinanceBot",
		UserRole:   "admin",
		PromptHash: promptHash,
		Log: &models.RequestLog{
			RequestID:        "req-audit-1",
			OrgID:            "org-acme",
			WorkspaceID:      "ws-1",
			ProjectID:        "proj-1",
			Model:            "claude-3-7-sonnet",
			ProviderType:     "anthropic",
			StatusCode:       200,
			LatencyMs:        650,
			TTFTMs:           110,
			InputTokens:      40,
			OutputTokens:     25,
			TotalTokens:      65,
			CostUSD:          0.005,
			ResponseHash:     respHash,
			Attempts:         attemptsJSON,
			ToolsInvoked:     []string{"postgres__query_ledger", "calculator"},
			MCPServersCalled: []string{"postgres"},
		},
		GatewayKey: &models.GatewayAPIKey{ID: "gk-1", UserID: "user-1"},
	}

	require.True(t, p.Enqueue(task))
	p.Stop()

	require.Len(t, audits.trails, 1)
	tr := audits.trails[0]
	assert.Equal(t, promptHash, tr.PromptHash)
	assert.Equal(t, respHash, tr.ResponseHash)
	assert.Equal(t, 110, tr.TTFTMS)
	assert.Equal(t, []string{"openai:cred-1", "anthropic:cred-2"}, []string(tr.FailoverChain))
	assert.Equal(t, []string{"postgres__query_ledger", "calculator"}, []string(tr.ToolsInvoked))
	assert.Equal(t, []string{"postgres"}, []string(tr.MCPServersCalled))
	assert.True(t, rec.VerifyTrail(tr).Valid)
}

func TestAsyncPostProcessor_OutboxFallbackOnPersistenceFailure(t *testing.T) {
	dbErr := errors.New("pq: terminating connection due to administrator command")
	rec := proxy.NewAuditRecorderWithKey(nil, []byte("test-hmac-key"))
	outbox := &mockOutboxStore{}

	p := &AsyncPostProcessor{
		taskChan:      make(chan *PostProcessTask, 8),
		requestLogs:   &mockLogStore{err: dbErr},
		gatewayKeys:   &mockKeyStore{err: dbErr},
		auditRecorder: &mockAuditStore{err: dbErr, recorder: rec},
		outbox:        outbox,
		workersCount:  1,
		overflow:      make(chan struct{}, 2),
	}
	p.start()

	task := &PostProcessTask{
		Ctx:        context.Background(),
		RequestID:  "req-outbox-1",
		PromptHash: proxy.HashPayload("prompt-bytes"),
		Log: &models.RequestLog{
			RequestID:    "req-outbox-1",
			OrgID:        "org-acme",
			WorkspaceID:  "ws-fin",
			ProjectID:    "proj-ledger",
			Model:        "gpt-4o",
			StatusCode:   200,
			CostUSD:      0.15,
			ResponseHash: proxy.HashPayload("resp-bytes"),
			TTFTMs:       85,
		},
		GatewayKey: &models.GatewayAPIKey{ID: "gk-9", UserID: "user-alice"},
	}

	require.True(t, p.Enqueue(task))
	p.Stop()

	require.Len(t, outbox.items, 3)

	byType := map[string]*models.DeliveryOutboxItem{}
	for _, item := range outbox.items {
		byType[item.EventType] = item
		assert.Equal(t, "org-acme", item.OrgID)
		assert.Equal(t, "user-alice", item.UserID)
		assert.Contains(t, item.LastError, "administrator command")
	}

	rlItem := byType[models.OutboxEventRequestLog]
	require.NotNil(t, rlItem)
	rl, err := models.DecodeOutboxRequestLog(rlItem.Payload)
	require.NoError(t, err)
	assert.Equal(t, "req-outbox-1", rl.RequestID)
	assert.Equal(t, 85, rl.TTFTMs)
	assert.Equal(t, "ws-fin", rl.WorkspaceID)

	usageItem := byType[models.OutboxEventGatewayKeyUsage]
	require.NotNil(t, usageItem)
	var up models.OutboxGatewayKeyUsagePayload
	require.NoError(t, json.Unmarshal(usageItem.Payload, &up))
	assert.Equal(t, "gk-9", up.GatewayKeyID)

	auditItem := byType[models.OutboxEventAIAuditTrail]
	require.NotNil(t, auditItem)
	var tr models.AIAuditTrail
	require.NoError(t, json.Unmarshal(auditItem.Payload, &tr))
	assert.True(t, rec.VerifyTrail(&tr).Valid)
}
