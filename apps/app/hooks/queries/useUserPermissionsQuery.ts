import { useQuery, UseQueryOptions } from '@tanstack/react-query';
import { apiGetUserPermissions, ApiUserPermissionsResponse } from '@/lib/api';
import { useTenantStore } from '@/stores/useTenantStore';

export function useUserPermissionsQuery(options?: Partial<UseQueryOptions<ApiUserPermissionsResponse>>) {
  const selectedOrgId = useTenantStore((s) => s.selectedOrgId);
  return useQuery<ApiUserPermissionsResponse>({
    queryKey: ['user-permissions', selectedOrgId ?? 'default'],
    queryFn: ({ signal }) => apiGetUserPermissions(signal),
    staleTime: 60 * 1000,
    retry: 1,
    ...options,
  });
}
