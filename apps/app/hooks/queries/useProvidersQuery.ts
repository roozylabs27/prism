import { useQuery } from '@tanstack/react-query';
import { apiGetProviders } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useCreateProviderMutation as useCreateProvider, useUpdateProviderMutation as useUpdateProvider, useDeleteProviderMutation as useDeleteProvider } from '@/hooks/mutations/useProviderMutations';

export function useProvidersQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['providers', ...scope],
    queryFn: apiGetProviders,
  });
}
