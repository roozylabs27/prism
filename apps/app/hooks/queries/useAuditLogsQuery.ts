import { useQuery } from '@tanstack/react-query';
import { apiGetAuditLogs } from '@/lib/api';

export function useAuditLogsQuery(params?: { page?: number; limit?: number }) {
  return useQuery({
    queryKey: ['audit-logs', params],
    queryFn: () => apiGetAuditLogs(params),
  });
}
