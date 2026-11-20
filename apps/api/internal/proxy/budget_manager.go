package proxy

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
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

// DistributedBudgetStore coordinates in-flight and short-TTL settled budget reservations
// across multiple API replicas (backed by Redis Lua scripts in production).
type DistributedBudgetStore interface {
	TryReserve(ctx context.Context, in goredis.BudgetReserveInput) (*goredis.BudgetReserveResult, error)
	Release(ctx context.Context, scopeKey, reservationID string, actualCost float64) (int64, error)
	InFlightAndOptimistic(ctx context.Context, scopeKey string) (*goredis.BudgetSnapshot, error)
}

// BudgetReservation represents an in-flight spend hold for a single request.
// Callers must pass it to Release once the request finishes or fails.
type BudgetReservation struct {
	UserID          string
	ScopeKey        string
	ReservedUSD     float64
	ReservationID   string
	usedDistributed bool
	released        bool
}

type BudgetManager struct {
	budgetRepo  budgetStore
	distStore   DistributedBudgetStore
	failClosed  bool
	spendCache  sync.Map // scopeKey -> budgetSpendCacheEntry
	userToScope sync.Map // userID -> latest scopeKey (supports InvalidateSpendCache(userID))

	mu       sync.Mutex
	inFlight map[string]float64 // scopeKey -> reserved in-flight USD
}

// SetDistributedStore attaches a shared multi-replica budget store (Redis Lua).
func (bm *BudgetManager) SetDistributedStore(store DistributedBudgetStore) {
	if bm != nil {
		bm.distStore = store
	}
}

// SetFailClosed configures whether distributed store failures abort requests instead
// of falling back to local in-process reservation state.
func (bm *BudgetManager) SetFailClosed(failClosed bool) {
	if bm != nil {
		bm.failClosed = failClosed
	}
}

func budgetScopeKey(ctx context.Context, userID string) string {
	if orgID := database.TenantContextFrom(ctx).OrgID; orgID != "" {
		return "org:" + orgID
	}
	return "user:" + userID
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
	scopeKey := budgetScopeKey(ctx, userID)
	bm.userToScope.Store(userID, scopeKey)
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
			m, d, err := bm.getSpendCached(ctx, scopeKey, userID)
			if err != nil {
				return nil, err
			}
			inFlight := bm.inFlightSpend(scopeKey)
			if bm.distStore != nil {
				snap, distErr := bm.distStore.InFlightAndOptimistic(ctx, scopeKey)
				if distErr != nil {
					if bm.failClosed {
						return nil, distErr
					}
				} else if snap != nil {
					inFlight = snap.InFlight + snap.RecentSettled
				}
			}
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

	scopeKey := budgetScopeKey(ctx, userID)
	bm.userToScope.Store(userID, scopeKey)

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

	monthlyDB, dailyDB, err := bm.getSpendCached(ctx, scopeKey, userID)
	if err != nil {
		return nil, nil, err
	}

	if bm.distStore != nil {
		res, status, distErr := bm.reserveDistributed(ctx, userID, scopeKey, enabledBudgets, monthlyDB, dailyDB, estimatedCostUSD)
		if distErr == nil {
			return res, status, nil
		}
		if bm.failClosed {
			return nil, nil, distErr
		}
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.inFlight == nil {
		bm.inFlight = make(map[string]float64)
	}
	if v, ok := bm.spendCache.Load(scopeKey); ok {
		entry := v.(budgetSpendCacheEntry)
		if time.Since(entry.fetchedAt) < budgetSpendCacheTTL {
			monthlyDB = entry.monthly
			dailyDB = entry.daily
		}
	}

	inFlight := bm.inFlight[scopeKey]
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

	bm.inFlight[scopeKey] = inFlight + estimatedCostUSD
	return &BudgetReservation{
		UserID:      userID,
		ScopeKey:    scopeKey,
		ReservedUSD: estimatedCostUSD,
	}, worst, nil
}

func (bm *BudgetManager) reserveDistributed(
	ctx context.Context,
	userID, scopeKey string,
	enabledBudgets []models.Budget,
	monthlyDB, dailyDB, estimatedCostUSD float64,
) (*BudgetReservation, *models.BudgetStatus, error) {
	var tightestMonthly, tightestDaily float64
	var tightestMonthlyBudget, tightestDailyBudget *models.Budget
	hasHardLimit := false

	for i := range enabledBudgets {
		b := &enabledBudgets[i]
		if !b.HardLimit {
			continue
		}
		hasHardLimit = true
		if b.MonthlyLimit > 0 && (tightestMonthly == 0 || b.MonthlyLimit < tightestMonthly) {
			tightestMonthly = b.MonthlyLimit
			tightestMonthlyBudget = b
		}
		if b.DailyLimit > 0 && (tightestDaily == 0 || b.DailyLimit < tightestDaily) {
			tightestDaily = b.DailyLimit
			tightestDailyBudget = b
		}
	}

	res, err := bm.distStore.TryReserve(ctx, goredis.BudgetReserveInput{
		ScopeKey:      scopeKey,
		EstimatedCost: estimatedCostUSD,
		MonthlySpent:  monthlyDB,
		DailySpent:    dailyDB,
		MonthlyLimit:  tightestMonthly,
		DailyLimit:    tightestDaily,
		HardStop:      hasHardLimit,
		TTL:           10 * time.Minute,
	})
	if err != nil {
		return nil, nil, err
	}

	if !res.Allowed {
		evalMonthly := monthlyDB + res.RecentSettled + res.InFlight + estimatedCostUSD
		evalDaily := dailyDB + res.RecentSettled + res.InFlight + estimatedCostUSD
		targetBudget := tightestMonthlyBudget
		if res.ExceededPeriod == "daily" || targetBudget == nil {
			targetBudget = tightestDailyBudget
		}
		if targetBudget == nil {
			targetBudget = &enabledBudgets[0]
		}
		st := evaluateBudget(targetBudget, evalMonthly, evalDaily)
		st.Status = "exceeded"
		if res.ExceededPeriod != "" {
			st.LimitPeriod = res.ExceededPeriod
		}
		return nil, st, nil
	}

	preReserveInFlight := res.InFlight - estimatedCostUSD
	if preReserveInFlight < 0 {
		preReserveInFlight = 0
	}
	evalMonthly := monthlyDB + res.RecentSettled + preReserveInFlight
	evalDaily := dailyDB + res.RecentSettled + preReserveInFlight

	var worst *models.BudgetStatus
	for i := range enabledBudgets {
		st := evaluateBudget(&enabledBudgets[i], evalMonthly, evalDaily)
		if worst == nil || worseBudget(st, worst) {
			worst = st
		}
	}

	return &BudgetReservation{
		UserID:          userID,
		ScopeKey:        scopeKey,
		ReservedUSD:     estimatedCostUSD,
		ReservationID:   res.ReservationID,
		usedDistributed: true,
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
	if res.released {
		bm.mu.Unlock()
		return
	}
	res.released = true

	scopeKey := res.ScopeKey
	if scopeKey == "" {
		scopeKey = "user:" + res.UserID
	}

	usedDistributed := res.usedDistributed
	reservationID := res.ReservationID
	if !usedDistributed {
		if bm.inFlight != nil {
			next := bm.inFlight[scopeKey] - res.ReservedUSD
			if next <= 1e-12 {
				delete(bm.inFlight, scopeKey)
			} else {
				bm.inFlight[scopeKey] = next
			}
		}

		if actualCostUSD > 0 {
			if v, ok := bm.spendCache.Load(scopeKey); ok {
				entry := v.(budgetSpendCacheEntry)
				entry.monthly += actualCostUSD
				entry.daily += actualCostUSD
				bm.spendCache.Store(scopeKey, entry)
			}
		}
	}
	bm.mu.Unlock()

	if usedDistributed && bm.distStore != nil && reservationID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = bm.distStore.Release(ctx, scopeKey, reservationID, actualCostUSD)
	}
}

// InvalidateSpendCache clears cached DB spend entries associated with userID.
func (bm *BudgetManager) InvalidateSpendCache(userID string) {
	if bm == nil || userID == "" {
		return
	}
	bm.spendCache.Delete(userID)
	bm.spendCache.Delete("user:" + userID)
	if v, ok := bm.userToScope.Load(userID); ok {
		if scopeKey, ok := v.(string); ok && scopeKey != "" {
			bm.spendCache.Delete(scopeKey)
		}
	}
}

// InvalidateSpendCacheForOrg clears cached DB spend for orgID.
func (bm *BudgetManager) InvalidateSpendCacheForOrg(orgID string) {
	if bm == nil || orgID == "" {
		return
	}
	bm.spendCache.Delete("org:" + orgID)
}

func (bm *BudgetManager) inFlightSpend(scopeKey string) float64 {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	if bm.inFlight == nil {
		return 0
	}
	return bm.inFlight[scopeKey]
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

func (bm *BudgetManager) getSpendCached(ctx context.Context, scopeKey, userID string) (float64, float64, error) {
	if v, ok := bm.spendCache.Load(scopeKey); ok {
		entry := v.(budgetSpendCacheEntry)
		if time.Since(entry.fetchedAt) < budgetSpendCacheTTL {
			return entry.monthly, entry.daily, nil
		}
		bm.spendCache.Delete(scopeKey)
	}
	monthly, daily, err := bm.budgetRepo.GetCombinedSpend(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	bm.spendCache.Store(scopeKey, budgetSpendCacheEntry{
		monthly:   monthly,
		daily:     daily,
		fetchedAt: time.Now(),
	})
	return monthly, daily, nil
}
