'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { Input } from '@/components/atoms/Input';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { useOrganizationQuery } from '@/hooks/queries/useOrganizationQuery';
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
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Settings, Pencil, Layers, FolderKanban, Plus, Trash2, Check, X } from 'lucide-react';
import { ApiSetting, ApiWorkspace, ApiProject } from '@/lib/api';
import { SettingFormSheet } from './_components/SettingFormSheet';
import { toast } from 'sonner';
import { getErrorMessage } from '@/types/ui';

export default function SettingsOrganizationPage() {
  const { data, isLoading, isError, refetch } = useOrganizationQuery();
  const settings: ApiSetting[] = data ?? [];

  const { selectedOrgId } = useTenantStore();
  const { data: workspaces = [], isLoading: wsLoading } = useWorkspacesQuery(selectedOrgId);

  const [selectedWsId, setSelectedWsId] = useState<string>('');
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

  const [newWsName, setNewWsName] = useState('');
  const [newWsSlug, setNewWsSlug] = useState('');
  const [editingWsId, setEditingWsId] = useState<string | null>(null);
  const [editingWsName, setEditingWsName] = useState('');

  const [newProjName, setNewProjName] = useState('');
  const [newProjSlug, setNewProjSlug] = useState('');
  const [editingProjId, setEditingProjId] = useState<string | null>(null);
  const [editingProjName, setEditingProjName] = useState('');

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSetting, setEditingSetting] = useState<ApiSetting | null>(null);

  const openEditDrawer = (s: ApiSetting) => {
    setEditingSetting(s);
    setDrawerOpen(true);
  };

  const handleCreateWorkspace = () => {
    if (!newWsName.trim()) return;
    createWorkspace.mutate(
      { name: newWsName.trim(), slug: newWsSlug.trim() || undefined },
      {
        onSuccess: (ws) => {
          toast.success(`Workspace "${ws.name}" created`);
          setNewWsName('');
          setNewWsSlug('');
          setSelectedWsId(ws.id);
        },
        onError: (err) => toast.error(`Failed: ${getErrorMessage(err)}`),
      }
    );
  };

  const handleSaveWorkspace = (ws: ApiWorkspace) => {
    if (!editingWsName.trim()) return;
    updateWorkspace.mutate(
      { workspaceId: ws.id, name: editingWsName.trim() },
      {
        onSuccess: () => {
          toast.success('Workspace updated');
          setEditingWsId(null);
        },
        onError: (err) => toast.error(`Update failed: ${getErrorMessage(err)}`),
      }
    );
  };

  const handleDeleteWorkspace = (ws: ApiWorkspace) => {
    deleteWorkspace.mutate(ws.id, {
      onSuccess: () => {
        toast.success(`Workspace "${ws.name}" deleted`);
        if (activeWsId === ws.id) setSelectedWsId('');
      },
      onError: (err) => toast.error(`Delete failed: ${getErrorMessage(err)}`),
    });
  };

  const handleCreateProject = () => {
    if (!activeWsId || !newProjName.trim()) return;
    createProject.mutate(
      { workspaceId: activeWsId, name: newProjName.trim(), slug: newProjSlug.trim() || undefined },
      {
        onSuccess: (proj) => {
          toast.success(`Project "${proj.name}" created`);
          setNewProjName('');
          setNewProjSlug('');
        },
        onError: (err) => toast.error(`Failed: ${getErrorMessage(err)}`),
      }
    );
  };

  const handleSaveProject = (proj: ApiProject) => {
    if (!activeWsId || !editingProjName.trim()) return;
    updateProject.mutate(
      { workspaceId: activeWsId, projectId: proj.id, name: editingProjName.trim() },
      {
        onSuccess: () => {
          toast.success('Project updated');
          setEditingProjId(null);
        },
        onError: (err) => toast.error(`Update failed: ${getErrorMessage(err)}`),
      }
    );
  };

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
    <AppLayout>
      <PageHeader
        title="Organization & System Settings"
        description="Configure organization details, workspaces, projects, and notification parameters."
      />

      {isError && <ErrorState onRetry={refetch} />}

      {!isError && (
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <Settings className="h-4 w-4 text-primary" />
                <span>Organization Parameters</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <p className="text-xs text-muted-foreground">Loading settings...</p>
              ) : settings.length === 0 ? (
                <EmptyState description="No organization settings configured yet." />
              ) : (
                <div className="space-y-2">
                  {settings.map((s, i) => (
                    <div
                      key={s.id ?? `${s.key}-${i}`}
                      className="flex items-center justify-between p-3 rounded-md border border-border bg-muted/20"
                    >
                      <div className="flex flex-col gap-0.5 min-w-0 flex-1 mr-3">
                        <span className="text-xs font-semibold text-foreground">{s.key}</span>
                        <span className="text-xs text-muted-foreground font-mono truncate">{s.value}</span>
                      </div>
                      <div className="flex items-center gap-2">
                        {s.scope === 'platform' && <Badge variant="outline">Platform default</Badge>}
                        {s.category && <Badge variant="violet">{s.category}</Badge>}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-foreground"
                          onClick={() => openEditDrawer(s)}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <div className="grid gap-6 lg:grid-cols-2">
            {/* Workspaces CRUD */}
            <Card>
              <CardHeader>
                <CardTitle className="text-sm font-semibold flex items-center gap-2">
                  <Layers className="h-4 w-4 text-prism-cyan" />
                  <span>Client Workspaces ({workspaces.length})</span>
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex flex-wrap gap-2">
                  <Input
                    value={newWsName}
                    onChange={(e) => setNewWsName(e.target.value)}
                    placeholder="Workspace name"
                    className="h-8 text-xs flex-1 min-w-[140px]"
                  />
                  <Input
                    value={newWsSlug}
                    onChange={(e) => setNewWsSlug(e.target.value)}
                    placeholder="slug (optional)"
                    className="h-8 text-xs w-[130px] font-mono"
                  />
                  <Button
                    size="sm"
                    variant="prismViolet"
                    className="h-8 gap-1 text-xs"
                    disabled={!newWsName.trim() || createWorkspace.isPending}
                    onClick={handleCreateWorkspace}
                  >
                    <Plus className="h-3.5 w-3.5" /> Add
                  </Button>
                </div>

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
                                value={editingWsName}
                                onChange={(e) => setEditingWsName(e.target.value)}
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
                                onClick={() => setSelectedWsId(ws.id)}
                                className="text-left min-w-0 flex-1 cursor-pointer"
                              >
                                <div className="font-semibold text-foreground truncate">{ws.name}</div>
                                <div className="font-mono text-[10px] text-muted-foreground truncate">{ws.slug}</div>
                              </button>
                              <div className="flex items-center gap-1">
                                <Button
                                  size="icon"
                                  variant="ghost"
                                  className="h-7 w-7"
                                  onClick={() => {
                                    setEditingWsId(ws.id);
                                    setEditingWsName(ws.name);
                                  }}
                                >
                                  <Pencil className="h-3.5 w-3.5" />
                                </Button>
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

            {/* Projects CRUD for Active Workspace */}
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
                <div className="flex flex-wrap gap-2">
                  <Input
                    value={newProjName}
                    onChange={(e) => setNewProjName(e.target.value)}
                    placeholder="Project name"
                    className="h-8 text-xs flex-1 min-w-[140px]"
                    disabled={!activeWsId}
                  />
                  <Input
                    value={newProjSlug}
                    onChange={(e) => setNewProjSlug(e.target.value)}
                    placeholder="slug (optional)"
                    className="h-8 text-xs w-[130px] font-mono"
                    disabled={!activeWsId}
                  />
                  <Button
                    size="sm"
                    variant="prismViolet"
                    className="h-8 gap-1 text-xs"
                    disabled={!activeWsId || !newProjName.trim() || createProject.isPending}
                    onClick={handleCreateProject}
                  >
                    <Plus className="h-3.5 w-3.5" /> Add
                  </Button>
                </div>

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
                              value={editingProjName}
                              onChange={(e) => setEditingProjName(e.target.value)}
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
                              <Button
                                size="icon"
                                variant="ghost"
                                className="h-7 w-7"
                                onClick={() => {
                                  setEditingProjId(proj.id);
                                  setEditingProjName(proj.name);
                                }}
                              >
                                <Pencil className="h-3.5 w-3.5" />
                              </Button>
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
        </div>
      )}

      <SettingFormSheet
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        editingSetting={editingSetting}
      />
    </AppLayout>
  );
}
