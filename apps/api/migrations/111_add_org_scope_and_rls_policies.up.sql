-- Wave 2 (P0-6 / D-10): Add org_id (+ workspace_id on budgets) across remaining
-- domain tables, backfill NULL/empty org_id values from organization memberships,
-- and harden PostgreSQL RLS policies across all 17 multi-tenant tables without any
-- hardcoded 'user_admin' bypass.

-- 1. Add org_id (and workspace_id on budgets) to the 7 remaining domain tables.
ALTER TABLE budgets
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS workspace_id VARCHAR(64) REFERENCES workspaces(id) ON DELETE SET NULL;

ALTER TABLE routing_policies
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE routing_rules
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE agent_templates
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE routing_decisions
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE cost_anomalies
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

ALTER TABLE mcp_server_invocations
    ADD COLUMN IF NOT EXISTS org_id VARCHAR(64) REFERENCES organizations(id) ON DELETE SET NULL;

-- 2. Indexes on new tenant columns.
CREATE INDEX IF NOT EXISTS idx_budgets_org_id ON budgets(org_id);
CREATE INDEX IF NOT EXISTS idx_budgets_workspace_id ON budgets(workspace_id);
CREATE INDEX IF NOT EXISTS idx_routing_policies_org_id ON routing_policies(org_id);
CREATE INDEX IF NOT EXISTS idx_routing_rules_org_id ON routing_rules(org_id);
CREATE INDEX IF NOT EXISTS idx_agent_templates_org_id ON agent_templates(org_id);
CREATE INDEX IF NOT EXISTS idx_routing_decisions_org_id ON routing_decisions(org_id);
CREATE INDEX IF NOT EXISTS idx_cost_anomalies_org_created ON cost_anomalies(org_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_server_invocations_org_created ON mcp_server_invocations(org_id, created_at DESC);

-- 3. Backfill org_id where NULL or empty from related parent records or earliest organization membership.
-- request_logs has no user_id column; it inherits ownership via gateway_api_key_id.
UPDATE request_logs rl
SET org_id = COALESCE(
    NULLIF(gak.org_id, ''),
    (SELECT om.org_id FROM organization_members om WHERE om.user_id = gak.user_id ORDER BY om.created_at ASC LIMIT 1)
)
FROM gateway_api_keys gak
WHERE rl.gateway_api_key_id = gak.id
  AND (rl.org_id IS NULL OR rl.org_id = '');

UPDATE routing_decisions rd
SET org_id = COALESCE(
    (SELECT NULLIF(rl.org_id, '') FROM request_logs rl WHERE rl.request_id = rd.request_id LIMIT 1),
    (SELECT om.org_id FROM organization_members om WHERE om.user_id = rd.user_id ORDER BY om.created_at ASC LIMIT 1)
)
WHERE (rd.org_id IS NULL OR rd.org_id = '')
  AND rd.user_id IS NOT NULL AND rd.user_id <> '';

UPDATE mcp_server_invocations msi
SET org_id = COALESCE(
    (SELECT NULLIF(ms.org_id, '') FROM mcp_servers ms WHERE ms.id = msi.mcp_server_id LIMIT 1),
    (SELECT om.org_id FROM organization_members om WHERE om.user_id = msi.user_id ORDER BY om.created_at ASC LIMIT 1)
)
WHERE (msi.org_id IS NULL OR msi.org_id = '');

DO $$
DECLARE
    tbl TEXT;
    backfill_tables TEXT[] := ARRAY[
        'providers',
        'credentials',
        'gateway_api_keys',
        'agents',
        'tools',
        'resources',
        'mcp_servers',
        'governance_policies',
        'ai_audit_trails',
        'budgets',
        'routing_policies',
        'routing_rules',
        'cost_anomalies'
    ];
BEGIN
    FOREACH tbl IN ARRAY backfill_tables LOOP
        EXECUTE format(
            'UPDATE %I t
             SET org_id = (
                 SELECT om.org_id
                 FROM organization_members om
                 WHERE om.user_id = t.user_id
                 ORDER BY om.created_at ASC
                 LIMIT 1
             )
             WHERE (t.org_id IS NULL OR t.org_id = '''')
               AND t.user_id IS NOT NULL
               AND t.user_id <> ''''
               AND EXISTS (SELECT 1 FROM organization_members om WHERE om.user_id = t.user_id)',
            tbl
        );
    END LOOP;
END $$;

UPDATE agent_templates t
SET org_id = (
    SELECT om.org_id
    FROM organization_members om
    WHERE om.user_id = t.user_id
    ORDER BY om.created_at ASC
    LIMIT 1
)
WHERE (t.org_id IS NULL OR t.org_id = '')
  AND t.is_preset = false
  AND t.user_id IS NOT NULL
  AND t.user_id <> '';

-- 4. Enable and refresh RLS policies across all 17 multi-tenant tables without 'user_admin' bypass.
DO $$
DECLARE
    rls_tables TEXT[] := ARRAY[
        'providers',
        'credentials',
        'gateway_api_keys',
        'agents',
        'agent_templates',
        'tools',
        'resources',
        'mcp_servers',
        'mcp_server_invocations',
        'governance_policies',
        'ai_audit_trails',
        'request_logs',
        'budgets',
        'routing_policies',
        'routing_rules',
        'routing_decisions',
        'cost_anomalies'
    ];
    pol RECORD;
    tbl TEXT;
    tenant_expr TEXT := $expr$
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
    $expr$;
    request_logs_expr TEXT := $rlexpr$
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
            AND gateway_api_key_id IN (
                SELECT id FROM gateway_api_keys
                WHERE (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), ''))
            )
        )
    $rlexpr$;
BEGIN
    -- Remove any legacy RLS policies (including hardcoded 'user_admin' bypass policies).
    FOR pol IN
        SELECT policyname, tablename
        FROM pg_policies
        WHERE schemaname = 'public'
          AND tablename = ANY(rls_tables)
    LOOP
        EXECUTE format('DROP POLICY IF EXISTS %I ON %I', pol.policyname, pol.tablename);
    END LOOP;

    FOREACH tbl IN ARRAY rls_tables LOOP
        EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
        IF tbl = 'agent_templates' THEN
            EXECUTE format('CREATE POLICY agent_templates_preset_select ON agent_templates FOR SELECT USING (is_preset = true OR (%s))', tenant_expr);
            EXECUTE format('CREATE POLICY agent_templates_tenant_insert ON agent_templates FOR INSERT WITH CHECK (%s)', tenant_expr);
            EXECUTE format('CREATE POLICY agent_templates_tenant_update ON agent_templates FOR UPDATE USING (%s) WITH CHECK (%s)', tenant_expr, tenant_expr);
            EXECUTE format('CREATE POLICY agent_templates_tenant_delete ON agent_templates FOR DELETE USING (%s)', tenant_expr);
        ELSIF tbl IN ('providers', 'agents', 'mcp_servers') THEN
            EXECUTE format('CREATE POLICY %I ON %I FOR SELECT USING ((user_id IS NULL AND (org_id IS NULL OR org_id = '''')) OR (%s))', tbl || '_shared_select', tbl, tenant_expr);
            EXECUTE format('CREATE POLICY %I ON %I FOR INSERT WITH CHECK (%s)', tbl || '_tenant_insert', tbl, tenant_expr);
            EXECUTE format('CREATE POLICY %I ON %I FOR UPDATE USING (%s) WITH CHECK (%s)', tbl || '_tenant_update', tbl, tenant_expr, tenant_expr);
            EXECUTE format('CREATE POLICY %I ON %I FOR DELETE USING (%s)', tbl || '_tenant_delete', tbl, tenant_expr);
        ELSIF tbl = 'request_logs' THEN
            EXECUTE format(
                'CREATE POLICY request_logs_tenant_policy ON request_logs FOR ALL USING (%s) WITH CHECK (%s)',
                request_logs_expr,
                request_logs_expr
            );
        ELSE
            EXECUTE format(
                'CREATE POLICY %I ON %I FOR ALL USING (%s) WITH CHECK (%s)',
                tbl || '_tenant_policy',
                tbl,
                tenant_expr,
                tenant_expr
            );
        END IF;
    END LOOP;
END $$;
