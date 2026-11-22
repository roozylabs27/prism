'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ApiEndpoints } from '@/constants/routes';
import { useAuth } from '@/context/AuthContext';
import { useTenantStore } from '@/stores/useTenantStore';

export interface SSEMessageEvent {
  id?: string;
  orgId?: string;
  type: string;
  payload: Record<string, unknown> | unknown;
  timestamp: string;
}

type SSEListener = (evt: SSEMessageEvent) => void;

interface SSEContextType {
  isConnected: boolean;
  lastEvent: SSEMessageEvent | null;
  subscribeToSSE: (listener: SSEListener) => () => void;
}

const SSEContext = createContext<SSEContextType>({
  isConnected: false,
  lastEvent: null,
  subscribeToSSE: () => () => {},
});

const INITIAL_RECONNECT_MS = 5000;
const MAX_RECONNECT_MS = 30000;
const INVALIDATION_COALESCE_MS = 250;

const EVENT_QUERY_KEY_MAP: Record<string, string[]> = {
  CREDENTIAL_STATUS_CHANGED: ['credentials', 'dashboard-health'],
  CREDENTIAL_QUARANTINED: ['credentials', 'dashboard-health'],
  CREDENTIAL_QUOTA_UPDATED: ['credentials', 'dashboard-health'],
  request_log_created: ['request-logs', 'active-model-activity', 'dashboard-stats', 'dashboard-usage-chart', 'pilot-outcomes'],
  active_streams_update: ['active-streams'],
  APPROVAL_REQUIRED: ['approvals', 'governance-policies'],
  APPROVAL_DECIDED: ['approvals', 'audit-trails'],
  budget_alert: ['budgets', 'budget-alerts', 'finops-summary'],
  cost_anomaly: ['cost-anomalies', 'finops-summary'],
  async_job_updated: ['async-jobs', 'request-logs'],
  routing_decision_created: ['routing-decisions', 'pilot-outcomes'],
  model_updated: ['models', 'providers', 'dashboard-health'],
  provider_updated: ['providers', 'dashboard-health'],
};

export const SSEProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuth();
  const selectedOrgId = useTenantStore((s) => s.selectedOrgId);
  const [isConnected, setIsConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<SSEMessageEvent | null>(null);
  const listenersRef = React.useRef<Set<SSEListener>>(new Set());
  const lastEventIdRef = React.useRef<string>('');

  const subscribeToSSE = React.useCallback((listener: SSEListener) => {
    listenersRef.current.add(listener);
    return () => {
      listenersRef.current.delete(listener);
    };
  }, []);

  useEffect(() => {
    if (!isAuthenticated) {
      return;
    }

    let eventSource: EventSource | null = null;
    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
    let coalesceTimeout: ReturnType<typeof setTimeout> | null = null;
    const pendingQueryKeys = new Set<string>();
    let reconnectDelayMs = INITIAL_RECONNECT_MS;
    let cancelled = false;

    const flushInvalidations = () => {
      coalesceTimeout = null;
      if (cancelled || pendingQueryKeys.size === 0) return;
      const keysToFlush = Array.from(pendingQueryKeys);
      pendingQueryKeys.clear();
      for (const key of keysToFlush) {
        queryClient.invalidateQueries({ queryKey: [key] });
      }
    };

    const enqueueInvalidations = (keys: string[]) => {
      for (const k of keys) {
        pendingQueryKeys.add(k);
      }
      if (!coalesceTimeout) {
        coalesceTimeout = setTimeout(flushInvalidations, INVALIDATION_COALESCE_MS);
      }
    };

    const scheduleReconnect = () => {
      if (cancelled) return;
      const delay = reconnectDelayMs;
      reconnectDelayMs = Math.min(reconnectDelayMs * 2, MAX_RECONNECT_MS);
      reconnectTimeout = setTimeout(connect, delay);
    };

    const buildSSEUrl = () => {
      const params = new URLSearchParams();
      if (selectedOrgId) {
        params.set('orgId', selectedOrgId);
      }
      if (lastEventIdRef.current) {
        params.set('lastEventId', lastEventIdRef.current);
      }
      const qs = params.toString();
      return qs ? `${ApiEndpoints.SSE}?${qs}` : ApiEndpoints.SSE;
    };

    const connect = () => {
      if (cancelled) return;

      try {
        eventSource = new EventSource(buildSSEUrl(), { withCredentials: true });

        eventSource.onopen = () => {
          reconnectDelayMs = INITIAL_RECONNECT_MS;
          setIsConnected(true);
        };

        const handleData = (eventData: string, eventType: string, lastEventId?: string) => {
          try {
            const data = JSON.parse(eventData);
            const evtOrgId =
              typeof data === 'object' && data !== null && typeof data.orgId === 'string'
                ? data.orgId
                : typeof data === 'object' &&
                    data !== null &&
                    typeof data.data === 'object' &&
                    data.data !== null &&
                    typeof (data.data as Record<string, unknown>).org_id === 'string'
                  ? ((data.data as Record<string, unknown>).org_id as string)
                  : undefined;

            // Defense in depth: drop events belonging to a different organization.
            if (selectedOrgId && evtOrgId && evtOrgId !== selectedOrgId) {
              return;
            }

            const resolvedId =
              lastEventId ||
              (typeof data === 'object' && data !== null && typeof data.id === 'string' ? data.id : undefined);
            if (resolvedId) {
              lastEventIdRef.current = resolvedId;
            }

            const resolvedType =
              (typeof data === 'object' &&
                data !== null &&
                ((typeof data.type === 'string' && data.type) ||
                  (typeof data.event === 'string' && data.event))) ||
              eventType;

            const sseEvt: SSEMessageEvent = {
              id: resolvedId,
              orgId: evtOrgId,
              type: resolvedType,
              payload: data,
              timestamp: new Date().toISOString(),
            };
            setLastEvent(sseEvt);

            // Broadcast every event to active subscribers synchronously
            listenersRef.current.forEach((listener) => {
              try {
                listener(sseEvt);
              } catch (e) {
                console.error('[SSE Listener Exception]', e);
              }
            });

            // Coalesce React Query cache invalidations by concrete event type
            const mappedKeys = EVENT_QUERY_KEY_MAP[resolvedType];
            if (mappedKeys && mappedKeys.length > 0) {
              enqueueInvalidations(mappedKeys);
            }
          } catch (err) {
            console.error('[SSE Parse Error]', err);
          }
        };

        eventSource.onmessage = (event) => {
          handleData(event.data, 'message', event.lastEventId);
        };

        eventSource.addEventListener('ping', () => {
          reconnectDelayMs = INITIAL_RECONNECT_MS;
          setIsConnected(true);
        });

        eventSource.addEventListener('connected', () => {
          reconnectDelayMs = INITIAL_RECONNECT_MS;
          setIsConnected(true);
        });

        eventSource.onerror = (err) => {
          console.warn('[SSE Connection Error]', err);
          setIsConnected(false);
          if (eventSource) {
            eventSource.close();
            eventSource = null;
          }
          scheduleReconnect();
        };
      } catch (err) {
        console.error('[SSE Initialization Error]', err);
        setIsConnected(false);
        scheduleReconnect();
      }
    };

    lastEventIdRef.current = '';
    reconnectTimeout = setTimeout(connect, 0);

    return () => {
      cancelled = true;
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      if (coalesceTimeout) clearTimeout(coalesceTimeout);
      pendingQueryKeys.clear();
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [queryClient, isAuthenticated, selectedOrgId]);

  const effectiveConnected = isAuthenticated && isConnected;

  return (
    <SSEContext.Provider value={{ isConnected: effectiveConnected, lastEvent, subscribeToSSE }}>
      {children}
    </SSEContext.Provider>
  );
};

export const useSSE = () => useContext(SSEContext);
