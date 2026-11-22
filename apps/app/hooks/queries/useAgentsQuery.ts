import { useQuery } from '@tanstack/react-query';
import {
  apiGetAgents,
  apiGetAgent,
  apiGetAgentStats,
  apiGetAgentTemplates,
} from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateAgentMutation as useCreateAgent,
  useUpdateAgentMutation as useUpdateAgent,
  useDeleteAgentMutation as useDeleteAgent,
  useInstantiateAgentTemplateMutation,
} from '@/hooks/mutations/useAgentMutations';

export function useAgentsQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['agents', ...scope],
    queryFn: ({ signal }) => apiGetAgents(signal),
  });
}

export function useAgentTemplatesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['agent-templates', ...scope],
    queryFn: ({ signal }) => apiGetAgentTemplates(signal),
  });
}

export function useAgentDetailQuery(id: string | null | undefined) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['agent', ...scope, id],
    queryFn: ({ signal }) => (id ? apiGetAgent(id, signal) : Promise.reject('No agent id')),
    enabled: Boolean(id),
  });
}

export function useAgentStatsQuery(id: string | null | undefined, days = 30) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['agent-stats', ...scope, id, days],
    queryFn: ({ signal }) => (id ? apiGetAgentStats(id, days, signal) : Promise.reject('No agent id')),
    enabled: Boolean(id),
  });
}
