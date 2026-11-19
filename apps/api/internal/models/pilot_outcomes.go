package models

import "time"

type PilotOutcomesTotals struct {
	TotalRequests           int64   `json:"totalRequests"`
	ErrorCount              int64   `json:"errorCount"`
	ErrorRatePct            float64 `json:"errorRatePct"`
	FailoverCount           int64   `json:"failoverCount"`
	InputTokens             int64   `json:"inputTokens"`
	OutputTokens            int64   `json:"outputTokens"`
	TotalTokens             int64   `json:"totalTokens"`
	ActualCostUSD           float64 `json:"actualCostUsd"`
	AvgLatencyMs            float64 `json:"avgLatencyMs"`
	P95LatencyMs            float64 `json:"p95LatencyMs"`
	SmartRoutedRequests     int64   `json:"smartRoutedRequests"`
	SmartRoutedCostUSD      float64 `json:"smartRoutedCostUsd"`
	BaselineFlagshipCostUSD float64 `json:"baselineFlagshipCostUsd"`
	EstimatedSavingsUSD     float64 `json:"estimatedSavingsUsd"`
}

type PilotWorkspaceOutcome struct {
	WorkspaceID             string  `json:"workspaceId"`
	WorkspaceName           string  `json:"workspaceName"`
	TotalRequests           int64   `json:"totalRequests"`
	ErrorCount              int64   `json:"errorCount"`
	ErrorRatePct            float64 `json:"errorRatePct"`
	FailoverCount           int64   `json:"failoverCount"`
	InputTokens             int64   `json:"inputTokens"`
	OutputTokens            int64   `json:"outputTokens"`
	TotalTokens             int64   `json:"totalTokens"`
	ActualCostUSD           float64 `json:"actualCostUsd"`
	AvgLatencyMs            float64 `json:"avgLatencyMs"`
	P95LatencyMs            float64 `json:"p95LatencyMs"`
	SmartRoutedRequests     int64   `json:"smartRoutedRequests"`
	BaselineFlagshipCostUSD float64 `json:"baselineFlagshipCostUsd"`
	EstimatedSavingsUSD     float64 `json:"estimatedSavingsUsd"`
}

type PilotGovernanceSummary struct {
	TotalAuditEvents    int64 `json:"totalAuditEvents"`
	CompliantEvents     int64 `json:"compliantEvents"`
	DeniedEvents        int64 `json:"deniedEvents"`
	PolicyDenials403    int64 `json:"policyDenials403"`
	BudgetQuotaDenials429 int64 `json:"budgetQuotaDenials429"`
	MCPToolInvocations  int64 `json:"mcpToolInvocations"`
	DelegatedTaskCalls  int64 `json:"delegatedTaskCalls"`
}

type PilotRoutingComparison struct {
	PolicyName              string  `json:"policyName"`
	Model                   string  `json:"model"`
	Requests                int64   `json:"requests"`
	ActualCostUSD           float64 `json:"actualCostUsd"`
	BaselineFlagshipCostUSD float64 `json:"baselineFlagshipCostUsd"`
	EstimatedSavingsUSD     float64 `json:"estimatedSavingsUsd"`
	AvgLatencyMs            float64 `json:"avgLatencyMs"`
}

type PilotOutcomesReport struct {
	OrgID            string                   `json:"orgId"`
	WorkspaceFilter  string                   `json:"workspaceFilter,omitempty"`
	WindowDays       int                      `json:"windowDays"`
	GeneratedAt      time.Time                `json:"generatedAt"`
	MethodologyNote  string                   `json:"methodologyNote"`
	Totals           PilotOutcomesTotals      `json:"totals"`
	Workspaces       []PilotWorkspaceOutcome  `json:"workspaces"`
	Governance       PilotGovernanceSummary   `json:"governance"`
	RoutingBreakdown []PilotRoutingComparison `json:"routingBreakdown"`
}
