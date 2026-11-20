DROP INDEX IF EXISTS idx_routing_decisions_user_created_status;

ALTER TABLE routing_decisions
    DROP COLUMN IF EXISTS error_message,
    DROP COLUMN IF EXISTS retry_count,
    DROP COLUMN IF EXISTS ttft_ms,
    DROP COLUMN IF EXISTS latency_ms,
    DROP COLUMN IF EXISTS status_code;
