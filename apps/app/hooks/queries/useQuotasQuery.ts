import { useQuery } from '@tanstack/react-query';
import { apiGetQuotas } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useUpdateQuotaMutation as useUpdateQuota } from '@/hooks/mutations/useQuotaMutations';

export function useQuotasQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['quotas', ...scope],
    queryFn: () => apiGetQuotas(),
  });
}
