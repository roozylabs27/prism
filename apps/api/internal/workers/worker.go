package workers

import (
	"context"
	"log"
	"math/rand/v2"
	"sync"
	"time"
)

type Manager struct {
	mu   sync.Mutex
	wg   sync.WaitGroup
	jobs []job
}

type job struct {
	name     string
	interval time.Duration
	fn       func(ctx context.Context)
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) Register(name string, every time.Duration, fn func(ctx context.Context)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.jobs = append(m.jobs, job{name: name, interval: every, fn: fn})
}

func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		m.wg.Add(1)
		go m.run(ctx, j)
	}
}

// Shutdown waits for all worker goroutines to exit after ctx cancellation, or
// returns ctx.Err() if the shutdown deadline expires first.
func (m *Manager) Shutdown(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.wg.Wait()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// maxStartJitter caps the delay before a job's first run.
var maxStartJitter = 2 * time.Minute

// firstRunDelay picks when a job first runs after start: soon, so a job is
// not skipped when the server restarts more often than its interval (a deploy
// every half hour would otherwise starve an hourly job), and jittered so jobs
// and instances do not all start at once.
func firstRunDelay(interval time.Duration) time.Duration {
	limit := min(interval/4, maxStartJitter)
	if limit <= 0 {
		return 0
	}
	return rand.N(limit)
}

func (m *Manager) run(ctx context.Context, j job) {
	defer m.wg.Done()
	first := time.NewTimer(firstRunDelay(j.interval))
	defer first.Stop()
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	log.Printf("[worker] %s started (every %v)", j.name, j.interval)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[worker] %s stopped", j.name)
			return
		case <-first.C:
		case <-ticker.C:
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[worker] %s panic: %v", j.name, r)
				}
			}()
			j.fn(ctx)
		}()
	}
}
