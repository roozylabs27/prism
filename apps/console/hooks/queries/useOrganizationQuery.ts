import { useQuery } from '@tanstack/react-query';
import { apiGetSettings } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export { useUpdateSettingsMutation as useUpdateSettings } from '@/hooks/mutations/useSettingMutations';

export function useOrganizationQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['settings', ...scope],
    queryFn: ({ signal }) => apiGetSettings(signal),
  });
}
