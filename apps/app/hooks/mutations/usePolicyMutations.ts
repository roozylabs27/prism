import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiCreatePolicy,
  apiUpdatePolicy,
  apiDeletePolicy,
  apiSetDefaultPolicy,
  apiCreateRoutingRule,
  apiDeleteRoutingRule,
  ApiRoutingPolicy,
} from '@/lib/api';

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

export function useCreatePolicyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: Partial<ApiRoutingPolicy>) => apiCreatePolicy(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

export function useUpdatePolicyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<ApiRoutingPolicy> }) =>
      apiUpdatePolicy(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

export function useDeletePolicyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiDeletePolicy(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

export function useSetDefaultPolicyMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiSetDefaultPolicy(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['policies'] });
    },
  });
}

export { useCreatePolicyMutation as useCreatePolicy };
export { useUpdatePolicyMutation as useUpdatePolicy };
export { useDeletePolicyMutation as useDeletePolicy };
export { useSetDefaultPolicyMutation as useSetDefaultPolicy };
