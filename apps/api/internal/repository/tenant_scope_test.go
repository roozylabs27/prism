package repository

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestTenantScopeHelpers(t *testing.T) {
	t.Run("empty context falls back to user_id", func(t *testing.T) {
		ctx := context.Background()
		clause, args := tenantOwnerClause(ctx, "p.", "user_alice", []any{"arg0"})
		assert.Equal(t, "p.user_id = $2", clause)
		assert.Equal(t, []any{"arg0", "user_alice"}, args)

		wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "p.", args)
		assert.Empty(t, wsClause)
		assert.Equal(t, args, wsArgs)
	})

	t.Run("org and workspace context produce org-shared and workspace-narrowed predicates", func(t *testing.T) {
		ctx := database.WithTenantContext(context.Background(), database.TenantDBContext{
			UserID:      "user_alice",
			OrgID:       "org_a",
			WorkspaceID: "ws_1",
			ProjectID:   "proj_1",
		})
		assert.Equal(t, "org_a", tenantOrgID(ctx))
		assert.Equal(t, "user_alice", tenantUserID(ctx))
		assert.Equal(t, "ws_1", tenantWorkspaceID(ctx))
		assert.Equal(t, "proj_1", tenantProjectID(ctx))

		clause, args := tenantOwnerClause(ctx, "", "user_alice", nil)
		assert.Equal(t, "(org_id = $1 OR ((org_id IS NULL OR org_id = '') AND user_id = $2))", clause)
		assert.Equal(t, []any{"org_a", "user_alice"}, args)

		wsClause, wsArgs := appendOptionalWorkspaceClause(ctx, "a.", args)
		assert.Equal(t, "(a.workspace_id IS NULL OR a.workspace_id = '' OR a.workspace_id = $3)", wsClause)
		assert.Equal(t, []any{"org_a", "user_alice", "ws_1"}, wsArgs)
	})
}
