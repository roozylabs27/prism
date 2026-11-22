import { useQuery } from '@tanstack/react-query';
import { apiGetProjects, apiGetWorkspaces } from '@/lib/api';
export {
  useCreateWorkspaceMutation,
  useUpdateWorkspaceMutation,
  useDeleteWorkspaceMutation,
  useCreateProjectMutation,
  useUpdateProjectMutation,
  useDeleteProjectMutation,
} from '@/hooks/mutations/useWorkspaceMutations';

export function useWorkspacesQuery(orgId?: string | null) {
  return useQuery({
    queryKey: ['workspaces', orgId ?? 'default'],
    queryFn: apiGetWorkspaces,
    enabled: orgId !== null,
  });
}

export function useProjectsQuery(workspaceId?: string | null) {
  return useQuery({
    queryKey: ['projects', workspaceId],
    queryFn: () => apiGetProjects(workspaceId!),
    enabled: Boolean(workspaceId),
  });
}
