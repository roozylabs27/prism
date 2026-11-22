package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelProviderScopeClause(t *testing.T) {
	t.Run("org context includes own org and shared system providers only", func(t *testing.T) {
		ctx := database.WithTenantContext(context.Background(), database.TenantDBContext{
			UserID: "u_alice",
			OrgID:  "org_a",
		})
		clause, args, order := modelProviderScopeClause(ctx, []any{"gpt-4o"})
		assert.Contains(t, clause, "p.org_id = $2")
		assert.Contains(t, clause, "p.user_id IS NULL")
		assert.Equal(t, []any{"gpt-4o", "org_a", "u_alice"}, args)
		assert.True(t, strings.Contains(order, "CASE WHEN p.org_id = $2"))
	})

	t.Run("empty context without system bypass restricts to shared system providers only", func(t *testing.T) {
		clause, args, _ := modelProviderScopeClause(context.Background(), nil)
		assert.Equal(t, "(p.user_id IS NULL AND (p.org_id IS NULL OR p.org_id = ''))", clause)
		assert.Empty(t, args)
	})

	t.Run("system bypass allows global worker enumeration", func(t *testing.T) {
		ctx := database.WithSystemBypassRLS(context.Background())
		clause, args, _ := modelProviderScopeClause(ctx, nil)
		assert.Equal(t, "1=1", clause)
		assert.Empty(t, args)
	})
}

func TestModelRepository_TenantIsolation(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()

	_, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES
			('alice', 'Alice', 'alice-model@example.com'),
			('bob', 'Bob', 'bob-model@example.com');
		INSERT INTO organizations (id, name, slug) VALUES
			('org_a', 'Org A', 'org-a-model'),
			('org_b', 'Org B', 'org-b-model');
		INSERT INTO providers (id, user_id, org_id, name, slug, type, enabled) VALUES
			('prov_a', 'alice', 'org_a', 'Alice Private OpenAI', 'alice-private', 'openai', true),
			('prov_b', 'bob', 'org_b', 'Bob Private OpenAI', 'bob-private', 'openai', true),
			('prov_sys', NULL, NULL, 'System OpenAI', 'system-openai', 'openai', true);
	`)
	require.NoError(t, err)

	modelRepo := NewModelRepository(db)
	for _, m := range []*models.Model{
		{ID: "m_a", ProviderID: "prov_a", Name: "corp-secret-a", Slug: "corp-secret-a", DisplayName: "Corp Secret A", Enabled: true},
		{ID: "m_b", ProviderID: "prov_b", Name: "corp-secret-b", Slug: "corp-secret-b", DisplayName: "Corp Secret B", Enabled: true},
		{ID: "m_coll_a", ProviderID: "prov_a", Name: "Shared Slug A", Slug: "custom-shared", DisplayName: "Shared A", Enabled: true},
		{ID: "m_coll_b", ProviderID: "prov_b", Name: "Shared Slug B", Slug: "custom-shared", DisplayName: "Shared B", Enabled: true},
		{ID: "m_sys", ProviderID: "prov_sys", Name: "gpt-4o", Slug: "gpt-4o", DisplayName: "GPT-4o", Enabled: true},
	} {
		require.NoError(t, modelRepo.Create(ctx, m))
	}

	ctxA := database.WithTenantContext(ctx, database.TenantDBContext{UserID: "alice", OrgID: "org_a"})
	ctxB := database.WithTenantContext(ctx, database.TenantDBContext{UserID: "bob", OrgID: "org_b"})

	listA, err := modelRepo.ListEnabled(ctxA)
	require.NoError(t, err)
	var slugsA []string
	for _, m := range listA {
		slugsA = append(slugsA, m.Slug)
	}
	assert.Contains(t, slugsA, "corp-secret-a")
	assert.Contains(t, slugsA, "gpt-4o")
	assert.NotContains(t, slugsA, "corp-secret-b")

	_, err = modelRepo.FindBySlug(ctxA, "corp-secret-b")
	assert.Error(t, err, "org A must not resolve org B private model by slug")

	collA, err := modelRepo.FindBySlug(ctxA, "custom-shared")
	require.NoError(t, err)
	assert.Equal(t, "prov_a", collA.ProviderID)

	collB, err := modelRepo.FindBySlug(ctxB, "custom-shared")
	require.NoError(t, err)
	assert.Equal(t, "prov_b", collB.ProviderID)
}
