import { useQuery } from '@tanstack/react-query';
import {
  apiGetBudgetAlerts,
  apiGetBudgets,
  apiGetBudgetStatus,
  apiGetFinOpsAnomalies,
} from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateBudgetMutation as useCreateBudget,
  useUpdateBudgetMutation as useUpdateBudget,
  useDeleteBudgetMutation as useDeleteBudget,
  useAcknowledgeBudgetAlertMutation,
} from '@/hooks/mutations/useBudgetMutations';

export function useBudgetsQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['budgets', ...scope],
    queryFn: ({ signal }) => apiGetBudgets(signal),
  });
}

export function useBudgetStatusQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['budget-status', ...scope],
    queryFn: ({ signal }) => apiGetBudgetStatus(signal),
  });
}

export function useFinOpsAnomaliesQuery(enabled = true) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['finops-anomalies', ...scope],
    queryFn: ({ signal }) => apiGetFinOpsAnomalies(50, signal),
    enabled,
  });
}

export function useBudgetAlertsQuery(enabled = true) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['budget-alerts', ...scope],
    queryFn: ({ signal }) => apiGetBudgetAlerts(50, signal),
    enabled,
  });
}
