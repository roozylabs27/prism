import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetCredentials, PageParams } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useDeleteCredentialMutation as useDeleteCredential } from '@/hooks/mutations/useCredentialMutations';

export function useCredentialsQuery(providerId: string = 'openai', params?: PageParams) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['credentials', ...scope, providerId, params],
    queryFn: ({ signal }) => apiGetCredentials(providerId, params, signal),
    placeholderData: keepPreviousData,
  });
}
