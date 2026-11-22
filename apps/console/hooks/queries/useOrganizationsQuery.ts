import { useQuery } from '@tanstack/react-query';
import { apiGetOrganizations, ApiOrganization } from '@/lib/api';

export function useOrganizationsQuery() {
  return useQuery<ApiOrganization[]>({
    queryKey: ['user-organizations'],
    queryFn: ({ signal }) => apiGetOrganizations(signal),
    staleTime: 5 * 60 * 1000,
  });
}
