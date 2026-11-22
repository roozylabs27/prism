'use client';

import Link from 'next/link';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Badge } from '@/components/atoms/Badge';
import { DataTable, type Column } from '@/components/organisms/DataTable';
import { AppRoutes } from '@/constants/routes';
import { Activity, ArrowUpRight } from 'lucide-react';

export interface RecentActivity {
  id: string;
  time: string;
  route: string;
  model: string;
  latency: number;
  cost: number;
  status: 'success' | 'error';
}

const ACTIVITY_COLUMNS: Column<RecentActivity>[] = [
  {
    title: 'Time & Date',
    dataIndex: 'time',
    key: 'time',
    render: (time) => (
      <span className="font-mono text-muted-foreground whitespace-nowrap text-xs">{String(time ?? '')}</span>
    ),
  },
  {
    title: 'Client App / Model',
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
      <span className="font-mono text-xs tabular-nums whitespace-nowrap">{String(lat ?? 0)} ms</span>
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

export function DashboardRecentActivityCard({ activities }: { activities: RecentActivity[] }) {
  return (
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
          columns={ACTIVITY_COLUMNS}
          rowKey="id"
          pageSize={10}
          searchPlaceholder="Filter activity logs..."
          emptyText="No gateway requests recorded yet."
        />
      </CardContent>
    </Card>
  );
}
