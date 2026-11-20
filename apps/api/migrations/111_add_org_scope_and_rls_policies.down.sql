-- Rollback Wave 2 (P0-6 / D-10): Drop new RLS policies and org_id/workspace_id
-- columns added by 111_add_org_scope_and_rls_policies.up.sql.

DO $$
DECLARE
    tbl TEXT;
    new_rls_tables TEXT[] := ARRAY[
        'providers',
        'budgets',
        'routing_policies',
        'routing_rules',
        'routing_decisions',
        'cost_anomalies'
    ];
    all_tables TEXT[] := ARRAY[
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
BEGIN
    FOREACH tbl IN ARRAY all_tables LOOP
        IF tbl = 'agent_templates' THEN
            DROP POLICY IF EXISTS agent_templates_preset_select ON agent_templates;
            DROP POLICY IF EXISTS agent_templates_tenant_insert ON agent_templates;
            DROP POLICY IF EXISTS agent_templates_tenant_update ON agent_templates;
            DROP POLICY IF EXISTS agent_templates_tenant_delete ON agent_templates;
            ALTER TABLE agent_templates DISABLE ROW LEVEL SECURITY;
        ELSIF tbl IN ('providers', 'agents', 'mcp_servers') THEN
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_shared_select', tbl);
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_tenant_insert', tbl);
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_tenant_update', tbl);
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_tenant_delete', tbl);
        ELSE
            EXECUTE format('DROP POLICY IF EXISTS %I ON %I', tbl || '_tenant_policy', tbl);
        END IF;
    END LOOP;

    FOREACH tbl IN ARRAY new_rls_tables LOOP
        EXECUTE format('ALTER TABLE %I DISABLE ROW LEVEL SECURITY', tbl);
    END LOOP;
END $$;

DROP INDEX IF EXISTS idx_mcp_server_invocations_org_created;
DROP INDEX IF EXISTS idx_cost_anomalies_org_created;
DROP INDEX IF EXISTS idx_routing_decisions_org_id;
DROP INDEX IF EXISTS idx_agent_templates_org_id;
DROP INDEX IF EXISTS idx_routing_rules_org_id;
DROP INDEX IF EXISTS idx_routing_policies_org_id;
DROP INDEX IF EXISTS idx_budgets_workspace_id;
DROP INDEX IF EXISTS idx_budgets_org_id;

ALTER TABLE mcp_server_invocations DROP COLUMN IF EXISTS org_id;
ALTER TABLE cost_anomalies DROP COLUMN IF EXISTS org_id;
ALTER TABLE routing_decisions DROP COLUMN IF EXISTS org_id;
ALTER TABLE agent_templates DROP COLUMN IF EXISTS org_id;
ALTER TABLE routing_rules DROP COLUMN IF EXISTS org_id;
ALTER TABLE routing_policies DROP COLUMN IF EXISTS org_id;
ALTER TABLE budgets DROP COLUMN IF EXISTS workspace_id, DROP COLUMN IF EXISTS org_id;
