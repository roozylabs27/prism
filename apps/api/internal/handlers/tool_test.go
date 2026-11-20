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
	"github.com/roozylabs/prism/internal/utils"
)

type fakeToolRepo struct {
	tools    map[string]models.Tool
	backends *fakeToolBackendRepo
	deleted  bool
}

func newFakeToolRepo() (*fakeToolRepo, *fakeToolBackendRepo) {
	b := &fakeToolBackendRepo{byTool: map[string][]models.ToolBackend{}}
	return &fakeToolRepo{tools: map[string]models.Tool{}, backends: b}, b
}

func (f *fakeToolRepo) ListByUserID(_ context.Context, userID string) ([]models.Tool, error) {
	var out []models.Tool
	for _, t := range f.tools {
		if t.UserID == userID {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeToolRepo) FindByID(_ context.Context, id, userID string) (*models.Tool, error) {
	t, ok := f.tools[id]
	if !ok || t.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := t
	return &cp, nil
}

func (f *fakeToolRepo) CreateWithBackendsTx(_ context.Context, t *models.Tool, backends []models.ToolBackend) error {
	if t.ID == "" {
		t.ID = "tool-1"
	}
	f.tools[t.ID] = *t
	f.backends.byTool[t.ID] = nil
	for i, b := range backends {
		b.ID = b.Name + "-id"
		if b.ID == "-id" {
			b.ID = "backend-" + string(rune('a'+i))
		}
		b.ToolID = t.ID
		f.backends.byTool[t.ID] = append(f.backends.byTool[t.ID], b)
	}
	return nil
}

func (f *fakeToolRepo) Update(_ context.Context, t *models.Tool) error {
	if _, ok := f.tools[t.ID]; !ok {
		return sql.ErrNoRows
	}
	f.tools[t.ID] = *t
	return nil
}

func (f *fakeToolRepo) Delete(_ context.Context, id, userID string) error {
	t, ok := f.tools[id]
	if !ok || t.UserID != userID {
		return sql.ErrNoRows
	}
	delete(f.tools, id)
	f.deleted = true
	return nil
}

func (f *fakeToolRepo) GetToolWithBackendsByID(_ context.Context, id, userID string) (*models.ToolWithBackends, error) {
	t, ok := f.tools[id]
	if !ok || t.UserID != userID {
		return nil, sql.ErrNoRows
	}
	bs := append([]models.ToolBackend(nil), f.backends.byTool[id]...)
	return &models.ToolWithBackends{Tool: t, Backends: bs}, nil
}

type fakeToolBackendRepo struct {
	byTool map[string][]models.ToolBackend
	seq    int
}

func (f *fakeToolBackendRepo) ListByToolID(_ context.Context, toolID string) ([]models.ToolBackend, error) {
	return append([]models.ToolBackend(nil), f.byTool[toolID]...), nil
}

func (f *fakeToolBackendRepo) DeleteByToolID(_ context.Context, toolID string) error {
	delete(f.byTool, toolID)
	return nil
}

func (f *fakeToolBackendRepo) Create(_ context.Context, b *models.ToolBackend) error {
	f.seq++
	if b.ID == "" {
		b.ID = "be-" + b.Name
	}
	f.byTool[b.ToolID] = append(f.byTool[b.ToolID], *b)
	return nil
}

func toolHTTPRequest(h *ToolHandler, method, id, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "user-1")
		c.Next()
	})
	r.POST("/tools", h.Create)
	r.PUT("/tools/:id", h.Update)

	path := "/tools"
	if id != "" {
		path += "/" + id
	}
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestToolHandler_RejectsPrivateBackendURLs(t *testing.T) {
	tools, backends := newFakeToolRepo()
	h := NewToolHandler(tools, backends, nil, "01234567890123456789012345678901")

	w := toolHTTPRequest(h, http.MethodPost, "", `{
		"name": "metadata_probe",
		"backends": [{"name": "imds", "endpointUrl": "http://169.254.169.254/latest/meta-data"}]
	}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blocked backend URL on create, got %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("TOOL_BACKEND_URL_BLOCKED")) {
		t.Fatalf("expected TOOL_BACKEND_URL_BLOCKED in response body, got %s", w.Body.String())
	}

	// Create valid tool with public IP backend, then verify blocked URL on Update rejects BEFORE wiping existing backends.
	w = toolHTTPRequest(h, http.MethodPost, "", `{
		"name": "weather_api",
		"backends": [{"name": "primary", "endpointUrl": "https://93.184.216.34/weather", "authToken": "secret-1"}]
	}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for public backend URL, got %d: %s", w.Code, w.Body.String())
	}

	w = toolHTTPRequest(h, http.MethodPut, "tool-1", `{
		"backends": [{"name": "loopback", "endpointUrl": "http://127.0.0.1:8080/internal"}]
	}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blocked backend URL on update, got %d: %s", w.Code, w.Body.String())
	}
	if len(backends.byTool["tool-1"]) != 1 || backends.byTool["tool-1"][0].Name != "primary" {
		t.Fatalf("expected original backends to remain untouched when update SSRF check fails, got %+v", backends.byTool["tool-1"])
	}
}

func TestToolHandler_UpdatePreservesEncryptedAuthTokenAndPartialFields(t *testing.T) {
	encKey := "01234567890123456789012345678901"
	tools, backends := newFakeToolRepo()
	h := NewToolHandler(tools, backends, nil, encKey)

	enc, err := utils.EncryptAES256GCM("super-secret-token", encKey)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	tools.tools["tool-1"] = models.Tool{
		ID:          "tool-1",
		UserID:      "user-1",
		Name:        "billing_lookup",
		DisplayName: "Billing Lookup",
		Description: "Checks invoices",
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Enabled:     true,
	}
	backends.byTool["tool-1"] = []models.ToolBackend{
		{
			ID:                 "be-existing-1",
			ToolID:             "tool-1",
			Name:               "primary",
			EndpointURL:        "https://93.184.216.34/v1/billing",
			AuthTokenEncrypted: &enc,
			AuthHeaderName:     "X-API-Key",
			AuthHeaderPrefix:   "",
			TimeoutMs:          15000,
			Priority:           1,
			Enabled:            true,
		},
	}

	// Update timeout/URL while omitting authToken: existing secret and header metadata must be preserved.
	w := toolHTTPRequest(h, http.MethodPut, "tool-1", `{
		"backends": [{
			"id": "be-existing-1",
			"name": "primary",
			"endpointUrl": "https://93.184.216.34/v2/billing",
			"authToken": "",
			"timeoutMs": 20000,
			"priority": 1
		}]
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", w.Code, w.Body.String())
	}

	updated := backends.byTool["tool-1"]
	if len(updated) != 1 {
		t.Fatalf("expected 1 backend, got %d", len(updated))
	}
	if updated[0].AuthTokenEncrypted == nil {
		t.Fatalf("expected AuthTokenEncrypted to be preserved")
	}
	decrypted, err := utils.DecryptAES256GCM(*updated[0].AuthTokenEncrypted, encKey)
	if err != nil || decrypted != "super-secret-token" {
		t.Fatalf("expected decrypted token %q, got %q (err=%v)", "super-secret-token", decrypted, err)
	}
	if updated[0].AuthHeaderName != "X-API-Key" {
		t.Fatalf("expected AuthHeaderName X-API-Key, got %q", updated[0].AuthHeaderName)
	}
	if tools.tools["tool-1"].DisplayName != "Billing Lookup" {
		t.Fatalf("expected partial update to preserve DisplayName, got %q", tools.tools["tool-1"].DisplayName)
	}

	// Partial enabled toggle should succeed without wiping backends or metadata.
	w = toolHTTPRequest(h, http.MethodPut, "tool-1", `{"enabled": false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on partial enabled toggle, got %d: %s", w.Code, w.Body.String())
	}
	if tools.tools["tool-1"].Enabled {
		t.Fatalf("expected Enabled=false after toggle")
	}
	if len(backends.byTool["tool-1"]) != 1 {
		t.Fatalf("expected backends preserved when omitted from partial update")
	}
}
