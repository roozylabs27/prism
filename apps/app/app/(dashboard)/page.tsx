'use client';

import { useMemo, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { Button } from '@/components/atoms/Button';
import type { TrafficChartData, ModelShareItem } from '@/components/organisms/ChartContainer';
import { ErrorState } from '@/components/molecules/StateAlerts';
import {
  useDashboardStatsQuery,
  useUsageChartQuery,
  useDashboardHealthQuery,
  usePilotOutcomesQuery,
} from '@/hooks/queries/useDashboardQuery';
import { useLogsQuery, useLogAnalyticsQuery } from '@/hooks/queries/useLogsQuery';
import { useWorkspacesQuery } from '@/hooks/queries/useWorkspacesQuery';
import { useTenantStore } from '@/stores/useTenantStore';
import { ApiRequestLog } from '@/lib/api';
import { ClientOnboardingModal } from '@/components/organisms/ClientOnboardingModal';
import {
  DashboardOperationalBreakdown,
  type DashboardHealthSignal,
} from '@/features/dashboard/components/DashboardOperationalBreakdown';
import { DashboardPilotOutcomesCard } from '@/features/dashboard/components/DashboardPilotOutcomesCard';
import { DashboardKpiGrid } from '@/features/dashboard/components/DashboardKpiGrid';
import { DashboardTrafficCharts } from '@/features/dashboard/components/DashboardTrafficCharts';
import {
  DashboardRecentActivityCard,
  type RecentActivity,
} from '@/features/dashboard/components/DashboardRecentActivityCard';
import { AppRoutes } from '@/constants/routes';
import { Download, RefreshCw } from 'lucide-react';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/molecules/Select';
import { toast } from 'sonner';

const DATE_RANGE_OPTIONS = [
  { value: '24h', label: 'Last 24 Hours', days: 1 },
  { value: '7d', label: 'Last 7 Days', days: 7 },
  { value: '30d', label: 'Last 30 Days', days: 30 },
  { value: '90d', label: 'Last 90 Days', days: 90 },
  { value: 'all', label: 'All Time', days: 0 },
];

export default function DashboardPage() {
  const [dateRange, setDateRange] = useState<string>('30d');
  const [pilotWorkspaceId, setPilotWorkspaceId] = useState<string>('all');
  const [onboardingOpen, setOnboardingOpen] = useState(false);
  const selectedDays = DATE_RANGE_OPTIONS.find((opt) => opt.value === dateRange)?.days ?? 30;
  const { selectedOrgId } = useTenantStore();
  const { data: workspaces = [] } = useWorkspacesQuery(selectedOrgId);

  const {
    data: stats,
    isLoading,
    isError: isStatsError,
    refetch: refetchStats,
    isFetching: isFetchingStats,
  } = useDashboardStatsQuery(dateRange);
  const { data: usageData, refetch: refetchUsage } = useUsageChartQuery(dateRange);
  const { data: logsData, refetch: refetchLogs } = useLogsQuery({ limit: 10 });
  const { data: healthData, isLoading: isHealthLoading, refetch: refetchHealth } = useDashboardHealthQuery();
  const { data: analyticsData, isLoading: isAnalyticsLoading, refetch: refetchAnalytics } =
    useLogAnalyticsQuery(selectedDays);
  const activePilotWorkspace = pilotWorkspaceId !== 'all' ? pilotWorkspaceId : undefined;
  const { data: pilotOutcomes, refetch: refetchPilot } = usePilotOutcomesQuery(
    selectedDays || 90,
    activePilotWorkspace
  );

  const currentRangeLabel =
    ({ '24h': '24h', '7d': '7d', '30d': '30d', '90d': '90d', all: 'All Time' } as Record<string, string>)[
      dateRange
    ] || '30d';

  const { trafficData, modelKeys, sparklineSeries } = useMemo(() => {
    if (!usageData || usageData.length === 0) {
      return { trafficData: [] as TrafficChartData[], modelKeys: [] as string[], sparklineSeries: [] as number[] };
    }
    const dateMap = new Map<string, TrafficChartData>();
    const modelsSet = new Set<string>();
    for (const p of usageData) {
      const model = p.model || 'default';
      modelsSet.add(model);
      const existing = dateMap.get(p.date) || { time: p.date, requests: 0 };
      existing.requests += p.requests;
      existing[model] = (Number(existing[model]) || 0) + p.requests;
      dateMap.set(p.date, existing);
    }
    const series = Array.from(dateMap.values());
    return {
      trafficData: series,
      modelKeys: Array.from(modelsSet),
      sparklineSeries: series.map((item) => item.requests),
    };
  }, [usageData]);

  const modelShareItems: ModelShareItem[] = useMemo(() => {
    if (analyticsData?.models && analyticsData.models.length > 0) {
      return analyticsData.models.map((m) => ({
        name: m.model,
        requests: m.requests,
        tokens: m.tokens,
        costUsd: m.costUsd,
      }));
    }
    if (!usageData || usageData.length === 0) return [];
    const byModel = new Map<string, ModelShareItem>();
    for (const p of usageData) {
      const name = p.model || 'default';
      const existing = byModel.get(name) || { name, requests: 0, tokens: 0, costUsd: 0 };
      existing.requests += p.requests;
      existing.tokens = (existing.tokens ?? 0) + p.tokens;
      existing.costUsd = (existing.costUsd ?? 0) + (p.estimatedCost ?? 0);
      byModel.set(name, existing);
    }
    return Array.from(byModel.values());
  }, [analyticsData, usageData]);

  const activities: RecentActivity[] = useMemo(() => {
    if (!logsData?.data) return [];
    return logsData.data.slice(0, 10).map((log: ApiRequestLog) => ({
      id: log.id,
      time: new Date(log.createdAt).toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      }),
      route: log.clientApp || log.model,
      model: log.model,
      latency: log.latencyMs,
      cost: log.estimatedCost ?? log.costUSD ?? 0,
      status: (log.statusCode < 400 ? 'success' : 'error') as 'success' | 'error',
    }));
  }, [logsData]);

  const healthSignals = useMemo<DashboardHealthSignal[]>(() => {
    const signals: DashboardHealthSignal[] = [];
    for (const p of (healthData ?? []).filter((item) => item.status !== 'healthy').slice(0, 2)) {
      signals.push({
        id: `prov-${p.id}`,
        severity: 'error',
        title: `${p.name} provider ${p.status}`,
        description: `${p.type} pool (${p.credCount} credential${p.credCount !== 1 ? 's' : ''}) requires attention.`,
        chip: p.status.toUpperCase(),
        href: AppRoutes.PROVIDERS,
        cta: 'Inspect',
      });
    }
    for (const log of (logsData?.data ?? []).filter((l) => l.statusCode >= 400 || l.retryCount > 0).slice(0, 2)) {
      if (signals.length >= 3) break;
      signals.push({
        id: `log-${log.id}`,
        severity: log.statusCode >= 400 ? 'error' : 'info',
        title: log.statusCode >= 400 ? `HTTP ${log.statusCode} on ${log.model}` : `Failover retry (${log.retryCount}x) on ${log.model}`,
        description: log.errorMessage
          ? log.errorMessage.slice(0, 80)
          : `Resolved in ${log.latencyMs}ms via ${log.providerName || 'gateway'}.`,
        chip: `${log.latencyMs}ms`,
        href: AppRoutes.LOGS,
        cta: 'View logs',
      });
    }
    if (signals.length === 0 && (healthData?.length ?? 0) > 0) {
      const healthyCount = (healthData ?? []).filter((p) => p.status === 'healthy').length;
      signals.push({
        id: 'all-healthy',
        severity: 'ok',
        title: 'All configured providers healthy',
        description: `${healthyCount} of ${healthData?.length ?? 0} upstream pools are operational.`,
        chip: 'OPERATIONAL',
        href: AppRoutes.PROVIDERS,
        cta: 'Providers',
      });
    }
    const savings = analyticsData?.estimatedSavingsUsd ?? 0;
    if (signals.length < 3 && savings > 0) {
      signals.push({
        id: 'smart-savings',
        severity: 'info',
        title: 'Smart routing cost savings',
        description: `Estimated savings compared against baseline flagship pricing (${currentRangeLabel}).`,
        chip: `$${savings.toFixed(2)} saved`,
        href: AppRoutes.POLICIES,
        cta: 'Policies',
      });
    }
    return signals.slice(0, 3);
  }, [healthData, logsData, analyticsData, currentRangeLabel]);

  const handleRefreshAll = () => {
    refetchStats();
    refetchUsage();
    refetchLogs();
    refetchHealth();
    refetchAnalytics();
    refetchPilot();
    toast.info('Refreshing gateway telemetry');
  };

  const handleExportReport = () => {
    const payload = {
      exportedAt: new Date().toISOString(),
      period: dateRange,
      stats: stats ?? null,
      analytics: analyticsData ?? null,
      providerHealth: healthData ?? [],
      recentLogs: activities,
    };
    const blob = new Blob([JSON.stringify(payload, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `prism-gateway-overview-${dateRange}.json`;
    link.click();
    URL.revokeObjectURL(url);
    toast.success(`Exported prism-gateway-overview-${dateRange}.json`);
  };

  const totalModelReqs = useMemo(
    () => (analyticsData?.models ?? []).reduce((sum, m) => sum + (m.requests || 0), 0),
    [analyticsData]
  );

  return (
    <AppLayout>
      <div className="space-y-6">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h1 className="text-xl sm:text-2xl font-semibold tracking-tight text-foreground">
              API Gateway Overview
            </h1>
            <p className="mt-1 text-xs text-muted-foreground">
              Real-time routing telemetry, token spend, and provider availability
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <Select value={dateRange} onValueChange={setDateRange}>
              <SelectTrigger className="w-[165px] h-9 text-xs bg-card border-border">
                <SelectValue placeholder="Select period" />
              </SelectTrigger>
              <SelectContent>
                {DATE_RANGE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value} className="text-xs">
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Button
              variant="outline"
              size="sm"
              className="h-9 gap-1.5 text-xs bg-card"
              onClick={handleRefreshAll}
              disabled={isFetchingStats}
            >
              <RefreshCw className={`h-3.5 w-3.5 ${isFetchingStats ? 'animate-spin' : ''}`} />
              <span>Refresh</span>
            </Button>

            <Button
              variant="default"
              size="sm"
              className="h-9 gap-1.5 text-xs font-semibold"
              onClick={handleExportReport}
              disabled={isLoading}
            >
              <Download className="h-3.5 w-3.5" />
              <span>Export report</span>
            </Button>
          </div>
        </div>

        {isStatsError && !stats && (
          <ErrorState
            title="Failed to load dashboard metrics"
            description="Could not fetch real-time gateway statistics. Check your network connection or API service status."
            onRetry={handleRefreshAll}
          />
        )}

        <DashboardKpiGrid
          stats={stats}
          analyticsData={analyticsData}
          currentRangeLabel={currentRangeLabel}
          sparklineSeries={sparklineSeries}
          isLoading={isLoading}
        />

        <DashboardTrafficCharts
          dateRange={dateRange}
          onDateRangeChange={setDateRange}
          totalRequests={stats?.totalRequests}
          trafficData={trafficData}
          modelKeys={modelKeys}
          modelShareItems={modelShareItems}
          isAnalyticsLoading={isAnalyticsLoading}
        />

        <DashboardOperationalBreakdown
          selectedDays={selectedDays}
          isAnalyticsLoading={isAnalyticsLoading}
          analyticsData={analyticsData}
          totalModelReqs={totalModelReqs}
          isHealthLoading={isHealthLoading}
          healthData={healthData}
          healthSignals={healthSignals}
        />

        {pilotOutcomes && (
          <DashboardPilotOutcomesCard
            pilotOutcomes={pilotOutcomes}
            pilotWorkspaceId={pilotWorkspaceId}
            onPilotWorkspaceChange={setPilotWorkspaceId}
            workspaces={workspaces}
            selectedDays={selectedDays}
            activePilotWorkspace={activePilotWorkspace}
            onOpenOnboarding={() => setOnboardingOpen(true)}
          />
        )}

        <DashboardRecentActivityCard activities={activities} />

        <ClientOnboardingModal
          open={onboardingOpen}
          onOpenChange={setOnboardingOpen}
          onCompleted={({ workspaceId }) => setPilotWorkspaceId(workspaceId)}
        />
      </div>
    </AppLayout>
  );
}
