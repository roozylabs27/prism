package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

const (
	// PilotBaselineInputPricePerToken reflects $3.00 / 1M input tokens (consistent with GetLogAnalytics).
	PilotBaselineInputPricePerToken = 0.000003
	// PilotBaselineOutputPricePerToken reflects $15.00 / 1M output tokens (consistent with GetLogAnalytics).
	PilotBaselineOutputPricePerToken = 0.000015
)

type PilotOutcomesRepository struct {
	db *sql.DB
}

func NewPilotOutcomesRepository(db *sql.DB) *PilotOutcomesRepository {
	return &PilotOutcomesRepository{db: db}
}

func (r *PilotOutcomesRepository) GetPilotOutcomes(ctx context.Context, orgID, userID, workspaceID string, days int) (*models.PilotOutcomesReport, error) {
	if orgID == "" || userID == "" {
		return nil, errors.New("organization and user context required")
	}
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}

	report := &models.PilotOutcomesReport{
		OrgID:           orgID,
		WorkspaceFilter: workspaceID,
		WindowDays:      days,
		GeneratedAt:     time.Now().UTC(),
		MethodologyNote: "actualCostUsd uses Prism logged pricing; baselineFlagshipCostUsd uses a $3/1M input & $15/1M output flagship reference benchmark. estimatedSavingsUsd is modeled delta (baseline minus actual) and requires operator quality sign-off before claiming net savings.",
		Workspaces:      []models.PilotWorkspaceOutcome{},
		RoutingBreakdown: []models.PilotRoutingComparison{},
	}

	// 1. Overall totals & exact p95 latency across matching request_logs
	logArgs := []interface{}{orgID, days}
	wsClause := ""
	if workspaceID != "" {
		wsClause = " AND COALESCE(NULLIF(rl.workspace_id, ''), gak.workspace_id) = $3"
		logArgs = append(logArgs, workspaceID)
	}

	totalsQuery := fmt.Sprintf(`
		SELECT COALESCE(COUNT(*), 0),
		       COALESCE(COUNT(*) FILTER (WHERE rl.status_code >= 400), 0),
		       COALESCE(COUNT(*) FILTER (WHERE rl.retry_count > 0), 0),
		       COALESCE(SUM(rl.input_tokens), 0),
		       COALESCE(SUM(rl.output_tokens), 0),
		       COALESCE(SUM(rl.total_tokens), 0),
		       COALESCE(SUM(rl.cost_usd), 0),
		       COALESCE(AVG(rl.latency_ms), 0),
		       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY rl.latency_ms), 0),
		       COALESCE(COUNT(*) FILTER (WHERE rd.id IS NOT NULL), 0),
		       COALESCE(SUM(CASE WHEN rd.id IS NOT NULL THEN rl.cost_usd ELSE 0 END), 0),
		       COALESCE(SUM(rl.input_tokens * %f + rl.output_tokens * %f), 0)
		FROM request_logs rl
		LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		LEFT JOIN LATERAL (
			SELECT id FROM routing_decisions d WHERE d.request_id = rl.request_id LIMIT 1
		) rd ON true
		WHERE COALESCE(NULLIF(rl.org_id, ''), gak.org_id) = $1
		  AND rl.created_at >= NOW() - make_interval(days => $2)%s`,
		PilotBaselineInputPricePerToken, PilotBaselineOutputPricePerToken, wsClause,
	)

	var t models.PilotOutcomesTotals
	err := r.db.QueryRowContext(ctx, totalsQuery, logArgs...).Scan(
		&t.TotalRequests,
		&t.ErrorCount,
		&t.FailoverCount,
		&t.InputTokens,
		&t.OutputTokens,
		&t.TotalTokens,
		&t.ActualCostUSD,
		&t.AvgLatencyMs,
		&t.P95LatencyMs,
		&t.SmartRoutedRequests,
		&t.SmartRoutedCostUSD,
		&t.BaselineFlagshipCostUSD,
	)
	if err != nil {
		return nil, err
	}
	if t.TotalRequests > 0 {
		t.ErrorRatePct = (float64(t.ErrorCount) / float64(t.TotalRequests)) * 100.0
	}
	t.EstimatedSavingsUSD = t.BaselineFlagshipCostUSD - t.ActualCostUSD
	report.Totals = t

	// 2. Per-workspace attribution breakdown
	wsQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(rl.workspace_id, ''), NULLIF(gak.workspace_id, ''), 'unassigned') AS ws_id,
		       COALESCE(w.name, CASE WHEN COALESCE(NULLIF(rl.workspace_id, ''), gak.workspace_id, '') = '' THEN 'Unassigned / Org-Wide Key' ELSE COALESCE(NULLIF(rl.workspace_id, ''), gak.workspace_id) END) AS ws_name,
		       COUNT(*),
		       COUNT(*) FILTER (WHERE rl.status_code >= 400),
		       COUNT(*) FILTER (WHERE rl.retry_count > 0),
		       COALESCE(SUM(rl.input_tokens), 0),
		       COALESCE(SUM(rl.output_tokens), 0),
		       COALESCE(SUM(rl.total_tokens), 0),
		       COALESCE(SUM(rl.cost_usd), 0),
		       COALESCE(AVG(rl.latency_ms), 0),
		       COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY rl.latency_ms), 0),
		       COUNT(*) FILTER (WHERE rd.id IS NOT NULL),
		       COALESCE(SUM(rl.input_tokens * %f + rl.output_tokens * %f), 0)
		FROM request_logs rl
		LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		LEFT JOIN workspaces w ON w.id = NULLIF(COALESCE(NULLIF(rl.workspace_id, ''), gak.workspace_id), '') AND w.org_id = $1
		LEFT JOIN LATERAL (
			SELECT id FROM routing_decisions d WHERE d.request_id = rl.request_id LIMIT 1
		) rd ON true
		WHERE COALESCE(NULLIF(rl.org_id, ''), gak.org_id) = $1
		  AND rl.created_at >= NOW() - make_interval(days => $2)%s
		GROUP BY 1, 2
		ORDER BY 9 DESC, 3 DESC`,
		PilotBaselineInputPricePerToken, PilotBaselineOutputPricePerToken, wsClause,
	)

	wsRows, err := r.db.QueryContext(ctx, wsQuery, logArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = wsRows.Close() }()

	for wsRows.Next() {
		var w models.PilotWorkspaceOutcome
		if err := wsRows.Scan(
			&w.WorkspaceID,
			&w.WorkspaceName,
			&w.TotalRequests,
			&w.ErrorCount,
			&w.FailoverCount,
			&w.InputTokens,
			&w.OutputTokens,
			&w.TotalTokens,
			&w.ActualCostUSD,
			&w.AvgLatencyMs,
			&w.P95LatencyMs,
			&w.SmartRoutedRequests,
			&w.BaselineFlagshipCostUSD,
		); err != nil {
			return nil, err
		}
		if w.TotalRequests > 0 {
			w.ErrorRatePct = (float64(w.ErrorCount) / float64(w.TotalRequests)) * 100.0
		}
		w.EstimatedSavingsUSD = w.BaselineFlagshipCostUSD - w.ActualCostUSD
		report.Workspaces = append(report.Workspaces, w)
	}
	if err := wsRows.Err(); err != nil {
		return nil, err
	}

	// 3. Governance & MCP enforcement summary (hard caps + policy denials caught)
	auditArgs := []interface{}{orgID, days}
	auditWSClause := ""
	if workspaceID != "" {
		auditWSClause = " AND gak.workspace_id = $3"
		auditArgs = append(auditArgs, workspaceID)
	}

	auditQuery := `
		SELECT COALESCE(COUNT(*), 0),
		       COALESCE(COUNT(*) FILTER (WHERE a.compliance_status = 'compliant'), 0),
		       COALESCE(COUNT(*) FILTER (WHERE a.compliance_status = 'denied'), 0),
		       COALESCE(COUNT(*) FILTER (WHERE a.compliance_status = 'denied' AND a.status_code = 403), 0),
		       COALESCE(COUNT(*) FILTER (WHERE a.compliance_status = 'denied' AND a.status_code = 429), 0),
		       COALESCE(COUNT(*) FILTER (WHERE COALESCE(array_length(a.mcp_servers_called, 1), 0) > 0), 0),
		       COALESCE(COUNT(*) FILTER (WHERE 'delegate_ai_task' = ANY(a.tools_invoked)), 0)
		FROM ai_audit_trails a
		LEFT JOIN gateway_api_keys gak ON a.gateway_key_id = gak.id::text
		WHERE COALESCE(NULLIF(a.org_id, ''), gak.org_id) = $1
		  AND a.created_at >= NOW() - make_interval(days => $2)` + auditWSClause

	var gov models.PilotGovernanceSummary
	if err := r.db.QueryRowContext(ctx, auditQuery, auditArgs...).Scan(
		&gov.TotalAuditEvents,
		&gov.CompliantEvents,
		&gov.DeniedEvents,
		&gov.PolicyDenials403,
		&gov.BudgetQuotaDenials429,
		&gov.MCPToolInvocations,
		&gov.DelegatedTaskCalls,
	); err != nil {
		return nil, err
	}
	report.Governance = gov

	// 4. Smart routing policy & selected model breakdown
	routeArgs := []interface{}{orgID, days}
	routeWSClause := ""
	if workspaceID != "" {
		routeWSClause = " AND COALESCE(NULLIF(rl.workspace_id, ''), gak.workspace_id) = $3"
		routeArgs = append(routeArgs, workspaceID)
	}

	routeQuery := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(rd.policy_name, ''), 'default') AS policy_name,
		       COALESCE(NULLIF(rd.selected_model, ''), 'unknown') AS selected_model,
		       COUNT(*),
		       COALESCE(SUM(CASE WHEN rl.cost_usd IS NOT NULL AND rl.cost_usd > 0 THEN rl.cost_usd ELSE rd.actual_cost END), 0),
		       COALESCE(SUM(COALESCE(rl.input_tokens, 0) * %f + COALESCE(rl.output_tokens, 0) * %f), 0),
		       COALESCE(AVG(COALESCE(rl.latency_ms, 0)), 0)
		FROM routing_decisions rd
		LEFT JOIN request_logs rl ON rl.request_id = rd.request_id
		LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		WHERE COALESCE(NULLIF(rl.org_id, ''), gak.org_id) = $1
		  AND rd.created_at >= NOW() - make_interval(days => $2)%s
		GROUP BY 1, 2
		ORDER BY 3 DESC, 4 DESC`,
		PilotBaselineInputPricePerToken, PilotBaselineOutputPricePerToken, routeWSClause,
	)

	rRows, err := r.db.QueryContext(ctx, routeQuery, routeArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rRows.Close() }()

	for rRows.Next() {
		var rc models.PilotRoutingComparison
		if err := rRows.Scan(
			&rc.PolicyName,
			&rc.Model,
			&rc.Requests,
			&rc.ActualCostUSD,
			&rc.BaselineFlagshipCostUSD,
			&rc.AvgLatencyMs,
		); err != nil {
			return nil, err
		}
		rc.EstimatedSavingsUSD = rc.BaselineFlagshipCostUSD - rc.ActualCostUSD
		report.RoutingBreakdown = append(report.RoutingBreakdown, rc)
	}
	if err := rRows.Err(); err != nil {
		return nil, err
	}

	return report, nil
}
