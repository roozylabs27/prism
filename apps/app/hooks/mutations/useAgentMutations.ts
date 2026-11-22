import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiCreateAgent,
  apiUpdateAgent,
  apiDeleteAgent,
  apiInstantiateAgentTemplate,
  ApiCreateAgentRequest,
} from '@/lib/api';

export function useInstantiateAgentTemplateMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ templateId, name }: { templateId: string; name?: string }) =>
      apiInstantiateAgentTemplate(templateId, name),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['agents'] });
    },
  });
}

export function useCreateAgentMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: ApiCreateAgentRequest) => apiCreateAgent(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
    },
  });
}

export function useUpdateAgentMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<ApiCreateAgentRequest> }) =>
      apiUpdateAgent(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
    },
  });
}

export function useDeleteAgentMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => apiDeleteAgent(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['agents'] });
    },
  });
}

// Re-export aliases for backwards compatibility
export { useCreateAgentMutation as useCreateAgent };
export { useUpdateAgentMutation as useUpdateAgent };
export { useDeleteAgentMutation as useDeleteAgent };
