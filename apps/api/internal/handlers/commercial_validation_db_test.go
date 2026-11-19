package handlers

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommercialValidation_WorkspacesKeysAndPilotOutcomes_Postgres(t *testing.T) {
	db := migratedDB(t)
	ctx := context.Background()

	_, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES
			('alice', 'Alice Agency', 'alice@agency.local'),
			('bob', 'Bob Other', 'bob@other.local');
		INSERT INTO organizations (id, name, slug, max_workspaces, max_projects_per_workspace) VALUES
			('org_agency', 'Agency Org', 'agency-org', 2, 2),
			('org_other', 'Other Org', 'other-org', 2, 2);
		INSERT INTO organization_members (id, org_id, user_id, role_id)
		SELECT 'om_agency', 'org_agency', 'alice', id FROM roles WHERE slug = 'owner' AND is_system LIMIT 1;
		INSERT INTO organization_members (id, org_id, user_id, role_id)
		SELECT 'om_other', 'org_other', 'bob', id FROM roles WHERE slug = 'owner' AND is_system LIMIT 1;
		INSERT INTO workspaces (id, org_id, name, slug) VALUES
			('ws_foreign', 'org_other', 'Foreign Client', 'foreign-client');
	`)
	require.NoError(t, err)

	wsRepo := repository.NewWorkspaceRepository(db)
	gkRepo := repository.NewGatewayKeyRepository(db)
	credRepo := repository.NewCredentialRepository(db)
	logRepo := repository.NewRequestLogRepository(db)
	decRepo := repository.NewRoutingDecisionRepository(db)
	auditRepo := repository.NewAuditTrailRepository(db)
	pilotRepo := repository.NewPilotOutcomesRepository(db)

	wsHandler := NewWorkspaceHandler(wsRepo)
	gkHandler := NewGatewayKeyHandler(gkRepo, credRepo, nil)
	gkHandler.SetScopeValidator(wsRepo)
	pilotHandler := NewPilotOutcomesHandler(pilotRepo)

	register := func(r *gin.Engine) {
		r.GET("/api/workspaces", wsHandler.ListWorkspaces)
		r.POST("/api/workspaces", wsHandler.CreateWorkspace)
		r.GET("/api/workspaces/:workspaceId/projects", wsHandler.ListProjects)
		r.POST("/api/workspaces/:workspaceId/projects", wsHandler.CreateProject)
		r.POST("/api/gateway-keys", gkHandler.Create)
		r.GET("/api/analytics/pilot-outcomes", pilotHandler.GetSummary)
		r.GET("/api/analytics/pilot-outcomes/export", pilotHandler.Export)
	}

	aliceRouter := asUser("alice", &models.TenantContext{OrgID: "org_agency", Role: "owner"}, register)
	bobRouter := asUser("bob", &models.TenantContext{OrgID: "org_other", Role: "owner"}, register)

	// 1. Create two isolated client workspaces (up to org_agency max_workspaces=2)
	wAlpha := doJSON(aliceRouter, http.MethodPost, "/api/workspaces", `{"name":"Client Alpha","slug":"client-alpha"}`)
	requireStatus(t, wAlpha, http.StatusCreated)
	var wsAlpha models.Workspace
	require.NoError(t, json.Unmarshal(wAlpha.Body.Bytes(), &wsAlpha))

	wBeta := doJSON(aliceRouter, http.MethodPost, "/api/workspaces", `{"name":"Client Beta","slug":"client-beta"}`)
	requireStatus(t, wBeta, http.StatusCreated)
	var wsBeta models.Workspace
	require.NoError(t, json.Unmarshal(wBeta.Body.Bytes(), &wsBeta))

	// Third workspace violates max_workspaces=2 -> 409 WORKSPACE_LIMIT_REACHED
	wLimit := doJSON(aliceRouter, http.MethodPost, "/api/workspaces", `{"name":"Client Gamma","slug":"client-gamma"}`)
	requireStatus(t, wLimit, http.StatusConflict)
	assert.Contains(t, wLimit.Body.String(), "WORKSPACE_LIMIT_REACHED")

	wListWS := doJSON(aliceRouter, http.MethodGet, "/api/workspaces", "")
	requireStatus(t, wListWS, http.StatusOK)
	assert.Contains(t, wListWS.Body.String(), "Client Alpha")
	assert.Contains(t, wListWS.Body.String(), "Client Beta")
	assert.NotContains(t, wListWS.Body.String(), "Foreign Client")

	// 2. Create project under Client Alpha & verify foreign workspace isolation
	wProj := doJSON(aliceRouter, http.MethodPost, "/api/workspaces/"+wsAlpha.ID+"/projects", `{"name":"Support Bot"}`)
	requireStatus(t, wProj, http.StatusCreated)
	var projAlpha models.Project
	require.NoError(t, json.Unmarshal(wProj.Body.Bytes(), &projAlpha))

	wForeignProj := doJSON(aliceRouter, http.MethodPost, "/api/workspaces/ws_foreign/projects", `{"name":"Sneaky"}`)
	requireStatus(t, wForeignProj, http.StatusNotFound)

	// 3. Gateway Keys: org-wide NULL pointers + pinned workspace/project + mismatch rejection
	wOrgKey := doJSON(aliceRouter, http.MethodPost, "/api/gateway-keys", `{"name":"org-wide"}`)
	requireStatus(t, wOrgKey, http.StatusCreated)
	orgKeyBody := decodeBody(t, wOrgKey)
	orgKeyStored, err := gkRepo.FindByKeyHash(ctx, utils.HashSHA256(orgKeyBody["key"].(string)))
	require.NoError(t, err)
	assert.Nil(t, orgKeyStored.WorkspaceID)
	assert.Nil(t, orgKeyStored.ProjectID)

	wAlphaKey := doJSON(aliceRouter, http.MethodPost, "/api/gateway-keys", `{"name":"alpha-key","workspaceId":"`+wsAlpha.ID+`","projectId":"`+projAlpha.ID+`"}`)
	requireStatus(t, wAlphaKey, http.StatusCreated)
	alphaKeyBody := decodeBody(t, wAlphaKey)
	alphaKeyID := alphaKeyBody["id"].(string)
	alphaKeyStored, err := gkRepo.FindByKeyHash(ctx, utils.HashSHA256(alphaKeyBody["key"].(string)))
	require.NoError(t, err)
	require.NotNil(t, alphaKeyStored.WorkspaceID)
	require.NotNil(t, alphaKeyStored.ProjectID)
	assert.Equal(t, wsAlpha.ID, *alphaKeyStored.WorkspaceID)
	assert.Equal(t, projAlpha.ID, *alphaKeyStored.ProjectID)

	wBetaKey := doJSON(aliceRouter, http.MethodPost, "/api/gateway-keys", `{"name":"beta-key","workspaceId":"`+wsBeta.ID+`"}`)
	requireStatus(t, wBetaKey, http.StatusCreated)
	betaKeyBody := decodeBody(t, wBetaKey)
	betaKeyID := betaKeyBody["id"].(string)

	wMismatch := doJSON(aliceRouter, http.MethodPost, "/api/gateway-keys", `{"name":"bad-key","workspaceId":"`+wsBeta.ID+`","projectId":"`+projAlpha.ID+`"}`)
	requireStatus(t, wMismatch, http.StatusBadRequest)
	assert.Contains(t, wMismatch.Body.String(), "PROJECT_WORKSPACE_MISMATCH")

	// 4. Seed real telemetry rows across Client Alpha and Client Beta
	require.NoError(t, logRepo.Create(ctx, &models.RequestLog{
		RequestID:       "req-alpha-1",
		OrgID:           "org_agency",
		WorkspaceID:     wsAlpha.ID,
		ProjectID:       projAlpha.ID,
		GatewayAPIKeyID: &alphaKeyID,
		Model:           "gpt-4o-mini",
		StatusCode:      200,
		LatencyMs:       120,
		InputTokens:     1000,
		OutputTokens:    500,
		TotalTokens:     1500,
		CostUSD:         0.00045,
	}))
	require.NoError(t, logRepo.Create(ctx, &models.RequestLog{
		RequestID:       "req-alpha-2",
		OrgID:           "org_agency",
		WorkspaceID:     wsAlpha.ID,
		ProjectID:       projAlpha.ID,
		GatewayAPIKeyID: &alphaKeyID,
		Model:           "gpt-4o",
		StatusCode:      200,
		LatencyMs:       300,
		RetryCount:      1,
		InputTokens:     500,
		OutputTokens:    200,
		TotalTokens:     700,
		CostUSD:         0.00325,
	}))
	require.NoError(t, logRepo.Create(ctx, &models.RequestLog{
		RequestID:       "req-beta-1",
		OrgID:           "org_agency",
		WorkspaceID:     wsBeta.ID,
		GatewayAPIKeyID: &betaKeyID,
		Model:           "gpt-4o",
		StatusCode:      500,
		LatencyMs:       900,
		ErrorMessage:    sql.NullString{String: "upstream error", Valid: true},
		InputTokens:     200,
		OutputTokens:    0,
		TotalTokens:     200,
		CostUSD:         0.0,
	}))

	require.NoError(t, decRepo.Create(ctx, &repository.RoutingDecisionLog{
		RequestID:        "req-alpha-1",
		UserID:           "alice",
		TaskType:         "classification",
		Complexity:       "low",
		PolicyName:       "cost-saver",
		SelectedModel:    "gpt-4o-mini",
		SelectedProvider: "openai",
		EstimatedCost:    0.0005,
		ActualCost:       0.00045,
	}))

	require.NoError(t, auditRepo.Create(ctx, &models.AIAuditTrail{
		RequestID:        "req-alpha-1",
		UserID:           "alice",
		GatewayKeyID:     &alphaKeyID,
		ModelSlug:        "gpt-4o-mini",
		ComplianceStatus: "compliant",
		StatusCode:       200,
		MCPServersCalled: []string{"crm"},
		ToolsInvoked:     []string{"delegate_ai_task"},
		SignatureHash:    "sig-pg-1",
	}))
	require.NoError(t, auditRepo.Create(ctx, &models.AIAuditTrail{
		RequestID:        "req-deny-403",
		UserID:           "alice",
		GatewayKeyID:     &alphaKeyID,
		ModelSlug:        "crm__drop",
		ComplianceStatus: "denied",
		StatusCode:       403,
		SignatureHash:    "sig-pg-2",
	}))
	require.NoError(t, auditRepo.Create(ctx, &models.AIAuditTrail{
		RequestID:        "req-deny-429",
		UserID:           "alice",
		GatewayKeyID:     &betaKeyID,
		ModelSlug:        "gpt-4o",
		ComplianceStatus: "denied",
		StatusCode:       429,
		SignatureHash:    "sig-pg-3",
	}))

	// 5. Query full Pilot Outcomes report against real PostgreSQL
	wReport := doJSON(aliceRouter, http.MethodGet, "/api/analytics/pilot-outcomes?days=30", "")
	requireStatus(t, wReport, http.StatusOK)
	var report models.PilotOutcomesReport
	require.NoError(t, json.Unmarshal(wReport.Body.Bytes(), &report))

	assert.Equal(t, int64(3), report.Totals.TotalRequests)
	assert.Equal(t, int64(1), report.Totals.ErrorCount)
	assert.Equal(t, int64(1), report.Totals.FailoverCount)
	assert.Equal(t, int64(1), report.Totals.SmartRoutedRequests)
	assert.InDelta(t, 0.00370, report.Totals.ActualCostUSD, 0.00001)
	assert.Greater(t, report.Totals.P95LatencyMs, 0.0)
	assert.Len(t, report.Workspaces, 2)
	assert.Equal(t, int64(1), report.Governance.PolicyDenials403)
	assert.Equal(t, int64(1), report.Governance.BudgetQuotaDenials429)
	assert.Equal(t, int64(1), report.Governance.MCPToolInvocations)
	assert.Equal(t, int64(1), report.Governance.DelegatedTaskCalls)
	require.Len(t, report.RoutingBreakdown, 1)
	assert.Equal(t, "cost-saver", report.RoutingBreakdown[0].PolicyName)
	assert.Equal(t, "gpt-4o-mini", report.RoutingBreakdown[0].Model)

	// 6. Workspace-scoped filter isolates only Client Alpha
	wAlphaOnly := doJSON(aliceRouter, http.MethodGet, "/api/analytics/pilot-outcomes?days=30&workspaceId="+wsAlpha.ID, "")
	requireStatus(t, wAlphaOnly, http.StatusOK)
	var alphaReport models.PilotOutcomesReport
	require.NoError(t, json.Unmarshal(wAlphaOnly.Body.Bytes(), &alphaReport))
	assert.Equal(t, int64(2), alphaReport.Totals.TotalRequests)
	assert.Equal(t, int64(0), alphaReport.Governance.BudgetQuotaDenials429)

	// 7. Foreign org isolation: Bob cannot see Alice's agency telemetry
	wBob := doJSON(bobRouter, http.MethodGet, "/api/analytics/pilot-outcomes?days=30", "")
	requireStatus(t, wBob, http.StatusOK)
	var bobReport models.PilotOutcomesReport
	require.NoError(t, json.Unmarshal(wBob.Body.Bytes(), &bobReport))
	assert.Equal(t, int64(0), bobReport.Totals.TotalRequests)
	assert.Equal(t, int64(0), bobReport.Governance.TotalAuditEvents)

	// 8. Export CSV end-to-end
	wCSV := doJSON(aliceRouter, http.MethodGet, "/api/analytics/pilot-outcomes/export?format=csv&days=30", "")
	requireStatus(t, wCSV, http.StatusOK)
	rows, err := csv.NewReader(strings.NewReader(wCSV.Body.String())).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 5)
}
