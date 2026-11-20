package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
)

type fakeAgentRepo struct {
	agents map[string]models.Agent
}

func (f *fakeAgentRepo) ListByUserID(_ context.Context, userID string) ([]models.Agent, error) {
	var out []models.Agent
	for _, a := range f.agents {
		if a.UserID == userID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAgentRepo) FindByID(_ context.Context, id, userID string) (*models.Agent, error) {
	a, ok := f.agents[id]
	if !ok || a.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := a
	cp.AllowedModels = append([]string(nil), a.AllowedModels...)
	cp.AllowedTools = append([]string(nil), a.AllowedTools...)
	cp.AllowedResources = append([]string(nil), a.AllowedResources...)
	cp.AllowedMCPServers = append([]string(nil), a.AllowedMCPServers...)
	return &cp, nil
}

func (f *fakeAgentRepo) Create(_ context.Context, a *models.Agent) error {
	if a.ID == "" {
		a.ID = "agent-1"
	}
	f.agents[a.ID] = *a
	return nil
}

func (f *fakeAgentRepo) Update(_ context.Context, a *models.Agent) error {
	if _, ok := f.agents[a.ID]; !ok {
		return sql.ErrNoRows
	}
	f.agents[a.ID] = *a
	return nil
}

func (f *fakeAgentRepo) Delete(_ context.Context, id, userID string) error {
	a, ok := f.agents[id]
	if !ok || a.UserID != userID {
		return sql.ErrNoRows
	}
	delete(f.agents, id)
	return nil
}

func TestAgentHandler_UpdatePointerSemantics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeAgentRepo{
		agents: map[string]models.Agent{
			"agent-1": {
				ID:                   "agent-1",
				UserID:               "user-1",
				Name:                 "coder",
				DisplayName:          "Coder Agent",
				Description:          "Writes code",
				AgentType:            "coding",
				SystemPromptOverride: "You are a concise Go engineer.",
				AllowedModels:        []string{"gpt-4o", "claude-3-5-sonnet"},
				AllowedTools:         []string{"search"},
				AllowedResources:     []string{"docs://readme"},
				AllowedMCPServers:    []string{"github"},
				MaxBudgetCents:       500,
				Status:               "active",
				Enabled:              true,
			},
		},
	}
	h := NewAgentHandler(repo, nil, nil)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "user-1")
		c.Next()
	})
	r.PUT("/agents/:id", h.Update)

	put := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/agents/agent-1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// 1. Omitted fields must be preserved when updating enabled only.
	if w := put(`{"enabled": false}`); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	got := repo.agents["agent-1"]
	if got.Enabled {
		t.Fatalf("expected enabled=false")
	}
	if got.SystemPromptOverride != "You are a concise Go engineer." {
		t.Fatalf("expected systemPromptOverride preserved, got %q", got.SystemPromptOverride)
	}
	if len(got.AllowedModels) != 2 || got.MaxBudgetCents != 500 || got.DisplayName != "Coder Agent" {
		t.Fatalf("expected omitted fields preserved, got %+v", got)
	}

	// 2. Explicit empty string / empty arrays / zero budget must clear fields.
	w := put(`{
		"systemPromptOverride": "",
		"allowedModels": [],
		"allowedTools": [],
		"maxBudgetCents": 0
	}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on explicit clear, got %d: %s", w.Code, w.Body.String())
	}
	got = repo.agents["agent-1"]
	if got.SystemPromptOverride != "" {
		t.Fatalf("expected cleared systemPromptOverride, got %q", got.SystemPromptOverride)
	}
	if len(got.AllowedModels) != 0 || len(got.AllowedTools) != 0 {
		t.Fatalf("expected cleared allowlists, got models=%v tools=%v", got.AllowedModels, got.AllowedTools)
	}
	if got.MaxBudgetCents != 0 {
		t.Fatalf("expected zero budget, got %d", got.MaxBudgetCents)
	}

	// 3. Negative budget must fail 400.
	if w := put(`{"maxBudgetCents": -10}`); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on negative budget, got %d: %s", w.Code, w.Body.String())
	}
}
