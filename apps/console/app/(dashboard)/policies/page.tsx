'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardContent } from '@/components/molecules/Card';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/molecules/Tabs';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import {
  usePoliciesQuery,
  useUpdatePolicy,
  useDeletePolicy,
  useSetDefaultPolicy,
  useRoutingRulesQuery,
  useCreateRoutingRuleMutation,
  useDeleteRoutingRuleMutation,
} from '@/hooks/queries/usePoliciesQuery';
import { useProvidersQuery } from '@/hooks/queries/useProvidersQuery';
import { ApiRoutingPolicy } from '@/lib/api';
import { Input } from '@/components/atoms/Input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/molecules/Select';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Plus, Star, Trash2 } from 'lucide-react';
import { toast } from 'sonner';

import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { PermissionGuard } from '@/components/PermissionProvider';
import { getErrorMessage } from '@/types/ui';
import { toUiWeightPercents, type PolicyUiWeights } from '@/features/routing/utils/weights';
import { PolicyFormDialog } from './_components/PolicyFormDialog';
import { PolicyTuner } from './_components/PolicyTuner';

export default function PoliciesPage() {
  const { data: policies, isLoading, isError, refetch } = usePoliciesQuery();
  const { data: routingRules = [] } = useRoutingRulesQuery();
  const { data: providersData = [] } = useProvidersQuery();
  const providersList = Array.isArray(providersData) ? providersData : [];
  const updateMutation = useUpdatePolicy();
  const deleteMutation = useDeletePolicy();
  const setDefaultMutation = useSetDefaultPolicy();
  const createRuleMutation = useCreateRoutingRuleMutation();
  const deleteRuleMutation = useDeleteRoutingRuleMutation();

  const [selectedPolicyId, setSelectedPolicyId] = useState<string>('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [rulePattern, setRulePattern] = useState('');
  const [ruleProviderId, setRuleProviderId] = useState('');
  const [rulePriority, setRulePriority] = useState('10');

  const handleCreateRule = () => {
    if (!rulePattern.trim()) return;
    createRuleMutation.mutate(
      {
        modelPattern: rulePattern.trim(),
        providerId: ruleProviderId || undefined,
        priority: Number(rulePriority) || 10,
        enabled: true,
      },
      {
        onSuccess: () => {
          toast.success(`Routing rule "${rulePattern.trim()}" created`);
          setRulePattern('');
        },
        onError: (err) => toast.error(`Rule failed: ${getErrorMessage(err)}`),
      }
    );
  };

  const activePolicyId =
    (selectedPolicyId && policies?.some((p) => p.id === selectedPolicyId) ? selectedPolicyId : '') ||
    policies?.[0]?.id ||
    '';
  const selectedPolicy = policies?.find((p) => p.id === activePolicyId);

  const handleSaveWeights = (weights: PolicyUiWeights) => {
    if (!activePolicyId) return;
    updateMutation.mutate(
      {
        id: activePolicyId,
        data: { weights },
      },
      {
        onSuccess: () => toast.success('Policy weights updated'),
        onError: (err) => toast.error(`Failed to update: ${getErrorMessage(err)}`),
      }
    );
  };

  const handleDelete = (policy: ApiRoutingPolicy) => {
    deleteMutation.mutate(policy.id, {
      onSuccess: () => {
        toast.success(`Policy "${policy.name}" deleted`);
        if (activePolicyId === policy.id) {
          setSelectedPolicyId(policies?.find((p) => p.id !== policy.id)?.id ?? '');
        }
      },
      onError: (err) => toast.error(`Failed to delete: ${getErrorMessage(err)}`),
    });
  };

  const handleSetDefault = (policy: ApiRoutingPolicy) => {
    setDefaultMutation.mutate(policy.id, {
      onSuccess: () => toast.success(`"${policy.name}" set as default`),
      onError: (err) => toast.error(`Failed to set default: ${getErrorMessage(err)}`),
    });
  };

  return (
    <AppLayout>
      <PageHeader
        title="Routing Policies"
        description="Configure dynamic multi-factor routing policies to balance quality, cost, and latency."
        extra={
          <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to create routing policies">
            <Button
              variant="prismViolet"
              size="sm"
              className="gap-1.5"
              onClick={() => setDrawerOpen(true)}
            >
              <Plus className="h-4 w-4" /> Create Custom Policy
            </Button>
          </PermissionGuard>
        }
      />

      {isLoading ? (
        <div className="grid gap-6 lg:grid-cols-2">
          <Card><CardContent className="py-12 text-center text-muted-foreground">Loading policies...</CardContent></Card>
          <Card><CardContent className="py-12 text-center text-muted-foreground">Loading policies...</CardContent></Card>
        </div>
      ) : isError ? (
        <ErrorState title="Failed to load policies" onRetry={() => refetch()} />
      ) : !policies || policies.length === 0 ? (
        <EmptyState
          title="No routing policies found"
          description="Create a routing policy to get started."
          action={
            <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to create routing policies">
              <Button variant="prismViolet" size="sm" onClick={() => setDrawerOpen(true)}>Create Policy</Button>
            </PermissionGuard>
          }
        />
      ) : (
        <Tabs defaultValue="tuner">
          <TabsList>
            <TabsTrigger value="tuner">Policy Weight Tuner</TabsTrigger>
            <TabsTrigger value="active">Active Policies</TabsTrigger>
            <TabsTrigger value="rules">Model Routing Rules ({routingRules.length})</TabsTrigger>
          </TabsList>

          <TabsContent value="tuner">
            <PolicyTuner
              policies={policies}
              selectedPolicyId={activePolicyId}
              selectedPolicy={selectedPolicy}
              onSelectedPolicyChange={setSelectedPolicyId}
              saving={updateMutation.isPending}
              onSave={handleSaveWeights}
            />
          </TabsContent>

          <TabsContent value="active">
            <div className="grid gap-4 lg:grid-cols-2">
              {policies.map((policy) => (
                <Card key={policy.id}>
                  <CardContent className="pt-6 space-y-3">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="font-bold text-sm text-foreground">{policy.name}</span>
                        {policy.isDefault && <Badge variant="violet">Default</Badge>}
                        {!policy.enabled && <Badge variant="outline">Disabled</Badge>}
                      </div>
                      <div className="flex items-center gap-1">
                        {!policy.isDefault && (
                          <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to update routing policies">
                            <ConfirmDialog
                              title="Set Default Policy"
                              description={`Set "${policy.name}" as the default routing policy for all gateway requests without explicit policy override?`}
                              confirmText="Set Default"
                              onConfirm={() => handleSetDefault(policy)}
                              trigger={
                                <Button
                                  variant="ghost"
                                  size="sm"
                                  className="h-7 px-2 gap-1 text-xs"
                                  disabled={setDefaultMutation.isPending}
                                >
                                  <Star className="h-3 w-3" /> Set Default
                                </Button>
                              }
                            />
                          </PermissionGuard>
                        )}
                        <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to delete routing policies">
                          <ConfirmDialog
                            title="Delete Routing Policy"
                            description={`Delete policy "${policy.name}"? This cannot be undone.`}
                            confirmText="Delete"
                            onConfirm={() => handleDelete(policy)}
                            trigger={
                              <Button
                                variant="ghost"
                                size="sm"
                                className="h-7 px-2 gap-1 text-xs text-destructive hover:text-destructive"
                                disabled={deleteMutation.isPending}
                              >
                                <Trash2 className="h-3 w-3" /> Delete
                              </Button>
                            }
                          />
                        </PermissionGuard>
                      </div>
                    </div>
                    <div className="flex flex-wrap gap-4 text-xs text-muted-foreground font-mono">
                      {(() => {
                        const pcts = toUiWeightPercents(policy.weights);
                        return (
                          <>
                            <span>Task Match: {pcts.task_match}%</span>
                            <span>Quality: {pcts.quality}%</span>
                            <span>Cost: {pcts.cost}%</span>
                            <span>Speed: {pcts.speed}%</span>
                          </>
                        );
                      })()}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </TabsContent>

          <TabsContent value="rules" className="space-y-4">
            <Card>
              <CardContent className="pt-6 space-y-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    placeholder="Model glob pattern (e.g., claude-*)"
                    value={rulePattern}
                    onChange={(e) => setRulePattern(e.target.value)}
                    className="h-8 text-xs font-mono flex-1 min-w-[180px]"
                  />
                  <Select value={ruleProviderId} onValueChange={setRuleProviderId}>
                    <SelectTrigger className="h-8 w-[180px] text-xs">
                      <SelectValue
                        placeholder={
                          providersList && providersList.length > 0
                            ? "Pin provider..."
                            : "No providers"
                        }
                      />
                    </SelectTrigger>
                    <SelectContent>
                      {providersList && providersList.length > 0 ? (
                        providersList.map((p) => (
                          <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
                        ))
                      ) : (
                        <SelectItem value="__none__" disabled>
                          No providers configured
                        </SelectItem>
                      )}
                    </SelectContent>
                  </Select>
                  <Input
                    type="number"
                    placeholder="Priority"
                    value={rulePriority}
                    onChange={(e) => setRulePriority(e.target.value)}
                    className="h-8 w-[90px] text-xs font-mono"
                  />
                  <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to create routing rules">
                    <Button
                      size="sm"
                      variant="prismViolet"
                      className="h-8 gap-1 text-xs"
                      disabled={!rulePattern.trim() || createRuleMutation.isPending}
                      onClick={handleCreateRule}
                    >
                      <Plus className="h-3.5 w-3.5" /> Add Rule
                    </Button>
                  </PermissionGuard>
                </div>

                {routingRules.length === 0 ? (
                  <p className="text-xs text-muted-foreground">No model routing rules defined yet.</p>
                ) : (
                  <div className="space-y-2">
                    {routingRules.map((r) => (
                      <div key={r.id} className="flex items-center justify-between gap-2 rounded-md border border-border px-3 py-2 text-xs">
                        <div className="flex items-center gap-2 font-mono">
                          <Badge variant="outline">P{r.priority}</Badge>
                          <span className="font-semibold text-foreground">{r.modelPattern}</span>
                          {r.providerName && <span className="text-muted-foreground">-&gt; {r.providerName}</span>}
                        </div>
                        <PermissionGuard permission="routing:update" disabledTooltip="You do not have permission to delete routing rules">
                          <ConfirmDialog
                            title="Delete Routing Rule"
                            description={`Delete routing rule for "${r.modelPattern}"?`}
                            confirmText="Delete"
                            onConfirm={() =>
                              deleteRuleMutation.mutate(r.id, {
                                onSuccess: () => toast.success('Routing rule deleted'),
                                onError: (err) => toast.error(`Delete failed: ${getErrorMessage(err)}`),
                              })
                            }
                            trigger={
                              <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive">
                                <Trash2 className="h-3.5 w-3.5" />
                              </Button>
                            }
                          />
                        </PermissionGuard>
                      </div>
                    ))}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      )}

      <PolicyFormDialog open={drawerOpen} onOpenChange={setDrawerOpen} />
    </AppLayout>
  );
}
