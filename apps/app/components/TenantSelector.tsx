'use client';

import { useEffect } from 'react';
import { Building2, ChevronDown, Check, Loader2, FolderKanban, Layers } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/molecules/DropdownMenu';
import { Button } from '@/components/atoms/Button';
import { useTenantStore } from '@/stores/useTenantStore';
import { useOrganizationsQuery } from '@/hooks/queries/useOrganizationsQuery';
import { useWorkspacesQuery, useProjectsQuery } from '@/hooks/queries/useWorkspacesQuery';
import { useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { apiGetUserPermissions, ApiUserPermissionsResponse } from '@/lib/api/auth';

const PRESERVED_GLOBAL_QUERY_ROOTS = new Set(['organizations', 'feature-flags']);

export function purgeTenantQueries(queryClient: QueryClient) {
  const isTenantScoped = (query: { queryKey: readonly unknown[] }) => {
    const root = typeof query.queryKey[0] === 'string' ? query.queryKey[0] : '';
    return !PRESERVED_GLOBAL_QUERY_ROOTS.has(root);
  };
  void queryClient.cancelQueries({ predicate: isTenantScoped });
  queryClient.removeQueries({ predicate: isTenantScoped });
  void queryClient.invalidateQueries({ queryKey: ['user-permissions'] });
}

export function TenantSelector() {
  const queryClient = useQueryClient();
  const {
    selectedOrgId,
    selectedOrgName,
    selectedWorkspaceId,
    selectedProjectId,
    setSelectedOrg,
    setSelectedWorkspaceId,
    setSelectedProjectId,
  } = useTenantStore();

  const { data: orgs, isLoading: isLoadingOrgs } = useOrganizationsQuery();
  const { data: userPerms, isLoading: isLoadingPerms } = useQuery<ApiUserPermissionsResponse>({
    queryKey: ['user-permissions'],
    queryFn: apiGetUserPermissions,
    staleTime: 60 * 1000,
  });
  const { data: workspaces = [], isSuccess: isWorkspacesLoaded } = useWorkspacesQuery(selectedOrgId);
  const { data: projects = [], isSuccess: isProjectsLoaded } = useProjectsQuery(selectedWorkspaceId);

  const isLoading = isLoadingOrgs || isLoadingPerms;

  // Authoritatively sync org from user token / API response
  useEffect(() => {
    if (!isLoading && orgs && orgs.length > 0) {
      const isSelectedValid = selectedOrgId && orgs.some((o) => o.id === selectedOrgId);
      if (!isSelectedValid) {
        const targetOrg = (userPerms?.organizationId && orgs.find((o) => o.id === userPerms.organizationId)) || orgs[0];
        setSelectedOrg(targetOrg.id, targetOrg.name);
      } else {
        const currentOrg = orgs.find((o) => o.id === selectedOrgId);
        if (currentOrg && currentOrg.name !== selectedOrgName) {
          setSelectedOrg(currentOrg.id, currentOrg.name);
        }
      }
    }
  }, [isLoading, orgs, userPerms, selectedOrgId, selectedOrgName, setSelectedOrg]);

  // Clear stale workspace/project IDs when they no longer exist under the active org/workspace
  useEffect(() => {
    if (selectedWorkspaceId && isWorkspacesLoaded && !workspaces.some((w) => w.id === selectedWorkspaceId)) {
      setSelectedWorkspaceId(null);
    }
  }, [selectedWorkspaceId, isWorkspacesLoaded, workspaces, setSelectedWorkspaceId]);

  useEffect(() => {
    if (selectedProjectId && isProjectsLoaded && !projects.some((p) => p.id === selectedProjectId)) {
      setSelectedProjectId(null);
    }
  }, [selectedProjectId, isProjectsLoaded, projects, setSelectedProjectId]);

  const handleSelectOrg = (orgId: string, orgName: string) => {
    if (orgId === selectedOrgId) return;
    setSelectedOrg(orgId, orgName);
    purgeTenantQueries(queryClient);
  };

  const handleSelectWorkspace = (workspaceId: string | null) => {
    if (workspaceId === selectedWorkspaceId) return;
    setSelectedWorkspaceId(workspaceId);
    purgeTenantQueries(queryClient);
  };

  const handleSelectProject = (projectId: string | null) => {
    if (projectId === selectedProjectId) return;
    setSelectedProjectId(projectId);
    purgeTenantQueries(queryClient);
  };

  const currentDisplayName = selectedOrgName || orgs?.find((o) => o.id === selectedOrgId)?.name || null;
  const activeWorkspace = workspaces.find((w) => w.id === selectedWorkspaceId) ?? null;
  const activeProject = projects.find((p) => p.id === selectedProjectId) ?? null;

  if (isLoading && !currentDisplayName) {
    return (
      <Button variant="outline" size="sm" className="h-8 gap-2 px-2.5 text-xs font-medium opacity-70" disabled>
        <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
        <span className="max-w-[120px] truncate">Loading tenant...</span>
      </Button>
    );
  }

  if (!orgs || orgs.length === 0) {
    return (
      <Button variant="outline" size="sm" className="h-8 gap-2 px-2.5 text-xs font-medium opacity-60" disabled>
        <Building2 className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="max-w-[120px] truncate">No Organization</span>
      </Button>
    );
  }

  return (
    <div className="flex items-center gap-1.5">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="sm" className="h-8 gap-2 rounded-lg border-border/80 bg-card px-2.5 text-xs font-medium hover:bg-accent">
            <Building2 className="h-3.5 w-3.5 text-prism-violet" />
            <span className="max-w-[140px] truncate">{currentDisplayName || 'Select Organization'}</span>
            <ChevronDown className="h-3 w-3 opacity-60" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-60">
          <DropdownMenuLabel className="text-[11px] text-muted-foreground uppercase tracking-wider">
            Active Organization
          </DropdownMenuLabel>
          <DropdownMenuSeparator />
          {orgs.map((org) => {
            const isSelected = org.id === selectedOrgId;
            return (
              <DropdownMenuItem
                key={org.id}
                onClick={() => handleSelectOrg(org.id, org.name)}
                className="flex items-center justify-between cursor-pointer"
              >
                <div className="flex flex-col max-w-[190px]">
                  <span className="truncate text-xs font-medium">{org.name}</span>
                  {org.slug && <span className="truncate text-[10px] text-muted-foreground font-mono">{org.slug}</span>}
                </div>
                {isSelected && <Check className="h-3.5 w-3.5 text-primary shrink-0 ml-2" />}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>

      {workspaces.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              aria-label="Select Workspace"
              className="inline-flex h-8 gap-1.5 rounded-lg border-border/80 bg-card px-2 sm:px-2.5 text-xs font-medium hover:bg-accent"
            >
              <Layers className="h-3.5 w-3.5 text-prism-cyan shrink-0" />
              <span className="max-w-[88px] sm:max-w-[120px] truncate">{activeWorkspace ? activeWorkspace.name : 'All Workspaces'}</span>
              <ChevronDown className="h-3 w-3 opacity-60 shrink-0" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-56">
            <DropdownMenuLabel className="text-[11px] text-muted-foreground uppercase tracking-wider">
              Workspace Scope
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => handleSelectWorkspace(null)} className="flex items-center justify-between cursor-pointer">
              <span className="text-xs">All Workspaces</span>
              {!selectedWorkspaceId && <Check className="h-3.5 w-3.5 text-primary" />}
            </DropdownMenuItem>
            {workspaces.map((ws) => (
              <DropdownMenuItem
                key={ws.id}
                onClick={() => handleSelectWorkspace(ws.id)}
                className="flex items-center justify-between cursor-pointer"
              >
                <div className="flex flex-col max-w-[170px]">
                  <span className="truncate text-xs font-medium">{ws.name}</span>
                  <span className="truncate text-[10px] text-muted-foreground font-mono">{ws.slug}</span>
                </div>
                {ws.id === selectedWorkspaceId && <Check className="h-3.5 w-3.5 text-primary shrink-0 ml-2" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}

      {selectedWorkspaceId && projects.length > 0 && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              aria-label="Select Project"
              className="inline-flex h-8 gap-1.5 rounded-lg border-border/80 bg-card px-2 sm:px-2.5 text-xs font-medium hover:bg-accent"
            >
              <FolderKanban className="h-3.5 w-3.5 text-emerald-500 shrink-0" />
              <span className="max-w-[80px] sm:max-w-[120px] truncate">{activeProject ? activeProject.name : 'All Projects'}</span>
              <ChevronDown className="h-3 w-3 opacity-60 shrink-0" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-56">
            <DropdownMenuLabel className="text-[11px] text-muted-foreground uppercase tracking-wider">
              Project Scope
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => handleSelectProject(null)} className="flex items-center justify-between cursor-pointer">
              <span className="text-xs">All Projects</span>
              {!selectedProjectId && <Check className="h-3.5 w-3.5 text-primary" />}
            </DropdownMenuItem>
            {projects.map((proj) => (
              <DropdownMenuItem
                key={proj.id}
                onClick={() => handleSelectProject(proj.id)}
                className="flex items-center justify-between cursor-pointer"
              >
                <div className="flex flex-col max-w-[170px]">
                  <span className="truncate text-xs font-medium">{proj.name}</span>
                  <span className="truncate text-[10px] text-muted-foreground font-mono">{proj.slug}</span>
                </div>
                {proj.id === selectedProjectId && <Check className="h-3.5 w-3.5 text-primary shrink-0 ml-2" />}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  );
}
