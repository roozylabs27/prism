-- P1-6: Persist execution outcomes back onto routing_decisions so historical replay
-- and analytics compare predicted routing choices with actual status, latency, TTFT, retries, and errors.

ALTER TABLE routing_decisions
    ADD COLUMN IF NOT EXISTS status_code INT,
    ADD COLUMN IF NOT EXISTS latency_ms INT,
    ADD COLUMN IF NOT EXISTS ttft_ms INT,
    ADD COLUMN IF NOT EXISTS retry_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error_message TEXT;

CREATE INDEX IF NOT EXISTS idx_routing_decisions_user_created_status
    ON routing_decisions (user_id, created_at DESC, status_code);
