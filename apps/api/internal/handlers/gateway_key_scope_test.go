package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGatewayKeyStore struct {
	created []*models.GatewayAPIKey
}

func (f *fakeGatewayKeyStore) Create(_ context.Context, k *models.GatewayAPIKey) error {
	cp := *k
	cp.ID = "gk_test"
	k.ID = cp.ID
	f.created = append(f.created, &cp)
	return nil
}

func (f *fakeGatewayKeyStore) ListByUserIDWithFilter(_ context.Context, _, _ string, _, _ int, _ ...string) ([]models.GatewayAPIKey, int64, error) {
	return nil, 0, nil
}

func (f *fakeGatewayKeyStore) Delete(_ context.Context, _ string, _ ...string) (string, error) {
	return "", nil
}

type fakeGatewayScopeValidator struct {
	workspaces map[string]models.Workspace
	projects   map[string]models.Project
}

func (f fakeGatewayScopeValidator) FindWorkspaceInOrg(_ context.Context, orgID, workspaceID string) (*models.Workspace, error) {
	ws, ok := f.workspaces[workspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrWorkspaceNotFound
	}
	return &ws, nil
}

func (f fakeGatewayScopeValidator) FindProjectInOrg(_ context.Context, orgID, projectID string) (*models.Project, error) {
	p, ok := f.projects[projectID]
	if !ok {
		return nil, repository.ErrProjectNotFound
	}
	ws, ok := f.workspaces[p.WorkspaceID]
	if !ok || ws.OrgID != orgID {
		return nil, repository.ErrProjectNotFound
	}
	return &p, nil
}

func TestGatewayKeyHandler_ScopeValidationAndNilEmptyPointers(t *testing.T) {
	store := &fakeGatewayKeyStore{}
	validator := fakeGatewayScopeValidator{
		workspaces: map[string]models.Workspace{
			"ws_alpha": {ID: "ws_alpha", OrgID: "org_agency", Name: "Alpha"},
			"ws_beta":  {ID: "ws_beta", OrgID: "org_agency", Name: "Beta"},
			"ws_other": {ID: "ws_other", OrgID: "org_other", Name: "Foreign"},
		},
		projects: map[string]models.Project{
			"proj_alpha_1": {ID: "proj_alpha_1", WorkspaceID: "ws_alpha", Name: "Alpha Bot"},
			"proj_other_1": {ID: "proj_other_1", WorkspaceID: "ws_other", Name: "Foreign Bot"},
		},
	}

	h := &GatewayKeyHandler{keys: store}
	h.SetScopeValidator(validator)

	orgOnlyTenant := &models.TenantContext{OrgID: "org_agency"}
	r := asUser("alice", orgOnlyTenant, func(r *gin.Engine) {
		r.POST("/api/gateway-keys", h.Create)
	})

	// 1. Org-only session creates org-wide key with nil WorkspaceID & ProjectID (no empty string FK bug)
	wOrgOnly := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"org-wide"}`)
	requireStatus(t, wOrgOnly, http.StatusCreated)
	require.Len(t, store.created, 1)
	assert.Equal(t, "org_agency", *store.created[0].OrgID)
	assert.Nil(t, store.created[0].WorkspaceID)
	assert.Nil(t, store.created[0].ProjectID)

	// 2. Explicit workspaceId + matching projectId pins both cleanly
	wScoped := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"client-alpha","workspaceId":"ws_alpha","projectId":"proj_alpha_1"}`)
	requireStatus(t, wScoped, http.StatusCreated)
	require.Len(t, store.created, 2)
	require.NotNil(t, store.created[1].WorkspaceID)
	require.NotNil(t, store.created[1].ProjectID)
	assert.Equal(t, "ws_alpha", *store.created[1].WorkspaceID)
	assert.Equal(t, "proj_alpha_1", *store.created[1].ProjectID)

	// 3. Explicit projectId alone auto-pins parent workspaceId
	wProjOnly := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"proj-auto-ws","projectId":"proj_alpha_1"}`)
	requireStatus(t, wProjOnly, http.StatusCreated)
	require.Len(t, store.created, 3)
	require.NotNil(t, store.created[2].WorkspaceID)
	assert.Equal(t, "ws_alpha", *store.created[2].WorkspaceID)

	// 4. Project belonging to another workspace in the same org -> 400 mismatch
	wMismatch := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"bad-mix","workspaceId":"ws_beta","projectId":"proj_alpha_1"}`)
	requireStatus(t, wMismatch, http.StatusBadRequest)
	assert.Contains(t, wMismatch.Body.String(), "PROJECT_WORKSPACE_MISMATCH")

	// 5. Foreign org workspace or project -> 404
	wForeignWS := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"foreign-ws","workspaceId":"ws_other"}`)
	requireStatus(t, wForeignWS, http.StatusNotFound)
	assert.Contains(t, wForeignWS.Body.String(), "WORKSPACE_NOT_FOUND")

	wForeignProj := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"foreign-proj","projectId":"proj_other_1"}`)
	requireStatus(t, wForeignProj, http.StatusNotFound)
	assert.Contains(t, wForeignProj.Body.String(), "PROJECT_NOT_FOUND")
}

type fakeGatewayAgentValidator struct {
	agents map[string]*models.Agent
}

func (f fakeGatewayAgentValidator) FindForUser(_ context.Context, _, ref string) (*models.Agent, error) {
	if ag, ok := f.agents[ref]; ok {
		return ag, nil
	}
	return nil, sql.ErrNoRows
}

func TestGatewayKeyHandler_AgentBindingValidation(t *testing.T) {
	store := &fakeGatewayKeyStore{}
	validator := fakeGatewayAgentValidator{
		agents: map[string]*models.Agent{
			"ag_active": {ID: "ag_active", Name: "coder", Enabled: true},
			"coder":     {ID: "ag_active", Name: "coder", Enabled: true},
			"ag_off":    {ID: "ag_off", Name: "disabled", Enabled: false},
		},
	}

	h := &GatewayKeyHandler{keys: store}
	h.SetAgentValidator(validator)

	r := asUser("alice", &models.TenantContext{OrgID: "org_agency"}, func(r *gin.Engine) {
		r.POST("/api/gateway-keys", h.Create)
	})

	// Resolving by agent name pins canonical agent ID
	wValid := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"coder-key","agentId":"coder"}`)
	requireStatus(t, wValid, http.StatusCreated)
	require.Len(t, store.created, 1)
	require.NotNil(t, store.created[0].AgentID)
	assert.Equal(t, "ag_active", *store.created[0].AgentID)

	// Disabled agent rejected with 400
	wDisabled := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"off-key","agentId":"ag_off"}`)
	requireStatus(t, wDisabled, http.StatusBadRequest)
	assert.Contains(t, wDisabled.Body.String(), "AGENT_DISABLED")

	// Unknown agent rejected with 404
	wUnknown := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"ghost-key","agentId":"ag_missing"}`)
	requireStatus(t, wUnknown, http.StatusNotFound)
	assert.Contains(t, wUnknown.Body.String(), "AGENT_NOT_FOUND")
}

type fakeGatewayKeyCredCounter struct {
	count     int64
	gotCtx    context.Context
	gotProvID string
	gotUserID []string
}

func (f *fakeGatewayKeyCredCounter) CountActiveByProviderID(ctx context.Context, providerID string, userID ...string) (int64, error) {
	f.gotCtx = ctx
	f.gotProvID = providerID
	f.gotUserID = append([]string(nil), userID...)
	return f.count, nil
}

func TestGatewayKeyHandler_CredentialCounterReceivesScopedTenantContext(t *testing.T) {
	store := &fakeGatewayKeyStore{}
	counter := &fakeGatewayKeyCredCounter{count: 1}
	h := &GatewayKeyHandler{keys: store, credentials: counter}

	r := asUser("alice", &models.TenantContext{OrgID: "org_agency"}, func(r *gin.Engine) {
		r.POST("/api/gateway-keys", h.Create)
	})

	w := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"prov-bound","providerId":"prov_org"}`)
	requireStatus(t, w, http.StatusCreated)
	require.NotNil(t, counter.gotCtx)
	assert.Equal(t, "prov_org", counter.gotProvID)
	assert.Equal(t, []string{"alice"}, counter.gotUserID)
	dbCtx := database.TenantContextFrom(counter.gotCtx)
	assert.Equal(t, "org_agency", dbCtx.OrgID)
	assert.Equal(t, "alice", dbCtx.UserID)

	counter.count = 0
	wNone := doJSON(r, http.MethodPost, "/api/gateway-keys", `{"name":"prov-empty","providerId":"prov_org"}`)
	requireStatus(t, wNone, http.StatusBadRequest)
	assert.Contains(t, wNone.Body.String(), "PROVIDER_NO_CREDENTIALS")
}
