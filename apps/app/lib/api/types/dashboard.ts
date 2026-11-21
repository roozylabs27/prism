export interface ApiDashboardStats {
  totalRequests: number;
  totalTokens: number;
  inputTokens?: number;
  outputTokens?: number;
  totalEstimatedCost?: number;
  avgLatency: number;
  errorRate: number;
  activeProviders: number;
  activeCredentials: number;
  activeKeys: number;
}

export interface ApiUsagePoint {
  date: string;
  model?: string;
  requests: number;
  tokens: number;
  estimatedCost?: number;
}

export interface ApiProviderHealth {
  id: string;
  name: string;
  type: string;
  status: 'healthy' | 'degraded' | 'down';
  credCount: number;
}

export interface ApiActiveStreams {
  totalActive: number;
  byModel: Record<string, number>;
  byCredential?: Record<string, number>;
}

export interface ApiPilotOutcomesReport {
  orgId: string;
  workspaceFilter?: string;
  windowDays: number;
  generatedAt: string;
  methodologyNote: string;
  totals: {
    totalRequests: number;
    errorCount: number;
    errorRatePct: number;
    failoverCount: number;
    totalTokens: number;
    actualCostUsd: number;
    avgLatencyMs: number;
    p95LatencyMs: number;
    smartRoutedRequests: number;
    baselineFlagshipCostUsd: number;
    estimatedSavingsUsd: number;
  };
  workspaces: Array<{
    workspaceId: string;
    workspaceName: string;
    totalRequests: number;
    actualCostUsd: number;
    estimatedSavingsUsd: number;
    avgLatencyMs: number;
  }>;
  governance: {
    totalAuditEvents: number;
    compliantEvents: number;
    deniedEvents: number;
    policyDenials403: number;
    budgetQuotaDenials429: number;
    mcpToolInvocations: number;
  };
}
