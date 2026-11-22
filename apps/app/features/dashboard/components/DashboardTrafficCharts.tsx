'use client';

import Link from 'next/link';
import { ArrowUpRight } from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import {
  LazyTrafficChart,
  LazyModelDonutChart,
  type TrafficChartData,
  type ModelShareItem,
} from '@/components/organisms/ChartContainer';
import { AppRoutes } from '@/constants/routes';

const QUICK_RANGES = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' },
  { value: '90d', label: '90d' },
];

interface DashboardTrafficChartsProps {
  dateRange: string;
  onDateRangeChange: (value: string) => void;
  totalRequests?: number;
  trafficData: TrafficChartData[];
  modelKeys: string[];
  modelShareItems: ModelShareItem[];
  isAnalyticsLoading: boolean;
}

export function DashboardTrafficCharts({
  dateRange,
  onDateRangeChange,
  totalRequests,
  trafficData,
  modelKeys,
  modelShareItems,
  isAnalyticsLoading,
}: DashboardTrafficChartsProps) {
  return (
    <section className="grid gap-5 grid-cols-1 lg:grid-cols-3" aria-label="Traffic charts">
      <Card className="lg:col-span-2">
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between pb-3">
          <div>
            <CardTitle className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              Request Traffic
            </CardTitle>
            <p className="mt-1 font-mono text-sm font-semibold text-foreground tabular-nums">
              {totalRequests != null ? `${totalRequests.toLocaleString()} requests` : 'Loading series'}
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
                  onClick={() => onDateRangeChange(r.value)}
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
  );
}
