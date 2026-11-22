import { useQuery } from '@tanstack/react-query';
import { apiGetTools, apiGetTool } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateToolMutation as useCreateTool,
  useUpdateToolMutation as useUpdateTool,
  useDeleteToolMutation as useDeleteTool,
  useTestToolMutation as useTestTool,
} from '@/hooks/mutations/useToolMutations';

export function useToolsQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['tools', ...scope],
    queryFn: apiGetTools,
  });
}

export function useToolDetailQuery(toolId?: string) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['tools', ...scope, toolId],
    queryFn: () => apiGetTool(toolId!),
    enabled: Boolean(toolId),
  });
}

