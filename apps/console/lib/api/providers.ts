import { api, resolveSignal, ApiSignal } from './client';
import { ApiProvider } from './types/provider';

export async function apiGetProviders(signal?: ApiSignal): Promise<ApiProvider[]> {
  const response = await api.get<ApiProvider[]>('/providers', {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiCreateProvider(data: Partial<ApiProvider>): Promise<ApiProvider> {
  const response = await api.post<ApiProvider>('/providers', data);
  return response.data;
}

export async function apiUpdateProvider(id: string, data: Partial<ApiProvider>): Promise<ApiProvider> {
  const response = await api.put<ApiProvider>(`/providers/${id}`, data);
  return response.data;
}

export async function apiDeleteProvider(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/providers/${id}`);
  return response.data;
}
