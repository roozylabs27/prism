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
});
