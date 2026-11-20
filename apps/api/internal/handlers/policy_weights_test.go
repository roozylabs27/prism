package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
)

type fakeRoutingPolicyRepo struct {
	policies map[string]models.RoutingPolicy
}

func (f *fakeRoutingPolicyRepo) ListByUserID(_ context.Context, userID string) ([]models.RoutingPolicy, error) {
	var out []models.RoutingPolicy
	for _, p := range f.policies {
		if p.UserID == userID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRoutingPolicyRepo) FindByID(_ context.Context, id, userID string) (*models.RoutingPolicy, error) {
	p, ok := f.policies[id]
	if !ok || p.UserID != userID {
		return nil, sql.ErrNoRows
	}
	cp := p
	return &cp, nil
}

func (f *fakeRoutingPolicyRepo) Create(_ context.Context, p *models.RoutingPolicy) error {
	if p.ID == "" {
		p.ID = "pol-1"
	}
	f.policies[p.ID] = *p
	return nil
}

func (f *fakeRoutingPolicyRepo) Update(_ context.Context, p *models.RoutingPolicy) error {
	if _, ok := f.policies[p.ID]; !ok {
		return sql.ErrNoRows
	}
	f.policies[p.ID] = *p
	return nil
}

func (f *fakeRoutingPolicyRepo) Delete(_ context.Context, id, userID string) error {
	p, ok := f.policies[id]
	if !ok || p.UserID != userID {
		return sql.ErrNoRows
	}
	delete(f.policies, id)
	return nil
}

func (f *fakeRoutingPolicyRepo) SetDefault(_ context.Context, id, userID string) error {
	return nil
}

func (f *fakeRoutingPolicyRepo) FindByName(_ context.Context, name, userID string) (*models.RoutingPolicy, error) {
	for _, p := range f.policies {
		if p.Name == name && p.UserID == userID {
			cp := p
			return &cp, nil
		}
	}
	return nil, sql.ErrNoRows
}

func TestRoutingPolicyHandler_NormalizesAndValidatesWeights(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRoutingPolicyRepo{policies: map[string]models.RoutingPolicy{}}
	h := NewRoutingPolicyHandler(repo)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "user-1")
		c.Next()
	})
	r.POST("/policies", h.Create)
	r.PUT("/policies/:id", h.Update)

	// 1. Percentage weights submitted on create normalize cleanly to 1.0.
	req := httptest.NewRequest(http.MethodPost, "/policies", bytes.NewBufferString(`{
		"name": "percent-weights",
		"weights": {"task_match": 35, "quality": 35, "cost": 15, "speed": 15}
	}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var created models.RoutingPolicy
	_ = json.Unmarshal(w.Body.Bytes(), &created)
	if math.Abs(created.Weights["task_match"]-0.35) > 1e-6 || math.Abs(created.Weights["cost"]-0.15) > 1e-6 {
		t.Fatalf("expected normalized weights on create, got %v", created.Weights)
	}

	// 2. Negative weight rejected with 400 INVALID_WEIGHTS on Create.
	reqBad := httptest.NewRequest(http.MethodPost, "/policies", bytes.NewBufferString(`{
		"name": "bad-weights",
		"weights": {"quality": -10, "cost": 50}
	}`))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusBadRequest || !bytes.Contains(wBad.Body.Bytes(), []byte("INVALID_WEIGHTS")) {
		t.Fatalf("expected 400 INVALID_WEIGHTS on negative create weight, got %d: %s", wBad.Code, wBad.Body.String())
	}

	// 3. Update normalizes weights and rejects negative weights.
	reqUpd := httptest.NewRequest(http.MethodPut, "/policies/pol-1", bytes.NewBufferString(`{
		"weights": {"task_match": 20, "quality": 40, "cost": 20, "speed": 20}
	}`))
	reqUpd.Header.Set("Content-Type", "application/json")
	wUpd := httptest.NewRecorder()
	r.ServeHTTP(wUpd, reqUpd)
	if wUpd.Code != http.StatusOK {
		t.Fatalf("expected 200 on update, got %d: %s", wUpd.Code, wUpd.Body.String())
	}
	if math.Abs(repo.policies["pol-1"].Weights["quality"]-0.4) > 1e-6 {
		t.Fatalf("expected normalized quality weight 0.4, got %v", repo.policies["pol-1"].Weights)
	}

	reqUpdBad := httptest.NewRequest(http.MethodPut, "/policies/pol-1", bytes.NewBufferString(`{
		"weights": {"speed": -1}
	}`))
	reqUpdBad.Header.Set("Content-Type", "application/json")
	wUpdBad := httptest.NewRecorder()
	r.ServeHTTP(wUpdBad, reqUpdBad)
	if wUpdBad.Code != http.StatusBadRequest || !bytes.Contains(wUpdBad.Body.Bytes(), []byte("INVALID_WEIGHTS")) {
		t.Fatalf("expected 400 INVALID_WEIGHTS on negative update weight, got %d: %s", wUpdBad.Code, wUpdBad.Body.String())
	}
}
