import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  apiGetAgents,
  apiGetAgent,
  apiGetAgentStats,
  apiGetAgentTemplates,
  apiInstantiateAgentTemplate,
} from '@/lib/api';
export { useCreateAgentMutation as useCreateAgent, useUpdateAgentMutation as useUpdateAgent, useDeleteAgentMutation as useDeleteAgent } from '@/hooks/mutations/useAgentMutations';

export function useAgentsQuery() {
  return useQuery({
    queryKey: ['agents'],
    queryFn: apiGetAgents,
  });
}

export function useAgentTemplatesQuery() {
  return useQuery({
    queryKey: ['agent-templates'],
    queryFn: apiGetAgentTemplates,
  });
}

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

export function useAgentDetailQuery(id: string | null | undefined) {
  return useQuery({
    queryKey: ['agent', id],
    queryFn: () => (id ? apiGetAgent(id) : Promise.reject('No agent id')),
    enabled: Boolean(id),
  });
}

export function useAgentStatsQuery(id: string | null | undefined, days = 30) {
  return useQuery({
    queryKey: ['agent-stats', id, days],
    queryFn: () => (id ? apiGetAgentStats(id, days) : Promise.reject('No agent id')),
    enabled: Boolean(id),
  });
}
