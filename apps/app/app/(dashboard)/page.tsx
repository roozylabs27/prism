'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import { AppLayout } from '@/components/AppLayout';
import { MetricCard } from '@/components/molecules/MetricCard';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { StatusDot, Badge, StatusType } from '@/components/atoms/Badge';
import { DataTable, Column } from '@/components/organisms/DataTable';
import {
  LazyTrafficChart,
  LazyModelDonutChart,
  TrafficChartData,
  ModelShareItem,
  MODEL_PALETTE,
} from '@/components/organisms/ChartContainer';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import {
  useDashboardStatsQuery,
  useUsageChartQuery,
  useDashboardHealthQuery,
} from '@/hooks/queries/useDashboardQuery';
import { useLogsQuery, useLogAnalyticsQuery } from '@/hooks/queries/useLogsQuery';
import { ApiRequestLog, ApiProviderHealth } from '@/lib/api';
import { AppRoutes } from '@/constants/routes';
import {
  Activity,
  Zap,
  DollarSign,
  CheckCircle2,
  Server,
  Download,
  RefreshCw,
  ArrowUpRight,
  AlertCircle,
  ShieldCheck,
  Cpu,
} from 'lucide-react';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/molecules/Select';
import { toast } from 'sonner';

function providerStatusToDot(status: ApiProviderHealth['status']): StatusType {
  if (status === 'down') return 'disabled';
  return status;
}

interface RecentActivity {
  id: string;
  time: string;
  route: string;
  model: string;
  latency: number;
  cost: number;
  status: 'success' | 'error';
}

const DATE_RANGE_OPTIONS = [
  { value: '24h', label: 'Last 24 Hours', days: 1 },
  { value: '7d', label: 'Last 7 Days', days: 7 },
  { value: '30d', label: 'Last 30 Days', days: 30 },
  { value: '90d', label: 'Last 90 Days', days: 90 },
  { value: 'all', label: 'All Time', days: 90 },
];

const QUICK_RANGES = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
];

function formatCompactCount(num: number): string {
  if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(2)}M`;
  if (num >= 10_000) return `${(num / 1_000).toFixed(1)}K`;
  return num.toLocaleString();
}

function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)}K`;
  return tokens.toLocaleString();
}

export default function DashboardPage() {
  const [dateRange, setDateRange] = useState<string>('30d');
  const selectedDays = DATE_RANGE_OPTIONS.find((opt) => opt.value === dateRange)?.days ?? 30;

  const {
    data: stats,
    isLoading,
    isError: isStatsError,
    refetch: refetchStats,
    isFetching: isFetchingStats,
  } = useDashboardStatsQuery(dateRange);
  const { data: usageData, refetch: refetchUsage } = useUsageChartQuery(dateRange);
  const { data: logsData, refetch: refetchLogs } = useLogsQuery({ limit: 10 });
  const {
    data: healthData,
    isLoading: isHealthLoading,
    refetch: refetchHealth,
  } = useDashboardHealthQuery();
  const { data: analyticsData, isLoading: isAnalyticsLoading, refetch: refetchAnalytics } = useLogAnalyticsQuery(selectedDays);

  const rangeLabelMap: Record<string, string> = {
    '24h': '24h',
    '7d': '7d',
    '30d': '30d',
    '90d': '90d',
    all: 'All Time',
  };
  const currentRangeLabel = rangeLabelMap[dateRange] || '30d';

  const { trafficData, modelKeys, sparklineSeries } = useMemo(() => {
    if (!usageData || usageData.length === 0) {
      return {
        trafficData: [] as TrafficChartData[],
        modelKeys: [] as string[],
        sparklineSeries: [] as number[],
      };
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

  // Prefer LogAnalytics model breakdown, fallback to aggregating usageData points
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
    return logsData.data.slice(0, 10).map((log: ApiRequestLog) => {
      const d = new Date(log.createdAt);
      const formattedTime = d.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      });
      return {
        id: log.id,
        time: formattedTime,
        route: log.clientApp || log.model,
        model: log.model,
        latency: log.latencyMs,
        cost: log.estimatedCost ?? log.costUSD ?? 0,
        status: (log.statusCode < 400 ? 'success' : 'error') as 'success' | 'error',
      };
    });
  }, [logsData]);

  // Build real routing & health signals from live provider health and recent request logs
  const healthSignals = useMemo(() => {
    const signals: Array<{
      id: string;
      severity: 'error' | 'info' | 'ok';
      title: string;
      description: string;
      chip: string;
      href: string;
      cta: string;
    }> = [];

    const degradedProviders = (healthData ?? []).filter((p) => p.status !== 'healthy');
    for (const p of degradedProviders.slice(0, 2)) {
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

    const erroredOrRetried = (logsData?.data ?? []).filter(
      (l) => l.statusCode >= 400 || l.retryCount > 0
    );
    for (const log of erroredOrRetried.slice(0, 2)) {
      if (signals.length >= 3) break;
      signals.push({
        id: `log-${log.id}`,
        severity: log.statusCode >= 400 ? 'error' : 'info',
        title:
          log.statusCode >= 400
            ? `HTTP ${log.statusCode} on ${log.model}`
            : `Failover retry (${log.retryCount}x) on ${log.model}`,
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
        description: `Estimated savings compared against baseline flagship pricing (${selectedDays}d).`,
        chip: `$${savings.toFixed(2)} saved`,
        href: AppRoutes.POLICIES,
        cta: 'Policies',
      });
    }

    return signals.slice(0, 3);
  }, [healthData, logsData, analyticsData, selectedDays]);

  const activityColumns: Column<RecentActivity>[] = [
    {
      title: 'Time & Date',
      dataIndex: 'time',
      key: 'time',
      render: (time) => (
        <span className="font-mono text-muted-foreground whitespace-nowrap text-xs">
          {String(time ?? '')}
        </span>
      ),
    },
    {
      title: 'Route Policy / App',
      dataIndex: 'route',
      key: 'route',
      render: (route) => (
        <span className="font-mono text-foreground font-medium text-xs truncate max-w-[150px] block">
          {String(route ?? '')}
        </span>
      ),
    },
    {
      title: 'Resolved Model',
      dataIndex: 'model',
      key: 'model',
      render: (model) => (
        <Badge variant="outline" className="font-mono text-[11px]">
          {String(model ?? '')}
        </Badge>
      ),
    },
    {
      title: 'Latency',
      dataIndex: 'latency',
      key: 'latency',
      render: (lat) => (
        <span className="font-mono text-xs tabular-nums whitespace-nowrap">
          {String(lat ?? 0)} ms
        </span>
      ),
    },
    {
      title: 'Cost',
      dataIndex: 'cost',
      key: 'cost',
      render: (cost) => (
        <span className="font-mono text-emerald-700 dark:text-emerald-400 text-xs tabular-nums whitespace-nowrap">
          ${Number(cost || 0).toFixed(4)}
        </span>
      ),
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Badge
          variant={status === 'success' ? 'success' : 'destructive'}
          className="text-[10px] uppercase font-bold"
        >
          {String(status ?? '').toUpperCase()}
        </Badge>
      ),
    },
  ];

  const tokenSubtitle = stats
    ? stats.inputTokens != null &&
      stats.outputTokens != null &&
      (stats.inputTokens > 0 || stats.outputTokens > 0)
      ? `${formatTokens(stats.inputTokens)} in / ${formatTokens(stats.outputTokens)} out`
      : `${formatTokens(stats.totalTokens)} total tokens`
    : undefined;

  const handleRefreshAll = () => {
    refetchStats();
    refetchUsage();
    refetchLogs();
    refetchHealth();
    refetchAnalytics();
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
        {/* Page Header + Controls */}
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

        {/* Row 1: 4 KPI Cards (Hero Card + 3 Supporting KPIs) */}
        <section className="grid gap-4 grid-cols-1 sm:grid-cols-2 xl:grid-cols-4" aria-label="Gateway KPIs">
          <MetricCard
            featured
            liveLabel="LIVE"
            title={`Total Requests (${currentRangeLabel})`}
            value={
              stats?.totalRequests != null
                ? formatCompactCount(stats.totalRequests)
                : '-'
            }
            subtitle={
              stats
                ? `${stats.activeKeys} active key${stats.activeKeys !== 1 ? 's' : ''} · ${stats.activeProviders} provider${stats.activeProviders !== 1 ? 's' : ''}`
                : `selected ${currentRangeLabel} window`
            }
            sparkline={sparklineSeries}
            actionHref={AppRoutes.LOGS}
            actionLabel="Open request logs"
            loading={isLoading}
          />

          <MetricCard
            title="Tokens Processed"
            value={
              stats?.totalTokens != null
                ? stats.totalTokens === 0
                  ? '0'
                  : formatTokens(stats.totalTokens)
                : '-'
            }
            subtitle={tokenSubtitle}
            icon={<Zap className="h-4 w-4 text-prism-cyan" />}
            loading={isLoading}
          />

          <MetricCard
            title="Gateway Spend"
            value={
              stats?.totalEstimatedCost != null
                ? `$${stats.totalEstimatedCost.toFixed(2)}`
                : '-'
            }
            delta={
              analyticsData && analyticsData.estimatedSavingsUsd && analyticsData.estimatedSavingsUsd > 0
                ? `Saved $${analyticsData.estimatedSavingsUsd.toFixed(2)}`
                : undefined
            }
            deltaType="positive"
            subtitle="via Smart Routing"
            icon={<DollarSign className="h-4 w-4 text-emerald-500" />}
            loading={isLoading}
          />

          <MetricCard
            title="Success Rate"
            value={
              stats && stats.totalRequests > 0
                ? `${(100 - stats.errorRate).toFixed(2)}%`
                : '-'
            }
            subtitle={
              stats && stats.totalRequests > 0
                ? `Avg ${Math.round(stats.avgLatency)}ms latency · ${stats.errorRate.toFixed(2)}% errors`
                : 'No requests recorded yet'
            }
            icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
            loading={isLoading}
          />
        </section>

        {/* Row 2: 2:1 Asymmetric Charts (Request Traffic + Traffic by Model Donut) */}
        <section className="grid gap-5 grid-cols-1 lg:grid-cols-3" aria-label="Traffic charts">
          <Card className="lg:col-span-2">
            <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between pb-3">
              <div>
                <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  Request Traffic
                </CardTitle>
                <p className="mt-1 font-mono text-sm font-semibold text-foreground tabular-nums">
                  {stats?.totalRequests != null
                    ? `${stats.totalRequests.toLocaleString()} requests`
                    : 'Loading series'}
                  <span className="ml-2 font-sans text-xs font-normal text-muted-foreground">
                    {dateRange === '24h' ? 'per hour' : 'per day'}
                  </span>
                </p>
              </div>

              <div className="flex items-center gap-2">
                <div
                  className="inline-flex rounded-lg border border-border bg-muted p-0.5"
                  role="group"
                  aria-label="Chart range quick selector"
                >
                  {QUICK_RANGES.map((r) => (
                    <button
                      key={r.value}
                      type="button"
                      onClick={() => setDateRange(r.value)}
                      className={`rounded-md px-2.5 py-1 text-[11px] font-medium transition-colors cursor-pointer ${
                        dateRange === r.value
                          ? 'bg-card text-foreground shadow-sm border border-border'
                          : 'text-muted-foreground hover:text-foreground'
                      }`}
                    >
                      {r.label}
                    </button>
                  ))}
                </div>

                <Link
                  href={AppRoutes.LOGS}
                  className="inline-flex h-7 items-center gap-1 rounded-md border border-border bg-background px-2.5 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
                >
                  <span>View logs</span>
                  <ArrowUpRight className="h-3 w-3" />
                </Link>
              </div>
            </CardHeader>

            <CardContent>
              <LazyTrafficChart data={trafficData} modelKeys={modelKeys} height={240} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-3">
              <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Traffic by Model
              </CardTitle>
              <Link
                href={AppRoutes.MODELS}
                className="inline-flex h-7 items-center gap-1 rounded-md border border-border bg-background px-2.5 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                <span>Models</span>
                <ArrowUpRight className="h-3 w-3" />
              </Link>
            </CardHeader>
            <CardContent>
              <LazyModelDonutChart items={modelShareItems} loading={isAnalyticsLoading && modelShareItems.length === 0} />
            </CardContent>
          </Card>
        </section>

        {/* Row 3: 3-Column Operational Breakdown */}
        <section className="grid gap-5 grid-cols-1 lg:grid-cols-3" aria-label="Operational breakdown">
          {/* Col 1: Model Cost & Latency */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-3">
              <CardTitle className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <Cpu className="h-3.5 w-3.5 text-prism-violet" />
                <span>Model Cost &amp; Latency</span>
              </CardTitle>
              <span className="font-mono text-[11px] text-muted-foreground">{selectedDays}d</span>
            </CardHeader>
            <CardContent className="space-y-3">
              {isAnalyticsLoading ? (
                <div className="space-y-2.5">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="h-12 animate-pulse rounded-lg bg-muted/40" />
                  ))}
                </div>
              ) : analyticsData?.models && analyticsData.models.length > 0 ? (
                analyticsData.models.slice(0, 4).map((m, idx) => {
                  const pct = totalModelReqs > 0 ? Math.round((m.requests / totalModelReqs) * 100) : 0;
                  const barColor = MODEL_PALETTE[idx % MODEL_PALETTE.length];
                  return (
                    <div
                      key={m.model}
                      className="rounded-lg border border-border/80 bg-background/50 p-2.5 space-y-1.5"
                    >
                      <div className="flex items-center justify-between gap-2 text-xs">
                        <span className="font-mono font-semibold text-foreground truncate">
                          {m.model}
                        </span>
                        <span className="font-mono text-[11px] font-semibold tabular-nums text-emerald-700 dark:text-emerald-400 shrink-0">
                          ${m.costUsd.toFixed(3)}
                        </span>
                      </div>
                      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                        <div
                          className="h-full rounded-full"
                          style={{ width: `${Math.max(pct, 4)}%`, backgroundColor: barColor }}
                        />
                      </div>
                      <div className="flex items-center justify-between text-[11px] font-mono text-muted-foreground tabular-nums">
                        <span>{m.requests.toLocaleString()} req</span>
                        <span>
                          avg {Math.round(m.avgLatencyMs)}ms
                          {m.avgTtftMs ? ` · ttft ${Math.round(m.avgTtftMs)}ms` : ''}
                        </span>
                      </div>
                    </div>
                  );
                })
              ) : (
                <EmptyState
                  title="No model analytics yet"
                  description="Per-model latency and USD breakdown populate after requests complete."
                  className="p-6"
                />
              )}
            </CardContent>
          </Card>

          {/* Col 2: Provider Availability */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-3">
              <CardTitle className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <Server className="h-3.5 w-3.5 text-prism-cyan" />
                <span>Provider Availability</span>
              </CardTitle>
              <Link
                href={AppRoutes.PROVIDERS}
                className="inline-flex h-7 items-center gap-1 rounded-md border border-border bg-background px-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                <span>Manage</span>
                <ArrowUpRight className="h-3 w-3" />
              </Link>
            </CardHeader>
            <CardContent className="space-y-2.5">
              {isHealthLoading ? (
                <div className="space-y-2.5">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="h-11 animate-pulse rounded-lg bg-muted/40" />
                  ))}
                </div>
              ) : healthData && healthData.length > 0 ? (
                healthData.map((provider: ApiProviderHealth) => (
                  <div
                    key={provider.id || provider.name}
                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-1.5 rounded-lg border border-border/80 bg-background/50 px-3 py-2.5"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <StatusDot status={providerStatusToDot(provider.status)} />
                      <span className="truncate text-xs font-semibold text-foreground">
                        {provider.name}
                      </span>
                    </div>
                    <span
                      className={`font-mono text-[11px] shrink-0 ${
                        provider.status === 'degraded'
                          ? 'text-amber-600 dark:text-amber-400'
                          : 'text-muted-foreground'
                      }`}
                    >
                      {provider.type} ({provider.credCount} cred{provider.credCount !== 1 ? 's' : ''})
                    </span>
                  </div>
                ))
              ) : (
                <EmptyState
                  title="No providers configured"
                  description="Connect an upstream provider such as OpenAI, Anthropic, or Gemini."
                  className="p-6"
                />
              )}
            </CardContent>
          </Card>

          {/* Col 3: Routing & Health Signals */}
          <Card>
            <CardHeader className="flex flex-row items-center justify-between pb-3">
              <CardTitle className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                <Activity className="h-3.5 w-3.5 text-prism-violet" />
                <span>Routing &amp; Health Feed</span>
              </CardTitle>
              <Link
                href={AppRoutes.POLICIES}
                className="inline-flex h-7 items-center gap-1 rounded-md border border-border bg-background px-2 text-[11px] font-medium text-muted-foreground transition-colors hover:text-foreground"
              >
                <span>Policies</span>
                <ArrowUpRight className="h-3 w-3" />
              </Link>
            </CardHeader>
            <CardContent className="space-y-2.5">
              {healthSignals.length > 0 ? (
                healthSignals.map((sig) => (
                  <div
                    key={sig.id}
                    className="rounded-lg border border-border/80 bg-background/50 p-3 space-y-1.5"
                  >
                    <div className="flex items-center gap-2 text-xs font-semibold text-foreground">
                      {sig.severity === 'error' ? (
                        <AlertCircle className="h-3.5 w-3.5 shrink-0 text-rose-500" />
                      ) : sig.severity === 'ok' ? (
                        <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-emerald-500" />
                      ) : (
                        <Activity className="h-3.5 w-3.5 shrink-0 text-prism-cyan" />
                      )}
                      <span className="truncate">{sig.title}</span>
                    </div>
                    <p className="text-[11px] leading-relaxed text-muted-foreground">
                      {sig.description}
                    </p>
                    <div className="flex items-center justify-between pt-0.5">
                      <Badge
                        variant={
                          sig.severity === 'error'
                            ? 'destructive'
                            : sig.severity === 'ok'
                              ? 'success'
                              : 'violet'
                        }
                        className="font-mono text-[10px]"
                      >
                        {sig.chip}
                      </Badge>
                      <Link
                        href={sig.href}
                        className="text-[11px] font-semibold text-prism-violet hover:underline underline-offset-4"
                      >
                        {sig.cta} →
                      </Link>
                    </div>
                  </div>
                ))
              ) : (
                <EmptyState
                  title="No routing events yet"
                  description="Provider health alerts, failover retries, and savings summaries appear here."
                  className="p-6"
                />
              )}
            </CardContent>
          </Card>
        </section>

        {/* Row 4: Recent AI Activity Table */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle className="flex items-center gap-2 text-sm font-semibold">
              <Activity className="h-4 w-4 text-prism-violet" />
              <span>Recent AI Gateway Activity Logs</span>
            </CardTitle>
            <Link
              href={AppRoutes.LOGS}
              className="inline-flex items-center gap-1 text-xs font-medium text-prism-violet hover:underline underline-offset-4"
            >
              <span>All logs</span>
              <ArrowUpRight className="h-3.5 w-3.5" />
            </Link>
          </CardHeader>
          <CardContent className="overflow-x-auto">
            <DataTable
              dataSource={activities}
              columns={activityColumns}
              rowKey="id"
              pageSize={10}
              searchPlaceholder="Filter activity logs..."
              emptyText="No gateway requests recorded yet."
            />
          </CardContent>
        </Card>
      </div>
    </AppLayout>
  );
}
