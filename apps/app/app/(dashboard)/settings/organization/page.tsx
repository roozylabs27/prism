'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { useOrganizationQuery } from '@/hooks/queries/useOrganizationQuery';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { PermissionGuard } from '@/components/PermissionProvider';
import { Settings, Pencil } from 'lucide-react';
import { ApiSetting } from '@/lib/api';
import { ClientOnboardingModal } from '@/components/organisms/ClientOnboardingModal';
import { WorkspacesProjectsSection } from '@/features/settings/components/WorkspacesProjectsSection';
import { SettingFormSheet } from './_components/SettingFormSheet';

export default function SettingsOrganizationPage() {
  const { data, isLoading, isError, refetch } = useOrganizationQuery();
  const settings: ApiSetting[] = data ?? [];

  const [selectedWsId, setSelectedWsId] = useState<string>('');
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSetting, setEditingSetting] = useState<ApiSetting | null>(null);
  const [onboardingOpen, setOnboardingOpen] = useState(false);

  const openEditDrawer = (s: ApiSetting) => {
    setEditingSetting(s);
    setDrawerOpen(true);
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
                        <PermissionGuard permission="organization:update" disabledTooltip="You do not have permission to edit organization settings">
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-7 w-7 text-muted-foreground hover:text-foreground"
                            onClick={() => openEditDrawer(s)}
                          >
                            <Pencil className="h-3.5 w-3.5" />
                          </Button>
                        </PermissionGuard>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <WorkspacesProjectsSection
            selectedWsId={selectedWsId}
            onSelectWsId={setSelectedWsId}
            onOpenOnboarding={() => setOnboardingOpen(true)}
          />
        </div>
      )}

      <SettingFormSheet
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        editingSetting={editingSetting}
      />

      <ClientOnboardingModal
        open={onboardingOpen}
        onOpenChange={setOnboardingOpen}
        onCompleted={({ workspaceId }) => setSelectedWsId(workspaceId)}
      />
    </AppLayout>
  );
}
