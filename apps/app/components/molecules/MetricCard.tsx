import type { ReactNode } from 'react';
import Link from 'next/link';
import { ArrowRight } from 'lucide-react';
import { Card, CardContent } from '@/components/molecules/Card';
import { cn } from '@/lib/utils';

export interface MetricCardProps {
  title: string;
  value: string | number;
  delta?: string;
  deltaType?: 'positive' | 'negative' | 'neutral';
  subtitle?: string;
  icon?: ReactNode;
  featured?: boolean;
  liveLabel?: string;
  sparkline?: number[];
  actionHref?: string;
  actionLabel?: string;
  className?: string;
  loading?: boolean;
}

function SparklineSvg({ values }: { values: number[] }) {
  if (values.length < 2) return null;
  const width = 92;
  const height = 34;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const range = max - min || 1;

  const coords = values.map((v, i) => {
    const x = 2 + (i / (values.length - 1)) * (width - 6);
    const y = height - 4 - ((v - min) / range) * (height - 9);
    return { x: Number(x.toFixed(1)), y: Number(y.toFixed(1)) };
  });

  const linePath = coords.map((p, idx) => `${idx === 0 ? 'M' : 'L'}${p.x} ${p.y}`).join(' ');
  const areaPath = `${linePath} L${coords[coords.length - 1].x} ${height} L${coords[0].x} ${height} Z`;
  const last = coords[coords.length - 1];

  return (
    <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="shrink-0" aria-hidden="true">
      <defs>
        <linearGradient id="prismHeroSparkGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#22D3EE" stopOpacity="0.34" />
          <stop offset="100%" stopColor="#8B5CF6" stopOpacity="0.0" />
        </linearGradient>
      </defs>
      <path d={areaPath} fill="url(#prismHeroSparkGrad)" />
      <path d={linePath} fill="none" stroke="#22D3EE" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx={last.x} cy={last.y} r="2.75" fill="#22D3EE" />
    </svg>
  );
}

export function MetricCard({
  title,
  value,
  delta,
  deltaType = 'positive',
  subtitle,
  icon,
  featured = false,
  liveLabel,
  sparkline,
  actionHref,
  actionLabel = 'View details',
  className,
  loading = false,
}: MetricCardProps) {
  const hasFooter = Boolean(subtitle || delta || actionHref);

  return (
    <Card
      className={cn(
        'overflow-hidden transition-colors hover:border-border/90',
        featured && 'hero-card-wash border-violet-500/30 dark:border-violet-400/25',
        className
      )}
    >
      <CardContent className="flex h-full flex-col justify-between gap-3 p-4 sm:p-5">
        <div className="space-y-2.5">
          <div className="flex items-center justify-between gap-2">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-muted-foreground">
              {title}
            </span>
            <div className="flex items-center gap-2">
              {liveLabel && (
                <span className="inline-flex items-center gap-1.5 rounded-full border border-violet-500/30 bg-violet-500/10 px-2 py-0.5 font-mono text-[10px] font-semibold text-prism-violet">
                  <span className="h-1.5 w-1.5 rounded-full bg-cyan-400" />
                  {liveLabel}
                </span>
              )}
              {icon && <div className="text-muted-foreground/80">{icon}</div>}
            </div>
          </div>

          <div className="flex items-end justify-between gap-3">
            <div className="font-mono text-2xl font-semibold tracking-tight tabular-nums text-foreground">
              {loading ? <span className="animate-pulse text-muted-foreground">...</span> : value}
            </div>
            {!loading && sparkline && sparkline.length >= 2 && <SparklineSvg values={sparkline} />}
          </div>
        </div>

        {hasFooter && (
          <div className="flex items-center justify-between gap-2 border-t border-border/70 pt-2.5">
            <div className="flex items-center gap-1.5 min-w-0">
              {delta && (
                <span
                  className={cn(
                    'shrink-0 rounded-md px-1.5 py-0.5 font-mono text-[11px] font-semibold tabular-nums',
                    deltaType === 'positive' && 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
                    deltaType === 'negative' && 'bg-rose-500/10 text-rose-700 dark:text-rose-400',
                    deltaType === 'neutral' && 'bg-muted text-muted-foreground'
                  )}
                >
                  {delta}
                </span>
              )}
              {subtitle && <p className="truncate text-[11px] text-muted-foreground">{subtitle}</p>}
            </div>

            {actionHref && (
              <Link
                href={actionHref}
                aria-label={actionLabel}
                className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-violet-500/30 bg-violet-500/10 text-prism-violet transition-colors hover:bg-violet-500 hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
