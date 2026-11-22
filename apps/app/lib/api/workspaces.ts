import { api, resolveSignal, ApiSignal } from './client';
import type { ApiWorkspace } from './types';

export interface ApiProject {
  id: string;
  workspaceId: string;
  name: string;
  slug: string;
  createdAt: string;
  updatedAt: string;
}

export async function apiGetWorkspaces(signal?: ApiSignal): Promise<ApiWorkspace[]> {
  const response = await api.get<ApiWorkspace[] | { data?: ApiWorkspace[] }>('/workspaces', {
    signal: resolveSignal(signal),
  });
  if (response.data && typeof response.data === 'object' && 'data' in response.data && Array.isArray(response.data.data)) {
    return response.data.data;
  }
  return Array.isArray(response.data) ? response.data : [];
}

export async function apiCreateWorkspace(data: { name: string; slug?: string }): Promise<ApiWorkspace> {
  const response = await api.post<ApiWorkspace>('/workspaces', data);
  return response.data;
}

export async function apiUpdateWorkspace(workspaceId: string, data: { name: string; slug?: string }): Promise<ApiWorkspace> {
  const response = await api.put<ApiWorkspace>(`/workspaces/${workspaceId}`, data);
  return response.data;
}

export async function apiDeleteWorkspace(workspaceId: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/workspaces/${workspaceId}`);
  return response.data;
}

export async function apiGetProjects(workspaceId: string, signal?: ApiSignal): Promise<ApiProject[]> {
  const response = await api.get<ApiProject[] | { data?: ApiProject[] }>(`/workspaces/${workspaceId}/projects`, {
    signal: resolveSignal(signal),
  });
  if (response.data && typeof response.data === 'object' && 'data' in response.data && Array.isArray(response.data.data)) {
    return response.data.data;
  }
  return Array.isArray(response.data) ? response.data : [];
}

export async function apiCreateProject(workspaceId: string, data: { name: string; slug?: string }): Promise<ApiProject> {
  const response = await api.post<ApiProject>(`/workspaces/${workspaceId}/projects`, data);
  return response.data;
}

export async function apiUpdateProject(workspaceId: string, projectId: string, data: { name: string; slug?: string }): Promise<ApiProject> {
  const response = await api.put<ApiProject>(`/workspaces/${workspaceId}/projects/${projectId}`, data);
  return response.data;
}

export async function apiDeleteProject(workspaceId: string, projectId: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/workspaces/${workspaceId}/projects/${projectId}`);
  return response.data;
}
