import { useQuery } from '@tanstack/react-query';
import { apiGetAuditTrails } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useVerifyAuditIntegrity } from '@/hooks/mutations/useAuditLogMutations';

export function useAuditTrailsQuery(params?: { limit?: number; page?: number }) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['audit-trails', ...scope, params],
    queryFn: ({ signal }) => apiGetAuditTrails(params, signal),
  });
}
