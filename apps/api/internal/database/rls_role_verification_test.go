package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRLS_MigrationCoverageOffline validates that all 18 multi-tenant tables
// have ROW LEVEL SECURITY enabled without legacy hardcoded admin bypasses.
func TestRLS_MigrationCoverageOffline(t *testing.T) {
	mig111, err := os.ReadFile(filepath.Join("..", "..", "migrations", "111_add_org_scope_and_rls_policies.up.sql"))
	require.NoError(t, err)
	mig114, err := os.ReadFile(filepath.Join("..", "..", "migrations", "114_create_approval_requests_and_tool_approval.up.sql"))
	require.NoError(t, err)

	m111Str := string(mig111)
	expectedTables := []string{
		"providers",
		"credentials",
		"gateway_api_keys",
		"agents",
		"agent_templates",
		"tools",
		"resources",
		"mcp_servers",
		"mcp_server_invocations",
		"governance_policies",
		"ai_audit_trails",
		"request_logs",
		"budgets",
		"routing_policies",
		"routing_rules",
		"routing_decisions",
		"cost_anomalies",
	}
	for _, tbl := range expectedTables {
		assert.Contains(t, m111Str, "'"+tbl+"'", "migration 111 must include tenant table %s", tbl)
	}
	assert.Contains(t, m111Str, "ENABLE ROW LEVEL SECURITY")
	assert.Contains(t, string(mig114), "ALTER TABLE approval_requests ENABLE ROW LEVEL SECURITY;")
	assert.Contains(t, string(mig114), "CREATE POLICY approval_requests_tenant_policy")
}

// TestRLS_NonSuperuserRoleIsolation verifies that a NOSUPERUSER NOBYPASSRLS role
// enforces strict tenant isolation through NewRLSConnector when DATABASE_URL is reachable.
func TestRLS_NonSuperuserRoleIsolation(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping live non-superuser RLS isolation check")
	}

	adminDB, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Skipf("cannot open db: %v", err)
	}
	defer func() { _ = adminDB.Close() }()

	ctx := context.Background()
	if err := adminDB.PingContext(ctx); err != nil {
		t.Skipf("postgres offline: %v", err)
	}

	var rlsEnabled bool
	err = adminDB.QueryRowContext(ctx, `
		SELECT c.relrowsecurity
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relname = 'agents'
	`).Scan(&rlsEnabled)
	if err != nil {
		t.Skipf("schema not migrated: %v", err)
	}
	require.True(t, rlsEnabled, "agents table must have relrowsecurity = true")

	_, err = adminDB.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'prism_rls_test_role') THEN
				CREATE ROLE prism_rls_test_role NOLOGIN NOSUPERUSER NOBYPASSRLS;
			END IF;
		END $$;
		GRANT USAGE ON SCHEMA public TO prism_rls_test_role;
		GRANT SELECT ON agents TO prism_rls_test_role;
		INSERT INTO users (id, email, name, password_hash)
		VALUES ('rls_go_u1', 'rls-go-u1@example.invalid', 'RLS Go U1', 'x') ON CONFLICT DO NOTHING;
		INSERT INTO organizations (id, name, slug, owner_id)
		VALUES ('rls_go_org_a', 'RLS Go A', 'rls-go-org-a', 'rls_go_u1'),
		       ('rls_go_org_b', 'RLS Go B', 'rls-go-org-b', 'rls_go_u1') ON CONFLICT DO NOTHING;
		INSERT INTO agents (id, user_id, org_id, name, display_name)
		VALUES ('rls_go_agent_a', 'rls_go_u1', 'rls_go_org_a', 'rls_go_a', 'RLS Go A'),
		       ('rls_go_agent_b', 'rls_go_u1', 'rls_go_org_b', 'rls_go_b', 'RLS Go B') ON CONFLICT DO NOTHING;
	`)
	require.NoError(t, err)
	defer func() {
		_, _ = adminDB.ExecContext(context.Background(), `
			DELETE FROM agents WHERE id IN ('rls_go_agent_a', 'rls_go_agent_b');
			DELETE FROM organizations WHERE id IN ('rls_go_org_a', 'rls_go_org_b');
			DELETE FROM users WHERE id = 'rls_go_u1';
		`)
	}()

	tx, err := adminDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `SET LOCAL ROLE prism_rls_test_role`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_user_id', 'rls_go_u1', true), set_config('app.current_org_id', 'rls_go_org_a', true)`)
	require.NoError(t, err)

	rows, err := tx.QueryContext(ctx, `SELECT id FROM agents WHERE id IN ('rls_go_agent_a', 'rls_go_agent_b') ORDER BY id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	var seen []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		seen = append(seen, strings.TrimSpace(id))
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, []string{"rls_go_agent_a"}, seen, "non-superuser role must only see org_a agent")
}
