export interface ApiRoutingPolicy {
  id: string;
  name: string;
  description?: string;
  strategy: 'lowest_latency' | 'lowest_cost' | 'highest_throughput' | 'round_robin' | 'custom_weighted' | 'semantic';
  weightLatency?: number;
  weightCost?: number;
  weightThroughput?: number;
  weightErrorRate?: number;
  weights?: {
    task_match?: number;
    quality?: number;
    cost?: number;
    speed?: number;
    latency?: number;
    throughput?: number;
    errorRate?: number;
  };
  constraints?: Record<string, unknown>;
  isDefault: boolean;
  enabled: boolean;
  fallbackModelId?: string;
  cooldownPeriodSec?: number;
  targetModel?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ApiRoutingDecision {
  id: string;
  requestId: string;
  selectedModel: string;
  selectedProvider: string;
  selectedCredentialId?: string;
  policyId?: string;
  policyName?: string;
  strategy: string;
  statusCode?: number;
  latencyMs: number;
  ttftMs?: number;
  retryCount?: number;
  estimatedCost?: number;
  actualCost?: number;
  costUSD: number;
  candidatesCount: number;
  downgradeReason?: string;
  scoresBreakdown?: Record<string, unknown>;
  errorMessage?: string;
  createdAt: string;
}

export interface ApiRoutingSimulationReq {
  prompt: string;
  targetModel?: string;
  policyId?: string;
  budgetStatus?: string;
  customWeights?: Record<string, number>;
  weightLatency?: number;
  weightCost?: number;
  weightThroughput?: number;
  weightErrorRate?: number;
}

export interface ApiModelScoreDetail {
  modelId: string;
  modelSlug?: string;
  slug?: string;
  displayName?: string;
  providerName: string;
  totalScore: number;
  score?: number;
  latencyScore?: number;
  costScore?: number;
  throughputScore?: number;
  healthScore?: number;
  speedScore?: number;
  inputPrice1M?: number;
  estimatedLatencyMs?: number;
  estimatedCostUSD?: number;
  isHealthy?: boolean;
}

export interface ApiRoutingSimulationRes {
  selectedModel: string;
  selectedProvider: string;
  policyName?: string;
  strategy: string;
  taskType?: string;
  complexity?: string;
  confidenceScore: number;
  simulatedLatencyMs: number;
  simulatedCostUSD: number;
  candidates: ApiModelScoreDetail[];
  reasoning: string;
}

export interface ApiRoutingReplayReq {
  policyId?: string;
  customWeights?: Record<string, number>;
  budgetStatus?: string;
  limit?: number;
}

export interface ApiRoutingReplayItem {
  requestId: string;
  promptPreview?: string;
  taskType?: string;
  complexity?: string;
  originalModel: string;
  originalProvider: string;
  originalCost: number;
  originalStatusCode?: number;
  originalLatencyMs?: number;
  originalError?: string;
  simulatedModel: string;
  simulatedProvider: string;
  simulatedCost: number;
  changed: boolean;
  costDelta: number;
}

export interface ApiRoutingReplayRes {
  policyName: string;
  evaluatedCount: number;
  changedCount: number;
  changeRate: number;
  historicalTotalCost: number;
  simulatedTotalCost: number;
  estimatedCostDelta: number;
  items: ApiRoutingReplayItem[];
}

export interface ApiCostRecommendation {
  type: string;
  title: string;
  description: string;
  potentialSavingsUSD: number;
  targetModel?: string;
  suggestedAlternative?: string;
}

export interface ApiFinOpsSummary {
  periodDays: number;
  totalSpendUSD: number;
  projectedMonthlySpendUSD: number;
  topSpenderClientApp?: string;
  topSpenderModel?: string;
  costSavingsUSD: number;
  recommendations: ApiCostRecommendation[];
}
