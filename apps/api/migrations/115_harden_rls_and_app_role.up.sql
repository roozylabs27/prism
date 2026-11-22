-- Wave 1 (REM-01 / AUD-P0-01): Harden PostgreSQL RLS policies to fail closed when
-- tenant GUCs are unset (requiring explicit app.rls_bypass='on' instead of empty GUCs),
-- create non-superuser application role `prism_app`, and extend RLS to all remaining
-- multi-tenant tables.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'prism_app') THEN
        CREATE ROLE prism_app NOLOGIN NOSUPERUSER NOBYPASSRLS;
    END IF;
END $$;

GRANT USAGE ON SCHEMA public TO prism_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO prism_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO prism_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO prism_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO prism_app;

-- 1. Replace empty-GUC fail-open clause on the 18 existing RLS tables with explicit app.rls_bypass='on'.
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
        'cost_anomalies',
        'approval_requests'
    ];
    pol RECORD;
    tbl TEXT;
    bypass_expr TEXT := $bp$NULLIF(current_setting('app.rls_bypass', true), '') = 'on'$bp$;
    tenant_expr TEXT := $expr$
        (
            NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
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
            NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
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

    -- 2. Extend fail-closed RLS to remaining org-scoped tables.
    FOREACH tbl IN ARRAY ARRAY[
        'workspaces',
        'organization_members',
        'tenant_quotas',
        'audit_logs',
        'organization_subscriptions',
        'billing_invoices',
        'daily_usage_aggregates',
        'member_invites',
        'delivery_outbox'
    ] LOOP
        IF to_regclass('public.' || tbl) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', tbl);
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_tenant_policy', tbl);
            EXECUTE format(
                'CREATE POLICY %I ON %I FOR ALL USING ((%s) OR org_id = NULLIF(current_setting(''app.current_org_id'', true), '''')) WITH CHECK ((%s) OR org_id = NULLIF(current_setting(''app.current_org_id'', true), ''''))',
                tbl || '_tenant_policy', tbl, bypass_expr, bypass_expr
            );
        END IF;
    END LOOP;

    -- mcp_registry: public entries + org/user private entries
    IF to_regclass('public.mcp_registry') IS NOT NULL THEN
        ALTER TABLE mcp_registry ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS mcp_registry_tenant_policy ON mcp_registry;
        DROP POLICY IF EXISTS mcp_registry_public_select ON mcp_registry;
        DROP POLICY IF EXISTS mcp_registry_tenant_insert ON mcp_registry;
        DROP POLICY IF EXISTS mcp_registry_tenant_update ON mcp_registry;
        DROP POLICY IF EXISTS mcp_registry_tenant_delete ON mcp_registry;
        CREATE POLICY mcp_registry_public_select ON mcp_registry FOR SELECT
            USING (visibility = 'public' OR NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), '')));
        CREATE POLICY mcp_registry_tenant_insert ON mcp_registry FOR INSERT
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), '')));
        CREATE POLICY mcp_registry_tenant_update ON mcp_registry FOR UPDATE
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), '')));
        CREATE POLICY mcp_registry_tenant_delete ON mcp_registry FOR DELETE
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id <> '' AND org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR ((org_id IS NULL OR org_id = '') AND user_id = NULLIF(current_setting('app.current_user_id', true), '')));
    END IF;

    -- settings: platform defaults readable by all; mutations restricted to caller org_id or bypass
    IF to_regclass('public.settings') IS NOT NULL THEN
        ALTER TABLE settings ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS settings_select_policy ON settings;
        DROP POLICY IF EXISTS settings_write_policy ON settings;
        CREATE POLICY settings_select_policy ON settings FOR SELECT
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR org_id IS NULL
                   OR org_id = NULLIF(current_setting('app.current_org_id', true), ''));
        CREATE POLICY settings_write_policy ON settings FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id = NULLIF(current_setting('app.current_org_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR (org_id IS NOT NULL AND org_id = NULLIF(current_setting('app.current_org_id', true), '')));
    END IF;

    -- projects & workspace_members (scoped via parent workspaces.org_id)
    IF to_regclass('public.projects') IS NOT NULL THEN
        ALTER TABLE projects ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS projects_tenant_policy ON projects;
        CREATE POLICY projects_tenant_policy ON projects FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM workspaces w WHERE w.id = projects.workspace_id AND w.org_id = NULLIF(current_setting('app.current_org_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM workspaces w WHERE w.id = projects.workspace_id AND w.org_id = NULLIF(current_setting('app.current_org_id', true), '')));
    END IF;

    IF to_regclass('public.workspace_members') IS NOT NULL THEN
        ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS workspace_members_tenant_policy ON workspace_members;
        CREATE POLICY workspace_members_tenant_policy ON workspace_members FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM workspaces w WHERE w.id = workspace_members.workspace_id AND w.org_id = NULLIF(current_setting('app.current_org_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM workspaces w WHERE w.id = workspace_members.workspace_id AND w.org_id = NULLIF(current_setting('app.current_org_id', true), '')));
    END IF;

    -- budget_alerts (scoped via parent budgets row)
    IF to_regclass('public.budget_alerts') IS NOT NULL THEN
        ALTER TABLE budget_alerts ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS budget_alerts_tenant_policy ON budget_alerts;
        CREATE POLICY budget_alerts_tenant_policy ON budget_alerts FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (
                       SELECT 1 FROM budgets b WHERE b.id = budget_alerts.budget_id
                         AND ((b.org_id IS NOT NULL AND b.org_id <> '' AND b.org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                           OR ((b.org_id IS NULL OR b.org_id = '') AND b.user_id = NULLIF(current_setting('app.current_user_id', true), '')))
                   ))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (
                       SELECT 1 FROM budgets b WHERE b.id = budget_alerts.budget_id
                         AND ((b.org_id IS NOT NULL AND b.org_id <> '' AND b.org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                           OR ((b.org_id IS NULL OR b.org_id = '') AND b.user_id = NULLIF(current_setting('app.current_user_id', true), '')))
                   ));
    END IF;

    -- request_payloads & tool_invocations (scoped via gateway_api_keys / request_logs)
    IF to_regclass('public.request_payloads') IS NOT NULL THEN
        ALTER TABLE request_payloads ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS request_payloads_tenant_policy ON request_payloads;
        CREATE POLICY request_payloads_tenant_policy ON request_payloads FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM gateway_api_keys g WHERE g.id = request_payloads.gateway_api_key_id AND g.org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR EXISTS (SELECT 1 FROM request_logs rl WHERE rl.request_id = request_payloads.request_id::varchar AND rl.org_id = NULLIF(current_setting('app.current_org_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM gateway_api_keys g WHERE g.id = request_payloads.gateway_api_key_id AND g.org_id = NULLIF(current_setting('app.current_org_id', true), ''))
                   OR request_payloads.gateway_api_key_id IS NULL);
    END IF;

    IF to_regclass('public.tool_invocations') IS NOT NULL THEN
        ALTER TABLE tool_invocations ENABLE ROW LEVEL SECURITY;
        DROP POLICY IF EXISTS tool_invocations_tenant_policy ON tool_invocations;
        CREATE POLICY tool_invocations_tenant_policy ON tool_invocations FOR ALL
            USING (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM request_logs rl WHERE rl.request_id = tool_invocations.request_id::varchar AND rl.org_id = NULLIF(current_setting('app.current_org_id', true), '')))
            WITH CHECK (NULLIF(current_setting('app.rls_bypass', true), '') = 'on'
                   OR EXISTS (SELECT 1 FROM request_logs rl WHERE rl.request_id = tool_invocations.request_id::varchar AND rl.org_id = NULLIF(current_setting('app.current_org_id', true), '')));
    END IF;
END $$;
