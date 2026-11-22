package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
)

func TestMCPRegistryListsPublicAndOwnEntriesVerifiedFirst(t *testing.T) {
	f := newTenantFixture(t)
	ctx := context.Background()
	r := NewMCPRegistryRepository(f.db)

	for _, s := range []*models.MCPRegistryServer{
		{UserID: "alice", Name: "zeta tools", Slug: "zeta", ServerURL: "https://zeta.example", TransportType: "sse", Visibility: "public", Capabilities: []byte(`{"tools":true}`)},
		{UserID: "alice", Name: "alpha private", Slug: "alpha", ServerURL: "https://alpha.example", TransportType: "stdio", Visibility: "private", Capabilities: []byte(`{}`)},
		{UserID: "", Name: "curated", Slug: "curated", ServerURL: "https://mcp.example", TransportType: "sse", Visibility: "public", Capabilities: []byte(`{}`), IsVerified: true},
		{UserID: "bob", Name: "bob private", Slug: "bob-private", ServerURL: "https://bob.example", TransportType: "sse", Visibility: "private", Capabilities: []byte(`{}`)},
	} {
		if err := r.Create(ctx, s); err != nil {
			t.Fatalf("create %s: %v", s.Slug, err)
		}
		if s.ID == "" || s.CreatedAt.IsZero() {
			t.Fatalf("create did not fill id/timestamps: %+v", s)
		}
	}

	list, err := r.ListPublicAndTenant(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, s := range list {
		slugs = append(slugs, s.Slug)
	}
	want := []string{"curated", "bob-private", "zeta"}
	if len(slugs) != len(want) {
		t.Fatalf("bob sees %v, want %v", slugs, want)
	}
	for i := range want {
		if slugs[i] != want[i] {
			t.Fatalf("bob sees %v, want verified first then by name: %v", slugs, want)
		}
	}

	got, err := r.FindBySlug(ctx, "zeta", "bob")
	if err != nil || got.UserID != "alice" || got.OrganizationID != nil {
		t.Fatalf("public entry = %+v, %v", got, err)
	}
	// jsonb normalises spacing, so compare the decoded value.
	var caps map[string]bool
	if err := json.Unmarshal(got.Capabilities, &caps); err != nil || !caps["tools"] {
		t.Fatalf("capabilities = %s (%v)", got.Capabilities, err)
	}
	if _, err := r.FindBySlug(ctx, "alpha", "bob"); err == nil {
		t.Fatal("bob read alice's private entry")
	}
	if got, err := r.FindBySlug(ctx, "alpha", "alice"); err != nil || got.TransportType != "stdio" {
		t.Fatalf("owner read = %+v, %v", got, err)
	}
	if got, err := r.FindBySlug(ctx, "curated", "nobody"); err != nil || got.UserID != "" || !got.IsVerified {
		t.Fatalf("system entry = %+v, %v", got, err)
	}
}

func TestMCPRegistryScopesPrivateEntriesByOrg(t *testing.T) {
	f := newTenantFixture(t)
	r := NewMCPRegistryRepository(f.db)

	orgA := "org_alice"
	orgB := "org_bob"
	ctxOrgA := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: orgA, UserID: "alice"})
	ctxOrgB := database.WithTenantContext(context.Background(), database.TenantDBContext{OrgID: orgB, UserID: "alice"})

	if err := r.Create(ctxOrgA, &models.MCPRegistryServer{
		UserID: "alice", Name: "orgA private", Slug: "orga-priv", ServerURL: "https://a.example", TransportType: "sse", Visibility: "private", Capabilities: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.FindBySlug(ctxOrgB, "orga-priv", "alice"); err == nil {
		t.Fatal("user alice in org_bob unexpectedly read org_alice private catalog entry")
	}
	if list, err := r.ListPublicAndTenant(ctxOrgB, "alice"); err != nil {
		t.Fatal(err)
	} else {
		for _, item := range list {
			if item.Slug == "orga-priv" {
				t.Fatalf("org_bob listing exposed org_alice private entry: %+v", item)
			}
		}
	}
}
