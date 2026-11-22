'use client';

import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Input } from '@/components/atoms/Input';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { EmptyState } from '@/components/molecules/StateAlerts';
import { PermissionGuard } from '@/components/PermissionProvider';
import {
  useWorkspacesQuery,
  useProjectsQuery,
  useCreateWorkspaceMutation,
  useUpdateWorkspaceMutation,
  useDeleteWorkspaceMutation,
  useCreateProjectMutation,
  useUpdateProjectMutation,
  useDeleteProjectMutation,
} from '@/hooks/queries/useWorkspacesQuery';
import { useTenantStore } from '@/stores/useTenantStore';
import type { ApiWorkspace, ApiProject } from '@/lib/api';
import {
  createWorkspaceSchema,
  createProjectSchema,
  renameEntitySchema,
  type CreateWorkspaceValues,
  type CreateProjectValues,
  type RenameEntityValues,
} from '../schemas/workspace.schema';
import { Layers, FolderKanban, Plus, Trash2, Pencil, Check, X, Rocket } from 'lucide-react';
import { toast } from 'sonner';
import { getErrorMessage } from '@/types/ui';

export function WorkspacesProjectsSection({
  selectedWsId,
  onSelectWsId,
  onOpenOnboarding,
}: {
  selectedWsId: string;
  onSelectWsId: (id: string) => void;
  onOpenOnboarding: () => void;
}) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);
  const { selectedOrgId } = useTenantStore();
  const { data: workspaces = [], isLoading: wsLoading } = useWorkspacesQuery(mounted ? selectedOrgId : null);

  const activeWsId =
    (selectedWsId && workspaces.some((w) => w.id === selectedWsId) ? selectedWsId : '') ||
    workspaces[0]?.id ||
    '';

  const { data: projects = [], isLoading: projLoading } = useProjectsQuery(activeWsId || null);

  const createWorkspace = useCreateWorkspaceMutation();
  const updateWorkspace = useUpdateWorkspaceMutation();
  const deleteWorkspace = useDeleteWorkspaceMutation();
  const createProject = useCreateProjectMutation();
  const updateProject = useUpdateProjectMutation();
  const deleteProject = useDeleteProjectMutation();

  const [editingWsId, setEditingWsId] = useState<string | null>(null);
  const [editingProjId, setEditingProjId] = useState<string | null>(null);

  const wsForm = useForm<CreateWorkspaceValues>({
    resolver: zodResolver(createWorkspaceSchema),
    defaultValues: { name: '', slug: '' },
  });

  const projForm = useForm<CreateProjectValues>({
    resolver: zodResolver(createProjectSchema),
    defaultValues: { name: '', slug: '' },
  });

  const renameWsForm = useForm<RenameEntityValues>({
    resolver: zodResolver(renameEntitySchema),
    defaultValues: { name: '' },
  });

  const renameProjForm = useForm<RenameEntityValues>({
    resolver: zodResolver(renameEntitySchema),
    defaultValues: { name: '' },
  });

  const handleCreateWorkspace = wsForm.handleSubmit((values) => {
    createWorkspace.mutate(
      { name: values.name.trim(), slug: values.slug?.trim() || undefined },
      {
        onSuccess: (ws) => {
          toast.success(`Workspace "${ws.name}" created`);
          wsForm.reset({ name: '', slug: '' });
          onSelectWsId(ws.id);
        },
        onError: (err) => toast.error(`Failed: ${getErrorMessage(err)}`),
      }
    );
  });

  const handleSaveWorkspace = (ws: ApiWorkspace) =>
    renameWsForm.handleSubmit((values) => {
      updateWorkspace.mutate(
        { workspaceId: ws.id, name: values.name.trim() },
        {
          onSuccess: () => {
            toast.success('Workspace updated');
            setEditingWsId(null);
          },
          onError: (err) => toast.error(`Update failed: ${getErrorMessage(err)}`),
        }
      );
    })();

  const handleDeleteWorkspace = (ws: ApiWorkspace) => {
    deleteWorkspace.mutate(ws.id, {
      onSuccess: () => {
        toast.success(`Workspace "${ws.name}" deleted`);
        if (activeWsId === ws.id) onSelectWsId('');
      },
      onError: (err) => toast.error(`Delete failed: ${getErrorMessage(err)}`),
    });
  };

  const handleCreateProject = projForm.handleSubmit((values) => {
    if (!activeWsId) return;
    createProject.mutate(
      { workspaceId: activeWsId, name: values.name.trim(), slug: values.slug?.trim() || undefined },
      {
        onSuccess: (proj) => {
          toast.success(`Project "${proj.name}" created`);
          projForm.reset({ name: '', slug: '' });
        },
        onError: (err) => toast.error(`Failed: ${getErrorMessage(err)}`),
      }
    );
  });

  const handleSaveProject = (proj: ApiProject) =>
    renameProjForm.handleSubmit((values) => {
      if (!activeWsId) return;
      updateProject.mutate(
        { workspaceId: activeWsId, projectId: proj.id, name: values.name.trim() },
        {
          onSuccess: () => {
            toast.success('Project updated');
            setEditingProjId(null);
          },
          onError: (err) => toast.error(`Update failed: ${getErrorMessage(err)}`),
        }
      );
    })();

  const handleDeleteProject = (proj: ApiProject) => {
    if (!activeWsId) return;
    deleteProject.mutate(
      { workspaceId: activeWsId, projectId: proj.id },
      {
        onSuccess: () => toast.success(`Project "${proj.name}" deleted`),
        onError: (err) => toast.error(`Delete failed: ${getErrorMessage(err)}`),
      }
    );
  };

  return (
    <div className="grid gap-6 lg:grid-cols-2">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-2">
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <Layers className="h-4 w-4 text-prism-cyan" />
            <span>Client Workspaces ({workspaces.length})</span>
          </CardTitle>
          <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to onboard workspaces">
            <Button size="sm" variant="outline" className="h-7 gap-1 text-xs" onClick={onOpenOnboarding}>
              <Rocket className="h-3.5 w-3.5 text-prism-violet" />
              Onboard Client
            </Button>
          </PermissionGuard>
        </CardHeader>
        <CardContent className="space-y-4">
          <form onSubmit={handleCreateWorkspace} className="space-y-1.5">
            <div className="flex flex-wrap gap-2">
              <Input
                {...wsForm.register('name')}
                placeholder="Workspace name"
                error={Boolean(wsForm.formState.errors.name)}
                className="h-8 text-xs flex-1 min-w-[140px]"
              />
              <Input
                {...wsForm.register('slug')}
                placeholder="slug (optional)"
                error={Boolean(wsForm.formState.errors.slug)}
                className="h-8 text-xs w-[130px] font-mono"
              />
              <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to create workspaces">
                <Button
                  type="submit"
                  size="sm"
                  variant="prismViolet"
                  className="h-8 gap-1 text-xs"
                  disabled={createWorkspace.isPending}
                >
                  <Plus className="h-3.5 w-3.5" /> Add
                </Button>
              </PermissionGuard>
            </div>
            {(wsForm.formState.errors.name || wsForm.formState.errors.slug) && (
              <p className="text-[11px] text-destructive">
                {wsForm.formState.errors.name?.message || wsForm.formState.errors.slug?.message}
              </p>
            )}
          </form>

          {wsLoading ? (
            <p className="text-xs text-muted-foreground">Loading workspaces...</p>
          ) : workspaces.length === 0 ? (
            <EmptyState description="No workspaces in this organization yet." />
          ) : (
            <div className="space-y-2">
              {workspaces.map((ws) => {
                const active = ws.id === activeWsId;
                return (
                  <div
                    key={ws.id}
                    className={`flex items-center justify-between gap-2 rounded-md border p-2.5 text-xs ${
                      active ? 'border-primary bg-primary/5' : 'border-border bg-muted/10'
                    }`}
                  >
                    {editingWsId === ws.id ? (
                      <div className="flex items-center gap-1.5 flex-1">
                        <Input
                          {...renameWsForm.register('name')}
                          error={Boolean(renameWsForm.formState.errors.name)}
                          className="h-7 text-xs"
                        />
                        <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleSaveWorkspace(ws)}>
                          <Check className="h-3.5 w-3.5 text-emerald-500" />
                        </Button>
                        <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => setEditingWsId(null)}>
                          <X className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ) : (
                      <>
                        <button
                          type="button"
                          onClick={() => onSelectWsId(ws.id)}
                          className="text-left min-w-0 flex-1 cursor-pointer"
                        >
                          <div className="font-semibold text-foreground truncate">{ws.name}</div>
                          <div className="font-mono text-[10px] text-muted-foreground truncate">{ws.slug}</div>
                        </button>
                        <div className="flex items-center gap-1">
                          <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to edit workspaces">
                            <Button
                              size="icon"
                              variant="ghost"
                              className="h-7 w-7"
                              onClick={() => {
                                setEditingWsId(ws.id);
                                renameWsForm.reset({ name: ws.name });
                              }}
                            >
                              <Pencil className="h-3.5 w-3.5" />
                            </Button>
                          </PermissionGuard>
                          <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to delete workspaces">
                            <ConfirmDialog
                              title="Delete Workspace"
                              description={`Delete workspace "${ws.name}" and its projects?`}
                              confirmText="Delete"
                              onConfirm={() => handleDeleteWorkspace(ws)}
                              trigger={
                                <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive">
                                  <Trash2 className="h-3.5 w-3.5" />
                                </Button>
                              }
                            />
                          </PermissionGuard>
                        </div>
                      </>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <FolderKanban className="h-4 w-4 text-emerald-500" />
            <span>
              Projects {workspaces.find((w) => w.id === activeWsId)?.name ? `(${workspaces.find((w) => w.id === activeWsId)?.name})` : ''}
            </span>
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <form onSubmit={handleCreateProject} className="space-y-1.5">
            <div className="flex flex-wrap gap-2">
              <Input
                {...projForm.register('name')}
                placeholder="Project name"
                error={Boolean(projForm.formState.errors.name)}
                className="h-8 text-xs flex-1 min-w-[140px]"
                disabled={!activeWsId}
              />
              <Input
                {...projForm.register('slug')}
                placeholder="slug (optional)"
                error={Boolean(projForm.formState.errors.slug)}
                className="h-8 text-xs w-[130px] font-mono"
                disabled={!activeWsId}
              />
              <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to create projects">
                <Button
                  type="submit"
                  size="sm"
                  variant="prismViolet"
                  className="h-8 gap-1 text-xs"
                  disabled={!activeWsId || createProject.isPending}
                >
                  <Plus className="h-3.5 w-3.5" /> Add
                </Button>
              </PermissionGuard>
            </div>
            {(projForm.formState.errors.name || projForm.formState.errors.slug) && (
              <p className="text-[11px] text-destructive">
                {projForm.formState.errors.name?.message || projForm.formState.errors.slug?.message}
              </p>
            )}
          </form>

          {!activeWsId ? (
            <EmptyState description="Select or create a workspace to manage its projects." />
          ) : projLoading ? (
            <p className="text-xs text-muted-foreground">Loading projects...</p>
          ) : projects.length === 0 ? (
            <EmptyState description="No projects in this workspace yet." />
          ) : (
            <div className="space-y-2">
              {projects.map((proj) => (
                <div
                  key={proj.id}
                  className="flex items-center justify-between gap-2 rounded-md border border-border bg-muted/10 p-2.5 text-xs"
                >
                  {editingProjId === proj.id ? (
                    <div className="flex items-center gap-1.5 flex-1">
                      <Input
                        {...renameProjForm.register('name')}
                        error={Boolean(renameProjForm.formState.errors.name)}
                        className="h-7 text-xs"
                      />
                      <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => handleSaveProject(proj)}>
                        <Check className="h-3.5 w-3.5 text-emerald-500" />
                      </Button>
                      <Button size="icon" variant="ghost" className="h-7 w-7" onClick={() => setEditingProjId(null)}>
                        <X className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  ) : (
                    <>
                      <div className="min-w-0 flex-1">
                        <div className="font-semibold text-foreground truncate">{proj.name}</div>
                        <div className="font-mono text-[10px] text-muted-foreground truncate">{proj.slug}</div>
                      </div>
                      <div className="flex items-center gap-1">
                        <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to edit projects">
                          <Button
                            size="icon"
                            variant="ghost"
                            className="h-7 w-7"
                            onClick={() => {
                              setEditingProjId(proj.id);
                              renameProjForm.reset({ name: proj.name });
                            }}
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                        </PermissionGuard>
                        <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to delete projects">
                          <ConfirmDialog
                            title="Delete Project"
                            description={`Delete project "${proj.name}"?`}
                            confirmText="Delete"
                            onConfirm={() => handleDeleteProject(proj)}
                            trigger={
                              <Button size="icon" variant="ghost" className="h-7 w-7 text-destructive hover:text-destructive">
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            }
                          />
                        </PermissionGuard>
                      </div>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
