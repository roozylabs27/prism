import { api, resolveSignal, ApiSignal } from './client';
import {
  ApiRoutingPolicy,
  ApiRoutingDecision,
  ApiRoutingSimulationReq,
  ApiRoutingSimulationRes,
  ApiRoutingReplayReq,
  ApiRoutingReplayRes,
  ApiFinOpsSummary,
} from './types/policy';

export async function apiGetPolicies(signal?: ApiSignal): Promise<ApiRoutingPolicy[]> {
  const response = await api.get<ApiRoutingPolicy[]>('/policies', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiCreatePolicy(data: Partial<ApiRoutingPolicy>): Promise<ApiRoutingPolicy> {
  const response = await api.post<ApiRoutingPolicy>('/policies', data);
  return response.data;
}

export async function apiUpdatePolicy(id: string, data: Partial<ApiRoutingPolicy>): Promise<ApiRoutingPolicy> {
  const response = await api.put<ApiRoutingPolicy>(`/policies/${id}`, data);
  return response.data;
}

export async function apiSetDefaultPolicy(id: string): Promise<ApiRoutingPolicy> {
  const response = await api.put<ApiRoutingPolicy>(`/policies/${id}/default`);
  return response.data;
}

export async function apiDeletePolicy(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/policies/${id}`);
  return response.data;
}

export async function apiGetRoutingDecisions(limit?: number, signal?: ApiSignal): Promise<ApiRoutingDecision[]> {
  const response = await api.get<ApiRoutingDecision[]>('/routing/decisions', {
    params: limit ? { limit } : undefined,
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiSimulateRouting(data: ApiRoutingSimulationReq): Promise<ApiRoutingSimulationRes> {
  const response = await api.post<ApiRoutingSimulationRes>('/routing/simulate', data);
  return response.data;
}

export async function apiReplayRouting(data: ApiRoutingReplayReq = {}): Promise<ApiRoutingReplayRes> {
  const response = await api.post<ApiRoutingReplayRes>('/routing/replay', data);
  return response.data;
}

export interface ApiRoutingRule {
  id: string;
  userId?: string;
  orgId?: string;
  modelPattern: string;
  providerId?: string | null;
  priority: number;
  enabled: boolean;
  providerName?: string;
  providerType?: string;
  createdAt: string;
  updatedAt: string;
}

export async function apiGetRoutingRules(signal?: ApiSignal): Promise<ApiRoutingRule[]> {
  const response = await api.get<ApiRoutingRule[]>('/routing-rules', {
    signal: resolveSignal(signal),
  });
  return Array.isArray(response.data) ? response.data : [];
}

export async function apiCreateRoutingRule(data: {
  modelPattern: string;
  providerId?: string;
  priority?: number;
  enabled?: boolean;
}): Promise<ApiRoutingRule> {
  const response = await api.post<ApiRoutingRule>('/routing-rules', data);
  return response.data;
}

export async function apiDeleteRoutingRule(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/routing-rules/${id}`);
  return response.data;
}

export async function apiGetFinOpsSummary(): Promise<ApiFinOpsSummary> {
  const response = await api.get<ApiFinOpsSummary | { data: ApiFinOpsSummary }>('/analytics/finops');
  if (response.data && typeof response.data === 'object' && 'data' in response.data) {
    return response.data.data;
  }
  return response.data as ApiFinOpsSummary;
}
