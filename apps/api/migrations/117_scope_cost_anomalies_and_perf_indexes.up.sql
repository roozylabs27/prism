-- Migration 117: Per-org cost anomaly uniqueness, workspace scope on approval_requests,
-- and composite performance indexes on high-growth audit and request log tables.

-- 1. Allow a user belonging to multiple organizations to receive independent hourly anomalies per org.
DROP INDEX IF EXISTS uq_cost_anomalies_user_window;
CREATE UNIQUE INDEX IF NOT EXISTS uq_cost_anomalies_org_user_window
    ON cost_anomalies (COALESCE(org_id, ''), user_id, metric, window_start)
    WHERE user_id IS NOT NULL;

-- 2. Add optional workspace_id to approval_requests for workspace-scoped governance holds.
ALTER TABLE approval_requests ADD COLUMN IF NOT EXISTS workspace_id VARCHAR(64);
CREATE INDEX IF NOT EXISTS idx_approval_requests_org_ws
    ON approval_requests (org_id, workspace_id)
    WHERE workspace_id IS NOT NULL;

-- 3. Composite indexes supporting dashboard/audit queries under org + workspace predicates.
CREATE INDEX IF NOT EXISTS idx_ai_audit_trails_org_created
    ON ai_audit_trails (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_request_logs_org_ws_created
    ON request_logs (org_id, workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_request_logs_org_created
    ON request_logs (org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_approval_requests_org_status_created
    ON approval_requests (org_id, status, created_at DESC);
