import { useQuery } from '@tanstack/react-query';
import { apiGetResource, apiGetResources } from '@/lib/api';
import { useTenantScopeKey } from './useTenantScopeKey';
export {
  useCreateResourceMutation as useCreateResource,
  useUpdateResourceMutation as useUpdateResource,
  useDeleteResourceMutation as useDeleteResource,
  useTestResourceMutation as useTestResource,
} from '@/hooks/mutations/useResourceMutations';

export function useResourcesQuery() {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['resources', ...scope],
    queryFn: ({ signal }) => apiGetResources(signal),
  });
}

export function useResourceDetailQuery(resourceId?: string) {
  const scope = useTenantScopeKey();
  return useQuery({
    queryKey: ['resources', ...scope, resourceId],
    queryFn: ({ signal }) => apiGetResource(resourceId!, signal),
    enabled: Boolean(resourceId),
  });
}
