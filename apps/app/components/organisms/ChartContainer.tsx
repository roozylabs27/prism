'use client';

import { useMemo } from 'react';
import dynamic from 'next/dynamic';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts';
import { Activity, Layers } from 'lucide-react';

export interface TrafficChartData {
  time: string;
  requests: number;
  [key: string]: string | number | undefined;
}

export interface TrafficChartProps {
  data: TrafficChartData[];
  modelKeys?: string[];
  height?: number;
}

export interface ModelShareItem {
  name: string;
  requests: number;
  tokens?: number;
  costUsd?: number;
}

export interface ModelDonutChartProps {
  items: ModelShareItem[];
  loading?: boolean;
}

export const MODEL_PALETTE = [
  '#8B5CF6', // Prism Violet
  '#06B6D4', // Prism Cyan
  '#10B981', // Emerald
  '#F59E0B', // Amber
  '#EC4899', // Pink
  '#3B82F6', // Blue
  '#A78BFA', // Soft Violet
  '#22D3EE', // Bright Cyan
];

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name?: string;
    dataKey?: string;
    value?: number | string;
    color?: string;
    payload?: TrafficChartData;
  }>;
  label?: string;
  modelKeys?: string[];
}

const CustomTooltip = ({ active, payload, label, modelKeys = [] }: CustomTooltipProps) => {
  if (active && payload && payload.length) {
    const rawData = payload[0].payload;
    const totalRequests = rawData?.requests ?? 0;

    return (
      <div className="min-w-[190px] rounded-lg border border-border bg-popover px-3 py-2.5 shadow-xl">
        <div className="mb-2 flex items-center justify-between gap-4 border-b border-border pb-1.5 text-[11px] font-mono text-muted-foreground">
          <span className="font-semibold text-foreground">{label}</span>
          <span className="font-bold tabular-nums text-foreground">
            {Number(totalRequests).toLocaleString()} req
          </span>
        </div>

        {modelKeys.length > 0 ? (
          <div className="space-y-1.5 text-xs font-mono">
            {modelKeys.map((model, idx) => {
              const reqCount = Number(rawData?.[model]) || 0;
              if (reqCount === 0 && modelKeys.length > 1) return null;
              const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
              return (
                <div key={model} className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-1.5 truncate max-w-[140px]">
                    <span className="inline-block h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                    <span className="truncate text-muted-foreground">{model}</span>
                  </div>
                  <span className="font-semibold tabular-nums text-foreground">
                    {reqCount.toLocaleString()}
                  </span>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="flex items-center justify-between gap-4 text-xs font-mono">
            <span className="text-muted-foreground">Requests:</span>
            <span className="font-bold tabular-nums text-foreground">
              {Number(totalRequests).toLocaleString()}
            </span>
          </div>
        )}
      </div>
    );
  }
  return null;
};

function TrafficChartInner({ data, modelKeys = [], height = 240 }: TrafficChartProps) {
  const hasData = Array.isArray(data) && data.length > 0 && data.some((d) => d.requests > 0);

  const activeModelKeys = useMemo(() => {
    if (modelKeys && modelKeys.length > 0) return modelKeys;
    if (!data || data.length === 0) return [];
    const keys = new Set<string>();
    for (const d of data) {
      for (const k of Object.keys(d)) {
        if (k !== 'time' && k !== 'requests' && typeof d[k] === 'number') {
          keys.add(k);
        }
      }
    }
    return Array.from(keys);
  }, [data, modelKeys]);

  if (!hasData) {
    return (
      <div
        style={{ width: '100%', height }}
        className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border bg-muted/15 p-6 text-center"
      >
        <Activity className="mb-2.5 h-7 w-7 text-muted-foreground/50" />
        <p className="text-xs font-semibold text-foreground">No Request Traffic Recorded</p>
        <p className="mt-1 max-w-xs text-[11px] leading-relaxed text-muted-foreground">
          Traffic metrics appear here once requests are dispatched through your Gateway API Keys.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3" style={{ width: '100%' }}>
      {activeModelKeys.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 px-0.5">
          {activeModelKeys.map((model, idx) => {
            const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
            const totalForModel = data.reduce((acc, curr) => acc + (Number(curr[model]) || 0), 0);
            return (
              <div
                key={model}
                className="inline-flex items-center gap-1.5 rounded-md border border-border bg-background/60 px-2 py-1 text-[11px] font-mono"
              >
                <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                <span className="font-medium text-foreground">{model}</span>
                <span className="tabular-nums text-muted-foreground">({totalForModel.toLocaleString()})</span>
              </div>
            );
          })}
        </div>
      )}

      <div style={{ width: '100%', height }}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 10, right: 10, left: -18, bottom: 0 }}>
            <defs>
              {activeModelKeys.length > 0 ? (
                activeModelKeys.map((model, idx) => {
                  const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
                  return (
                    <linearGradient key={`grad-${model}`} id={`grad-${model}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor={color} stopOpacity={0.35} />
                      <stop offset="95%" stopColor={color} stopOpacity={0.0} />
                    </linearGradient>
                  );
                })
              ) : (
                <linearGradient id="violetGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8B5CF6" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#8B5CF6" stopOpacity={0} />
                </linearGradient>
              )}
            </defs>

            <XAxis
              dataKey="time"
              stroke="currentColor"
              className="font-mono text-[10px] text-muted-foreground"
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              stroke="currentColor"
              className="font-mono text-[10px] text-muted-foreground"
              tickLine={false}
              axisLine={false}
              allowDecimals={false}
            />
            <Tooltip
              content={<CustomTooltip modelKeys={activeModelKeys} />}
              cursor={{ stroke: 'rgba(139, 92, 246, 0.45)', strokeWidth: 1, strokeDasharray: '4 4' }}
            />

            {activeModelKeys.length > 0 ? (
              activeModelKeys.map((model, idx) => {
                const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
                return (
                  <Area
                    key={model}
                    type="monotone"
                    dataKey={model}
                    name={model}
                    stroke={color}
                    strokeWidth={2}
                    fillOpacity={1}
                    fill={`url(#grad-${model})`}
                    activeDot={{ r: 4, stroke: color, strokeWidth: 2, fill: 'hsl(var(--card))' }}
                  />
                );
              })
            ) : (
              <Area
                type="monotone"
                dataKey="requests"
                stroke="#8B5CF6"
                strokeWidth={2}
                fillOpacity={1}
                fill="url(#violetGradient)"
                activeDot={{ r: 4, stroke: '#8B5CF6', strokeWidth: 2, fill: 'hsl(var(--card))' }}
              />
            )}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

function ModelDonutChartInner({ items, loading = false }: ModelDonutChartProps) {
  const total = useMemo(
    () => items.reduce((sum, item) => sum + (item.requests || 0), 0),
    [items]
  );

  const topItems = useMemo(() => {
    const sorted = [...items].filter((i) => i.requests > 0).sort((a, b) => b.requests - a.requests);
    if (sorted.length <= 4) return sorted;
    const head = sorted.slice(0, 3);
    const rest = sorted.slice(3).reduce(
      (acc, cur) => ({
        name: 'Other models',
        requests: acc.requests + cur.requests,
        costUsd: (acc.costUsd ?? 0) + (cur.costUsd ?? 0),
      }),
      { name: 'Other models', requests: 0, costUsd: 0 }
    );
    return [...head, rest];
  }, [items]);

  if (loading) {
    return (
      <div className="flex h-[240px] flex-col items-center justify-center gap-3">
        <div className="h-32 w-32 animate-pulse rounded-full border-8 border-muted" />
        <div className="h-3 w-36 animate-pulse rounded bg-muted" />
      </div>
    );
  }

  if (topItems.length === 0 || total === 0) {
    return (
      <div className="flex h-[240px] flex-col items-center justify-center rounded-xl border border-dashed border-border bg-muted/15 p-5 text-center">
        <Layers className="mb-2 h-6 w-6 text-muted-foreground/50" />
        <p className="text-xs font-semibold text-foreground">No Model Distribution Yet</p>
        <p className="mt-1 text-[11px] text-muted-foreground">
          Model share appears once requests resolve through Prism.
        </p>
      </div>
    );
  }

  return (
    <div className="flex flex-col items-center gap-4">
      <div className="relative h-[156px] w-[156px]">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={topItems}
              dataKey="requests"
              nameKey="name"
              cx="50%"
              cy="50%"
              innerRadius={48}
              outerRadius={68}
              paddingAngle={3}
              stroke="none"
            >
              {topItems.map((entry, idx) => (
                <Cell key={entry.name} fill={MODEL_PALETTE[idx % MODEL_PALETTE.length]} />
              ))}
            </Pie>
            <Tooltip
              formatter={(value: number) => [`${Number(value).toLocaleString()} req`, 'Traffic']}
              contentStyle={{
                backgroundColor: 'hsl(var(--popover))',
                borderColor: 'hsl(var(--border))',
                borderRadius: '8px',
                fontSize: '12px',
                color: 'hsl(var(--popover-foreground))',
              }}
            />
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            Total
          </span>
          <span className="font-mono text-base font-semibold tabular-nums text-foreground">
            {total >= 1_000_000
              ? `${(total / 1_000_000).toFixed(2)}M`
              : total >= 1_000
                ? `${(total / 1_000).toFixed(1)}K`
                : total.toLocaleString()}
          </span>
        </div>
      </div>

      <div className="w-full space-y-2 border-t border-border/70 pt-3">
        {topItems.map((item, idx) => {
          const pct = total > 0 ? ((item.requests / total) * 100).toFixed(1) : '0.0';
          const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
          return (
            <div key={item.name} className="flex items-center justify-between gap-2 text-xs">
              <div className="flex items-center gap-2 min-w-0">
                <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                <span className="truncate font-mono text-[11px] font-medium text-foreground">
                  {item.name}
                </span>
              </div>
              <div className="flex items-center gap-2.5 shrink-0 font-mono text-[11px] tabular-nums">
                <span className="font-semibold text-foreground">{item.requests.toLocaleString()}</span>
                <span className="w-11 text-right text-muted-foreground">{pct}%</span>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export const LazyTrafficChart = dynamic(() => Promise.resolve(TrafficChartInner), {
  ssr: false,
  loading: () => <div className="h-[240px] w-full animate-pulse rounded-xl bg-muted/40" />,
});

export const LazyModelDonutChart = dynamic(() => Promise.resolve(ModelDonutChartInner), {
  ssr: false,
  loading: () => <div className="h-[240px] w-full animate-pulse rounded-xl bg-muted/40" />,
});
