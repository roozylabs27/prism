import { useQuery } from '@tanstack/react-query';
import { apiGetResource, apiGetResources } from '@/lib/api';
export {
  useCreateResourceMutation as useCreateResource,
  useUpdateResourceMutation as useUpdateResource,
  useDeleteResourceMutation as useDeleteResource,
  useTestResourceMutation as useTestResource,
} from '@/hooks/mutations/useResourceMutations';

export function useResourcesQuery() {
  return useQuery({
    queryKey: ['resources'],
    queryFn: apiGetResources,
  });
}

export function useResourceDetailQuery(resourceId?: string) {
  return useQuery({
    queryKey: ['resources', resourceId],
    queryFn: () => apiGetResource(resourceId!),
    enabled: Boolean(resourceId),
  });
}
