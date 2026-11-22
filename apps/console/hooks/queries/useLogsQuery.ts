import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetLogs, apiGetLogAnalytics } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';

export function useLogsQuery(params?: {
  page?: number;
  limit?: number;
  provider?: string;
  model?: string;
  status?: number;
  search?: string;
  agentId?: string;
}) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['request-logs', ...scope, params],
    queryFn: ({ signal }) => apiGetLogs(params, signal),
    placeholderData: keepPreviousData,
  });
}

export function useLogAnalyticsQuery(days: number = 30) {
  const scope = useTenantScopeKey();
  const boundedDays = days === 0 ? 0 : days > 0 && days <= 365 ? days : 30;
  return useQuery({
    queryKey: ['log-analytics', ...scope, boundedDays],
    queryFn: ({ signal }) => apiGetLogAnalytics({ days: boundedDays }, signal),
  });
}
