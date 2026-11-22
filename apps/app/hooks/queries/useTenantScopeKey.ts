import { useTenantStore } from '@/stores/useTenantStore';

export type TenantScopeKey = readonly [string, string, string];

/**
 * Returns the active [orgId, workspaceId, projectId] tuple for React Query key scoping,
 * ensuring cached responses are isolated across organization, workspace, and project switches.
 */
export function useTenantScopeKey(): TenantScopeKey {
  const selectedOrgId = useTenantStore((s) => s.selectedOrgId);
  const selectedWorkspaceId = useTenantStore((s) => s.selectedWorkspaceId);
  const selectedProjectId = useTenantStore((s) => s.selectedProjectId);
  return [selectedOrgId ?? 'none', selectedWorkspaceId ?? 'all', selectedProjectId ?? 'all'] as const;
}
