-- Durable transactional/fallback outbox for critical financial (request_logs, gateway key usage)
-- and security (ai_audit_trails) telemetry when primary persistence experiences transient errors.
-- Intentionally NOT tenant-RLS-restricted so the background OutboxWorker can claim batches across orgs
-- and re-establish per-item TenantDBContext before replaying into RLS-protected tables.

CREATE TABLE IF NOT EXISTS delivery_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(64) NOT NULL,
    org_id VARCHAR(64),
    user_id TEXT NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 10,
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_delivery_outbox_status CHECK (status IN ('pending', 'delivered', 'dead_letter'))
);

CREATE INDEX IF NOT EXISTS idx_delivery_outbox_pending
    ON delivery_outbox (status, next_attempt_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_delivery_outbox_org_status
    ON delivery_outbox (org_id, status, created_at DESC);
