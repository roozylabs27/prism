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
    queryFn: apiGetBudgets,
  });
}

export function useBudgetStatusQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['budget-status', ...scope],
    queryFn: apiGetBudgetStatus,
  });
}

export function useFinOpsAnomaliesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['finops-anomalies', ...scope],
    queryFn: () => apiGetFinOpsAnomalies(),
  });
}

export function useBudgetAlertsQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['budget-alerts', ...scope],
    queryFn: () => apiGetBudgetAlerts(),
  });
}
