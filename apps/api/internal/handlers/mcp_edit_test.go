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

type fakeMCPEditRepo struct {
	servers map[string]*models.MCPServer
}

func (f *fakeMCPEditRepo) ListByUserID(_ context.Context, userID string) ([]models.MCPServer, error) {
	var out []models.MCPServer
	for _, s := range f.servers {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (f *fakeMCPEditRepo) FindByID(_ context.Context, id, userID string) (*models.MCPServer, error) {
	s, ok := f.servers[id]
	if !ok || s.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := *s
	return &cp, nil
}

func (f *fakeMCPEditRepo) Create(_ context.Context, s *models.MCPServer) error {
	if s.ID == "" {
		s.ID = "mcp-1"
	}
	cp := *s
	f.servers[s.ID] = &cp
	return nil
}

func (f *fakeMCPEditRepo) Update(_ context.Context, s *models.MCPServer) error {
	if _, ok := f.servers[s.ID]; !ok {
		return sql.ErrNoRows
	}
	cp := *s
	f.servers[s.ID] = &cp
	return nil
}

func (f *fakeMCPEditRepo) Delete(_ context.Context, id, userID string) error {
	s, ok := f.servers[id]
	if !ok || s.UserID != userID {
		return sql.ErrNoRows
	}
	delete(f.servers, id)
	return nil
}

func (f *fakeMCPEditRepo) GetServerWithTools(_ context.Context, s *models.MCPServer) (*models.MCPServerWithTools, error) {
	return &models.MCPServerWithTools{Server: *s}, nil
}

func TestMCPHandler_PartialToggleAndAuthSecretPreservation(t *testing.T) {
	encKey := "01234567890123456789012345678901"
	rawHeaders, _ := json.Marshal(map[string]string{
		"Authorization": "Bearer super-real-token-9999",
		"X-Trace":       "abc",
	})
	encHeaders, err := utils.EncryptAES256GCM(string(rawHeaders), encKey)
	if err != nil {
		t.Fatalf("encrypt headers: %v", err)
	}

	repo := &fakeMCPEditRepo{
		servers: map[string]*models.MCPServer{
			"mcp-1": {
				ID:               "mcp-1",
				UserID:           "user-1",
				Name:             "github-mcp",
				DisplayName:      "GitHub MCP",
				Description:      "Repo tools",
				Type:             "remote",
				TransportType:    "http",
				EndpointURL:      "https://93.184.216.34/mcp",
				HeadersEncrypted: &encHeaders,
				HasHeaders:       true,
				Enabled:          true,
			},
		},
	}
	h := &MCPHandler{servers: repo, encKey: encKey}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "user-1")
		c.Next()
	})
	r.GET("/mcp/servers/:id", h.Get)
	r.PUT("/mcp/servers/:id", h.Update)

	// 1. GET returns HasAuthToken=true even when token is stored inside HeadersEncrypted.
	reqGet := httptest.NewRequest(http.MethodGet, "/mcp/servers/mcp-1", nil)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", wGet.Code, wGet.Body.String())
	}
	var editResp MCPServerEdit
	if err := json.Unmarshal(wGet.Body.Bytes(), &editResp); err != nil {
		t.Fatalf("decode editResp: %v", err)
	}
	if !editResp.HasAuthToken {
		t.Fatalf("expected HasAuthToken=true when Authorization header exists")
	}

	// 2. Partial PUT {"enabled": false} succeeds 200 and preserves metadata + secret.
	reqToggle := httptest.NewRequest(http.MethodPut, "/mcp/servers/mcp-1", bytes.NewBufferString(`{"enabled":false}`))
	reqToggle.Header.Set("Content-Type", "application/json")
	wToggle := httptest.NewRecorder()
	r.ServeHTTP(wToggle, reqToggle)
	if wToggle.Code != http.StatusOK {
		t.Fatalf("expected 200 on partial toggle, got %d: %s", wToggle.Code, wToggle.Body.String())
	}
	if repo.servers["mcp-1"].Enabled || repo.servers["mcp-1"].DisplayName != "GitHub MCP" {
		t.Fatalf("expected enabled=false and DisplayName preserved, got %+v", repo.servers["mcp-1"])
	}

	// 3. Edit custom header with masked bullets in authToken must keep the original Bearer secret.
	reqEdit := httptest.NewRequest(http.MethodPut, "/mcp/servers/mcp-1", bytes.NewBufferString(`{
		"authToken": "Bearer ••••9999",
		"headers": {"X-Trace": "updated"}
	}`))
	reqEdit.Header.Set("Content-Type", "application/json")
	wEdit := httptest.NewRecorder()
	r.ServeHTTP(wEdit, reqEdit)
	if wEdit.Code != http.StatusOK {
		t.Fatalf("expected 200 on header edit, got %d: %s", wEdit.Code, wEdit.Body.String())
	}

	decryptedMap, err := h.decryptHeadersMap(repo.servers["mcp-1"])
	if err != nil {
		t.Fatalf("decrypt merged headers: %v", err)
	}
	if decryptedMap["Authorization"] != "Bearer super-real-token-9999" {
		t.Fatalf("expected prior Authorization secret preserved, got %q", decryptedMap["Authorization"])
	}
	if decryptedMap["X-Trace"] != "updated" {
		t.Fatalf("expected X-Trace=updated, got %q", decryptedMap["X-Trace"])
	}
}
