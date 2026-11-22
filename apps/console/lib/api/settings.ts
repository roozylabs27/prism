import { api, resolveSignal, ApiSignal } from './client';
import { ApiHealthResponse, ApiSetting } from './types/common';

// The current organization's settings: its own values plus the platform
// defaults it has not overridden.
export async function apiGetSettings(signal?: ApiSignal): Promise<ApiSetting[]> {
  const response = await api.get<{ value: ApiSetting[] }>('/settings', {
    signal: resolveSignal(signal),
  });
  return response.data.value ?? [];
}

// Saves values for the current organization only.
export async function apiUpdateSettings(data: Record<string, string>): Promise<void> {
  await api.put('/settings', { settings: data });
}

export async function apiGetHealth(signal?: ApiSignal): Promise<ApiHealthResponse> {
  const response = await api.get<ApiHealthResponse>('/health', {
    signal: resolveSignal(signal),
  });
  return response.data;
}
