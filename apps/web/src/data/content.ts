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
  points: string[];
  meta: string[];
  hasAgentExample?: boolean;
}

export const capabilities: CapabilityPillar[] = [
  {
    index: '01',
    title: 'Routing & failover',
    summary:
      'Control how requests pick models and recover from upstream provider errors without scattering retry loops across apps.',
    points: [
      'Pin wildcard model patterns (such as claude-* or gpt-4o*) to specific providers via routing rules, or send model: "prism-auto" to score candidates by task, cost, latency, and credential health.',
      'Override policy per request with X-Routing-Policy (unknown or disabled policies return 404). On 429 or 5xx before the first token, Prism places the credential in a Redis Retry-After cooldown and retries the next candidate, recording spend only for the final completed response.',
    ],
    meta: ['/api/routing-rules', 'model: "prism-auto"', 'X-Routing-Policy', 'Retry-After cooldown'],
  },
  {
    index: '02',
    title: 'Governance & isolation',
    summary:
      'Separate workspaces, keys, spend limits, and tool permissions across client accounts or internal teams.',
    points: [
      'Isolate traffic into organizations, workspaces, and projects with role-based member invitations. Upstream keys stay encrypted with AES-256-GCM while callers use prism_sk_* keys that drop from the lookup cache immediately upon revocation.',
      'Set daily and monthly spend, request, and concurrent stream quotas at org, workspace, and project scopes, paired with org budgets and per-agent allowlists where !deny rules always override * wildcards.',
    ],
    meta: ['AES-256-GCM vault', 'prism_sk_* keys', 'Quotas & Budgets', '!deny > *'],
    hasAgentExample: true,
  },
  {
    index: '03',
    title: 'Visibility & security',
    summary:
      'Trace every request across providers, export usage without spreadsheets, and keep production secrets fail-closed.',
    points: [
      'Responses return X-Request-ID, X-Prism-Model, and X-Prism-Provider (plus X-Prism-Routing-Policy and non-stream X-Prism-Failovers). Export per-workspace usage, spend, and policy denials as CSV or JSON.',
      'Structured JSON logs redact secrets, Prometheus /metrics is guarded by METRICS_TOKEN, AI audit trails support HMAC verification, admin changes are logged, and REDIS_FAIL_CLOSED refuses traffic outside development when Redis is down.',
    ],
    meta: ['X-Request-ID', '/metrics', 'CSV / JSON export', 'HMAC audit trail', 'SSRF dial guard'],
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
