import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  apiApproveRequest,
  apiDenyRequest,
  apiGetApprovals,
  type ApiApprovalDecisionReq,
} from '@/lib/api';

export function useApprovalsQuery(status = 'pending') {
  return useQuery({
    queryKey: ['approvals', status],
    queryFn: () => apiGetApprovals(status),
    refetchInterval: 15_000,
  });
}

export function useApproveRequestMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      apiApproveRequest(id, { note } as ApiApprovalDecisionReq),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['approvals'] });
    },
  });
}

export function useDenyRequestMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, note }: { id: string; note?: string }) =>
      apiDenyRequest(id, { note } as ApiApprovalDecisionReq),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['approvals'] });
    },
  });
}
