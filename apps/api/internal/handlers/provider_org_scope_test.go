package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
)

type fakeOrgProviderStore struct {
	providers map[string]*models.Provider
}

func (f *fakeOrgProviderStore) ListByUserID(ctx context.Context, userID string) ([]models.Provider, error) {
	orgID := database.TenantContextFrom(ctx).OrgID
	var out []models.Provider
	for _, p := range f.providers {
		if (p.UserID == "" && p.OrgID == "") || (orgID != "" && p.OrgID == orgID) || (orgID == "" && p.UserID == userID) {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeOrgProviderStore) FindByID(_ context.Context, id string) (*models.Provider, error) {
	p, ok := f.providers[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *p
	return &cp, nil
}

func (f *fakeOrgProviderStore) Create(_ context.Context, p *models.Provider) error {
	if p.ID == "" {
		p.ID = "prov_created"
	}
	cp := *p
	f.providers[cp.ID] = &cp
	return nil
}

func (f *fakeOrgProviderStore) Update(_ context.Context, p *models.Provider) error {
	cp := *p
	f.providers[cp.ID] = &cp
	return nil
}

func (f *fakeOrgProviderStore) Delete(ctx context.Context, id, userID string) error {
	p, ok := f.providers[id]
	if !ok {
		return sql.ErrNoRows
	}
	orgID := database.TenantContextFrom(ctx).OrgID
	if (orgID != "" && p.OrgID != orgID) || (orgID == "" && p.UserID != userID) {
		return sql.ErrNoRows
	}
	delete(f.providers, id)
	return nil
}

type fakeOrgModelStore struct {
	models []models.Model
}

func (f *fakeOrgModelStore) ListWithFilter(_ context.Context, providerID, _ string, _, _ int, _ ...string) ([]models.Model, int64, error) {
	var out []models.Model
	for _, m := range f.models {
		if providerID == "" || providerID == "all" || m.ProviderID == providerID {
			out = append(out, m)
		}
	}
	return out, int64(len(out)), nil
}

func (f *fakeOrgModelStore) FindByID(_ context.Context, id string) (*models.Model, error) {
	for i := range f.models {
		if f.models[i].ID == id {
			cp := f.models[i]
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeOrgModelStore) Create(_ context.Context, m *models.Model) error {
	if m.ID == "" {
		m.ID = "mod_new"
	}
	f.models = append(f.models, *m)
	return nil
}

func (f *fakeOrgModelStore) Update(_ context.Context, _ *models.Model) error { return nil }
func (f *fakeOrgModelStore) Delete(_ context.Context, _ string) error       { return nil }

func TestSharedOrgProviderAccessAcrossHandlers(t *testing.T) {
	pStore := &fakeOrgProviderStore{
		providers: map[string]*models.Provider{
			"prov_a": {
				ID:      "prov_a",
				UserID:  "alice",
				OrgID:   "org_a",
				Name:    "Org A OpenAI",
				Slug:    "org-a-openai",
				BaseURL: "https://api.openai.com/v1",
				Type:    "openai",
				Enabled: true,
			},
			"prov_sys": {
				ID:      "prov_sys",
				UserID:  "",
				OrgID:   "",
				Name:    "System OpenAI",
				Slug:    "openai",
				BaseURL: "https://api.openai.com/v1",
				Type:    "openai",
				Enabled: true,
			},
		},
	}
	ph := NewProviderHandler(pStore, nil)

	orgA := &models.TenantContext{OrgID: "org_a"}
	orgB := &models.TenantContext{OrgID: "org_b"}

	// Bob in org_a can read and update Alice's org_a provider.
	rBob := asUser("bob", orgA, func(r *gin.Engine) {
		r.GET("/api/providers/:id", ph.Get)
		r.PUT("/api/providers/:id", ph.Update)
		r.DELETE("/api/providers/:id", ph.Delete)
	})
	w := doJSON(rBob, http.MethodGet, "/api/providers/prov_a", "")
	requireStatus(t, w, http.StatusOK)

	w = doJSON(rBob, http.MethodPut, "/api/providers/prov_a", `{"name":"Org A Shared"}`)
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, "alice", pStore.providers["prov_a"].UserID)
	assert.Equal(t, "org_a", pStore.providers["prov_a"].OrgID)

	// System provider is readable by org_a, but immutable (403).
	requireStatus(t, doJSON(rBob, http.MethodGet, "/api/providers/prov_sys", ""), http.StatusOK)
	requireStatus(t, doJSON(rBob, http.MethodPut, "/api/providers/prov_sys", `{"name":"Hijack"}`), http.StatusForbidden)
	requireStatus(t, doJSON(rBob, http.MethodDelete, "/api/providers/prov_sys", ""), http.StatusForbidden)

	// Carol in org_b receives 404 for org_a provider on read, write, and delete.
	rCarol := asUser("carol", orgB, func(r *gin.Engine) {
		r.GET("/api/providers/:id", ph.Get)
		r.PUT("/api/providers/:id", ph.Update)
		r.DELETE("/api/providers/:id", ph.Delete)
	})
	requireStatus(t, doJSON(rCarol, http.MethodGet, "/api/providers/prov_a", ""), http.StatusNotFound)
	requireStatus(t, doJSON(rCarol, http.MethodPut, "/api/providers/prov_a", `{"name":"CrossOrg"}`), http.StatusNotFound)
	requireStatus(t, doJSON(rCarol, http.MethodDelete, "/api/providers/prov_a", ""), http.StatusNotFound)

	// ModelHandler: org_a teammate can list and create models under org_a provider; foreign org gets 404; system write gets 403.
	mh := &ModelHandler{models: &fakeOrgModelStore{}, providers: pStore}
	rBobModels := asUser("bob", orgA, func(r *gin.Engine) {
		r.GET("/api/providers/:id/models", mh.List)
		r.POST("/api/providers/:id/models", mh.Create)
	})
	rCarolModels := asUser("carol", orgB, func(r *gin.Engine) {
		r.POST("/api/providers/:id/models", mh.Create)
	})
	requireStatus(t, doJSON(rBobModels, http.MethodPost, "/api/providers/prov_a/models", `{"name":"gpt-4o","slug":"gpt-4o","displayName":"GPT-4o"}`), http.StatusCreated)
	requireStatus(t, doJSON(rBobModels, http.MethodGet, "/api/providers/prov_sys/models", ""), http.StatusOK)
	requireStatus(t, doJSON(rBobModels, http.MethodPost, "/api/providers/prov_sys/models", `{"name":"gpt-4o","slug":"gpt-4o","displayName":"GPT-4o"}`), http.StatusForbidden)
	requireStatus(t, doJSON(rCarolModels, http.MethodPost, "/api/providers/prov_a/models", `{"name":"gpt-4o","slug":"gpt-4o","displayName":"GPT-4o"}`), http.StatusNotFound)

	// Credential attachment rule allows org_a teammate & system provider, denies foreign org & disabled provider.
	assert.True(t, providerAcceptsCredentialsFrom(pStore.providers["prov_a"], "bob", "org_a"))
	assert.False(t, providerAcceptsCredentialsFrom(pStore.providers["prov_a"], "carol", "org_b"))
	assert.True(t, providerAcceptsCredentialsFrom(pStore.providers["prov_sys"], "carol", "org_b"))
	disabledProv := *pStore.providers["prov_a"]
	disabledProv.Enabled = false
	assert.False(t, providerAcceptsCredentialsFrom(&disabledProv, "alice", "org_a"))

	// Bob in org_a can delete org_a provider.
	requireStatus(t, doJSON(rBob, http.MethodDelete, "/api/providers/prov_a", ""), http.StatusNoContent)
}
