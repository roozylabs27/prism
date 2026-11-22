import { useQuery } from '@tanstack/react-query';
import { apiGetOrganizationMembers } from '@/lib/api/organizations';
import { ApiOrganizationMember } from '@/lib/api/types/common';
import { useTenantScopeKey } from './useTenantScopeKey';

export function useMembersQuery() {
  const scope = useTenantScopeKey();
  return useQuery<ApiOrganizationMember[]>({
    queryKey: ['organization-members', ...scope],
    queryFn: ({ signal }) => apiGetOrganizationMembers(signal),
  });
}
