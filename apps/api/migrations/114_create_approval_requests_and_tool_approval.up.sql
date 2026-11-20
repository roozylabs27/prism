-- P1-4: Human-in-the-loop (HITL) approval workflow for sensitive Tool & MCP calls.

ALTER TABLE tools
    ADD COLUMN IF NOT EXISTS require_approval BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS approval_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id VARCHAR(64),
    user_id TEXT NOT NULL,
    agent_id VARCHAR(64),
    agent_name VARCHAR(255) NOT NULL DEFAULT '',
    tool_name VARCHAR(255) NOT NULL,
    mcp_server_name VARCHAR(255) NOT NULL DEFAULT '',
    arguments_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    arguments_hash VARCHAR(64) NOT NULL,
    policy_id VARCHAR(64) NOT NULL DEFAULT '',
    policy_name VARCHAR(255) NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'pending',
    decided_by TEXT,
    decision_note TEXT,
    decided_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '24 hours'),
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_approval_requests_status
        CHECK (status IN ('pending', 'approved', 'denied', 'expired', 'consumed'))
);

CREATE INDEX IF NOT EXISTS idx_approval_requests_org_status_created
    ON approval_requests (org_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_approval_requests_user_status_created
    ON approval_requests (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_approval_requests_pending_exp
    ON approval_requests (expires_at)
    WHERE status = 'pending';

ALTER TABLE approval_requests ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS approval_requests_tenant_policy ON approval_requests;
CREATE POLICY approval_requests_tenant_policy ON approval_requests FOR ALL
USING (
    (
        NULLIF(current_setting('app.current_user_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_org_id', true), '') IS NULL
    )
    OR (
        org_id IS NOT NULL
        AND org_id <> ''
        AND org_id = NULLIF(current_setting('app.current_org_id', true), '')
    )
    OR (
        (org_id IS NULL OR org_id = '')
        AND user_id = NULLIF(current_setting('app.current_user_id', true), '')
    )
)
WITH CHECK (
    (
        NULLIF(current_setting('app.current_user_id', true), '') IS NULL
        AND NULLIF(current_setting('app.current_org_id', true), '') IS NULL
    )
    OR (
        org_id IS NOT NULL
        AND org_id <> ''
        AND org_id = NULLIF(current_setting('app.current_org_id', true), '')
    )
    OR (
        (org_id IS NULL OR org_id = '')
        AND user_id = NULLIF(current_setting('app.current_user_id', true), '')
    )
);
