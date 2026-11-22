package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/telemetry"
)

const outboxBatchLimit = 50

// OutboxStore claims due fallback delivery items and records their outcome.
type OutboxStore interface {
	ClaimPendingBatch(ctx context.Context, limit int) ([]models.DeliveryOutboxItem, error)
	MarkDelivered(ctx context.Context, id string) error
	MarkFailed(ctx context.Context, id string, attempts, maxAttempts int, errMsg string) (deadLettered bool, err error)
}

// RequestLogCreator inserts a RequestLog row into primary storage.
type RequestLogCreator interface {
	Create(ctx context.Context, log *models.RequestLog) error
}

// AuditTrailRecorder inserts (and signs if needed) an AIAuditTrail row.
type AuditTrailRecorder interface {
	Record(ctx context.Context, trail *models.AIAuditTrail) error
}

// GatewayKeyUsageIncrementer increments request counters on a GatewayAPIKey.
type GatewayKeyUsageIncrementer interface {
	IncrementUsage(ctx context.Context, keyID string) error
}

// OutboxWorker replays failed financial and audit persistence events from
// delivery_outbox until they succeed or exhaust retry attempts into dead_letter.
type OutboxWorker struct {
	outbox      OutboxStore
	requestLogs RequestLogCreator
	audits      AuditTrailRecorder
	gatewayKeys GatewayKeyUsageIncrementer
}

func NewOutboxWorker(
	outbox OutboxStore,
	requestLogs RequestLogCreator,
	audits AuditTrailRecorder,
	gatewayKeys GatewayKeyUsageIncrementer,
) *OutboxWorker {
	return &OutboxWorker{
		outbox:      outbox,
		requestLogs: requestLogs,
		audits:      audits,
		gatewayKeys: gatewayKeys,
	}
}

type errPermanentOutbox struct {
	err error
}

func (e errPermanentOutbox) Error() string { return e.err.Error() }
func (e errPermanentOutbox) Unwrap() error { return e.err }

// Run claims a batch of due delivery_outbox rows and replays each under its
// original tenant context.
func (w *OutboxWorker) Run(ctx context.Context) {
	if w == nil || w.outbox == nil {
		return
	}

	sysCtx := database.WithSystemBypassRLS(ctx)
	items, err := w.outbox.ClaimPendingBatch(sysCtx, outboxBatchLimit)
	if err != nil {
		log.Printf("[outbox] claim batch failed: %v", err)
		return
	}

	for _, item := range items {
		replayErr := w.dispatch(ctx, item)
		if replayErr == nil {
			if markErr := w.outbox.MarkDelivered(sysCtx, item.ID); markErr != nil {
				log.Printf("[outbox] mark delivered %s (%s): %v", item.ID, item.EventType, markErr)
				continue
			}
			telemetry.RecordOutboxEvent(ctx, item.EventType, "delivered")
			continue
		}

		attempts := item.Attempts
		maxAttempts := item.MaxAttempts
		var perm errPermanentOutbox
		if errors.As(replayErr, &perm) {
			// Permanent validation/unknown-type error: escalate directly to dead_letter.
			if maxAttempts <= 0 {
				maxAttempts = 1
			}
			attempts = maxAttempts
		}

		deadLettered, markErr := w.outbox.MarkFailed(sysCtx, item.ID, attempts, maxAttempts, replayErr.Error())
		if markErr != nil {
			log.Printf("[outbox] mark failed %s (%s): %v", item.ID, item.EventType, markErr)
			continue
		}
		if deadLettered {
			telemetry.RecordOutboxEvent(ctx, item.EventType, "dead_letter")
			log.Printf("[outbox] DEAD LETTER item %s type=%s org=%s attempts=%d: %v", item.ID, item.EventType, item.OrgID, attempts, replayErr)
		} else {
			telemetry.RecordOutboxEvent(ctx, item.EventType, "failed")
		}
	}
}

func (w *OutboxWorker) dispatch(ctx context.Context, item models.DeliveryOutboxItem) error {
	tenantCtx := database.WithTenantContext(ctx, database.TenantDBContext{
		UserID: item.UserID,
		OrgID:  item.OrgID,
	})

	switch item.EventType {
	case models.OutboxEventRequestLog:
		if w.requestLogs == nil {
			return fmt.Errorf("request log store not configured")
		}
		logEntry, err := models.DecodeOutboxRequestLog(item.Payload)
		if err != nil {
			return errPermanentOutbox{err: fmt.Errorf("decode request_log payload: %w", err)}
		}
		itemCtx := database.WithTenantContext(ctx, database.TenantDBContext{
			UserID:      item.UserID,
			OrgID:       logEntry.OrgID,
			WorkspaceID: logEntry.WorkspaceID,
			ProjectID:   logEntry.ProjectID,
		})
		if logEntry.OrgID == "" && item.OrgID != "" {
			logEntry.OrgID = item.OrgID
			itemCtx = database.WithTenantContext(ctx, database.TenantDBContext{
				UserID:      item.UserID,
				OrgID:       item.OrgID,
				WorkspaceID: logEntry.WorkspaceID,
				ProjectID:   logEntry.ProjectID,
			})
		}
		return w.requestLogs.Create(itemCtx, logEntry)

	case models.OutboxEventAIAuditTrail:
		if w.audits == nil {
			return fmt.Errorf("audit trail recorder not configured")
		}
		var trail models.AIAuditTrail
		if err := json.Unmarshal(item.Payload, &trail); err != nil {
			return errPermanentOutbox{err: fmt.Errorf("decode ai_audit_trail payload: %w", err)}
		}
		if trail.OrgID == "" && item.OrgID != "" {
			trail.OrgID = item.OrgID
		}
		if trail.UserID == "" && item.UserID != "" {
			trail.UserID = item.UserID
		}
		itemCtx := database.WithTenantContext(ctx, database.TenantDBContext{
			UserID: trail.UserID,
			OrgID:  trail.OrgID,
		})
		return w.audits.Record(itemCtx, &trail)

	case models.OutboxEventGatewayKeyUsage:
		if w.gatewayKeys == nil {
			return fmt.Errorf("gateway key repository not configured")
		}
		var p models.OutboxGatewayKeyUsagePayload
		if err := json.Unmarshal(item.Payload, &p); err != nil {
			return errPermanentOutbox{err: fmt.Errorf("decode gateway_key_usage payload: %w", err)}
		}
		if p.GatewayKeyID == "" {
			return errPermanentOutbox{err: errors.New("gateway_key_usage payload missing gatewayKeyId")}
		}
		orgID := p.OrgID
		if orgID == "" {
			orgID = item.OrgID
		}
		userID := p.UserID
		if userID == "" {
			userID = item.UserID
		}
		itemCtx := database.WithTenantContext(tenantCtx, database.TenantDBContext{
			UserID: userID,
			OrgID:  orgID,
		})
		return w.gatewayKeys.IncrementUsage(itemCtx, p.GatewayKeyID)

	default:
		return errPermanentOutbox{err: fmt.Errorf("unknown outbox event_type %q", item.EventType)}
	}
}
