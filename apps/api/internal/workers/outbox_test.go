package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOutboxStore struct {
	items        []models.DeliveryOutboxItem
	deliveredIDs []string
	failedCalls  []struct {
		id          string
		attempts    int
		maxAttempts int
		errMsg      string
		deadLetter  bool
	}
}

func (s *fakeOutboxStore) ClaimPendingBatch(_ context.Context, limit int) ([]models.DeliveryOutboxItem, error) {
	var claimed []models.DeliveryOutboxItem
	for _, it := range s.items {
		if it.Status == "" || it.Status == models.OutboxStatusPending {
			it.Attempts++
			claimed = append(claimed, it)
			if len(claimed) >= limit {
				break
			}
		}
	}
	return claimed, nil
}

func (s *fakeOutboxStore) MarkDelivered(_ context.Context, id string) error {
	s.deliveredIDs = append(s.deliveredIDs, id)
	return nil
}

func (s *fakeOutboxStore) MarkFailed(_ context.Context, id string, attempts, maxAttempts int, errMsg string) (bool, error) {
	dead := maxAttempts > 0 && attempts >= maxAttempts
	s.failedCalls = append(s.failedCalls, struct {
		id          string
		attempts    int
		maxAttempts int
		errMsg      string
		deadLetter  bool
	}{id: id, attempts: attempts, maxAttempts: maxAttempts, errMsg: errMsg, deadLetter: dead})
	return dead, nil
}

type recordedLogCreate struct {
	ctx database.TenantDBContext
	log *models.RequestLog
}

type fakeRequestLogCreator struct {
	err     error
	created []recordedLogCreate
}

func (f *fakeRequestLogCreator) Create(ctx context.Context, log *models.RequestLog) error {
	if f.err != nil {
		return f.err
	}
	f.created = append(f.created, recordedLogCreate{
		ctx: database.TenantContextFrom(ctx),
		log: log,
	})
	return nil
}

type fakeAuditTrailRecorder struct {
	err     error
	records []*models.AIAuditTrail
	ctxs    []database.TenantDBContext
}

func (f *fakeAuditTrailRecorder) Record(ctx context.Context, trail *models.AIAuditTrail) error {
	if f.err != nil {
		return f.err
	}
	f.records = append(f.records, trail)
	f.ctxs = append(f.ctxs, database.TenantContextFrom(ctx))
	return nil
}

type fakeGatewayKeyIncrementer struct {
	err   error
	calls []struct {
		ctx   database.TenantDBContext
		keyID string
	}
}

func (f *fakeGatewayKeyIncrementer) IncrementUsage(ctx context.Context, keyID string) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, struct {
		ctx   database.TenantDBContext
		keyID string
	}{ctx: database.TenantContextFrom(ctx), keyID: keyID})
	return nil
}

func TestEncodeDecodeOutboxRequestLog(t *testing.T) {
	agentID := "agent-1"
	keyID := "key-1"
	now := time.Now().UTC().Truncate(time.Millisecond)
	orig := &models.RequestLog{
		ID:              "rl-1",
		RequestID:       "req-1",
		OrgID:           "org-acme",
		WorkspaceID:     "ws-prod",
		ProjectID:       "proj-api",
		AgentID:         &agentID,
		GatewayAPIKeyID: &keyID,
		Model:           "gpt-4o",
		StatusCode:      502,
		LatencyMs:       312,
		CostUSD:         0.042,
		ErrorMessage:    sql.NullString{String: "upstream timeout", Valid: true},
		RetryCount:      2,
		IsStream:        true,
		TTFTMs:          120,
		Attempts:        json.RawMessage(`[{"credential_id":"c1","status_code":502}]`),
		CreatedAt:       now,
	}

	raw, err := models.EncodeOutboxRequestLog(orig)
	require.NoError(t, err)

	decoded, err := models.DecodeOutboxRequestLog(raw)
	require.NoError(t, err)
	assert.Equal(t, orig.RequestID, decoded.RequestID)
	assert.Equal(t, orig.OrgID, decoded.OrgID)
	assert.Equal(t, orig.WorkspaceID, decoded.WorkspaceID)
	assert.Equal(t, orig.ProjectID, decoded.ProjectID)
	require.NotNil(t, decoded.AgentID)
	assert.Equal(t, agentID, *decoded.AgentID)
	assert.True(t, decoded.ErrorMessage.Valid)
	assert.Equal(t, "upstream timeout", decoded.ErrorMessage.String)
	assert.Equal(t, 120, decoded.TTFTMs)
	assert.JSONEq(t, string(orig.Attempts), string(decoded.Attempts))

	origNoErr := &models.RequestLog{RequestID: "req-2", Model: "claude-3-7-sonnet", StatusCode: 200}
	raw2, err := models.EncodeOutboxRequestLog(origNoErr)
	require.NoError(t, err)
	decoded2, err := models.DecodeOutboxRequestLog(raw2)
	require.NoError(t, err)
	assert.False(t, decoded2.ErrorMessage.Valid)
}

func TestOutboxWorker_ReplaysAllEventTypesAndSetsTenantContext(t *testing.T) {
	agentID := "agent-7"
	logRaw, err := models.EncodeOutboxRequestLog(&models.RequestLog{
		RequestID:    "req-rl",
		OrgID:        "org-1",
		WorkspaceID:  "ws-1",
		ProjectID:    "proj-1",
		AgentID:      &agentID,
		Model:        "gpt-4o",
		StatusCode:   500,
		LatencyMs:    88,
		ErrorMessage: sql.NullString{String: "db down during request", Valid: true},
		Attempts:     json.RawMessage(`[{"credential_id":"c-99"}]`),
	})
	require.NoError(t, err)

	auditRaw, err := json.Marshal(&models.AIAuditTrail{
		RequestID:        "req-audit",
		UserID:           "user-1",
		OrgID:            "org-1",
		ModelSlug:        "claude-3-7-sonnet",
		FailoverChain:    pq.StringArray{"anthropic:c1"},
		ToolsInvoked:     pq.StringArray{"linear__get_issue"},
		SignatureHash:    "v2:signedhash",
		ComplianceStatus: "compliant",
	})
	require.NoError(t, err)

	usageRaw, err := json.Marshal(models.OutboxGatewayKeyUsagePayload{
		GatewayKeyID: "gk-77",
		OrgID:        "org-1",
		UserID:       "user-1",
	})
	require.NoError(t, err)

	store := &fakeOutboxStore{
		items: []models.DeliveryOutboxItem{
			{ID: "ob-1", EventType: models.OutboxEventRequestLog, OrgID: "org-1", UserID: "user-1", Payload: logRaw, MaxAttempts: 10},
			{ID: "ob-2", EventType: models.OutboxEventAIAuditTrail, OrgID: "org-1", UserID: "user-1", Payload: auditRaw, MaxAttempts: 10},
			{ID: "ob-3", EventType: models.OutboxEventGatewayKeyUsage, OrgID: "org-1", UserID: "user-1", Payload: usageRaw, MaxAttempts: 10},
		},
	}

	logCreator := &fakeRequestLogCreator{}
	auditRec := &fakeAuditTrailRecorder{}
	keyInc := &fakeGatewayKeyIncrementer{}

	w := NewOutboxWorker(store, logCreator, auditRec, keyInc)
	w.Run(context.Background())

	assert.Equal(t, []string{"ob-1", "ob-2", "ob-3"}, store.deliveredIDs)
	assert.Empty(t, store.failedCalls)

	require.Len(t, logCreator.created, 1)
	assert.Equal(t, "org-1", logCreator.created[0].ctx.OrgID)
	assert.Equal(t, "ws-1", logCreator.created[0].ctx.WorkspaceID)
	assert.Equal(t, "proj-1", logCreator.created[0].ctx.ProjectID)
	assert.True(t, logCreator.created[0].log.ErrorMessage.Valid)
	assert.Equal(t, "db down during request", logCreator.created[0].log.ErrorMessage.String)

	require.Len(t, auditRec.records, 1)
	assert.Equal(t, "v2:signedhash", auditRec.records[0].SignatureHash)
	assert.Equal(t, "org-1", auditRec.ctxs[0].OrgID)

	require.Len(t, keyInc.calls, 1)
	assert.Equal(t, "gk-77", keyInc.calls[0].keyID)
	assert.Equal(t, "org-1", keyInc.calls[0].ctx.OrgID)
}

func TestOutboxWorker_RetriesAndDeadLetters(t *testing.T) {
	logRaw, err := models.EncodeOutboxRequestLog(&models.RequestLog{RequestID: "req-fail", OrgID: "org-1", Model: "gpt-4o"})
	require.NoError(t, err)

	store := &fakeOutboxStore{
		items: []models.DeliveryOutboxItem{
			// Attempts=0 -> claimed attempts=1 < maxAttempts(5) => retry
			{ID: "ob-retry", EventType: models.OutboxEventRequestLog, OrgID: "org-1", Payload: logRaw, Attempts: 0, MaxAttempts: 5},
			// Attempts=4 -> claimed attempts=5 == maxAttempts(5) => dead_letter
			{ID: "ob-dlq", EventType: models.OutboxEventRequestLog, OrgID: "org-1", Payload: logRaw, Attempts: 4, MaxAttempts: 5},
			// Unknown event type => permanent dead_letter on first try
			{ID: "ob-unknown", EventType: "bogus_type", OrgID: "org-1", Payload: json.RawMessage(`{}`), Attempts: 0, MaxAttempts: 10},
		},
	}

	w := NewOutboxWorker(store, &fakeRequestLogCreator{err: errors.New("connection refused")}, nil, nil)
	w.Run(context.Background())

	assert.Empty(t, store.deliveredIDs)
	require.Len(t, store.failedCalls, 3)

	assert.Equal(t, "ob-retry", store.failedCalls[0].id)
	assert.False(t, store.failedCalls[0].deadLetter)

	assert.Equal(t, "ob-dlq", store.failedCalls[1].id)
	assert.True(t, store.failedCalls[1].deadLetter)

	assert.Equal(t, "ob-unknown", store.failedCalls[2].id)
	assert.True(t, store.failedCalls[2].deadLetter)

	// Verify exponential retry backoff is bounded at 30m.
	assert.Equal(t, 15*time.Second, repository.OutboxRetryBackoff(1))
	assert.Equal(t, 30*time.Second, repository.OutboxRetryBackoff(2))
	assert.Equal(t, 30*time.Minute, repository.OutboxRetryBackoff(10))
}
