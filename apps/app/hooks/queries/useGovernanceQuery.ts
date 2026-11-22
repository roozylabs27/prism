import { useQuery } from '@tanstack/react-query';
import { apiGetGovernancePolicies } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateGovernancePolicyMutation as useCreateGovernancePolicy,
  useUpdateGovernancePolicyMutation as useUpdateGovernancePolicy,
  useDeleteGovernancePolicyMutation as useDeleteGovernancePolicy,
  useEvaluateRBACMutation as useEvaluateRBAC,
} from '@/hooks/mutations/useGovernanceMutations';

export function useGovernancePoliciesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['governance-policies', ...scope],
    queryFn: apiGetGovernancePolicies,
  });
}

export { useGovernancePoliciesQuery as useGovernanceQuery };
