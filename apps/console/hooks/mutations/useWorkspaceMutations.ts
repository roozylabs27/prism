import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiCreateProject,
  apiCreateWorkspace,
  apiDeleteProject,
  apiDeleteWorkspace,
  apiUpdateProject,
  apiUpdateWorkspace,
} from '@/lib/api';

export function useCreateWorkspaceMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: { name: string; slug?: string }) => apiCreateWorkspace(data),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['workspaces'] });
    },
  });
}

export function useUpdateWorkspaceMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, slug }: { workspaceId: string; name: string; slug?: string }) =>
      apiUpdateWorkspace(workspaceId, { name, slug }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['workspaces'] });
    },
  });
}

export function useDeleteWorkspaceMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (workspaceId: string) => apiDeleteWorkspace(workspaceId),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['workspaces'] });
      void qc.invalidateQueries({ queryKey: ['projects'] });
    },
  });
}

export function useCreateProjectMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, slug }: { workspaceId: string; name: string; slug?: string }) =>
      apiCreateProject(workspaceId, { name, slug }),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: ['projects', vars.workspaceId] });
    },
  });
}

export function useUpdateProjectMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      workspaceId,
      projectId,
      name,
      slug,
    }: {
      workspaceId: string;
      projectId: string;
      name: string;
      slug?: string;
    }) => apiUpdateProject(workspaceId, projectId, { name, slug }),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: ['projects', vars.workspaceId] });
    },
  });
}

export function useDeleteProjectMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, projectId }: { workspaceId: string; projectId: string }) =>
      apiDeleteProject(workspaceId, projectId),
    onSuccess: (_, vars) => {
      void qc.invalidateQueries({ queryKey: ['projects', vars.workspaceId] });
    },
  });
}
