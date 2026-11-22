import { useQuery } from '@tanstack/react-query';
import {
  apiGetDashboardStats,
  apiGetDashboardUsage,
  apiGetDashboardHealth,
  apiGetPilotOutcomes,
} from '@/lib/api';
import { usePermission } from '@/components/PermissionProvider';
import { useTenantScopeKey } from './useTenantScopeKey';

export function dateRangeToParams(dateRange?: string): { days?: number } | undefined {
  if (!dateRange || dateRange === '30d') return { days: 30 };
  if (dateRange === '24h') return { days: 1 };
  if (dateRange === '7d') return { days: 7 };
  if (dateRange === '90d') return { days: 90 };
  if (dateRange === 'all') return { days: 0 };
  return { days: 30 };
}

export function useDashboardStatsQuery(dateRange?: string) {
  const scope = useTenantScopeKey();
  const params = dateRangeToParams(dateRange);
  return useQuery({
    queryKey: ['dashboard-stats', ...scope, dateRange || '30d'],
    queryFn: ({ signal }) => apiGetDashboardStats(params, signal),
  });
}

export function useUsageChartQuery(dateRange?: string) {
  const scope = useTenantScopeKey();
  const params = dateRangeToParams(dateRange);
  return useQuery({
    queryKey: ['dashboard-usage-chart', ...scope, dateRange || '30d'],
    queryFn: ({ signal }) => apiGetDashboardUsage(params, signal),
  });
}

export function useDashboardHealthQuery() {
  const scope = useTenantScopeKey();
  const { hasPermission, isLoading: isPermsLoading } = usePermission();
  const canReadModels = !isPermsLoading && hasPermission('model:read');
  return useQuery({
    queryKey: ['dashboard-health', ...scope],
    queryFn: ({ signal }) => apiGetDashboardHealth(signal),
    enabled: canReadModels,
  });
}

export function usePilotOutcomesQuery(days = 30, workspaceId?: string) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['pilot-outcomes', ...scope, days, workspaceId || 'all'],
    queryFn: ({ signal }) => apiGetPilotOutcomes(days, workspaceId || undefined, signal),
  });
}
