import { useQuery } from '@tanstack/react-query';
import { apiGetBillingPlans, apiGetActiveSubscription, apiGetInvoices } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useUpgradeSubscriptionMutation as useUpgradeSubscription } from '@/hooks/mutations/useBillingMutations';

export function useBillingPlansQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['billing-plans', ...scope],
    queryFn: ({ signal }) => apiGetBillingPlans(signal),
  });
}

export function useActiveSubscriptionQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['billing-subscription', ...scope],
    queryFn: ({ signal }) => apiGetActiveSubscription(signal),
  });
}

export function useInvoicesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['billing-invoices', ...scope],
    queryFn: ({ signal }) => apiGetInvoices(signal),
  });
}

export { useActiveSubscriptionQuery as useBillingSubscriptionQuery };
export { useInvoicesQuery as useBillingInvoicesQuery };
