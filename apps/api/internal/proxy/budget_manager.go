package proxy

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

const budgetSpendCacheTTL = 10 * time.Second

type budgetSpendCacheEntry struct {
	monthly   float64
	daily     float64
	fetchedAt time.Time
}

type budgetStore interface {
	ListByUserID(ctx context.Context, userID string) ([]models.Budget, error)
	GetCombinedSpend(ctx context.Context, userID string) (monthlySpend float64, dailySpend float64, err error)
}

// BudgetReservation represents an in-flight spend hold for a single request.
// Callers must pass it to Release once the request finishes or fails.
type BudgetReservation struct {
	UserID      string
	ReservedUSD float64
	released    bool
}

type BudgetManager struct {
	budgetRepo budgetStore
	spendCache sync.Map // userID -> budgetSpendCacheEntry

	mu       sync.Mutex
	inFlight map[string]float64 // userID -> reserved in-flight USD
}

func NewBudgetManager(budgetRepo *repository.BudgetRepository) *BudgetManager {
	var store budgetStore
	if budgetRepo != nil {
		store = budgetRepo
	}
	return &BudgetManager{
		budgetRepo: store,
		inFlight:   make(map[string]float64),
	}
}

// GetStatus evaluates every enabled budget of the user against both its
// monthly and its daily limit (including active in-flight reservations) and
// reports the one in the worst state. Between equally bad budgets a hard limit
// wins, so an exceeded hard budget is never hidden behind a soft one.
func (bm *BudgetManager) GetStatus(ctx context.Context, userID string) (*models.BudgetStatus, error) {
	if bm == nil || bm.budgetRepo == nil || userID == "" {
		return nil, nil
	}
	budgets, err := bm.budgetRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var worst *models.BudgetStatus
	var monthly, daily float64
	spendLoaded := false
	for i := range budgets {
		budget := budgets[i]
		if !budget.Enabled {
			continue
		}
		if !spendLoaded {
			// Spend is only needed once there is a budget to measure.
			m, d, err := bm.getSpendCached(ctx, userID)
			if err != nil {
				return nil, err
			}
			inFlight := bm.inFlightSpend(userID)
			monthly = m + inFlight
			daily = d + inFlight
			spendLoaded = true
		}
		st := evaluateBudget(&budget, monthly, daily)
		if worst == nil || worseBudget(st, worst) {
			worst = st
		}
	}
	return worst, nil
}

// Reserve atomically checks enabled hard-limit budgets (combining persisted
// spend, active in-flight reservations, and estimatedCostUSD) and records an
// in-flight reservation when admitted. If a hard limit would be exceeded,
// returns (nil, exceededStatus, nil). Always pair a non-nil reservation with Release.
func (bm *BudgetManager) Reserve(ctx context.Context, userID string, estimatedCostUSD float64) (*BudgetReservation, *models.BudgetStatus, error) {
	if bm == nil || bm.budgetRepo == nil || userID == "" {
		return nil, nil, nil
	}
	if estimatedCostUSD < 0 {
		estimatedCostUSD = 0
	}

	budgets, err := bm.budgetRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	var enabledBudgets []models.Budget
	for i := range budgets {
		if budgets[i].Enabled {
			enabledBudgets = append(enabledBudgets, budgets[i])
		}
	}
	if len(enabledBudgets) == 0 {
		return nil, nil, nil
	}

	monthlyDB, dailyDB, err := bm.getSpendCached(ctx, userID)
	if err != nil {
		return nil, nil, err
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.inFlight == nil {
		bm.inFlight = make(map[string]float64)
	}
	if v, ok := bm.spendCache.Load(userID); ok {
		entry := v.(budgetSpendCacheEntry)
		if time.Since(entry.fetchedAt) < budgetSpendCacheTTL {
			monthlyDB = entry.monthly
			dailyDB = entry.daily
		}
	}

	inFlight := bm.inFlight[userID]
	projectedMonthly := monthlyDB + inFlight + estimatedCostUSD
	projectedDaily := dailyDB + inFlight + estimatedCostUSD

	var worst *models.BudgetStatus
	for i := range enabledBudgets {
		b := enabledBudgets[i]
		evalMonthly := monthlyDB + inFlight
		evalDaily := dailyDB + inFlight
		if b.HardLimit {
			if b.MonthlyLimit > 0 && (evalMonthly >= b.MonthlyLimit || (estimatedCostUSD > 0 && projectedMonthly > b.MonthlyLimit+1e-12)) {
				st := evaluateBudget(&b, projectedMonthly, projectedDaily)
				st.Status = "exceeded"
				st.LimitPeriod = "monthly"
				return nil, st, nil
			}
			if b.DailyLimit > 0 && (evalDaily >= b.DailyLimit || (estimatedCostUSD > 0 && projectedDaily > b.DailyLimit+1e-12)) {
				st := evaluateBudget(&b, projectedMonthly, projectedDaily)
				st.Status = "exceeded"
				st.LimitPeriod = "daily"
				return nil, st, nil
			}
		}
		st := evaluateBudget(&b, evalMonthly, evalDaily)
		if worst == nil || worseBudget(st, worst) {
			worst = st
		}
	}

	bm.inFlight[userID] = inFlight + estimatedCostUSD
	return &BudgetReservation{
		UserID:      userID,
		ReservedUSD: estimatedCostUSD,
	}, worst, nil
}

// Release frees an in-flight reservation and optimistically reflects actualCostUSD
// in the spend cache so subsequent requests see the committed spend before async
// persistence finishes. Safe to call multiple times or with a nil reservation.
func (bm *BudgetManager) Release(res *BudgetReservation, actualCostUSD float64) {
	if bm == nil || res == nil {
		return
	}
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if res.released {
		return
	}
	res.released = true

	if bm.inFlight != nil {
		next := bm.inFlight[res.UserID] - res.ReservedUSD
		if next <= 1e-12 {
			delete(bm.inFlight, res.UserID)
		} else {
			bm.inFlight[res.UserID] = next
		}
	}

	if actualCostUSD > 0 {
		if v, ok := bm.spendCache.Load(res.UserID); ok {
			entry := v.(budgetSpendCacheEntry)
			entry.monthly += actualCostUSD
			entry.daily += actualCostUSD
			bm.spendCache.Store(res.UserID, entry)
		}
	}
}

// InvalidateSpendCache clears the cached DB spend entry for userID.
func (bm *BudgetManager) InvalidateSpendCache(userID string) {
	if bm == nil || userID == "" {
		return
	}
	bm.spendCache.Delete(userID)
}

func (bm *BudgetManager) inFlightSpend(userID string) float64 {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.inFlight == nil {
		return 0
	}
	return bm.inFlight[userID]
}

func evaluateBudget(budget *models.Budget, monthlySpent, dailySpent float64) *models.BudgetStatus {
	st := &models.BudgetStatus{
		Budget:           budget,
		MonthlySpent:     monthlySpent,
		DailySpent:       dailySpent,
		MonthlyRemaining: math.Max(budget.MonthlyLimit-monthlySpent, 0),
		DailyRemaining:   math.Max(budget.DailyLimit-dailySpent, 0),
	}
	// A limit of zero means no limit for that period.
	var usage float64
	if budget.MonthlyLimit > 0 {
		usage = monthlySpent / budget.MonthlyLimit
		st.LimitPeriod = "monthly"
	}
	if budget.DailyLimit > 0 {
		if daily := dailySpent / budget.DailyLimit; daily > usage || st.LimitPeriod == "" {
			usage = daily
			st.LimitPeriod = "daily"
		}
	}
	st.UsagePercent = math.Round(usage*100*100) / 100
	st.Status = computeBudgetStatus(usage, budget.WarningThreshold, budget.CriticalThreshold)
	return st
}

var budgetSeverity = map[string]int{"healthy": 0, "warning": 1, "critical": 2, "exceeded": 3}

func worseBudget(a, b *models.BudgetStatus) bool {
	if sa, sb := budgetSeverity[a.Status], budgetSeverity[b.Status]; sa != sb {
		return sa > sb
	}
	if a.Budget.HardLimit != b.Budget.HardLimit {
		return a.Budget.HardLimit
	}
	return a.UsagePercent > b.UsagePercent
}

func computeBudgetStatus(usagePercent, warningThreshold, criticalThreshold float64) string {
	if usagePercent >= 1.0 {
		return "exceeded"
	}
	if usagePercent >= criticalThreshold {
		return "critical"
	}
	if usagePercent >= warningThreshold {
		return "warning"
	}
	return "healthy"
}

func (bm *BudgetManager) getSpendCached(ctx context.Context, userID string) (float64, float64, error) {
	if v, ok := bm.spendCache.Load(userID); ok {
		entry := v.(budgetSpendCacheEntry)
		if time.Since(entry.fetchedAt) < budgetSpendCacheTTL {
			return entry.monthly, entry.daily, nil
		}
		bm.spendCache.Delete(userID)
	}
	monthly, daily, err := bm.budgetRepo.GetCombinedSpend(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	bm.spendCache.Store(userID, budgetSpendCacheEntry{
		monthly:   monthly,
		daily:     daily,
		fetchedAt: time.Now(),
	})
	return monthly, daily, nil
}
