import { useQuery } from '@tanstack/react-query';
import { apiGetPendingInvites, ApiMemberInvite } from '@/lib/api/organizations';
import { useTenantScopeKey } from './useTenantScopeKey';

export const PENDING_INVITES_QUERY_KEY = ['organization-invites'] as const;

export function usePendingInvitesQuery() {
  const scope = useTenantScopeKey();
  return useQuery<ApiMemberInvite[]>({
    queryKey: [...PENDING_INVITES_QUERY_KEY, ...scope],
    queryFn: ({ signal }) => apiGetPendingInvites(signal),
  });
}
