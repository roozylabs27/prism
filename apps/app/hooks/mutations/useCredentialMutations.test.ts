import { describe, it, expect, vi, beforeEach } from 'vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, act } from '@testing-library/react';
import React from 'react';
import {
  useResetCooldownMutation,
  useDeleteCredentialMutation,
} from './useCredentialMutations';
import * as credentialsApi from '@/lib/api/credentials';

vi.mock('@/lib/api/credentials', () => ({
  apiAddCredential: vi.fn(),
  apiResetCredentialCooldown: vi.fn().mockResolvedValue({ message: 'reset' }),
  apiToggleCredential: vi.fn(),
  apiSetCredentialPriority: vi.fn(),
  apiDeleteCredential: vi.fn().mockResolvedValue({ message: 'deleted' }),
  apiTestCredential: vi.fn(),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return function Wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: queryClient }, children);
  };
}

describe('useCredentialMutations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('passes (credId, providerId) in the API order when resetting cooldown', async () => {
    const { result } = renderHook(() => useResetCooldownMutation(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ providerId: 'prov-1', credId: 'cred-9' });
    });

    expect(credentialsApi.apiResetCredentialCooldown).toHaveBeenCalledWith('cred-9', 'prov-1');
  });

  it('passes (credId, providerId) in the API order when deleting a credential', async () => {
    const { result } = renderHook(() => useDeleteCredentialMutation(), {
      wrapper: createWrapper(),
    });

    await act(async () => {
      await result.current.mutateAsync({ providerId: 'prov-1', credId: 'cred-9' });
    });

    expect(credentialsApi.apiDeleteCredential).toHaveBeenCalledWith('cred-9', 'prov-1');
  });
});
