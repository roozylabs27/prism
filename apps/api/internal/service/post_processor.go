package service

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type postProcessLogStore interface {
	Create(ctx context.Context, log *models.RequestLog) error
}

type postProcessKeyStore interface {
	IncrementUsage(ctx context.Context, keyID string) error
}

type postProcessAuditStore interface {
	Record(ctx context.Context, trail *models.AIAuditTrail) error
}

type PostProcessTask struct {
	Ctx              context.Context
	RequestID        string
	Log              *models.RequestLog
	GatewayKey       *models.GatewayAPIKey
	AgentID          string
	AgentName        string
	UserRole         string
	ClientIP         string
	UserAgent        string
	ClientApp        string
	PromptHash       string
	ToolsInvoked     []string
	MCPServersCalled []string
}

type AsyncPostProcessor struct {
	taskChan       chan *PostProcessTask
	wg             sync.WaitGroup
	requestLogs    postProcessLogStore
	gatewayKeys    postProcessKeyStore
	eventPublisher *goredis.EventPublisher
	auditRecorder  postProcessAuditStore
	outbox         OutboxEnqueuer
	workersCount   int

	// overflow bounds the extra goroutines used when the buffer is full.
	overflow chan struct{}
	// mu guards stopped and taskChan's closing against concurrent Enqueue.
	mu      sync.RWMutex
	stopped bool
}

func NewAsyncPostProcessor(
	requestLogs *repository.RequestLogRepository,
	gatewayKeys *repository.GatewayKeyRepository,
	eventPublisher *goredis.EventPublisher,
	auditRecorder *proxy.AuditRecorder,
	bufferSize int,
	workersCount int,
) *AsyncPostProcessor {
	if bufferSize <= 0 {
		bufferSize = 2000
	}
	if workersCount <= 0 {
		workersCount = 10
	}

	p := &AsyncPostProcessor{
		taskChan:       make(chan *PostProcessTask, bufferSize),
		eventPublisher: eventPublisher,
		workersCount:   workersCount,
		overflow:       make(chan struct{}, workersCount*4),
	}
	if requestLogs != nil {
		p.requestLogs = requestLogs
	}
	if gatewayKeys != nil {
		p.gatewayKeys = gatewayKeys
	}
	if auditRecorder != nil {
		p.auditRecorder = auditRecorder
	}

	p.start()
	return p
}

// SetOutbox attaches a durable delivery outbox used to queue request logs,
// key usage increments, and audit trails when synchronous DB delivery fails.
func (p *AsyncPostProcessor) SetOutbox(outbox OutboxEnqueuer) {
	if p == nil || isNilInterface(outbox) {
		return
	}
	p.mu.Lock()
	p.outbox = outbox
	p.mu.Unlock()
}

func (p *AsyncPostProcessor) getOutbox() OutboxEnqueuer {
	p.mu.RLock()
	o := p.outbox
	p.mu.RUnlock()
	return o
}

func (p *AsyncPostProcessor) start() {
	for i := 0; i < p.workersCount; i++ {
		p.wg.Add(1)
		go p.workerLoop()
	}
}

func (p *AsyncPostProcessor) workerLoop() {
	defer p.wg.Done()
	// Runs until Stop closes the channel, so queued tasks are drained first.
	for task := range p.taskChan {
		p.processTask(task)
	}
}

// Enqueue hands the task to the workers. When the buffer is full it runs the
// task on a bounded number of extra goroutines, and past that in the caller,
// so request logs slow requests down instead of being dropped or piling up
// goroutines. After Stop it runs the task in the caller. It reports whether
// the task was buffered.
func (p *AsyncPostProcessor) Enqueue(task *PostProcessTask) bool {
	if task == nil || task.Log == nil {
		return false
	}

	p.mu.RLock()
	if p.stopped {
		p.mu.RUnlock()
		p.processTask(task)
		return false
	}
	select {
	case p.taskChan <- task:
		p.mu.RUnlock()
		return true
	default:
	}
	select {
	case p.overflow <- struct{}{}:
		p.wg.Add(1)
		p.mu.RUnlock()
		go func() {
			defer p.wg.Done()
			defer func() { <-p.overflow }()
			p.processTask(task)
		}()
	default:
		p.mu.RUnlock()
		log.Printf("[AsyncPostProcessor] Warning: task buffer (size %d) and overflow full, processing in the request", cap(p.taskChan))
		p.processTask(task)
	}
	return false
}

func (p *AsyncPostProcessor) processTask(task *PostProcessTask) {
	if task == nil || task.Log == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[AsyncPostProcessor] Panic while processing request %s: %v", task.RequestID, r)
		}
	}()

	if task.Log.CreatedAt.IsZero() {
		task.Log.CreatedAt = time.Now().UTC()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ownerID := ""
	if task.GatewayKey != nil {
		ownerID = task.GatewayKey.UserID
	}
	ctx = database.WithTenantContext(ctx, database.TenantDBContext{
		UserID:      ownerID,
		OrgID:       task.Log.OrgID,
		WorkspaceID: task.Log.WorkspaceID,
		ProjectID:   task.Log.ProjectID,
	})

	outbox := p.getOutbox()

	// 1. Save Request Log to PostgreSQL (fallback to durable outbox on error)
	if !isNilInterface(p.requestLogs) {
		if err := p.requestLogs.Create(ctx, task.Log); err != nil {
			log.Printf("[AsyncPostProcessor] Error persisting request log (%s), falling back to outbox: %v", task.RequestID, err)
			enqueueOutboxFallback(ctx, outbox, models.OutboxEventRequestLog, task.Log.OrgID, ownerID, task.Log, err)
		}
	}

	// 2. Increment Gateway Key usage counter (fallback to durable outbox on error)
	if !isNilInterface(p.gatewayKeys) && task.GatewayKey != nil && task.GatewayKey.ID != "" {
		if err := p.gatewayKeys.IncrementUsage(ctx, task.GatewayKey.ID); err != nil {
			log.Printf("[AsyncPostProcessor] Error incrementing key usage (%s), falling back to outbox: %v", task.GatewayKey.ID, err)
			enqueueOutboxFallback(ctx, outbox, models.OutboxEventGatewayKeyUsage, task.Log.OrgID, ownerID, models.OutboxGatewayKeyUsagePayload{
				GatewayKeyID: task.GatewayKey.ID,
				OrgID:        task.Log.OrgID,
				UserID:       ownerID,
			}, err)
		}
	}

	// 3. Publish Redis SSE telemetry event
	if p.eventPublisher != nil {
		keyPrefix := ""
		gatewayKeyName := ""
		if task.GatewayKey != nil {
			keyPrefix = task.GatewayKey.KeyPrefix
			gatewayKeyName = task.GatewayKey.Name
		}

		eventData := map[string]interface{}{
			"requestId":      task.RequestID,
			"model":          task.Log.Model,
			"provider":       task.Log.ProviderType,
			"statusCode":     task.Log.StatusCode,
			"latencyMs":      task.Log.LatencyMs,
			"inputTokens":    task.Log.InputTokens,
			"outputTokens":   task.Log.OutputTokens,
			"totalTokens":    task.Log.TotalTokens,
			"costUsd":        task.Log.CostUSD,
			"clientApp":      task.Log.ClientApp,
			"orgId":          task.Log.OrgID,
			"workspaceId":    task.Log.WorkspaceID,
			"projectId":      task.Log.ProjectID,
			"keyPrefix":      keyPrefix,
			"gatewayKeyName": gatewayKeyName,
			"timestamp":      task.Log.CreatedAt,
		}
		_ = p.eventPublisher.Publish(ctx, ownerID, "request_log_created", eventData)
	}

	// 4. Record Cryptographic Audit Trail (fallback to durable outbox on error)
	if !isNilInterface(p.auditRecorder) {
		trail := buildSuccessAuditTrail(
			task.Log,
			task.GatewayKey,
			task.AgentID,
			task.AgentName,
			task.UserRole,
			task.PromptHash,
			task.ToolsInvoked,
			task.MCPServersCalled,
		)
		if trail != nil {
			if err := p.auditRecorder.Record(ctx, trail); err != nil {
				log.Printf("[AsyncPostProcessor] Error persisting audit trail (%s), falling back to outbox: %v", task.RequestID, err)
				enqueueOutboxFallback(ctx, outbox, models.OutboxEventAIAuditTrail, trail.OrgID, trail.UserID, trail, err)
			}
		}
	}
}

// Stop drains the queued tasks and waits for them. It is safe to call more
// than once and while requests are still enqueueing.
func (p *AsyncPostProcessor) Stop() {
	p.mu.Lock()
	if !p.stopped {
		p.stopped = true
		close(p.taskChan)
	}
	p.mu.Unlock()
	p.wg.Wait()
}
