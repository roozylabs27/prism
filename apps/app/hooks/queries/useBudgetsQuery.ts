import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  apiAcknowledgeBudgetAlert,
  apiGetBudgetAlerts,
  apiGetBudgets,
  apiGetBudgetStatus,
  apiGetFinOpsAnomalies,
} from '@/lib/api';
export { useCreateBudgetMutation as useCreateBudget, useUpdateBudgetMutation as useUpdateBudget, useDeleteBudgetMutation as useDeleteBudget } from '@/hooks/mutations/useBudgetMutations';

export function useBudgetsQuery() {
  return useQuery({
    queryKey: ['budgets'],
    queryFn: apiGetBudgets,
  });
}

export function useBudgetStatusQuery() {
  return useQuery({
    queryKey: ['budget-status'],
    queryFn: apiGetBudgetStatus,
  });
}

export function useFinOpsAnomaliesQuery() {
  return useQuery({
    queryKey: ['finops-anomalies'],
    queryFn: () => apiGetFinOpsAnomalies(),
  });
}

export function useBudgetAlertsQuery() {
  return useQuery({
    queryKey: ['budget-alerts'],
    queryFn: () => apiGetBudgetAlerts(),
  });
}

export function useAcknowledgeBudgetAlertMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiAcknowledgeBudgetAlert(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['budget-alerts'] });
    },
  });
}
