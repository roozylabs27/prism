import { api, resolveSignal, ApiSignal } from './client';
import {
  ApiMCPServer,
  ApiMCPServerWithTools,
  ApiCreateMCPServerRequest,
  ApiMCPTool,
  ApiMCPToolExecutionResult,
  ApiMCPServerStats,
  ApiMCPRegistryServer,
  ApiRegisterMCPRegistryRequest,
} from './types/mcp';

export async function apiGetMCPServers(signal?: ApiSignal): Promise<ApiMCPServer[]> {
  const response = await api.get<ApiMCPServer[]>('/mcp/servers', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiGetMCPServer(id: string, signal?: ApiSignal): Promise<ApiMCPServerWithTools> {
  const response = await api.get<ApiMCPServerWithTools>(`/mcp/servers/${id}`, {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiCreateMCPServer(data: ApiCreateMCPServerRequest): Promise<ApiMCPServer> {
  const response = await api.post<ApiMCPServer>('/mcp/servers', data);
  return response.data;
}

export async function apiUpdateMCPServer(id: string, data: Partial<ApiCreateMCPServerRequest>): Promise<ApiMCPServer> {
  const response = await api.put<ApiMCPServer>(`/mcp/servers/${id}`, data);
  return response.data;
}

export async function apiDeleteMCPServer(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/mcp/servers/${id}`);
  return response.data;
}

export async function apiSyncMCPServer(id: string): Promise<{ message: string; toolsCount: number }> {
  const response = await api.post<{ message: string; toolsCount: number }>(`/mcp/servers/${id}/sync`);
  return response.data;
}

export async function apiTestMCPTool(serverId: string, toolName: string, args: Record<string, unknown>): Promise<ApiMCPToolExecutionResult> {
  const response = await api.post<ApiMCPToolExecutionResult>(`/mcp/servers/${serverId}/test`, {
    tool: toolName,
    toolName,
    args,
  });
  return response.data;
}

export async function apiGetMCPServerTools(serverId: string, signal?: ApiSignal): Promise<ApiMCPTool[]> {
  const response = await api.get<ApiMCPTool[]>(`/mcp/servers/${serverId}/tools`, {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiGetMCPServerStats(id: string, days?: number, signal?: ApiSignal): Promise<ApiMCPServerStats> {
  const response = await api.get<ApiMCPServerStats | { data: ApiMCPServerStats }>(`/mcp/servers/${id}/stats`, {
    params: days ? { days } : undefined,
    signal: resolveSignal(signal),
  });
  if (response.data && typeof response.data === 'object' && 'data' in response.data) {
    return response.data.data;
  }
  return response.data as ApiMCPServerStats;
}

export async function apiGetMCPRegistryCatalog(signal?: ApiSignal): Promise<ApiMCPRegistryServer[]> {
  const response = await api.get<ApiMCPRegistryServer[] | { data?: ApiMCPRegistryServer[] }>('/mcp/registry', {
    signal: resolveSignal(signal),
  });
  if (response.data && typeof response.data === 'object' && 'data' in response.data && Array.isArray(response.data.data)) {
    return response.data.data;
  }
  return Array.isArray(response.data) ? response.data : [];
}

export async function apiRegisterMCPRegistryServer(data: ApiRegisterMCPRegistryRequest): Promise<ApiMCPServer> {
  const response = await api.post<ApiMCPServer>('/mcp/registry', data);
  return response.data;
}
