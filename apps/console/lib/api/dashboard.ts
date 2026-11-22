import { api } from './client';
import {
  ApiDashboardStats,
  ApiUsagePoint,
  ApiProviderHealth,
  ApiActiveStreams,
  ApiPilotOutcomesReport,
} from './types/dashboard';

export async function apiGetDashboardStats(
  params?: { days?: number; startDate?: string; endDate?: string },
  signal?: AbortSignal,
): Promise<ApiDashboardStats> {
  const response = await api.get<ApiDashboardStats>('/dashboard/stats', { params, signal });
  return response.data;
}

export async function apiGetDashboardUsage(
  params?: { days?: number; startDate?: string; endDate?: string },
  signal?: AbortSignal,
): Promise<ApiUsagePoint[]> {
  const response = await api.get<ApiUsagePoint[]>('/dashboard/usage', { params, signal });
  return response.data;
}

export async function apiGetDashboardHealth(signal?: AbortSignal): Promise<ApiProviderHealth[]> {
  const response = await api.get<ApiProviderHealth[]>('/dashboard/health', { signal });
  return response.data;
}

export async function apiGetActiveStreams(signal?: AbortSignal): Promise<ApiActiveStreams> {
  const response = await api.get<ApiActiveStreams>('/dashboard/active-streams', { signal });
  return response.data;
}

export async function apiGetPilotOutcomes(
  days = 30,
  workspaceId?: string,
  signal?: AbortSignal,
): Promise<ApiPilotOutcomesReport> {
  const response = await api.get<ApiPilotOutcomesReport>('/analytics/pilot-outcomes', {
    params: { days, ...(workspaceId ? { workspaceId } : {}) },
    signal,
  });
  return response.data;
}

export async function apiExportPilotOutcomes(format: 'csv' | 'json' = 'csv', days = 30, workspaceId?: string): Promise<Blob> {
  const response = await api.get('/analytics/pilot-outcomes/export', {
    params: { format, days, ...(workspaceId ? { workspaceId } : {}) },
    responseType: 'blob',
  });
  return response.data;
}
