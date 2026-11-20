package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

const (
	defaultOutboxMaxAttempts = 10
	outboxClaimLease         = 60 * time.Second
	outboxBaseBackoff        = 15 * time.Second
	outboxMaxBackoff         = 30 * time.Minute
)

// OutboxRepository persists and leases durable fallback delivery items across
// worker retries until delivery succeeds or max attempts move them to dead_letter.
type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Enqueue inserts a new pending delivery outbox item.
func (r *OutboxRepository) Enqueue(ctx context.Context, item *models.DeliveryOutboxItem) error {
	if item == nil {
		return fmt.Errorf("outbox enqueue: item is nil")
	}
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	if item.MaxAttempts <= 0 {
		item.MaxAttempts = defaultOutboxMaxAttempts
	}
	if item.Status == "" {
		item.Status = models.OutboxStatusPending
	}
	now := time.Now().UTC()
	if item.NextAttemptAt.IsZero() {
		item.NextAttemptAt = now
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO delivery_outbox (
			id, event_type, org_id, user_id, payload, attempts, max_attempts,
			status, last_error, next_attempt_at, created_at, updated_at
		) VALUES (
			$1, $2, NULLIF($3, ''), $4, $5::jsonb, $6, $7, $8, $9, $10, $11, $12
		)`,
		item.ID, item.EventType, item.OrgID, item.UserID, string(item.Payload),
		item.Attempts, item.MaxAttempts, item.Status, item.LastError,
		item.NextAttemptAt, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

// ClaimPendingBatch selects due pending items with FOR UPDATE SKIP LOCKED and
// extends their next_attempt_at lease so multiple replicas never double-replay.
func (r *OutboxRepository) ClaimPendingBatch(ctx context.Context, limit int) ([]models.DeliveryOutboxItem, error) {
	if limit <= 0 {
		limit = 50
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, event_type, COALESCE(org_id, ''), user_id, payload, attempts,
		       max_attempts, status, last_error, next_attempt_at, created_at, updated_at
		FROM delivery_outbox
		WHERE status = $1 AND next_attempt_at <= NOW()
		ORDER BY next_attempt_at ASC, created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED`,
		models.OutboxStatusPending, limit,
	)
	if err != nil {
		return nil, err
	}

	var items []models.DeliveryOutboxItem
	for rows.Next() {
		var item models.DeliveryOutboxItem
		var payloadBytes []byte
		if scanErr := rows.Scan(
			&item.ID, &item.EventType, &item.OrgID, &item.UserID, &payloadBytes,
			&item.Attempts, &item.MaxAttempts, &item.Status, &item.LastError,
			&item.NextAttemptAt, &item.CreatedAt, &item.UpdatedAt,
		); scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		item.Payload = payloadBytes
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, tx.Commit()
	}

	leaseUntil := time.Now().UTC().Add(outboxClaimLease)
	for i := range items {
		items[i].Attempts++
		items[i].NextAttemptAt = leaseUntil
		if _, err := tx.ExecContext(ctx, `
			UPDATE delivery_outbox
			SET attempts = $2, next_attempt_at = $3, updated_at = NOW()
			WHERE id = $1`,
			items[i].ID, items[i].Attempts, leaseUntil,
		); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

// MarkDelivered records successful replay of an outbox item.
func (r *OutboxRepository) MarkDelivered(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE delivery_outbox
		SET status = $2, last_error = '', updated_at = NOW()
		WHERE id = $1`,
		id, models.OutboxStatusDelivered,
	)
	return err
}

// OutboxRetryBackoff returns exponential retry delay bounded to outboxMaxBackoff.
func OutboxRetryBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return outboxBaseBackoff
	}
	shift := attempts - 1
	if shift > 8 {
		shift = 8
	}
	delay := outboxBaseBackoff << shift
	if delay > outboxMaxBackoff || delay <= 0 {
		return outboxMaxBackoff
	}
	return delay
}

// MarkFailed schedules a retry with exponential backoff or moves the item to
// dead_letter once attempts reach maxAttempts. Returns true when dead-lettered.
func (r *OutboxRepository) MarkFailed(ctx context.Context, id string, attempts, maxAttempts int, errMsg string) (bool, error) {
	if maxAttempts <= 0 {
		maxAttempts = defaultOutboxMaxAttempts
	}
	if attempts >= maxAttempts {
		_, err := r.db.ExecContext(ctx, `
			UPDATE delivery_outbox
			SET status = $2, last_error = $3, updated_at = NOW()
			WHERE id = $1`,
			id, models.OutboxStatusDeadLetter, errMsg,
		)
		return err == nil, err
	}

	nextAttempt := time.Now().UTC().Add(OutboxRetryBackoff(attempts))
	_, err := r.db.ExecContext(ctx, `
		UPDATE delivery_outbox
		SET status = $2, last_error = $3, next_attempt_at = $4, updated_at = NOW()
		WHERE id = $1`,
		id, models.OutboxStatusPending, errMsg, nextAttempt,
	)
	return false, err
}

// CountByStatus reports current pending and dead-letter outbox queue depths.
func (r *OutboxRepository) CountByStatus(ctx context.Context) (pending int64, deadLetter int64, err error) {
	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(COUNT(*) FILTER (WHERE status = 'pending'), 0),
			COALESCE(COUNT(*) FILTER (WHERE status = 'dead_letter'), 0)
		FROM delivery_outbox`,
	).Scan(&pending, &deadLetter)
	return pending, deadLetter, err
}
