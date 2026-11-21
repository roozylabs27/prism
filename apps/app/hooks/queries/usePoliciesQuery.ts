import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  apiCreateRoutingRule,
  apiDeleteRoutingRule,
  apiGetPolicies,
  apiGetRoutingRules,
} from '@/lib/api';
export {
  useCreatePolicyMutation as useCreatePolicy,
  useUpdatePolicyMutation as useUpdatePolicy,
  useDeletePolicyMutation as useDeletePolicy,
  useSetDefaultPolicyMutation as useSetDefaultPolicy,
} from '@/hooks/mutations/usePolicyMutations';

export function usePoliciesQuery() {
  return useQuery({
    queryKey: ['policies'],
    queryFn: apiGetPolicies,
  });
}

export function useRoutingRulesQuery() {
  return useQuery({
    queryKey: ['routing-rules'],
    queryFn: apiGetRoutingRules,
  });
}

export function useCreateRoutingRuleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { modelPattern: string; providerId?: string; priority?: number; enabled?: boolean }) =>
      apiCreateRoutingRule(data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['routing-rules'] });
    },
  });
}

export function useDeleteRoutingRuleMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiDeleteRoutingRule(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['routing-rules'] });
    },
  });
}
