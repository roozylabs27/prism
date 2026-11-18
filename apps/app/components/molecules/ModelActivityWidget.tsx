'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { Layers, Cpu, KeyRound, ArrowRight } from 'lucide-react';
import { useLogsQuery } from '@/hooks/queries/useLogsQuery';
import { ApiRequestLog } from '@/lib/api';
import { useSSE } from '@/context/SSEContext';
import { AppRoutes } from '@/constants/routes';

interface SSEEventData {
  byCredential?: Record<string, number>;
  byModel?: Record<string, number>;
  selectedModel?: string;
  model?: string;
  latencyMs?: number;
  gatewayApiKeyId?: string;
  gatewayKeyId?: string;
  gatewayKeyName?: string;
  credentialId?: string;
  credentialName?: string;
  keyPrefix?: string;
}

interface SSEEventPayload {
  type?: string;
  data?: SSEEventData;
  model?: string;
  gatewayKeyName?: string;
  credentialName?: string;
  keyPrefix?: string;
  latencyMs?: number;
}

export function ModelActivityWidget({ collapsed }: { collapsed?: boolean }) {
  const { lastEvent } = useSSE();
  const { data: logsData } = useLogsQuery({ limit: 1 });

  const latestLog: ApiRequestLog | undefined = logsData?.data?.[0];

  const [activeModel, setActiveModel] = useState<string>('prism-auto');
  const [activeCred, setActiveCred] = useState<string>('Gateway Key');
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [lastActiveTime, setLastActiveTime] = useState<number>(0);
  const [isRecentlyActive, setIsRecentlyActive] = useState<boolean>(false);

  useEffect(() => {
    if (latestLog) {
      const logTime = latestLog.createdAt ? new Date(latestLog.createdAt).getTime() : Date.now();
      setActiveModel(latestLog.model || 'prism-auto');
      setActiveCred(
        latestLog.credentialName ||
          (latestLog.gatewayKeyId ? `Key ${latestLog.gatewayKeyId.slice(0, 6)}` : 'Gateway Key')
      );
      setLatencyMs(latestLog.latencyMs ?? null);
      setLastActiveTime(logTime);
    }
  }, [latestLog]);

  // Consume global SSE stream
  useEffect(() => {
    if (!lastEvent || !lastEvent.payload) return;
    const payload = lastEvent.payload as SSEEventPayload;

    if (payload.type === 'active_streams_update' && payload.data) {
      const { byCredential, byModel } = payload.data;
      if (byCredential) {
        const credKeys = Object.keys(byCredential);
        if (credKeys.length > 0) {
          setActiveCred(credKeys[0]);
          setLastActiveTime(Date.now());
        }
      }
      if (byModel) {
        const modelKeys = Object.keys(byModel);
        if (modelKeys.length > 0) {
          setActiveModel(modelKeys[0]);
          setLastActiveTime(Date.now());
        }
      }
    }

    if (payload.type === 'ROUTING_DECISION' && payload.data) {
      if (payload.data.selectedModel) {
        setActiveModel(payload.data.selectedModel);
        setLastActiveTime(Date.now());
      }
    }

    if (payload.type === 'request_log_created' && payload.data) {
      const d = payload.data;
      if (d.model) setActiveModel(d.model);
      const cred = d.gatewayKeyName || d.credentialName || d.keyPrefix;
      if (cred) setActiveCred(cred);
      if (typeof d.latencyMs === 'number') setLatencyMs(d.latencyMs);
      setLastActiveTime(Date.now());
    }

    if (payload.model) {
      setActiveModel(payload.model);
      setLastActiveTime(Date.now());
    }
    const credName = payload.gatewayKeyName || payload.credentialName || payload.keyPrefix;
    if (credName) {
      setActiveCred(credName);
      setLastActiveTime(Date.now());
    }
    if (typeof payload.latencyMs === 'number') {
      setLatencyMs(payload.latencyMs);
      setLastActiveTime(Date.now());
    }
  }, [lastEvent]);

  useEffect(() => {
    const timer = setInterval(() => {
      const now = Date.now();
      setIsRecentlyActive(now - lastActiveTime < 15000);
    }, 1000);

    return () => clearInterval(timer);
  }, [lastActiveTime]);

  if (collapsed) {
    return (
      <Link
        href={AppRoutes.PLAYGROUND}
        className="flex items-center justify-center rounded-lg py-2 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
        title={`Smart Router (${isRecentlyActive ? 'ACTIVE' : 'STANDBY'}) | ${isRecentlyActive ? activeModel : 'prism-auto'}`}
        aria-label="Open Routing Simulator"
      >
        <Cpu className={`h-4 w-4 ${isRecentlyActive ? 'text-prism-cyan' : 'text-prism-violet'}`} />
      </Link>
    );
  }

  return (
    <div className="rounded-xl border border-border bg-background/60 p-3 space-y-2.5 text-xs">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-violet-500/30 bg-violet-500/10 text-prism-violet">
            <Cpu className="h-3.5 w-3.5" />
          </div>
          <div className="min-w-0">
            <span className="block font-semibold text-foreground leading-tight">Smart Router</span>
            <span className="block font-mono text-[10px] text-muted-foreground truncate">
              {isRecentlyActive && latencyMs != null ? `${latencyMs}ms last` : 'Policy ready'}
            </span>
          </div>
        </div>
        <span
          className={`shrink-0 rounded-full px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider border ${
            isRecentlyActive
              ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400'
              : 'border-border bg-muted/60 text-muted-foreground'
          }`}
        >
          {isRecentlyActive ? 'LIVE' : 'STANDBY'}
        </span>
      </div>

      <div className="space-y-1 font-mono text-[11px] rounded-lg border border-border/70 bg-card px-2.5 py-1.5">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 font-medium text-foreground min-w-0">
            <Layers className="h-3 w-3 shrink-0 text-prism-cyan" />
            <span className="truncate">{isRecentlyActive ? activeModel : 'prism-auto'}</span>
          </div>
          <span className="text-[10px] text-muted-foreground shrink-0">
            {isRecentlyActive && latencyMs != null ? `${latencyMs}ms` : 'Ready'}
          </span>
        </div>

        {isRecentlyActive && activeCred && (
          <div className="flex items-center gap-1 text-[10px] text-muted-foreground truncate">
            <KeyRound className="h-3 w-3 shrink-0" />
            <span className="truncate">{activeCred}</span>
          </div>
        )}
      </div>

      <Link
        href={AppRoutes.PLAYGROUND}
        className="inline-flex h-8 w-full items-center justify-center gap-1.5 rounded-lg bg-primary px-3 text-xs font-semibold text-primary-foreground shadow-sm transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span>Open Simulator</span>
        <ArrowRight className="h-3.5 w-3.5" />
      </Link>
    </div>
  );
}
