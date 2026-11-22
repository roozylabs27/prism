export const requestChecks = [
  {
    name: '1. Key & tenant scope',
    detail: 'Validates gateway key, resolves workspace scope, and blocks spoofed headers.',
  },
  {
    name: '2. Quotas & budget',
    detail: 'Checks spend, request, and concurrent stream caps before calling upstream.',
  },
  {
    name: '3. Agent policy',
    detail: 'Verifies allowed models, MCP servers, tools, and !deny rules.',
  },
  {
    name: '4. Route & failover',
    detail: 'Selects the route via rules or prism-auto, retrying pre-stream errors.',
  },
];

export const boundarySummary = {
  governed: [
    {
      label: 'Model inference',
      detail: 'Synchronous, streaming, and async completions routed through Prism model endpoints.',
    },
    {
      label: 'MCP servers',
      detail: 'Tool discovery and execution proxied through Prism over HTTP MCP (JSON-RPC 2.0).',
    },
    {
      label: 'Governed tools & resources',
      detail: 'Registered REST tools and resource queries invoked through Prism with SSRF guards.',
    },
  ],
  outside: [
    {
      label: 'Local agent runtime',
      detail: 'Shell commands, local file edits, and direct outbound HTTP calls on the agent host.',
    },
    {
      label: 'Direct provider calls',
      detail: 'Model APIs or MCP servers contacted directly with raw upstream credentials.',
    },
  ],
};

export interface CapabilityPillar {
  index: string;
  title: string;
  summary: string;
  controls: string;
  behavior: string;
  boundary: string;
  meta: string[];
  hasAgentExample?: boolean;
}

export const capabilities: CapabilityPillar[] = [
  {
    index: '01',
    title: 'Routing & failover',
    summary:
      'Choose which provider handles each request and recover from upstream rate limits without scattering retry loops across apps.',
    controls:
      'Wildcard routing rules (such as claude-* or gpt-4o*) pinned to specific providers, or model: "prism-auto" scoring candidates by task, cost, latency, and health.',
    behavior:
      'Omitting X-Routing-Policy uses the built-in balanced policy; passing a named policy selects an owner-scoped saved policy (or returns 404). On 429 or 5xx before the first token, Prism places the credential in a Redis Retry-After cooldown and tries the next candidate.',
    boundary:
      "Streaming failover stops once the first SSE token is flushed; mid-stream upstream failures emit stream_error instead of restarting. Failed attempts record zero spend in Prism's ledger, which does not change upstream provider billing.",
    meta: ['/api/routing-rules', 'model: "prism-auto"', 'Retry-After cooldown'],
  },
  {
    index: '02',
    title: 'Governance & isolation',
    summary:
      'Separate client workspaces, upstream credentials, spend caps, and agent permissions from one control plane.',
    controls:
      'Organizations, workspaces, and projects with role-based member invites; encrypted upstream provider credentials (AES-256-GCM); and revocable prism_sk_* caller keys.',
    behavior:
      'Daily and monthly spend, request, and concurrent-stream quotas apply per org, workspace, or project alongside org-level budgets. Agent policies restrict models, MCP servers, and tools, with !deny rules overriding * wildcards.',
    boundary:
      'Revoked keys drop from the lookup cache immediately, while budget reservations are scoped at the organization level in Redis. Runtime PostgreSQL RLS requires running the API under the non-superuser prism_app role.',
    meta: ['AES-256-GCM vault', 'Quotas & Budgets', '!deny > *'],
    hasAgentExample: true,
  },
  {
    index: '03',
    title: 'Visibility & security',
    summary:
      'Trace routed requests across providers, export workspace usage without manual spreadsheets, and fail closed on missing runtime state.',
    controls:
      'Per-request correlation and routing headers, structured JSON access logs with secret redaction, admin action logs, and HMAC-verifiable AI audit trails.',
    behavior:
      'Responses return X-Request-ID, X-Prism-Model, and X-Prism-Provider (plus non-stream X-Prism-Failovers). Per-workspace usage, spend, and policy denials export as CSV or JSON; Prometheus /metrics is bearer-guarded.',
    boundary:
      'Outside development, /metrics returns 404 until METRICS_TOKEN is set, and REDIS_FAIL_CLOSED refuses gateway traffic if Redis is unavailable. Paid hosted billing tiers remain disabled behind a feature flag.',
    meta: ['X-Request-ID', 'CSV / JSON export', 'HMAC audit trail'],
  },
];

export const agentPolicyExample = `{
  "name": "support-bot-acme",
  "allowedModels": ["prism-auto"],
  "allowedTools": ["*", "!delete_customer"],
  "allowedMcpServers": ["crm", "kb"],
  "maxBudgetCents": 5000
}`;

export const selfHostTrack = {
  title: 'Self-host with Docker Compose',
  tag: 'Self-hosted · Local Docker or VPS',
  summary:
    'Run the Go gateway, Next.js admin console, PostgreSQL, and Redis on your own host. No Prism software fee; you pay upstream AI providers directly for model usage.',
  prerequisites:
    'Docker & Docker Compose, plus OpenSSL (or another secret generator) to populate required secrets in .env before starting.',
  cloneCommand:
    'git clone https://github.com/roozylabs27/prism.git && cd prism && cp .env.example .env',
  envConfig: {
    secrets:
      'POSTGRES_PASSWORD, REDIS_PASSWORD (openssl rand -hex 24), ENCRYPTION_KEY (openssl rand -hex 32)',
    localDev:
      'APP_ENV=development, TURNSTILE_DISABLED=true, ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000, APP_PUBLIC_URL=http://localhost:3000',
    note: '.env.example leaves secrets blank and defaults to APP_ENV=production. Populate .env before starting Compose.',
  },
  startCommand: 'docker compose up -d --build && curl -fsS http://127.0.0.1:8080/ready',
  linkLabel: 'Open Quick Start on GitHub',
  guideLabel: 'Full local & production setup guide',
  note: 'Binds gateway on 127.0.0.1:8080, console on 127.0.0.1:3000, and web on 127.0.0.1:3001. Production deployments must keep APP_ENV=production with HTTPS origins and Turnstile.',
};

export const pilotTrack = {
  title: 'Design-partner staging pilot',
  tag: 'Guided 14-day cohort · 3 to 5 agencies',
  summary:
    'There are no paid hosted tiers enabled today. Agencies running 5 to 30 client accounts can work with us to configure isolated workspaces, pinned keys, and hard quota caps on staging.',
  prerequisites:
    '2 or more isolated client or project staging workflows (OpenAI SDK, n8n, LangGraph, CrewAI, or HTTP MCP client) ready to test for 7 to 14 days.',
  steps: [
    'Open a pilot request issue on GitHub with your client count, models, and MCP setup.',
    'Configure isolated workspaces, pinned gateway keys, and quota limits in a 30-minute onboarding call.',
    'Test a budget/tool denial end-to-end and review exported CSV/JSON cost attribution together.',
  ],
  linkLabel: 'Apply for a design-partner pilot',
  note: 'Opens a pre-filled GitHub issue template (requires a GitHub account).',
};
