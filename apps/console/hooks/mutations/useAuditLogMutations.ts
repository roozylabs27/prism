import { useMutation } from '@tanstack/react-query';
import { apiExportAuditLogs, apiVerifyAuditIntegrity } from '@/lib/api';

export function useExportAuditLogsMutation() {
  return useMutation({
    mutationFn: (params: { format: 'csv' | 'json'; type?: 'ai' | 'admin'; startDate?: string; endDate?: string }) =>
      apiExportAuditLogs(params),
  });
}

export function useVerifyAuditIntegrity() {
  return useMutation({
    mutationFn: (id: string) => apiVerifyAuditIntegrity(id),
  });
}
