DROP POLICY IF EXISTS approval_requests_tenant_policy ON approval_requests;
DROP INDEX IF EXISTS idx_approval_requests_pending_exp;
DROP INDEX IF EXISTS idx_approval_requests_user_status_created;
DROP INDEX IF EXISTS idx_approval_requests_org_status_created;
DROP TABLE IF EXISTS approval_requests;

ALTER TABLE tools
    DROP COLUMN IF EXISTS require_approval;
