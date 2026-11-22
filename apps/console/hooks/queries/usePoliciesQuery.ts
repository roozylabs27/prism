import { useQuery } from '@tanstack/react-query';
import { apiGetPolicies, apiGetRoutingRules } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreatePolicyMutation as useCreatePolicy,
  useUpdatePolicyMutation as useUpdatePolicy,
  useDeletePolicyMutation as useDeletePolicy,
  useSetDefaultPolicyMutation as useSetDefaultPolicy,
  useCreateRoutingRuleMutation,
  useDeleteRoutingRuleMutation,
} from '@/hooks/mutations/usePolicyMutations';

export function usePoliciesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['policies', ...scope],
    queryFn: ({ signal }) => apiGetPolicies(signal),
  });
}

export function useRoutingRulesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['routing-rules', ...scope],
    queryFn: ({ signal }) => apiGetRoutingRules(signal),
  });
}
