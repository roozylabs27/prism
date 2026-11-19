package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakePilotOutcomesStore struct {
	report   *models.PilotOutcomesReport
	err      error
	gotOrg   string
	gotUser  string
	gotWS    string
	gotDays  int
}

func (f *fakePilotOutcomesStore) GetPilotOutcomes(_ context.Context, orgID, userID, workspaceID string, days int) (*models.PilotOutcomesReport, error) {
	f.gotOrg = orgID
	f.gotUser = userID
	f.gotWS = workspaceID
	f.gotDays = days
	if f.err != nil {
		return nil, f.err
	}
	return f.report, nil
}

func TestPilotOutcomesHandler_SummaryAndSafeExport(t *testing.T) {
	sample := &models.PilotOutcomesReport{
		OrgID:           "org_agency",
		WindowDays:      14,
		GeneratedAt:     time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC),
		MethodologyNote: "modeled delta",
		Totals: models.PilotOutcomesTotals{
			TotalRequests:           120,
			ErrorCount:              3,
			ErrorRatePct:            2.5,
			FailoverCount:           2,
			TotalTokens:             48000,
			ActualCostUSD:           0.42,
			AvgLatencyMs:            210,
			P95LatencyMs:            540,
			SmartRoutedRequests:     80,
			BaselineFlagshipCostUSD: 1.90,
			EstimatedSavingsUSD:     1.48,
		},
		Workspaces: []models.PilotWorkspaceOutcome{
			{
				WorkspaceID:             "ws_alpha",
				WorkspaceName:           "=CMD('calc')", // CSV formula injection payload
				TotalRequests:           120,
				TotalTokens:             48000,
				ActualCostUSD:           0.42,
				BaselineFlagshipCostUSD: 1.90,
				EstimatedSavingsUSD:     1.48,
			},
		},
		Governance: models.PilotGovernanceSummary{
			TotalAuditEvents:      135,
			CompliantEvents:       128,
			DeniedEvents:          7,
			PolicyDenials403:      4,
			BudgetQuotaDenials429: 3,
			MCPToolInvocations:    15,
			DelegatedTaskCalls:    9,
		},
		RoutingBreakdown: []models.PilotRoutingComparison{
			{
				PolicyName:              "cost-saver",
				Model:                   "gpt-4o-mini",
				Requests:                80,
				ActualCostUSD:           0.12,
				BaselineFlagshipCostUSD: 1.40,
				EstimatedSavingsUSD:     1.28,
				AvgLatencyMs:            180,
			},
		},
	}

	store := &fakePilotOutcomesStore{report: sample}
	h := &PilotOutcomesHandler{repo: store}

	register := func(r *gin.Engine) {
		r.GET("/api/analytics/pilot-outcomes", h.GetSummary)
		r.GET("/api/analytics/pilot-outcomes/export", h.Export)
	}

	tenant := &models.TenantContext{OrgID: "org_agency", Role: "owner"}
	r := asUser("alice", tenant, register)

	// 1. Summary JSON with query params forwards filters properly
	wJSON := doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes?days=14&workspaceId=ws_alpha", "")
	requireStatus(t, wJSON, http.StatusOK)
	assert.Equal(t, "org_agency", store.gotOrg)
	assert.Equal(t, "alice", store.gotUser)
	assert.Equal(t, "ws_alpha", store.gotWS)
	assert.Equal(t, 14, store.gotDays)

	var gotReport models.PilotOutcomesReport
	require.NoError(t, json.Unmarshal(wJSON.Body.Bytes(), &gotReport))
	assert.Equal(t, int64(3), gotReport.Governance.BudgetQuotaDenials429)
	assert.Equal(t, int64(4), gotReport.Governance.PolicyDenials403)
	assert.InDelta(t, 1.48, gotReport.Totals.EstimatedSavingsUSD, 0.0001)

	// 2. CSV export neutralizes leading '=' spreadsheet formula
	wCSV := doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes/export?format=csv", "")
	requireStatus(t, wCSV, http.StatusOK)
	assert.Contains(t, wCSV.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, wCSV.Header().Get("Content-Disposition"), "prism_pilot_outcomes_")

	records, err := csv.NewReader(strings.NewReader(wCSV.Body.String())).ReadAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(records), 5)
	assert.Equal(t, "workspace", records[2][0])
	assert.Equal(t, "'=CMD('calc')", records[2][2])

	// 3. JSON download export sets Content-Disposition
	wDL := doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes/export?format=json", "")
	requireStatus(t, wDL, http.StatusOK)
	assert.Contains(t, wDL.Header().Get("Content-Disposition"), ".json")

	// 4. Invalid days or format rejected 400
	requireStatus(t, doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes?days=abc", ""), http.StatusBadRequest)
	requireStatus(t, doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes/export?format=xml", ""), http.StatusBadRequest)

	// 5. Store error returns 500
	store.err = errors.New("db timeout")
	requireStatus(t, doJSON(r, http.MethodGet, "/api/analytics/pilot-outcomes", ""), http.StatusInternalServerError)

	// 6. Missing tenant context fails closed (403)
	rNoOrg := asUser("alice", nil, register)
	requireStatus(t, doJSON(rNoOrg, http.MethodGet, "/api/analytics/pilot-outcomes", ""), http.StatusForbidden)
}
