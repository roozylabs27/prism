import { api } from './client';
import {
  ApiDashboardStats,
  ApiUsagePoint,
  ApiProviderHealth,
  ApiActiveStreams,
  ApiPilotOutcomesReport,
} from './types/dashboard';

export async function apiGetDashboardStats(params?: { days?: number; startDate?: string; endDate?: string }): Promise<ApiDashboardStats> {
  const response = await api.get<ApiDashboardStats>('/dashboard/stats', { params });
  return response.data;
}

export async function apiGetDashboardUsage(params?: { days?: number; startDate?: string; endDate?: string }): Promise<ApiUsagePoint[]> {
  const response = await api.get<ApiUsagePoint[]>('/dashboard/usage', { params });
  return response.data;
}

export async function apiGetDashboardHealth(): Promise<ApiProviderHealth[]> {
  const response = await api.get<ApiProviderHealth[]>('/dashboard/health');
  return response.data;
}

export async function apiGetActiveStreams(): Promise<ApiActiveStreams> {
  const response = await api.get<ApiActiveStreams>('/dashboard/active-streams');
  return response.data;
}

export async function apiGetPilotOutcomes(days = 30, workspaceId?: string): Promise<ApiPilotOutcomesReport> {
  const response = await api.get<ApiPilotOutcomesReport>('/analytics/pilot-outcomes', {
    params: { days, ...(workspaceId ? { workspaceId } : {}) },
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
