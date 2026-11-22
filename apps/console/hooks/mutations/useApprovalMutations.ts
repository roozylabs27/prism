import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiApproveRequest,
  apiDenyRequest,
  type ApiApprovalDecisionReq,
} from '@/lib/api';

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
