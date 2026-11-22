import { render, act } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const mockUseAuth = vi.fn();

vi.mock('@/context/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

import { SSEProvider, useSSE } from './SSEContext';

class MockEventSource {
  static instances: MockEventSource[] = [];
  url: string;
  options?: EventSourceInit;
  onopen: ((ev: Event) => unknown) | null = null;
  onmessage: ((ev: MessageEvent) => unknown) | null = null;
  onerror: ((ev: Event) => unknown) | null = null;
  closed = false;

  constructor(url: string, options?: EventSourceInit) {
    this.url = url;
    this.options = options;
    MockEventSource.instances.push(this);
  }

  addEventListener() {}
  close() {
    this.closed = true;
  }
}

function Probe({ onState }: { onState: (connected: boolean) => void }) {
  const { isConnected } = useSSE();
  onState(isConnected);
  return null;
}

describe('SSEProvider (HttpOnly session support)', () => {
  const originalEventSource = globalThis.EventSource;

  beforeEach(() => {
    vi.useFakeTimers();
    MockEventSource.instances = [];
    document.cookie = 'auth_token=; expires=Thu, 01 Jan 1970 00:00:00 GMT; path=/';
    globalThis.EventSource = MockEventSource as unknown as typeof EventSource;
  });

  afterEach(() => {
    vi.useRealTimers();
    globalThis.EventSource = originalEventSource;
  });

  it('connects with withCredentials even when JS cannot read an HttpOnly cookie', async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true });
    const qc = new QueryClient();
    let latestConnected = false;

    render(
      <QueryClientProvider client={qc}>
        <SSEProvider>
          <Probe onState={(v) => { latestConnected = v; }} />
        </SSEProvider>
      </QueryClientProvider>
    );

    expect(MockEventSource.instances).toHaveLength(0);
    await act(async () => {
      vi.advanceTimersByTime(10);
    });

    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.instances[0].url).toBe('/api/sse');
    expect(MockEventSource.instances[0].options).toEqual({ withCredentials: true });

    await act(async () => {
      MockEventSource.instances[0].onopen?.(new Event('open'));
    });
    expect(latestConnected).toBe(true);
  });

  it('does not open EventSource when unauthenticated and closes on sign-out', async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: false });
    const qc = new QueryClient();
    let latestConnected = false;

    const { rerender } = render(
      <QueryClientProvider client={qc}>
        <SSEProvider>
          <Probe onState={(v) => { latestConnected = v; }} />
        </SSEProvider>
      </QueryClientProvider>
    );

    await act(async () => {
      vi.advanceTimersByTime(50);
    });
    expect(MockEventSource.instances).toHaveLength(0);
    expect(latestConnected).toBe(false);

    mockUseAuth.mockReturnValue({ isAuthenticated: true });
    rerender(
      <QueryClientProvider client={qc}>
        <SSEProvider>
          <Probe onState={(v) => { latestConnected = v; }} />
        </SSEProvider>
      </QueryClientProvider>
    );

    await act(async () => {
      vi.advanceTimersByTime(10);
    });
    expect(MockEventSource.instances).toHaveLength(1);

    mockUseAuth.mockReturnValue({ isAuthenticated: false });
    rerender(
      <QueryClientProvider client={qc}>
        <SSEProvider>
          <Probe onState={(v) => { latestConnected = v; }} />
        </SSEProvider>
      </QueryClientProvider>
    );
    expect(MockEventSource.instances[0].closed).toBe(true);
    expect(latestConnected).toBe(false);
  });

  it('coalesces burst events within 250ms and maps approval, budget, and anomaly events', async () => {
    mockUseAuth.mockReturnValue({ isAuthenticated: true });
    const qc = new QueryClient();
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries');

    render(
      <QueryClientProvider client={qc}>
        <SSEProvider>
          <div />
        </SSEProvider>
      </QueryClientProvider>
    );

    await act(async () => {
      vi.advanceTimersByTime(10);
    });
    const es = MockEventSource.instances[0];
    expect(es).toBeDefined();

    // Emit a burst of 5 request_log_created + 1 APPROVAL_REQUIRED + 1 budget_alert + 1 cost_anomaly
    await act(async () => {
      for (let i = 0; i < 5; i++) {
        es.onmessage?.(
          new MessageEvent('message', {
            data: JSON.stringify({ id: `evt-${i}`, type: 'request_log_created', data: {} }),
          })
        );
      }
      es.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({ id: 'evt-app', type: 'APPROVAL_REQUIRED', data: {} }),
        })
      );
      es.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({ id: 'evt-bud', type: 'budget_alert', data: {} }),
        })
      );
      es.onmessage?.(
        new MessageEvent('message', {
          data: JSON.stringify({ id: 'evt-anom', type: 'cost_anomaly', data: {} }),
        })
      );
    });

    // Before the 250ms coalesce window expires, no invalidations have fired yet
    expect(invalidateSpy).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(260);
    });

    const invalidatedKeys = invalidateSpy.mock.calls.map((c) => (c[0] as { queryKey: string[] }).queryKey[0]);
    // Each unique queryKey is invalidated once despite 5 request_log_created events
    expect(invalidatedKeys.filter((k) => k === 'request-logs')).toHaveLength(1);
    expect(invalidatedKeys).toContain('approvals');
    expect(invalidatedKeys).toContain('budget-alerts');
    expect(invalidatedKeys).toContain('cost-anomalies');
  });
});
