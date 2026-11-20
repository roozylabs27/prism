import { useMutation } from '@tanstack/react-query';
import { ApiEndpoints } from '@/constants/routes';
import { getTenantHeaders } from '@/stores/useTenantStore';

export interface SandboxExecutionPayload {
  keyPrefix: string;
  routingPolicy: string;
  agentId: string;
  model: string;
  messages: Array<{ role: string; content: string }>;
  temperature: number;
  stream: boolean;
}

export function useSandboxExecutionMutation() {
  return useMutation({
    mutationFn: async (payload: SandboxExecutionPayload) => {
      const headers: Record<string, string> = {
        'Content-Type': 'application/json',
        ...getTenantHeaders(),
      };
      if (payload.keyPrefix) {
        headers['X-Sandbox-Key-Prefix'] = payload.keyPrefix;
      }
      // The policy id (or exact name) the smart router should use; the
      // backend answers 404 for one the caller does not own.
      if (payload.model === 'prism-auto' && payload.routingPolicy) {
        headers['X-Routing-Policy'] = payload.routingPolicy;
      }
      if (payload.agentId && payload.agentId !== 'default') {
        headers['X-Prism-Agent-ID'] = payload.agentId;
        headers['X-Prism-Agent-Name'] = payload.agentId;
        headers['X-Agent-Name'] = payload.agentId;
      }

      const body = {
        model: payload.model,
        messages: payload.messages,
        temperature: payload.temperature,
        stream: payload.stream,
      };

      const res = await fetch(ApiEndpoints.SANDBOX_CHAT_COMPLETIONS, {
        method: 'POST',
        credentials: 'include',
        headers,
        body: JSON.stringify(body),
      });

      if (!res.ok) {
        const errJson = await res.json().catch(() => ({}));
        throw new Error(errJson?.error?.message || `HTTP ${res.status} ${res.statusText}`);
      }

      return res;
    },
  });
}
