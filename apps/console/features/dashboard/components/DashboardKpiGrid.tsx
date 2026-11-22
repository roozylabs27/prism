'use client';

import { MetricCard } from '@/components/molecules/MetricCard';
import { AppRoutes } from '@/constants/routes';
import type { ApiDashboardStats, ApiLogAnalytics } from '@/lib/api';
import { CheckCircle2, DollarSign, Zap } from 'lucide-react';

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

export function DashboardKpiGrid({
  stats,
  analyticsData,
  currentRangeLabel,
  sparklineSeries,
  isLoading,
}: {
  stats?: ApiDashboardStats | null;
  analyticsData?: ApiLogAnalytics | null;
  currentRangeLabel: string;
  sparklineSeries: number[];
  isLoading: boolean;
}) {
  const tokenSubtitle = stats
    ? stats.inputTokens != null && stats.outputTokens != null && (stats.inputTokens > 0 || stats.outputTokens > 0)
      ? `${formatTokens(stats.inputTokens)} in / ${formatTokens(stats.outputTokens)} out`
      : `${formatTokens(stats.totalTokens)} total tokens`
    : undefined;

  return (
    <section className="grid gap-4 grid-cols-1 sm:grid-cols-2 xl:grid-cols-4" aria-label="Gateway KPIs">
      <MetricCard
        featured
        liveLabel="LIVE"
        title={`Total Requests (${currentRangeLabel})`}
        value={stats?.totalRequests != null ? formatCompactCount(stats.totalRequests) : '-'}
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
        value={stats?.totalEstimatedCost != null ? `$${stats.totalEstimatedCost.toFixed(2)}` : '-'}
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
        value={stats && stats.totalRequests > 0 ? `${(100 - stats.errorRate).toFixed(2)}%` : '-'}
        subtitle={
          stats && stats.totalRequests > 0
            ? `Avg ${Math.round(stats.avgLatency)}ms latency · ${stats.errorRate.toFixed(2)}% errors`
            : 'No requests recorded yet'
        }
        icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
        loading={isLoading}
      />
    </section>
  );
}
