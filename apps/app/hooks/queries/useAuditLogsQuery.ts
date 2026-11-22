import { useQuery } from '@tanstack/react-query';
import { apiGetAuditLogs } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';

export function useAuditLogsQuery(params?: { page?: number; limit?: number }) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['audit-logs', ...scope, params],
    queryFn: () => apiGetAuditLogs(params),
  });
}
