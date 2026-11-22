import { useQuery } from '@tanstack/react-query';
import { apiGetApprovals } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useApproveRequestMutation,
  useDenyRequestMutation,
} from '@/hooks/mutations/useApprovalMutations';

export function useApprovalsQuery(status = 'pending') {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['approvals', ...scope, status],
    queryFn: ({ signal }) => apiGetApprovals(status, 1, 50, signal),
    refetchInterval: 15_000,
  });
}
