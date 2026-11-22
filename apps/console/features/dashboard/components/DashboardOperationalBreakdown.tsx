'use client';

import Link from 'next/link';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { StatusDot, Badge, StatusType } from '@/components/atoms/Badge';
import { MODEL_PALETTE } from '@/components/organisms/ChartContainer';
import { EmptyState } from '@/components/molecules/StateAlerts';
import { AppRoutes } from '@/constants/routes';
import type { ApiLogAnalytics, ApiProviderHealth } from '@/lib/api';
import { Activity, AlertCircle, ArrowUpRight, Cpu, Server, ShieldCheck } from 'lucide-react';

export interface DashboardHealthSignal {
  id: string;
  severity: 'error' | 'info' | 'ok';
  title: string;
  description: string;
  chip: string;
  href: string;
  cta: string;
}

function providerStatusToDot(status: ApiProviderHealth['status']): StatusType {
  if (status === 'down') return 'disabled';
  return status;
}

interface DashboardOperationalBreakdownProps {
  selectedDays: number;
  isAnalyticsLoading: boolean;
  analyticsData?: ApiLogAnalytics | null;
  totalModelReqs: number;
  isHealthLoading: boolean;
  healthData?: ApiProviderHealth[];
  healthSignals: DashboardHealthSignal[];
  canViewProviderHealth?: boolean;
}

export function DashboardOperationalBreakdown({
  selectedDays,
  isAnalyticsLoading,
  analyticsData,
  totalModelReqs,
  isHealthLoading,
  healthData,
  healthSignals,
  canViewProviderHealth = true,
}: DashboardOperationalBreakdownProps) {
  return (
    <section className="grid gap-5 grid-cols-1 lg:grid-cols-3" aria-label="Operational breakdown">
      {/* Col 1: Model Cost & Latency */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-3">
          <CardTitle className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
            <Cpu className="h-3.5 w-3.5 text-prism-violet" />
            <span>Model Cost &amp; Latency</span>
          </CardTitle>
          <span className="font-mono text-[11px] text-muted-foreground">
            {selectedDays === 0 ? 'All Time' : `${selectedDays}d`}
          </span>
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
                    <span className="font-mono font-semibold text-foreground truncate">{m.model}</span>
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
          {!canViewProviderHealth ? (
            <EmptyState
              title="Provider availability restricted"
              description="Your role does not include model:read access to inspect upstream provider pools."
              className="p-6"
            />
          ) : isHealthLoading ? (
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
                  <span className="truncate text-xs font-semibold text-foreground">{provider.name}</span>
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
                <p className="text-[11px] leading-relaxed text-muted-foreground">{sig.description}</p>
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
  );
}
