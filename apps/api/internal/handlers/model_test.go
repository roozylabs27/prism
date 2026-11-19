package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func modelRoutes(h *ModelHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/providers/:id/models", h.List)
		r.POST("/api/providers/:id/models", h.Create)
		r.GET("/api/providers/:id/models/:modelId", h.Get)
		r.PUT("/api/providers/:id/models/:modelId", h.Update)
		r.PATCH("/api/providers/:id/models/:modelId/capabilities", h.UpdateCapabilities)
		r.DELETE("/api/providers/:id/models/:modelId", h.Delete)
	}
}

func TestModelHandlerCRUDAndDeleteGuards(t *testing.T) {
	f := newOwnerFixture(t)
	h := NewModelHandler(repository.NewModelRepository(f.db), repository.NewProviderRepository(f.db), repository.NewGatewayKeyRepository(f.db), nil)
	alice := asUser("alice", nil, modelRoutes(h))
	base := "/api/providers/" + f.provAlice + "/models"

	requireStatus(t, doJSON(alice, http.MethodPost, base, `{not json`), http.StatusBadRequest)

	w := doJSON(alice, http.MethodPost, base, `{"name":"GPT X","slug":"gpt-x","displayName":"GPT X","enabled":true,"inputPricePer1M":2.5}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	assert.Equal(t, f.provAlice, created["providerId"], "the provider comes from the path")
	assert.Equal(t, 2.5, created["inputPricePer1M"])

	requireStatus(t, doJSON(alice, http.MethodPost, base, `{"name":"Other","slug":"other","displayName":"Other","enabled":true}`), http.StatusCreated)

	w = doJSON(alice, http.MethodGet, base+"?search=gpt&limit=10", "")
	requireStatus(t, w, http.StatusOK)
	page := decodeBody(t, w)
	assert.Equal(t, 1.0, page["total"])
	w = doJSON(alice, http.MethodGet, base+"?limit=1&page=2", "")
	page = decodeBody(t, w)
	assert.Equal(t, 2.0, page["total"])
	assert.Equal(t, 2.0, page["totalPages"])
	assert.Len(t, page["data"], 1)

	w = doJSON(alice, http.MethodPatch, base+"/"+id+"/capabilities", `{"contextWindow":128000,"supportsTools":true,"qualityScore":0.9}`)
	requireStatus(t, w, http.StatusOK)
	caps := decodeBody(t, w)
	assert.Equal(t, 128000.0, caps["contextWindow"])
	assert.Equal(t, true, caps["supportsTools"])
	assert.Equal(t, 0.9, caps["qualityScore"])
	assert.Equal(t, 2.5, caps["inputPricePer1M"], "untouched capability values survive")

	w = doJSON(alice, http.MethodPut, base+"/"+id, `{"displayName":"GPT X (renamed)","enabled":false}`)
	requireStatus(t, w, http.StatusOK)
	w = doJSON(alice, http.MethodGet, base+"/"+id, "")
	requireStatus(t, w, http.StatusOK)
	got := decodeBody(t, w)
	assert.Equal(t, "GPT X (renamed)", got["displayName"])
	assert.Equal(t, false, got["enabled"])
	assert.Equal(t, "gpt-x", got["slug"], "fields missing from the body keep their values")

	// A gateway key that allows the model blocks deletion until it is disabled.
	_, err := f.db.Exec(`INSERT INTO gateway_api_keys (user_id, name, key_hash, key_prefix, enabled, allowed_models)
		VALUES ('alice', 'pinned', 'hash-pinned', 'prism_sk_pin', true, '{gpt-x}')`)
	require.NoError(t, err)
	w = doJSON(alice, http.MethodDelete, base+"/"+id, "")
	requireStatus(t, w, http.StatusConflict)
	assert.Contains(t, w.Body.String(), "MODEL_IN_USE_BY_KEYS")
	_, err = f.db.Exec(`UPDATE gateway_api_keys SET enabled = false WHERE key_hash = 'hash-pinned'`)
	require.NoError(t, err)
	requireStatus(t, doJSON(alice, http.MethodDelete, base+"/"+id, ""), http.StatusNoContent)
	requireStatus(t, doJSON(alice, http.MethodGet, base+"/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodDelete, base+"/"+id, ""), http.StatusNotFound)

	// Cross-tenant isolation: Bob cannot read or mutate Alice's provider or models.
	bob := asUser("bob", nil, modelRoutes(h))
	requireStatus(t, doJSON(alice, http.MethodPost, base, `{"name":"AliceOnly","slug":"alice-only","displayName":"AliceOnly","enabled":true}`), http.StatusCreated)
	requireStatus(t, doJSON(bob, http.MethodGet, base, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPost, base, `{"name":"Hacked","slug":"hacked","displayName":"Hacked","enabled":true}`), http.StatusNotFound)
}

type fakeModelStore struct {
	items map[string]*models.Model
}

func (f *fakeModelStore) ListWithFilter(_ context.Context, providerID, _ string, _, _ int, userID ...string) ([]models.Model, int64, error) {
	var out []models.Model
	for _, m := range f.items {
		if providerID != "" && providerID != "all" && m.ProviderID != providerID {
			continue
		}
		_ = userID
		out = append(out, *m)
	}
	return out, int64(len(out)), nil
}

func (f *fakeModelStore) FindByID(_ context.Context, id string) (*models.Model, error) {
	if m, ok := f.items[id]; ok {
		cp := *m
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeModelStore) Create(_ context.Context, m *models.Model) error {
	if m.ID == "" {
		m.ID = "m-created"
	}
	cp := *m
	f.items[m.ID] = &cp
	return nil
}

func (f *fakeModelStore) Update(_ context.Context, m *models.Model) error {
	cp := *m
	f.items[m.ID] = &cp
	return nil
}

func (f *fakeModelStore) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	return nil
}

type fakeModelProviderFinder map[string]*models.Provider

func (f fakeModelProviderFinder) FindByID(_ context.Context, id string) (*models.Provider, error) {
	if p, ok := f[id]; ok {
		return p, nil
	}
	return nil, sql.ErrNoRows
}

func TestModelHandler_CrossTenantAndSystemProviderGuards(t *testing.T) {
	provs := fakeModelProviderFinder{
		"prov_alice":  {ID: "prov_alice", UserID: "alice", Name: "Alice OpenAI"},
		"prov_bob":    {ID: "prov_bob", UserID: "bob", Name: "Bob OpenAI"},
		"prov_system": {ID: "prov_system", UserID: "", Name: "System OpenAI"},
	}
	store := &fakeModelStore{
		items: map[string]*models.Model{
			"m_alice": {ID: "m_alice", ProviderID: "prov_alice", Name: "gpt-4o", Slug: "gpt-4o", DisplayName: "GPT-4o", Enabled: true},
			"m_bob":   {ID: "m_bob", ProviderID: "prov_bob", Name: "gpt-4o", Slug: "gpt-4o", DisplayName: "Bob GPT-4o", Enabled: true},
			"m_sys":   {ID: "m_sys", ProviderID: "prov_system", Name: "gpt-4o-mini", Slug: "gpt-4o-mini", DisplayName: "Sys Mini", Enabled: true},
		},
	}
	h := &ModelHandler{models: store, providers: provs}
	alice := asUser("alice", nil, modelRoutes(h))

	// Bob's provider & model are invisible (404) for read and all mutations.
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_bob/models", ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_bob/models/m_bob", ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodPost, "/api/providers/prov_bob/models", `{"name":"x","slug":"x"}`), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodPut, "/api/providers/prov_bob/models/m_bob", `{"displayName":"pwn"}`), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodPatch, "/api/providers/prov_bob/models/m_bob/capabilities", `{"inputPricePer1M":0}`), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/providers/prov_bob/models/m_bob", ""), http.StatusNotFound)

	// Path-provider mismatch on Alice's provider pointing at Bob's model ID returns 404.
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_alice/models/m_bob", ""), http.StatusNotFound)
	requireStatus(t, doJSON(alice, http.MethodPatch, "/api/providers/prov_alice/models/m_bob/capabilities", `{"inputPricePer1M":0}`), http.StatusNotFound)

	// System providers are readable by tenants, but mutations return 403.
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_system/models", ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_system/models/m_sys", ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodPost, "/api/providers/prov_system/models", `{"name":"x","slug":"x"}`), http.StatusForbidden)
	requireStatus(t, doJSON(alice, http.MethodPut, "/api/providers/prov_system/models/m_sys", `{"displayName":"pwn"}`), http.StatusForbidden)
	requireStatus(t, doJSON(alice, http.MethodPatch, "/api/providers/prov_system/models/m_sys/capabilities", `{"inputPricePer1M":0}`), http.StatusForbidden)
	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/providers/prov_system/models/m_sys", ""), http.StatusForbidden)

	// Alice can read & update her own model.
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/providers/prov_alice/models/m_alice", ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodPut, "/api/providers/prov_alice/models/m_alice", `{"displayName":"Renamed","providerId":"prov_bob"}`), http.StatusOK)
	assert.Equal(t, "prov_alice", store.items["m_alice"].ProviderID, "update must not reassign providerId")
}
