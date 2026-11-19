package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeToolExecutor struct {
	calls int
}

func (f *fakeToolExecutor) Execute(_ context.Context, _, toolName string, _ map[string]interface{}, _ string) (*proxy.ToolExecutionResult, error) {
	f.calls++
	return &proxy.ToolExecutionResult{Tool: toolName, Backend: "mock-backend", StatusCode: 200, LatencyMs: 5, Result: map[string]interface{}{"ok": true}}, nil
}

type fakeResourceExecutor struct {
	calls int
}

func (f *fakeResourceExecutor) Execute(_ context.Context, _, resourceName string, _ map[string]interface{}, _ string) (*models.ResourceExecutionResult, error) {
	f.calls++
	return &models.ResourceExecutionResult{Resource: resourceName, Backend: "mock-res", BackendType: "rest", StatusCode: 200, LatencyMs: 4, Data: []interface{}{}}, nil
}

func (f *fakeResourceExecutor) ClosePools() {}

type fixedRoleResolver string

func (r fixedRoleResolver) GetOrganizationMemberRole(_ context.Context, _, _ string) (string, error) {
	return string(r), nil
}

func TestToolGatewayHandler_EnforcesDenyRulesAndGovernance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	exec := &fakeToolExecutor{}
	h := &ToolGatewayHandler{gateway: exec}
	adm := proxy.NewAdmissionController(nil, nil, nil, nil)
	h.SetGovernance(adm, nil, fixedRoleResolver("developer"))

	orgID := "org_1"
	agent := &models.Agent{
		ID:           "ag_1",
		Name:         "restricted-bot",
		Enabled:      true,
		AllowedTools: []string{"*", "!delete_customer"},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", OrgID: &orgID})
		c.Set("agentObject", agent)
		c.Next()
	})
	r.POST("/v1/tools/:toolName/execute", h.ExecuteTool)

	// Denied tool (!delete_customer) must be rejected before executor runs.
	wDeny := httptest.NewRecorder()
	reqDeny := httptest.NewRequest(http.MethodPost, "/v1/tools/delete_customer/execute", bytes.NewBufferString(`{"args":{"id":1}}`))
	reqDeny.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wDeny, reqDeny)
	assert.Equal(t, http.StatusForbidden, wDeny.Code, wDeny.Body.String())
	assert.Equal(t, 0, exec.calls, "executor must not run when tool is explicitly denied")

	// Allowed tool executes cleanly.
	wAllow := httptest.NewRecorder()
	reqAllow := httptest.NewRequest(http.MethodPost, "/v1/tools/search_kb/execute", bytes.NewBufferString(`{"args":{"q":"pricing"}}`))
	reqAllow.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wAllow, reqAllow)
	assert.Equal(t, http.StatusOK, wAllow.Code, wAllow.Body.String())
	assert.Equal(t, 1, exec.calls)
}

func TestResourceGatewayHandler_EnforcesDenyRulesAndGovernance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	exec := &fakeResourceExecutor{}
	h := NewResourceGatewayHandler(exec, nil, nil, "")
	adm := proxy.NewAdmissionController(nil, nil, nil, nil)
	h.SetGovernance(adm, nil, fixedRoleResolver("developer"))

	orgID := "org_1"
	agent := &models.Agent{
		ID:               "ag_1",
		Name:             "restricted-bot",
		Enabled:          true,
		AllowedResources: []string{"*", "!payroll_db"},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("gatewayKey", &models.GatewayAPIKey{ID: "gk_1", UserID: "u1", OrgID: &orgID})
		c.Set("agentObject", agent)
		c.Next()
	})
	r.POST("/v1/resources/:resourceName/query", h.ExecuteQuery)

	wDeny := httptest.NewRecorder()
	reqDeny := httptest.NewRequest(http.MethodPost, "/v1/resources/payroll_db/query", bytes.NewBufferString(`{"args":{}}`))
	reqDeny.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wDeny, reqDeny)
	assert.Equal(t, http.StatusForbidden, wDeny.Code, wDeny.Body.String())
	assert.Equal(t, 0, exec.calls)

	wAllow := httptest.NewRecorder()
	reqAllow := httptest.NewRequest(http.MethodPost, "/v1/resources/docs_index/query", bytes.NewBufferString(`{"args":{"q":"sla"}}`))
	reqAllow.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wAllow, reqAllow)
	assert.Equal(t, http.StatusOK, wAllow.Code, wAllow.Body.String())
	assert.Equal(t, 1, exec.calls)
}

func TestFeaturesHandler_IgnoresPlanTierQuerySpoofing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewFeaturesHandler()
	r := gin.New()
	r.GET("/api/features", h.GetFeatures)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/features?plan_tier=enterprise", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var resp FeaturesResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "free", resp.PlanTier, "query parameter must not elevate plan_tier")
}
