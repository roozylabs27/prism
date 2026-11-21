export const governed = [
  'Model inference sent to /v1/chat/completions (sync or SSE stream), /v1/messages (Anthropic format), and /v1/chat/completions/async (polled via /v1/jobs/:jobId)',
  'MCP tool discovery and execution sent through POST /v1/mcp (JSON-RPC 2.0 Streamable HTTP)',
  'Governed REST tool calls (/v1/tools/:name/execute) and data resource queries (/v1/resources/:name/query) with SSRF dial-time guards blocking private and loopback hosts',
  'Upstream provider credential selection, Retry-After cooldowns, and workspace or agent spend limits',
];

export const notGoverned = [
  'Shell commands, local file edits, or external HTTP calls an agent executes directly',
  'MCP servers an agent connects to outside Prism without routing through /v1/mcp',
  'Model requests that use a raw provider API key instead of a Prism gateway key',
];

export const agencyRows = [
  {
    problem: 'Spend is hard to isolate per client or team',
    prism: 'Create one workspace per client account and projects inside it. Every request is metered and attributed to its canonical workspace and project, with role-based member invitations.',
    where: 'Workspaces, Members (/api/workspaces)',
  },
  {
    problem: 'Provider keys are scattered across runtimes',
    prism: 'Store upstream provider API keys once in Prism, encrypted at rest with AES-256-GCM. Callers only receive Prism gateway keys (prism_sk_*) pinned to a workspace, project, or agent, and revocation drops keys from the gateway lookup cache immediately.',
    where: 'Credentials, Gateway keys',
  },
  {
    problem: 'One runaway loop or client can run up the bill',
    prism: 'Enforce daily and monthly spend limits, request quotas, and concurrent stream caps at organization, workspace, and project levels, backed by organization budgets. Requests over the cap return 429 before reaching upstream providers.',
    where: 'Quotas, Budgets',
  },
  {
    problem: 'Agents call destructive tools or expensive models',
    prism: 'Assign each agent an allowlist for models, tools, resources, and MCP servers with !deny rules (which override * wildcards) and a hard maxBudgetCents spend cap.',
    where: 'Agents (/api/agents)',
  },
  {
    problem: 'Client reporting and admin audit trails are fragmented',
    prism: 'Export per-workspace token usage, model spend, and policy denials as CSV or JSON, while an admin audit log records key, credential, member, budget, and policy changes.',
    where: 'Pilot outcomes, Audit trail',
  },
];

export const inference = [
  {
    title: 'Routing rules or prism-auto selection',
    body: 'Pin model patterns (such as claude-* or gpt-4o*) to specific providers by priority with routing rules, or send model: "prism-auto" to score eligible models by prompt task, cost, latency, and credential health. Override the active routing policy per request with X-Routing-Policy (unknown or disabled policies return 404).',
    ref: '/api/routing-rules · model: "prism-auto"',
  },
  {
    title: 'Failover across provider credentials',
    body: 'When an upstream credential returns 429 or 5xx (or fails before the first token on an SSE stream), Prism places it in a Redis cooldown honoring Retry-After and retries the next eligible credential. Prism only records token spend for the final completed response; if all candidates fail, the caller receives the error.',
    ref: 'Retry-After · 50x circuit breaker',
  },
  {
    title: 'Correlation headers, JSON logs, and Prometheus metrics',
    body: 'Responses include X-Request-ID, X-Prism-Model, and X-Prism-Provider (plus X-Prism-Routing-Policy on prism-auto routes and X-Prism-Failovers on non-streaming completions). The gateway writes structured JSON logs with secret redaction and exposes Prometheus instruments at /metrics (guarded by METRICS_TOKEN).',
    ref: 'X-Request-ID · X-Prism-Model · /metrics',
  },
  {
    title: 'Fail closed on Redis state and startup secrets',
    body: 'Spend reservations, quotas, and sliding-window rate limits run on Redis. Outside development, Prism refuses to start with empty or default secrets and rejects gateway calls with 503 when Redis is unreachable (REDIS_FAIL_CLOSED=true by default).',
    ref: 'REDIS_FAIL_CLOSED · METRICS_TOKEN',
  },
];

export const agentControls = [
  {
    term: 'Identity',
    def: 'Bind an agentId directly to a gateway key so callers cannot omit or spoof it, or pass X-Prism-Agent-ID per request for a registered agent.',
  },
  {
    term: 'Allowlists & !deny',
    def: 'Configure allowedModels, allowedTools, allowedResources, and allowedMcpServers. Prefix any entry with ! to deny it; explicit deny rules always override * wildcards.',
  },
  {
    term: 'Spending cap',
    def: 'maxBudgetCents blocks further calls once an agent reaches its monthly spend ceiling.',
  },
  {
    term: 'MCP gateway',
    def: 'HTTP MCP clients connect to POST /v1/mcp and only see and execute tools permitted for their agent identity.',
  },
  {
    term: 'Delegation',
    def: 'The built-in delegate_ai_task MCP tool dispatches a routed model call through Prism and returns the chosen model, provider, and USD cost.',
  },
  {
    term: 'Audit & HMAC verification',
    def: 'Allowed and denied model, tool, and MCP executions are logged with agent identity, policy decision, and HMAC signature verification (/api/audit-trail/:id/verify), alongside an admin audit log of configuration changes.',
  },
];

export const agentPolicyExample = `{
  "name": "support-bot-acme",
  "allowedModels": ["prism-auto"],
  "allowedTools": ["*", "!delete_customer"],
  "allowedMcpServers": ["crm", "kb"],
  "maxBudgetCents": 5000
}`;

export const selfHostOption = {
  title: 'Self-host with Docker Compose',
  tag: 'Self-hosted · Local Docker or VPS',
  who: 'For engineering teams running Prism on their own infrastructure. No Prism software fee; you run the containers and pay your upstream AI providers directly for model usage.',
  prerequisites:
    'Docker and Docker Compose (or Go 1.25+, pnpm 9+, PostgreSQL 15, and Redis 7), plus at least one upstream provider API key.',
  command:
    'git clone https://github.com/roozylabs27/prism.git && cd prism && cp .env.example .env && docker compose up -d --build',
  steps: [
    'Clone the repository, copy .env.example to .env, and set POSTGRES_PASSWORD, REDIS_PASSWORD, ENCRYPTION_KEY (32+ chars), and APP_ENV=development for local testing.',
    'Start the Go gateway (127.0.0.1:8080), Next.js admin console (127.0.0.1:3000), Postgres, and Redis with docker compose up -d --build.',
    'Sign up in the local console, add a provider credential, mint a prism_sk_* gateway key, and point your client base URL at http://localhost:8080/v1.',
  ],
  linkLabel: 'Self-host with Docker',
  expectAfterClick:
    'Opens the Quick Start section on GitHub. Follow the Getting Started guide afterwards to send your first /v1/chat/completions or /v1/mcp call.',
};

export const pilotOption = {
  title: 'Design-partner staging pilot',
  tag: 'Guided 14-day cohort · 3 to 5 agencies',
  who: 'For AI automation agencies running 5 to 30 client accounts who want hands-on help setting up isolated workspaces, quotas, and agent policies on staging traffic.',
  prerequisites:
    '2 or more isolated client or project staging workflows (OpenAI SDK, n8n, LangGraph, CrewAI, or HTTP MCP client) ready to route through Prism for 7 to 14 days.',
  steps: [
    'Open a pilot request issue on GitHub with your client count, model stack, and MCP tool setup.',
    'Join a 30-minute onboarding session to configure client workspaces, pinned gateway keys, and hard quota caps.',
    'Run staging traffic for 7 to 14 days, test a hard budget/tool denial end-to-end, and review exported CSV/JSON cost outcomes together.',
  ],
  capabilities: [
    'Isolated client workspaces & projects',
    'Client-pinned prism_sk_* keys',
    'prism-auto routing & credential failover',
    'Governed /v1/mcp + delegate_ai_task',
    'Weekly CSV/JSON cost & denial export',
  ],
  linkLabel: 'Apply for a design-partner pilot',
  expectAfterClick:
    'Opens a pre-filled GitHub issue template (requires a GitHub account) to share your workload profile, with the 1-page pilot charter linked below.',
};
