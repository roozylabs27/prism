package proxy

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
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
