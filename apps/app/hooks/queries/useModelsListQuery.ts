import { useQuery } from '@tanstack/react-query';
import { apiGetAllModels } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';

export function useModelsListQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['all-models', ...scope],
    queryFn: apiGetAllModels,
  });
}
