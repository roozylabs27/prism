import { api } from './client';
import { ApiRequestLog, ApiLogAnalytics } from './types/log';
import { PaginatedResult } from './types/common';

export async function apiGetLogs(
  params?: {
    page?: number;
    limit?: number;
    provider?: string;
    model?: string;
    status?: number;
    search?: string;
    agentId?: string;
  },
  signal?: AbortSignal,
): Promise<PaginatedResult<ApiRequestLog>> {
  const response = await api.get<PaginatedResult<ApiRequestLog>>('/logs', { params, signal });
  return response.data;
}

export async function apiGetLogAnalytics(
  params?: { days?: number; window?: string; limit?: number },
  signal?: AbortSignal,
): Promise<ApiLogAnalytics> {
  const response = await api.get<ApiLogAnalytics | { data: ApiLogAnalytics }>('/analytics/logs', { params, signal });
  if (response.data && typeof response.data === 'object' && 'data' in response.data) {
    return response.data.data;
  }
  return response.data as ApiLogAnalytics;
}
