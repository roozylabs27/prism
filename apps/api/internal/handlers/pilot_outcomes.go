package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type pilotOutcomesStore interface {
	GetPilotOutcomes(ctx context.Context, orgID, userID, workspaceID string, days int) (*models.PilotOutcomesReport, error)
}

type PilotOutcomesHandler struct {
	repo pilotOutcomesStore
}

func NewPilotOutcomesHandler(repo *repository.PilotOutcomesRepository) *PilotOutcomesHandler {
	var store pilotOutcomesStore
	if repo != nil {
		store = repo
	}
	return &PilotOutcomesHandler{repo: store}
}

func parsePilotDays(c *gin.Context) (int, bool) {
	days := 30
	if raw := strings.TrimSpace(c.Query("days")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			httputil.RespondBadRequest(c, "days must be a positive integer", err, "INVALID_DAYS")
			return 0, false
		}
		if n > 90 {
			n = 90
		}
		days = n
	}
	return days, true
}

func (h *PilotOutcomesHandler) GetSummary(c *gin.Context) {
	orgID := tenantOrgID(c)
	userID := c.GetString("userId")
	if orgID == "" || userID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Pilot outcomes repository unavailable", nil, "PILOT_OUTCOMES_UNAVAILABLE")
		return
	}
	days, ok := parsePilotDays(c)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Query("workspaceId"))

	report, err := h.repo.GetPilotOutcomes(c.Request.Context(), orgID, userID, workspaceID, days)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to compute pilot outcomes", err, "PILOT_OUTCOMES_FAILED")
		return
	}
	c.JSON(http.StatusOK, report)
}

func (h *PilotOutcomesHandler) Export(c *gin.Context) {
	orgID := tenantOrgID(c)
	userID := c.GetString("userId")
	if orgID == "" || userID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}
	if h.repo == nil {
		httputil.RespondInternalError(c, "Pilot outcomes repository unavailable", nil, "PILOT_OUTCOMES_UNAVAILABLE")
		return
	}
	days, ok := parsePilotDays(c)
	if !ok {
		return
	}
	workspaceID := strings.TrimSpace(c.Query("workspaceId"))
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	if format != "csv" && format != "json" {
		httputil.RespondBadRequest(c, "format must be csv or json", nil, "INVALID_FORMAT")
		return
	}

	report, err := h.repo.GetPilotOutcomes(c.Request.Context(), orgID, userID, workspaceID, days)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to export pilot outcomes", err, "PILOT_OUTCOMES_EXPORT_FAILED")
		return
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	if format == "json" {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"prism_pilot_outcomes_%s.json\"", stamp))
		c.Status(http.StatusOK)
		enc := json.NewEncoder(c.Writer)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"prism_pilot_outcomes_%s.csv\"", stamp))
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"Section", "KeyOrWorkspaceID", "NameOrModel", "RequestsOrEvents", "ErrorsOrDenied", "ErrorRateOr403Count", "FailoversOr429Count", "TotalTokensOrMCPCalls", "ActualCostUSDOrDelegated", "BaselineFlagshipCostUSD", "EstimatedSavingsUSD", "AvgLatencyMs", "P95LatencyMs"})

	t := report.Totals
	_ = w.Write([]string{
		"totals", csvCell(report.OrgID), "ALL_WORKSPACES",
		strconv.FormatInt(t.TotalRequests, 10),
		strconv.FormatInt(t.ErrorCount, 10),
		fmt.Sprintf("%.2f", t.ErrorRatePct),
		strconv.FormatInt(t.FailoverCount, 10),
		strconv.FormatInt(t.TotalTokens, 10),
		fmt.Sprintf("%.6f", t.ActualCostUSD),
		fmt.Sprintf("%.6f", t.BaselineFlagshipCostUSD),
		fmt.Sprintf("%.6f", t.EstimatedSavingsUSD),
		fmt.Sprintf("%.2f", t.AvgLatencyMs),
		fmt.Sprintf("%.2f", t.P95LatencyMs),
	})

	for _, ws := range report.Workspaces {
		_ = w.Write([]string{
			"workspace", csvCell(ws.WorkspaceID), csvCell(ws.WorkspaceName),
			strconv.FormatInt(ws.TotalRequests, 10),
			strconv.FormatInt(ws.ErrorCount, 10),
			fmt.Sprintf("%.2f", ws.ErrorRatePct),
			strconv.FormatInt(ws.FailoverCount, 10),
			strconv.FormatInt(ws.TotalTokens, 10),
			fmt.Sprintf("%.6f", ws.ActualCostUSD),
			fmt.Sprintf("%.6f", ws.BaselineFlagshipCostUSD),
			fmt.Sprintf("%.6f", ws.EstimatedSavingsUSD),
			fmt.Sprintf("%.2f", ws.AvgLatencyMs),
			fmt.Sprintf("%.2f", ws.P95LatencyMs),
		})
	}

	g := report.Governance
	_ = w.Write([]string{
		"governance", csvCell(report.OrgID), "AUDIT_AND_ENFORCEMENT",
		strconv.FormatInt(g.TotalAuditEvents, 10),
		strconv.FormatInt(g.DeniedEvents, 10),
		strconv.FormatInt(g.PolicyDenials403, 10),
		strconv.FormatInt(g.BudgetQuotaDenials429, 10),
		strconv.FormatInt(g.MCPToolInvocations, 10),
		strconv.FormatInt(g.DelegatedTaskCalls, 10),
		"", "", "", "",
	})

	for _, rc := range report.RoutingBreakdown {
		_ = w.Write([]string{
			"routing", csvCell(rc.PolicyName), csvCell(rc.Model),
			strconv.FormatInt(rc.Requests, 10),
			"", "", "", "",
			fmt.Sprintf("%.6f", rc.ActualCostUSD),
			fmt.Sprintf("%.6f", rc.BaselineFlagshipCostUSD),
			fmt.Sprintf("%.6f", rc.EstimatedSavingsUSD),
			fmt.Sprintf("%.2f", rc.AvgLatencyMs),
			"",
		})
	}
	w.Flush()
}
