package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
)

type fakeCustomProviderRepo struct {
	providers map[string]*models.Provider
	seq       int
}

func (f *fakeCustomProviderRepo) ListByUserID(_ context.Context, userID string) ([]models.Provider, error) {
	var out []models.Provider
	for _, p := range f.providers {
		if p.UserID == userID || p.UserID == "" {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeCustomProviderRepo) FindByID(_ context.Context, id string) (*models.Provider, error) {
	p, ok := f.providers[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	cp := *p
	return &cp, nil
}

func (f *fakeCustomProviderRepo) Create(_ context.Context, p *models.Provider) error {
	f.seq++
	if p.ID == "" {
		p.ID = "prov-" + string(rune('0'+f.seq))
	}
	cp := *p
	f.providers[p.ID] = &cp
	return nil
}

func (f *fakeCustomProviderRepo) Update(_ context.Context, p *models.Provider) error {
	if _, ok := f.providers[p.ID]; !ok {
		return sql.ErrNoRows
	}
	cp := *p
	f.providers[p.ID] = &cp
	return nil
}

func (f *fakeCustomProviderRepo) Delete(_ context.Context, id, userID string) error {
	p, ok := f.providers[id]
	if !ok || p.UserID != userID {
		return sql.ErrNoRows
	}
	delete(f.providers, id)
	return nil
}

func customProviderRequest(h *ProviderHandler, user, method, id, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", user)
		c.Next()
	})
	r.POST("/providers", h.Create)
	r.PUT("/providers/:id", h.Update)

	path := "/providers"
	if id != "" {
		path += "/" + id
	}
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCustomProviderCreateDefaultsAndUniqueSlug(t *testing.T) {
	store := &fakeCustomProviderRepo{providers: map[string]*models.Provider{}}
	h := NewProviderHandler(store, nil)

	w1 := customProviderRequest(h, "alice", http.MethodPost, "", `{"name":"Custom Proxy","type":"custom","baseUrl":"https://93.184.216.34/v1"}`)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w1.Code, w1.Body.String())
	}
	var p1 models.Provider
	if err := json.Unmarshal(w1.Body.Bytes(), &p1); err != nil {
		t.Fatalf("decode p1: %v", err)
	}
	if !p1.Enabled {
		t.Fatalf("expected custom provider create to default enabled=true")
	}
	if p1.Slug == "" {
		t.Fatalf("expected non-empty auto-generated slug")
	}

	w2 := customProviderRequest(h, "alice", http.MethodPost, "", `{"name":"Custom Proxy","type":"custom","baseUrl":"https://93.184.216.34/v1"}`)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 on second create, got %d: %s", w2.Code, w2.Body.String())
	}
	var p2 models.Provider
	_ = json.Unmarshal(w2.Body.Bytes(), &p2)
	if p2.Slug == "" || p2.Slug == p1.Slug {
		t.Fatalf("expected unique auto-generated slugs, got p1=%q p2=%q", p1.Slug, p2.Slug)
	}
}

func TestCustomProviderPartialUpdateToggleAndSSRF(t *testing.T) {
	store := &fakeCustomProviderRepo{
		providers: map[string]*models.Provider{
			"prov_alice": {
				ID:      "prov_alice",
				UserID:  "alice",
				Name:    "Alice Proxy",
				Slug:    "alice-proxy-123456",
				BaseURL: "https://93.184.216.34/v1",
				Type:    "custom",
				Enabled: true,
			},
		},
	}
	h := NewProviderHandler(store, nil)

	w := customProviderRequest(h, "alice", http.MethodPut, "prov_alice", `{"enabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on partial toggle, got %d: %s", w.Code, w.Body.String())
	}
	var updated models.Provider
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.Enabled {
		t.Fatalf("expected enabled=false")
	}
	if updated.Name != "Alice Proxy" || updated.BaseURL != "https://93.184.216.34/v1" || updated.Slug != "alice-proxy-123456" {
		t.Fatalf("expected omitted fields preserved on partial update, got %+v", updated)
	}

	// SSRF protection must still reject private baseUrl updates.
	wBad := customProviderRequest(h, "alice", http.MethodPut, "prov_alice", `{"baseUrl":"http://127.0.0.1:8080/v1"}`)
	if wBad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for loopback baseUrl on update, got %d: %s", wBad.Code, wBad.Body.String())
	}
}
