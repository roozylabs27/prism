import { useQuery } from '@tanstack/react-query';
import {
  apiGetMCPServers,
  apiGetMCPServer,
  apiGetMCPServerTools,
  apiGetMCPServerStats,
} from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateMCPServerMutation as useCreateMCPServer,
  useUpdateMCPServerMutation as useUpdateMCPServer,
  useDeleteMCPServerMutation as useDeleteMCPServer,
  useSyncMCPServerMutation as useSyncMCPServer,
  useTestMCPToolMutation as useTestMCPTool,
} from '@/hooks/mutations/useMCPMutations';

export function useMCPServersQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope],
    queryFn: ({ signal }) => apiGetMCPServers(signal),
  });
}

export function useMCPServerToolsQuery(serverId?: string | null) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, serverId, 'tools'],
    queryFn: ({ signal }) => (serverId ? apiGetMCPServerTools(serverId, signal) : Promise.resolve([])),
    enabled: Boolean(serverId),
  });
}

export function useMCPServerEditQuery(id: string) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, id, 'edit'],
    queryFn: ({ signal }) => apiGetMCPServer(id, signal),
    enabled: Boolean(id),
  });
}

export function useMCPServerStatsQuery(id: string, days: number = 30) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, id, 'stats', days],
    queryFn: ({ signal }) => apiGetMCPServerStats(id, days, signal),
    enabled: Boolean(id),
  });
}
