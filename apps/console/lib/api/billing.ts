import { api, resolveSignal, ApiSignal } from './client';
import {
  ApiBillingPlan,
  ApiSubscriptionStatus,
  ApiBillingInvoice,
} from './types/billing';
import { listData } from './envelope';

export async function apiGetBillingPlans(signal?: ApiSignal): Promise<ApiBillingPlan[]> {
  const response = await api.get<ApiBillingPlan[] | { data?: ApiBillingPlan[] }>('/billing/plans', {
    signal: resolveSignal(signal),
  });
  return listData(response.data);
}

export async function apiGetActiveSubscription(signal?: ApiSignal): Promise<ApiSubscriptionStatus> {
  const response = await api.get<ApiSubscriptionStatus>('/billing/subscription', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiUpgradeSubscription(planId: string): Promise<ApiSubscriptionStatus> {
  const response = await api.post<ApiSubscriptionStatus>('/billing/subscription/upgrade', {
    planSlug: planId,
    planId,
  });
  return response.data;
}

export async function apiGetInvoices(signal?: ApiSignal): Promise<ApiBillingInvoice[]> {
  const response = await api.get<ApiBillingInvoice[] | { data?: ApiBillingInvoice[] }>('/billing/invoices', {
    signal: resolveSignal(signal),
  });
  return listData(response.data);
}
