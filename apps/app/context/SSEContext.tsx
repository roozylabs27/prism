'use client';

import React, { createContext, useContext, useEffect, useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { ApiEndpoints } from '@/constants/routes';
import { useAuth } from '@/context/AuthContext';

export interface SSEMessageEvent {
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

export const SSEProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const queryClient = useQueryClient();
  const { isAuthenticated } = useAuth();
  const [isConnected, setIsConnected] = useState(false);
  const [lastEvent, setLastEvent] = useState<SSEMessageEvent | null>(null);
  const listenersRef = React.useRef<Set<SSEListener>>(new Set());

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
    let reconnectDelayMs = INITIAL_RECONNECT_MS;
    let cancelled = false;

    const scheduleReconnect = () => {
      if (cancelled) return;
      const delay = reconnectDelayMs;
      reconnectDelayMs = Math.min(reconnectDelayMs * 2, MAX_RECONNECT_MS);
      reconnectTimeout = setTimeout(connect, delay);
    };

    const connect = () => {
      if (cancelled) return;

      try {
        eventSource = new EventSource(ApiEndpoints.SSE, { withCredentials: true });

        eventSource.onopen = () => {
          reconnectDelayMs = INITIAL_RECONNECT_MS;
          setIsConnected(true);
        };

        const handleData = (eventData: string, eventType: string) => {
          try {
            const data = JSON.parse(eventData);
            const sseEvt: SSEMessageEvent = {
              type: eventType,
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

            // Live update React Query caches on actual gateway activity messages
            const eventName = (typeof data === 'object' && data !== null && (data.type || data.event)) || eventType;
            if (eventName === 'CREDENTIAL_STATUS_CHANGED' || eventName === 'CREDENTIAL_QUARANTINED' || eventName === 'CREDENTIAL_QUOTA_UPDATED') {
              queryClient.invalidateQueries({ queryKey: ['credentials'] });
              queryClient.invalidateQueries({ queryKey: ['dashboard-health'] });
            } else if (eventName === 'request_log_created') {
              queryClient.invalidateQueries({ queryKey: ['request-logs'] });
              queryClient.invalidateQueries({ queryKey: ['active-model-activity'] });
              queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] });
            } else if (eventName === 'active_streams_update') {
              queryClient.invalidateQueries({ queryKey: ['active-streams'] });
            } else if (eventType === 'message') {
              queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] });
              queryClient.invalidateQueries({ queryKey: ['dashboard-usage-chart'] });
              queryClient.invalidateQueries({ queryKey: ['dashboard-health'] });
              queryClient.invalidateQueries({ queryKey: ['active-streams'] });
              queryClient.invalidateQueries({ queryKey: ['request-logs'] });
              queryClient.invalidateQueries({ queryKey: ['credentials'] });
            }
          } catch (err) {
            console.error('[SSE Parse Error]', err);
          }
        };

        eventSource.onmessage = (event) => {
          handleData(event.data, 'message');
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

    reconnectTimeout = setTimeout(connect, 0);

    return () => {
      cancelled = true;
      if (reconnectTimeout) clearTimeout(reconnectTimeout);
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [queryClient, isAuthenticated]);

  const effectiveConnected = isAuthenticated && isConnected;

  return (
    <SSEContext.Provider value={{ isConnected: effectiveConnected, lastEvent, subscribeToSSE }}>
      {children}
    </SSEContext.Provider>
  );
};

export const useSSE = () => useContext(SSEContext);
