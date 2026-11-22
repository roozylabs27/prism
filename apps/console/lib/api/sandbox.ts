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

function buildSandboxHeaders(payload: SandboxExecutionPayload): Record<string, string> {
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
  return headers;
}

export async function apiExecuteSandboxChat(payload: SandboxExecutionPayload): Promise<Response> {
  const res = await fetch(ApiEndpoints.SANDBOX_CHAT_COMPLETIONS, {
    method: 'POST',
    credentials: 'include',
    headers: buildSandboxHeaders(payload),
    body: JSON.stringify({
      model: payload.model,
      messages: payload.messages,
      temperature: payload.temperature,
      stream: payload.stream,
    }),
  });

  if (!res.ok) {
    const errJson = await res.json().catch(() => ({}));
    throw new Error(errJson?.error?.message || `HTTP ${res.status} ${res.statusText}`);
  }
  return res;
}

export async function apiExecuteSandboxChatAsync(payload: SandboxExecutionPayload): Promise<{ job_id: string }> {
  const res = await fetch(ApiEndpoints.SANDBOX_CHAT_COMPLETIONS_ASYNC, {
    method: 'POST',
    credentials: 'include',
    headers: buildSandboxHeaders(payload),
    body: JSON.stringify({
      model: payload.model,
      messages: payload.messages,
      temperature: payload.temperature,
      stream: false,
    }),
  });

  if (!res.ok) {
    const errData = await res.json().catch(() => ({}));
    throw new Error(errData?.error?.message || `Failed to submit async job (Status ${res.status})`);
  }
  return res.json() as Promise<{ job_id: string }>;
}
