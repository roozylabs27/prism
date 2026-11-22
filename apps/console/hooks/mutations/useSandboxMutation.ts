import { useMutation } from '@tanstack/react-query';
import { apiExecuteSandboxChat, SandboxExecutionPayload } from '@/lib/api/sandbox';

export type { SandboxExecutionPayload };

export function useSandboxExecutionMutation() {
  return useMutation({
    mutationFn: (payload: SandboxExecutionPayload) => apiExecuteSandboxChat(payload),
  });
}
