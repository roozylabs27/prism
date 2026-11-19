package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMCPServerRepo struct {
	servers map[string]*models.MCPServer
}

func (f *fakeMCPServerRepo) ListByUserID(_ context.Context, userID string) ([]models.MCPServer, error) {
	var out []models.MCPServer
	for _, s := range f.servers {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (f *fakeMCPServerRepo) FindByID(_ context.Context, id, userID string) (*models.MCPServer, error) {
	if s, ok := f.servers[id]; ok && s.UserID == userID {
		cp := *s
		return &cp, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeMCPServerRepo) Create(_ context.Context, s *models.MCPServer) error {
	cp := *s
	f.servers[s.ID] = &cp
	return nil
}

func (f *fakeMCPServerRepo) Update(_ context.Context, s *models.MCPServer) error {
	cp := *s
	f.servers[s.ID] = &cp
	return nil
}

func (f *fakeMCPServerRepo) Delete(_ context.Context, id, userID string) error {
	if s, ok := f.servers[id]; ok && s.UserID == userID {
		delete(f.servers, id)
		return nil
	}
	return sql.ErrNoRows
}

func (f *fakeMCPServerRepo) GetServerWithTools(_ context.Context, s *models.MCPServer) (*models.MCPServerWithTools, error) {
	return &models.MCPServerWithTools{Server: *s}, nil
}

func TestMCPHandler_GetRedactsSensitiveHeadersAndUpdatePreservesThem(t *testing.T) {
	encKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	plainHeaders, err := json.Marshal(map[string]string{
		"Authorization": "Bearer super-secret-upstream-token-9876",
		"X-API-Key":     "sk-upstream-key-1234567890",
		"X-Custom-Tag":  "client-alpha",
	})
	require.NoError(t, err)
	enc, err := utils.EncryptAES256GCM(string(plainHeaders), encKey)
	require.NoError(t, err)

	repo := &fakeMCPServerRepo{
		servers: map[string]*models.MCPServer{
			"srv_1": {
				ID:               "srv_1",
				UserID:           "alice",
				Name:             "github-mcp",
				DisplayName:      "GitHub MCP",
				Type:             "remote",
				TransportType:    "http",
				EndpointURL:      "https://mcp.example.com/mcp",
				HeadersEncrypted: &enc,
				HasHeaders:       true,
				Enabled:          true,
			},
		},
	}

	h := &MCPHandler{servers: repo, encKey: encKey}
	r := asUser("alice", nil, func(r *gin.Engine) {
		r.GET("/api/mcp/servers/:id", h.Get)
		r.PUT("/api/mcp/servers/:id", h.Update)
	})

	// 1. GET must never expose plaintext secret headers
	w := doJSON(r, http.MethodGet, "/api/mcp/servers/srv_1", "")
	requireStatus(t, w, http.StatusOK)
	assert.NotContains(t, w.Body.String(), "super-secret-upstream-token-9876")
	assert.NotContains(t, w.Body.String(), "sk-upstream-key-1234567890")

	var editResp struct {
		Headers map[string]string `json:"headers"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &editResp))
	assert.Contains(t, editResp.Headers["Authorization"], "••••")
	assert.Contains(t, editResp.Headers["X-API-Key"], "••••")
	assert.Equal(t, "client-alpha", editResp.Headers["X-Custom-Tag"])

	// 2. Submitting the masked values back on PUT must preserve the real encrypted secret
	updatePayload, _ := json.Marshal(map[string]interface{}{
		"name":        "github-mcp",
		"displayName": "GitHub MCP Renamed",
		"headers": map[string]string{
			"Authorization": editResp.Headers["Authorization"],
			"X-API-Key":     editResp.Headers["X-API-Key"],
			"X-Custom-Tag":  "client-beta",
		},
	})
	uw := doJSON(r, http.MethodPut, "/api/mcp/servers/srv_1", string(updatePayload))
	requireStatus(t, uw, http.StatusOK)

	updatedDecrypted, err := h.decryptHeadersMap(repo.servers["srv_1"])
	require.NoError(t, err)
	assert.Equal(t, "Bearer super-secret-upstream-token-9876", updatedDecrypted["Authorization"])
	assert.Equal(t, "sk-upstream-key-1234567890", updatedDecrypted["X-API-Key"])
	assert.Equal(t, "client-beta", updatedDecrypted["X-Custom-Tag"])
}
