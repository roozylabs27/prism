-- Revert migration 115: disable RLS on newly added tables and restore migration 111/114 policies.
DO $$
DECLARE
    tbl TEXT;
BEGIN
    FOREACH tbl IN ARRAY ARRAY[
        'workspaces',
        'organization_members',
        'tenant_quotas',
        'audit_logs',
        'organization_subscriptions',
        'billing_invoices',
        'daily_usage_aggregates',
        'member_invites',
        'delivery_outbox',
        'mcp_registry',
        'settings',
        'projects',
        'workspace_members',
        'budget_alerts',
        'request_payloads',
        'tool_invocations'
    ] LOOP
        IF to_regclass('public.' || tbl) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', tbl);
        END IF;
    END LOOP;
END $$;

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
