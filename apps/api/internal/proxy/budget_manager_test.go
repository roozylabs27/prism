package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComputeBudgetStatus_Healthy(t *testing.T) {
	if s := computeBudgetStatus(0.50, 0.80, 0.90); s != "healthy" {
		t.Errorf("expected healthy, got %s", s)
	}
}

func TestComputeBudgetStatus_Warning(t *testing.T) {
	if s := computeBudgetStatus(0.85, 0.80, 0.90); s != "warning" {
		t.Errorf("expected warning, got %s", s)
	}
}

func TestComputeBudgetStatus_Critical(t *testing.T) {
	if s := computeBudgetStatus(0.95, 0.80, 0.90); s != "critical" {
		t.Errorf("expected critical, got %s", s)
	}
}

func TestComputeBudgetStatus_Exceeded(t *testing.T) {
	if s := computeBudgetStatus(1.00, 0.80, 0.90); s != "exceeded" {
		t.Errorf("expected exceeded, got %s", s)
	}
}

func TestComputeBudgetStatus_ExceededOver(t *testing.T) {
	if s := computeBudgetStatus(1.20, 0.80, 0.90); s != "exceeded" {
		t.Errorf("expected exceeded, got %s", s)
	}
}

func TestComputeBudgetStatus_CustomThresholds(t *testing.T) {
	if s := computeBudgetStatus(0.70, 0.50, 0.75); s != "warning" {
		t.Errorf("expected warning at 70%% with 50%% threshold, got %s", s)
	}
	if s := computeBudgetStatus(0.75, 0.50, 0.75); s != "critical" {
		t.Errorf("expected critical at 75%% with 75%% threshold, got %s", s)
	}
}

func TestComputeBudgetStatus_ExactlyAtThreshold(t *testing.T) {
	if s := computeBudgetStatus(0.80, 0.80, 0.90); s != "warning" {
		t.Errorf("expected warning at exactly 80%%, got %s", s)
	}
	if s := computeBudgetStatus(0.90, 0.80, 0.90); s != "critical" {
		t.Errorf("expected critical at exactly 90%%, got %s", s)
	}
}

type fakeBudgetStore struct {
	budgets []models.Budget
	monthly float64
	daily   float64
	err     error
}

func (f *fakeBudgetStore) ListByUserID(_ context.Context, _ string) ([]models.Budget, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.budgets, nil
}

func (f *fakeBudgetStore) GetCombinedSpend(_ context.Context, _ string) (float64, float64, error) {
	if f.err != nil {
		return 0, 0, f.err
	}
	return f.monthly, f.daily, nil
}

func TestBudgetManager_ConcurrentHardLimitReservation(t *testing.T) {
	store := &fakeBudgetStore{
		budgets: []models.Budget{{
			ID:                "b1",
			UserID:            "u1",
			Name:              "Agency Hard Cap",
			DailyLimit:        0.10,
			MonthlyLimit:      1.00,
			HardLimit:         true,
			WarningThreshold:  0.80,
			CriticalThreshold: 0.90,
			Enabled:           true,
		}},
	}
	bm := &BudgetManager{
		budgetRepo: store,
		inFlight:   make(map[string]float64),
	}

	const goroutines = 20
	const estPerReq = 0.02

	var admitted atomic.Int32
	var denied atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res, st, err := bm.Reserve(context.Background(), "u1", estPerReq)
			if err != nil {
				t.Errorf("unexpected Reserve error: %v", err)
				return
			}
			if res != nil {
				admitted.Add(1)
			} else if st != nil && st.Status == "exceeded" {
				denied.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()

	// $0.10 daily limit at $0.02/request admits at most 5 concurrent holds.
	assert.Equal(t, int32(5), admitted.Load(), "concurrent reservations must never exceed hard limit")
	assert.Equal(t, int32(15), denied.Load())
}

func TestBudgetManager_ReleaseUpdatesOptimisticSpendAndReclaimsHeadroom(t *testing.T) {
	store := &fakeBudgetStore{
		budgets: []models.Budget{{
			ID:           "b1",
			UserID:       "u1",
			Name:         "Daily Cap",
			DailyLimit:   0.05,
			HardLimit:    true,
			Enabled:      true,
			WarningThreshold: 0.8,
			CriticalThreshold: 0.9,
		}},
		daily:   0.02,
		monthly: 0.02,
	}
	bm := &BudgetManager{budgetRepo: store, inFlight: make(map[string]float64)}
	ctx := context.Background()

	res1, _, err := bm.Reserve(ctx, "u1", 0.03)
	require.NoError(t, err)
	require.NotNil(t, res1)

	// While in-flight, another $0.01 reservation must be blocked.
	resBlocked, stBlocked, err := bm.Reserve(ctx, "u1", 0.01)
	require.NoError(t, err)
	assert.Nil(t, resBlocked)
	require.NotNil(t, stBlocked)
	assert.Equal(t, "exceeded", stBlocked.Status)

	// Releasing with actualCost $0.01 brings total cached spend to $0.03,
	// freeing $0.02 of headroom and remaining idempotent on duplicate Release.
	bm.Release(res1, 0.01)
	bm.Release(res1, 0.01)

	res2, _, err := bm.Reserve(ctx, "u1", 0.02)
	require.NoError(t, err)
	require.NotNil(t, res2)

	// Fail closed when backing store errors.
	store.err = errors.New("db down")
	_, _, err = bm.Reserve(ctx, "u1", 0.01)
	require.Error(t, err)
}

func TestBudgetManager_OrgKeyedReservationAndIsolation(t *testing.T) {
	store := &fakeBudgetStore{
		budgets: []models.Budget{{
			ID:                "b_org",
			UserID:            "alice",
			Name:              "Org Shared Cap",
			DailyLimit:        0.10,
			HardLimit:         true,
			Enabled:           true,
			WarningThreshold:  0.80,
			CriticalThreshold: 0.90,
		}},
	}
	bm := &BudgetManager{budgetRepo: store, inFlight: make(map[string]float64)}

	aliceCtx := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: "org_a", UserID: "alice"})
	bobCtx := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: "org_a", UserID: "bob"})
	carolCtx := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: "org_b", UserID: "carol"})

	resAlice, _, err := bm.Reserve(aliceCtx, "alice", 0.06)
	require.NoError(t, err)
	require.NotNil(t, resAlice)
	assert.Equal(t, "org:org_a", resAlice.ScopeKey)

	// Bob in org_a shares the same org in-flight hold and is blocked cleanly.
	resBobDenied, stBob, err := bm.Reserve(bobCtx, "bob", 0.05)
	require.NoError(t, err)
	assert.Nil(t, resBobDenied)
	require.NotNil(t, stBob)
	assert.Equal(t, "exceeded", stBob.Status)

	// Carol in org_b is completely isolated from org_a's reservation.
	resCarol, _, err := bm.Reserve(carolCtx, "carol", 0.08)
	require.NoError(t, err)
	require.NotNil(t, resCarol)
	assert.Equal(t, "org:org_b", resCarol.ScopeKey)

	// Releasing Alice's hold frees headroom for Bob in org_a.
	bm.Release(resAlice, 0.02)
	resBobAllowed, _, err := bm.Reserve(bobCtx, "bob", 0.05)
	require.NoError(t, err)
	require.NotNil(t, resBobAllowed)
}

type fakeDistributedBudgetStore struct {
	mu       sync.Mutex
	inFlight map[string]map[string]float64
	settled  map[string]float64
	rev      map[string]int64
	seq      int64
	err      error
}

func newFakeDistributedBudgetStore() *fakeDistributedBudgetStore {
	return &fakeDistributedBudgetStore{
		inFlight: make(map[string]map[string]float64),
		settled:  make(map[string]float64),
		rev:      make(map[string]int64),
	}
}

func (f *fakeDistributedBudgetStore) TryReserve(_ context.Context, in goredis.BudgetReserveInput) (*goredis.BudgetReserveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var current float64
	for _, v := range f.inFlight[in.ScopeKey] {
		current += v
	}
	settled := f.settled[in.ScopeKey]
	if in.HardStop {
		if in.MonthlyLimit > 0 && (in.MonthlySpent+settled+current >= in.MonthlyLimit || (in.EstimatedCost > 0 && in.MonthlySpent+settled+current+in.EstimatedCost > in.MonthlyLimit+1e-12)) {
			return &goredis.BudgetReserveResult{
				Allowed:        false,
				InFlight:       current,
				RecentSettled:  settled,
				Revision:       f.rev[in.ScopeKey],
				ExceededPeriod: "monthly",
			}, nil
		}
		if in.DailyLimit > 0 && (in.DailySpent+settled+current >= in.DailyLimit || (in.EstimatedCost > 0 && in.DailySpent+settled+current+in.EstimatedCost > in.DailyLimit+1e-12)) {
			return &goredis.BudgetReserveResult{
				Allowed:        false,
				InFlight:       current,
				RecentSettled:  settled,
				Revision:       f.rev[in.ScopeKey],
				ExceededPeriod: "daily",
			}, nil
		}
	}
	f.seq++
	id := in.ReservationID
	if id == "" {
		id = fmt.Sprintf("res-%d", f.seq)
	}
	if f.inFlight[in.ScopeKey] == nil {
		f.inFlight[in.ScopeKey] = make(map[string]float64)
	}
	f.inFlight[in.ScopeKey][id] = in.EstimatedCost
	return &goredis.BudgetReserveResult{
		Allowed:       true,
		InFlight:      current + in.EstimatedCost,
		RecentSettled: settled,
		Revision:      f.rev[in.ScopeKey],
		ReservationID: id,
	}, nil
}

func (f *fakeDistributedBudgetStore) Release(_ context.Context, scopeKey, reservationID string, actualCost float64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return 0, f.err
	}
	if m := f.inFlight[scopeKey]; m != nil {
		delete(m, reservationID)
	}
	if actualCost > 0 {
		f.settled[scopeKey] += actualCost
	}
	f.rev[scopeKey]++
	return f.rev[scopeKey], nil
}

func (f *fakeDistributedBudgetStore) InFlightAndOptimistic(_ context.Context, scopeKey string) (*goredis.BudgetSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var current float64
	for _, v := range f.inFlight[scopeKey] {
		current += v
	}
	return &goredis.BudgetSnapshot{
		InFlight:      current,
		RecentSettled: f.settled[scopeKey],
		Revision:      f.rev[scopeKey],
	}, nil
}

func TestBudgetManager_DistributedMultiReplicaReservation(t *testing.T) {
	repo := &fakeBudgetStore{
		budgets: []models.Budget{{
			ID:           "b-dist",
			UserID:       "u1",
			Name:         "Cluster Cap",
			MonthlyLimit: 0.10,
			HardLimit:    true,
			Enabled:      true,
		}},
	}
	sharedDist := newFakeDistributedBudgetStore()

	// Two independent BudgetManager replicas sharing one Redis-like DistributedBudgetStore.
	replicaA := &BudgetManager{budgetRepo: repo, inFlight: make(map[string]float64)}
	replicaA.SetDistributedStore(sharedDist)
	replicaA.SetFailClosed(true)

	replicaB := &BudgetManager{budgetRepo: repo, inFlight: make(map[string]float64)}
	replicaB.SetDistributedStore(sharedDist)
	replicaB.SetFailClosed(true)

	ctx := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: "org_cluster", UserID: "u1"})
	const requests = 12
	var admitted atomic.Int32
	var denied atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		mgr := replicaA
		if i%2 == 1 {
			mgr = replicaB
		}
		go func(m *BudgetManager) {
			defer wg.Done()
			res, st, err := m.Reserve(ctx, "u1", 0.04)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if res != nil {
				admitted.Add(1)
			} else if st != nil && st.Status == "exceeded" {
				denied.Add(1)
			}
		}(mgr)
	}
	wg.Wait()

	assert.Equal(t, int32(2), admitted.Load(), "across two replicas, $0.10 budget fits at most two $0.04 requests")
	assert.Equal(t, int32(10), denied.Load())

	// Fail-closed rejects when Redis is down; fail-open falls back to local.
	sharedDist.err = errors.New("redis connection reset")
	_, _, err := replicaA.Reserve(ctx, "u1", 0.01)
	require.Error(t, err)

	replicaA.SetFailClosed(false)
	resFallback, _, err := replicaA.Reserve(ctx, "u1", 0.01)
	require.NoError(t, err)
	require.NotNil(t, resFallback)
}

type fakeStreamQuotaRepo struct {
	quotas map[string]*models.QuotaCheckResult
}

func (f *fakeStreamQuotaRepo) EvaluateQuota(_ context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	if q, ok := f.quotas[targetType+":"+targetID]; ok {
		return q, nil
	}
	return &models.QuotaCheckResult{Allowed: true, TargetType: targetType, TargetID: targetID}, nil
}

type fakeStreamLimiter struct {
	mu     sync.Mutex
	active map[string]map[string]struct{}
	err    error
}

func (f *fakeStreamLimiter) AcquireScopeStream(_ context.Context, scopeKey, reqID string, limit int, _ time.Duration) (bool, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, 0, f.err
	}
	if f.active == nil {
		f.active = make(map[string]map[string]struct{})
	}
	if f.active[scopeKey] == nil {
		f.active[scopeKey] = make(map[string]struct{})
	}
	if _, ok := f.active[scopeKey][reqID]; ok {
		return true, int64(len(f.active[scopeKey])), nil
	}
	if limit > 0 && len(f.active[scopeKey]) >= limit {
		return false, int64(len(f.active[scopeKey])), nil
	}
	f.active[scopeKey][reqID] = struct{}{}
	return true, int64(len(f.active[scopeKey])), nil
}

func (f *fakeStreamLimiter) ReleaseScopeStream(_ context.Context, scopeKey, reqID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active[scopeKey] != nil {
		delete(f.active[scopeKey], reqID)
	}
	return nil
}

func TestAdmissionController_MaxConcurrentStreamsEnforcement(t *testing.T) {
	quotaRepo := &fakeStreamQuotaRepo{
		quotas: map[string]*models.QuotaCheckResult{
			"organization:org1": {Allowed: true, TargetType: "organization", TargetID: "org1", MaxConcurrentStreams: 3},
			"workspace:ws1":     {Allowed: true, TargetType: "workspace", TargetID: "ws1", MaxConcurrentStreams: 1},
		},
	}
	limiter := &fakeStreamLimiter{}
	adm := NewAdmissionController(nil, nil, quotaRepo, nil)
	adm.SetStreamConcurrencyLimiter(limiter, true)

	ctx := context.Background()
	tenant := models.TenantContext{OrgID: "org1", WorkspaceID: "ws1"}

	rel1, deny1, err := adm.AcquireStreamSlots(ctx, "req-1", tenant)
	require.NoError(t, err)
	assert.Nil(t, deny1)
	require.NotNil(t, rel1)

	// Second concurrent stream in ws1 hits workspace limit=1 and must roll back its org1 slot!
	rel2, deny2, err := adm.AcquireStreamSlots(ctx, "req-2", tenant)
	require.NoError(t, err)
	assert.Nil(t, rel2)
	require.NotNil(t, deny2)
	assert.Equal(t, AdmissionDeny, deny2.Decision)
	assert.Equal(t, http.StatusTooManyRequests, deny2.HTTPStatus)
	assert.Equal(t, "max_concurrent_streams_exceeded", deny2.ErrorCode)
	assert.Len(t, limiter.active["organization:org1"], 1, "denied second stream must release its org slot")

	// Release req-1 frees both org1 and ws1 so req-2 can enter.
	rel1()
	rel2, deny2, err = adm.AcquireStreamSlots(ctx, "req-2", tenant)
	require.NoError(t, err)
	assert.Nil(t, deny2)
	require.NotNil(t, rel2)
	rel2()
}
