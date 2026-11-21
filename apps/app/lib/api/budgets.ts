import { api } from './client';
import { ApiBudget, ApiBudgetStatus } from './types/budget';

export async function apiGetBudgets(): Promise<ApiBudget[]> {
  const response = await api.get<ApiBudget[]>('/budgets');
  return response.data;
}

export async function apiGetBudgetStatus(): Promise<ApiBudgetStatus[]> {
  const response = await api.get<ApiBudgetStatus[]>('/budgets/status');
  return response.data;
}

export async function apiCreateBudget(data: Partial<ApiBudget>): Promise<ApiBudget> {
  const response = await api.post<ApiBudget>('/budgets', data);
  return response.data;
}

export async function apiUpdateBudget(id: string, data: Partial<ApiBudget>): Promise<ApiBudget> {
  const response = await api.put<ApiBudget>(`/budgets/${id}`, data);
  return response.data;
}

export interface ApiCostAnomaly {
  id: string;
  window_start: string;
  metric: string;
  observed: number;
  baseline_mean?: number | null;
  baseline_stddev?: number | null;
  z_score?: number | null;
  severity: string;
  created_at: string;
}

export interface ApiBudgetAlert {
  id: string;
  budget_id: string;
  alert_type: string;
  usage_percent?: number | null;
  monthly_spent?: number | null;
  monthly_limit?: number | null;
  acknowledged_at?: string | null;
  created_at: string;
}

export async function apiGetFinOpsAnomalies(limit = 50): Promise<ApiCostAnomaly[]> {
  const response = await api.get<{ anomalies?: ApiCostAnomaly[] }>('/analytics/finops/anomalies', {
    params: { limit },
  });
  return response.data?.anomalies ?? [];
}

export async function apiGetBudgetAlerts(limit = 50): Promise<ApiBudgetAlert[]> {
  const response = await api.get<{ alerts?: ApiBudgetAlert[] }>('/analytics/finops/budget-alerts', {
    params: { limit },
  });
  return response.data?.alerts ?? [];
}

export async function apiAcknowledgeBudgetAlert(id: string): Promise<{ message: string }> {
  const response = await api.post<{ message: string }>(`/analytics/finops/budget-alerts/${id}/acknowledge`);
  return response.data;
}

export async function apiDeleteBudget(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/budgets/${id}`);
  return response.data;
}
