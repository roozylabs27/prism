import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetLogs, apiGetLogAnalytics } from '@/lib/api';

export function useLogsQuery(params?: {
  page?: number;
  limit?: number;
  provider?: string;
  model?: string;
  status?: number;
  search?: string;
  agentId?: string;
}) {
  return useQuery({
    queryKey: ['request-logs', params],
    queryFn: () => apiGetLogs(params),
    placeholderData: keepPreviousData,
  });
}

export function useLogAnalyticsQuery(days: number = 30) {
  const boundedDays = days > 0 && days <= 90 ? days : 30;
  return useQuery({
    queryKey: ['log-analytics', boundedDays],
    queryFn: () => apiGetLogAnalytics({ days: boundedDays }),
  });
}
