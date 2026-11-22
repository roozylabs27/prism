-- Rollback Migration 117
DROP INDEX IF EXISTS idx_request_logs_org_created;
DROP INDEX IF EXISTS idx_request_logs_org_ws_created;
DROP INDEX IF EXISTS idx_ai_audit_trails_org_created;
DROP INDEX IF EXISTS idx_approval_requests_org_ws;
ALTER TABLE approval_requests DROP COLUMN IF EXISTS workspace_id;

DROP INDEX IF EXISTS uq_cost_anomalies_org_user_window;
CREATE UNIQUE INDEX IF NOT EXISTS uq_cost_anomalies_user_window
    ON cost_anomalies (user_id, metric, window_start)
    WHERE user_id IS NOT NULL;
