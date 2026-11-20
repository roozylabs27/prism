'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent, CardFooter } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge, StatusDot } from '@/components/atoms/Badge';
import { Switch } from '@/components/atoms/Switch';
import { useProvidersQuery, useDeleteProvider } from '@/hooks/queries/useProvidersQuery';
import { useUpdateProvider } from '@/hooks/mutations/useProviderMutations';
import { ApiProvider } from '@/lib/api';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { CardSkeletonGrid } from '@/components/molecules/CardSkeleton';
import Link from 'next/link';
import { AppRoutes } from '@/constants/routes';
import { Server, Plus, Trash2, KeyRound, Pencil } from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { getErrorMessage } from '@/types/ui';
import { ProviderFormDialog } from './_components/ProviderFormDialog';

export default function ProvidersPage() {
  const { data, isLoading, isError, refetch } = useProvidersQuery();
  const deleteMutation = useDeleteProvider();
  const updateMutation = useUpdateProvider();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<ApiProvider | null>(null);

  const openCreateDialog = () => {
    setEditingProvider(null);
    setDrawerOpen(true);
  };

  const openEditDialog = (provider: ApiProvider) => {
    setEditingProvider(provider);
    setDrawerOpen(true);
  };

  const handleToggle = (provider: ApiProvider, enabled: boolean) => {
    updateMutation.mutate(
      { id: provider.id, data: { enabled } },
      {
        onSuccess: () => toast.success(`${provider.name} ${enabled ? 'enabled' : 'disabled'}`),
        onError: (err) => toast.error(`Failed to update: ${getErrorMessage(err)}`),
      }
    );
  };

  const handleDelete = (provider: ApiProvider) => {
    deleteMutation.mutate(provider.id, {
      onSuccess: () => toast.success(`${provider.name} removed`),
      onError: (err) => toast.error(`Failed to remove: ${getErrorMessage(err)}`),
    });
  };

  const providers: ApiProvider[] = (data && Array.isArray(data)) ? data : [];

  return (
    <AppLayout>
      <PageHeader
        title="AI Model Providers"
        description="Manage connected AI provider adapters, rate limits, and health status."
        extra={
          <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={openCreateDialog}>
            <Plus className="h-4 w-4" /> Connect Custom Provider
          </Button>
        }
      />

      {isError ? (
        <ErrorState
          title="Failed to fetch providers"
          description="Could not communicate with Prism AI Adapter backend."
          onRetry={refetch}
        />
      ) : isLoading ? (
        <CardSkeletonGrid count={6} />
      ) : providers.length === 0 ? (
        <EmptyState
          title="No Connected Providers"
          description="There are no AI providers configured in this workspace."
          action={
            <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={openCreateDialog}>
              <Plus className="h-4 w-4" /> Connect Custom Provider
            </Button>
          }
        />
      ) : (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {providers.map((provider) => {
            const isSystem =
              provider.userId === 'user_admin' ||
              provider.userId === 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11' ||
              !provider.userId ||
              provider.userId === '';

            return (
              <Card key={provider.id} className="flex flex-col justify-between">
                <CardHeader>
                  <div className="flex items-center justify-between">
                    <CardTitle className="text-base font-bold flex items-center gap-2">
                      <Server className="h-5 w-5 text-primary" />
                      <span>{provider.name}</span>
                    </CardTitle>
                    <StatusDot status={provider.enabled ? 'healthy' : 'cooldown'} />
                  </div>
                  <div className="flex items-center gap-2 mt-1">
                    <span className="font-mono text-xs text-muted-foreground">type: {provider.type}</span>
                    {isSystem && (
                      <Badge variant="violet" className="text-[10px] py-0 px-1.5 font-mono">
                        System Adapter
                      </Badge>
                    )}
                  </div>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex items-center justify-between text-xs border-b border-border pb-2">
                    <span className="text-muted-foreground">Status</span>
                    <div className="flex items-center gap-2">
                      <Badge variant={provider.enabled ? 'success' : 'default'}>
                        {provider.enabled ? 'Enabled' : 'Disabled'}
                      </Badge>
                      {!isSystem && (
                        <Switch
                          checked={provider.enabled}
                          onCheckedChange={(checked) => handleToggle(provider, checked)}
                          disabled={updateMutation.isPending}
                          aria-label={`Toggle ${provider.name}`}
                        />
                      )}
                    </div>
                  </div>
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-muted-foreground">Base URL</span>
                    <span className="font-mono text-xs text-muted-foreground truncate max-w-[200px]">
                      {provider.baseUrl || '-'}
                    </span>
                  </div>
                </CardContent>
                <CardFooter className="border-t border-border pt-3 flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="w-full gap-1.5 text-xs"
                    asChild
                  >
                    <Link href={AppRoutes.CREDENTIALS}>
                      <KeyRound className="h-3.5 w-3.5 text-muted-foreground" /> Configure Keys
                    </Link>
                  </Button>

                  {!isSystem && (
                    <>
                      <Button
                        variant="outline"
                        size="sm"
                        className="gap-1.5 text-xs shrink-0"
                        onClick={() => openEditDialog(provider)}
                        aria-label={`Edit ${provider.name}`}
                      >
                        <Pencil className="h-3.5 w-3.5" /> Edit
                      </Button>
                      <ConfirmDialog
                        title="Remove Provider"
                        description={`Remove custom provider "${provider.name}"? This cannot be undone.`}
                        confirmText="Remove"
                        onConfirm={() => handleDelete(provider)}
                        trigger={
                          <Button
                            variant="destructive"
                            size="sm"
                            className="gap-1.5 text-xs shrink-0"
                            disabled={deleteMutation.isPending}
                          >
                            <Trash2 className="h-3.5 w-3.5" /> Remove
                          </Button>
                        }
                      />
                    </>
                  )}
                </CardFooter>
              </Card>
            );
          })}
        </div>
      )}

      <ProviderFormDialog
        open={drawerOpen}
        onOpenChange={(open) => {
          setDrawerOpen(open);
          if (!open) setEditingProvider(null);
        }}
        editingProvider={editingProvider}
      />
    </AppLayout>
  );
}
