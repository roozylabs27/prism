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
    queryFn: apiGetMCPServers,
  });
}

export function useMCPServerToolsQuery(serverId?: string | null) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, serverId, 'tools'],
    queryFn: () => (serverId ? apiGetMCPServerTools(serverId) : Promise.resolve([])),
    enabled: Boolean(serverId),
  });
}

export function useMCPServerEditQuery(id: string) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, id, 'edit'],
    queryFn: () => apiGetMCPServer(id),
    enabled: Boolean(id),
  });
}

export function useMCPServerStatsQuery(id: string, days: number = 30) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['mcp-servers', ...scope, id, 'stats', days],
    queryFn: () => apiGetMCPServerStats(id, days),
    enabled: Boolean(id),
  });
}
