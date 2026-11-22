import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetAllGatewayKeys, apiGetGatewayKeys } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useCreateGatewayKeyMutation as useCreateGatewayKey, useDeleteGatewayKeyMutation as useDeleteGatewayKey } from '@/hooks/mutations/useGatewayKeyMutations';

export function useGatewayKeysQuery(params?: Parameters<typeof apiGetGatewayKeys>[0]) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['gateway-keys', ...scope, params],
    queryFn: ({ signal }) => apiGetGatewayKeys(params, signal),
    placeholderData: keepPreviousData,
  });
}

/** Every gateway key, for pickers. */
export function useAllGatewayKeysQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['gateway-keys', 'all', ...scope],
    queryFn: ({ signal }) => apiGetAllGatewayKeys(signal),
  });
}
