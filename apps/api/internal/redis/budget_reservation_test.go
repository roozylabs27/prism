package redis

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func TestBudgetReservationStore_NilAndValidation(t *testing.T) {
	var nilStore *BudgetReservationStore
	ctx := context.Background()

	if _, err := nilStore.TryReserve(ctx, BudgetReserveInput{ScopeKey: "org:1", EstimatedCost: 1.0}); err == nil {
		t.Fatal("expected error when TryReserve called on nil store")
	}
	if _, err := nilStore.Release(ctx, "org:1", "r1", 0.5); err == nil {
		t.Fatal("expected error when Release called on nil store")
	}
	if _, err := nilStore.InFlightAndOptimistic(ctx, "org:1"); err == nil {
		t.Fatal("expected error when InFlightAndOptimistic called on nil store")
	}

	emptyStore := NewBudgetReservationStore(nil)
	if _, err := emptyStore.TryReserve(ctx, BudgetReserveInput{ScopeKey: "org:1"}); err == nil {
		t.Fatal("expected error when Redis client is nil")
	}
}

func TestBudgetReservationStore_KeysAndScriptsInitialized(t *testing.T) {
	keys := budgetKeys("org:acme")
	if len(keys) != 4 {
		t.Fatalf("expected 4 budget keys, got %d", len(keys))
	}
	if reserveSpendScript == nil || releaseSpendScript == nil || inFlightGetScript == nil {
		t.Fatal("expected Lua scripts to be initialized")
	}
}

func TestBudgetReservationStore_LiveRedisConcurrentReserve(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("skipping live Redis test (REDIS_URL not set)")
	}
	opt, err := goredis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("invalid REDIS_URL: %v", err)
	}
	rdb := goredis.NewClient(opt)
	defer rdb.Close()

	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("skipping live Redis test (ping failed: %v)", err)
	}

	scope := "test:budget:" + newReservationID()
	for _, k := range budgetKeys(scope) {
		_ = rdb.Del(ctx, k).Err()
	}
	defer func() {
		for _, k := range budgetKeys(scope) {
			_ = rdb.Del(ctx, k).Err()
		}
	}()

	store := NewBudgetReservationStore(rdb)
	var allowedCount atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := store.TryReserve(ctx, BudgetReserveInput{
				ScopeKey:      scope,
				EstimatedCost: 0.40,
				MonthlyLimit:  1.00,
				HardStop:      true,
				TTL:           time.Minute,
			})
			if err != nil {
				t.Errorf("TryReserve error: %v", err)
				return
			}
			if res.Allowed {
				allowedCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := allowedCount.Load(); got != 2 {
		t.Fatalf("expected exactly 2 reservations under $1.00 cap ($0.40 each), got %d", got)
	}
}
