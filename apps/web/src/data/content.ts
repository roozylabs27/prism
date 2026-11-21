export const governed = [
  'Model inference sent to /v1/chat/completions (sync or SSE stream) and /v1/chat/completions/async (polled via /v1/jobs/:jobId)',
  'MCP tool discovery and execution sent through POST /v1/mcp (JSON-RPC 2.0 Streamable HTTP)',
  'Governed REST tool calls (/v1/tools/:name/execute) and data resource queries (/v1/resources/:name/query)',
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
    prism: 'Create one workspace per client account and projects inside it. Every request is metered and attributed to its canonical workspace and project.',
    where: 'Workspaces (/api/workspaces)',
  },
  {
    problem: 'Provider keys are scattered across runtimes',
    prism: 'Store upstream provider API keys once in Prism, encrypted at rest with AES-256-GCM. Callers only receive Prism gateway keys (prism_sk_*) pinned to a workspace, project, or agent.',
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
    problem: 'Monthly client cost and reliability reports are manual',
    prism: 'Export per-workspace token usage, model spend, error rates, and policy denials as CSV or JSON in one request.',
    where: 'Pilot outcomes export',
  },
];

export const inference = [
  {
    title: 'Pin a model or delegate to prism-auto',
    body: 'Send an explicit model slug and Prism routes to it (honoring any custom routing rules). Send model: "prism-auto" and Prism scores eligible models by prompt task, cost, latency, and provider health, constrained to the models the agent is allowed to call.',
    ref: 'model: "prism-auto" · X-Routing-Policy',
  },
  {
    title: 'Failover across provider credentials',
    body: 'When an upstream credential returns 429 or 5xx, Prism places it in a Redis cooldown honoring Retry-After and retries the next eligible credential. Only the final successful attempt is billed; if all candidates fail, the caller receives the error.',
    ref: 'Retry-After · 50x circuit breaker',
  },
  {
    title: 'Every response records what served it',
    body: 'Responses include X-Request-ID, X-Prism-Model, and X-Prism-Provider. Requests routed via prism-auto also carry X-Prism-Routing-Policy, non-streaming completions report retries in X-Prism-Failovers, and budget warnings appear in X-Prism-Budget-Status.',
    ref: 'X-Prism-Model · X-Prism-Provider',
  },
  {
    title: 'Fail closed when Redis state is unavailable',
    body: 'Spend reservations, quotas, and rate limits rely on Redis. Outside development, setting REDIS_FAIL_CLOSED refuses gateway calls when Redis is unreachable rather than letting spend go unchecked.',
    ref: 'REDIS_FAIL_CLOSED',
  },
];

export const agentControls = [
  {
    term: 'Identity',
    def: 'Bind an agentId directly to a gateway key so callers cannot omit or spoof it, or pass X-Prism-Agent-ID per request.',
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
    term: 'Audit records',
    def: 'Every allowed and denied model, tool, and MCP execution is logged with agent identity, policy decision, and reason.',
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
  tag: 'MIT Licensed · Local or VPS',
  who: 'For engineering teams evaluating Prism on their own Docker host or infrastructure.',
  prerequisites:
    'Docker and Docker Compose (or Go 1.25+, pnpm 9+, PostgreSQL 15, and Redis 7), plus at least one upstream provider API key.',
  command:
    'git clone https://github.com/roozylabs27/prism.git && cd prism && cp .env.example .env && docker compose up -d --build',
  steps: [
    'Clone the repository, copy .env.example to .env, and set your database/encryption secrets.',
    'Start the Go gateway (:8080), Next.js admin console (:3000), Postgres, and Redis with Docker Compose.',
    'Add a provider credential in the console, mint a workspace-scoped prism_sk_* key, and point your client base URL at localhost:8080/v1.',
  ],
  linkLabel: 'Self-host with Docker',
  expectAfterClick:
    'Opens the Quick Start section on GitHub. Follow the Getting Started guide afterwards to send your first /v1/chat/completions or /v1/mcp call.',
};

export const pilotOption = {
  title: 'Design-partner staging pilot',
  tag: 'Guided 14-day cohort · 3 to 5 agencies',
  who: 'For AI automation agencies running 5 to 30 client accounts who want hands-on help setting up isolated workspaces, quotas, and agent policies.',
  prerequisites:
    '2 or more isolated client or project staging workflows (OpenAI SDK, n8n, LangGraph, CrewAI, or HTTP MCP client) ready to route through Prism for 7 to 14 days.',
  steps: [
    'Open a short pilot request with your client count, model stack, and MCP tool setup.',
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
    'Opens a pre-filled GitHub issue to share your workload profile, with the 1-page pilot charter linked below.',
};
