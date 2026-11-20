export const governed = [
  'Model calls sent to /v1/chat/completions and /v1/chat/completions/async',
  'Tool calls sent through the MCP endpoint at /v1/mcp',
  'Tool and resource calls sent to /v1/tools and /v1/resources',
  'Which provider credential is used, and how much each client spends',
];

export const notGoverned = [
  'Shell commands, file access or API calls an agent makes directly',
  'MCP servers an agent connects to without going through Prism',
  'Model calls that use a provider key directly instead of a Prism key',
];

export const agencyRows = [
  {
    problem: 'Spend is hard to split by client',
    prism: 'One workspace per client, projects inside it. Every request is metered and attributed to its workspace and project.',
    where: 'Workspaces, Projects',
  },
  {
    problem: 'Provider keys are scattered across projects',
    prism: 'Your own provider keys are stored once, encrypted with AES-256-GCM. Clients get Prism gateway keys pinned to their workspace or project.',
    where: 'Credentials, Gateway keys',
  },
  {
    problem: 'One client can run up the bill',
    prism: 'Budgets and daily request quotas at organization, workspace and project level. Requests over the limit are refused before they reach the provider.',
    where: 'Budgets, Quotas',
  },
  {
    problem: 'Agents reach tools they should not',
    prism: 'Each agent has an allowlist for models, tools, resources and MCP servers, with explicit deny rules and a spending cap.',
    where: 'Agents',
  },
  {
    problem: 'Monthly client reports are manual',
    prism: 'Usage and cost per client exports as CSV or JSON.',
    where: 'Analytics export',
  },
];

export const inference = [
  {
    title: 'Routing you can pin or delegate',
    body: 'Send a model name and Prism uses it. Send prism-auto and Prism scores the eligible models by task, cost, latency and provider health, limited to the models the agent is allowed to use. Choose a policy per request with the X-Routing-Policy header.',
    ref: 'model: "prism-auto"',
  },
  {
    title: 'Failover across credentials',
    body: 'When a provider rate-limits or fails, that credential cools down for the period the provider asks for in Retry-After, and the request moves to the next eligible credential. If every credential fails, the client gets the error.',
    ref: 'X-Prism-Failovers',
  },
  {
    title: 'Every response says what happened',
    body: 'Response headers record the model and provider that served the request, the routing policy used, how many failovers occurred and the budget status.',
    ref: 'X-Prism-Model · X-Prism-Provider',
  },
  {
    title: 'Fail closed when state is unavailable',
    body: 'Budget and quota checks depend on Redis. Outside development, Prism refuses requests when Redis is unreachable rather than letting spend go unchecked.',
    ref: 'REDIS_FAIL_CLOSED',
  },
];

export const agentControls = [
  { term: 'Identity', def: 'Agents are registered and identify themselves with the X-Prism-Agent-ID header on each call.' },
  { term: 'Allowlists', def: 'AllowedModels, AllowedTools, AllowedResources and AllowedMCPServers. Prefix an entry with ! to deny it.' },
  { term: 'Spending cap', def: 'MaxBudgetCents stops an agent once its spend reaches the cap.' },
  { term: 'MCP gateway', def: 'Claude Code, Claude Desktop and other MCP clients connect to /v1/mcp and only see the tools their agent is allowed to call.' },
  { term: 'Delegation', def: 'The built-in delegate_ai_task tool runs a model call through Prism and returns the model, provider and cost it used.' },
  { term: 'Audit records', def: 'Every MCP tool call, allowed or denied, is recorded with the agent, the decision and the reason.' },
];

export const agentPolicyExample = `{
  "name": "support-bot-acme",
  "allowedModels": ["prism-auto"],
  "allowedTools": ["*", "!delete_customer"],
  "allowedMcpServers": ["crm", "kb"],
  "maxBudgetCents": 5000
}`;

export const shipped = [
  'OpenAI-compatible chat completions, streaming and async jobs',
  'Adapters for OpenAI, Anthropic, Google and OpenCode, plus OpenAI-compatible endpoints',
  'prism-auto routing with per-request policy and agent model filtering',
  'Encrypted provider credentials with key rotation tooling',
  'Workspaces, projects and scoped gateway keys',
  'Budgets and quotas at organization, workspace and project level',
  'Agent allowlists with deny rules and spending caps',
  'MCP gateway at /v1/mcp with delegate_ai_task',
  'Per-client usage export (CSV and JSON)',
];

export const inProgress = [
  { item: 'Anthropic Messages API endpoint (/v1/messages)', note: 'Needed to point Claude Code inference at Prism. Today Claude Code connects through MCP only.' },
  { item: 'OpenAI Responses API endpoint (/v1/responses)', note: 'Not available yet.' },
  { item: 'Human approval for sensitive tool calls', note: 'Not built. Deny rules are the current control.' },
  { item: 'Durable audit delivery', note: 'Audit records are written asynchronously and can be lost if the process crashes.' },
  { item: 'Hosted plans and billing', note: 'Disabled. Prism is self-hosted or run as a pilot today.' },
];

export const startOptions = [
  {
    title: 'Self-host',
    who: 'For teams that want to evaluate on their own infrastructure.',
    steps: ['Clone the repository', 'Copy .env.example to .env', 'Run docker compose up -d'],
    linkLabel: 'Read the quick start',
    linkKey: 'quickstart' as const,
  },
  {
    title: 'Design-partner pilot',
    who: 'For agencies running 5 to 30 client accounts who want help setting up workspaces, keys and agent policies.',
    steps: ['Tell us how many clients and agents you run', 'We set up a pilot together', 'We review cost and control outcomes after the pilot'],
    linkLabel: 'Request a pilot',
    linkKey: 'pilotRequest' as const,
  },
];
