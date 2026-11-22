import { api, resolveSignal, ApiSignal } from './client';
import { ApiTenantQuota } from './types/billing';
import { listData } from './envelope';

export async function apiGetQuotas(signal?: ApiSignal): Promise<ApiTenantQuota[]> {
  const response = await api.get<ApiTenantQuota[] | { data?: ApiTenantQuota[] }>('/quotas', {
    signal: resolveSignal(signal),
  });
  return listData(response.data);
}

export async function apiUpdateQuota(targetType: string, targetId: string, data: Partial<ApiTenantQuota>): Promise<ApiTenantQuota> {
  const response = await api.put<ApiTenantQuota | { quota?: ApiTenantQuota }>(`/quotas/${targetType}/${targetId}`, data);
  const body = response.data;
  if (body && typeof body === 'object' && 'quota' in body && body.quota) {
    return body.quota;
  }
  return body as ApiTenantQuota;
}
