// Samples target a self-hosted gateway on the default port from docker-compose (127.0.0.1:8080).
// Verified against apps/api routes: /v1/chat/completions, /v1/messages, /v1/mcp, X-Prism-Agent-ID, X-Routing-Policy.

const base = 'http://localhost:8080';

export interface IntegrationExample {
  id: string;
  label: string;
  note: string;
  prerequisites: string;
  code: string;
  expectedResponse: string;
  failureCases: string;
}

export const integrations: IntegrationExample[] = [
  {
    id: 'python',
    label: 'Python (OpenAI SDK)',
    note: 'Uses the official openai Python package against /v1/chat/completions. Omit X-Prism-Agent-ID until you register the agent in /api/agents.',
    prerequisites: 'pip install openai · Mint a gateway key (starts with prism_sk_) in the Admin Console -> Gateway Keys.',
    code: `from openai import OpenAI

client = OpenAI(
    base_url="${base}/v1",
    api_key="YOUR_PRISM_API_KEY",  # replace with your prism_sk_... key
    default_headers={
        "X-Prism-Agent-ID": "support-bot-acme",  # optional: must match a registered agent
    },
)

response = client.chat.completions.create(
    model="prism-auto",
    messages=[{"role": "user", "content": "Summarize this support ticket."}],
)
print(response.choices[0].message.content)`,
    expectedResponse: 'HTTP 200 JSON ChatCompletion object (choices[0].message.content, usage) plus X-Request-ID, X-Prism-Model, and X-Prism-Provider headers.',
    failureCases: '401 API_KEY_PREFIX_INVALID (key must start with prism_sk_) · 403 AGENT_NOT_FOUND (header names an unregistered agent) · 429 budget_exceeded_error.',
  },
  {
    id: 'typescript',
    label: 'TypeScript (OpenAI SDK)',
    note: 'Uses the official openai npm package (or @roozylabs/prism SDK). Set baseURL to your Prism gateway /v1 endpoint.',
    prerequisites: 'npm install openai · Set PRISM_API_KEY=YOUR_PRISM_API_KEY (minted in Gateway Keys with prefix prism_sk_).',
    code: `import OpenAI from "openai";

const prism = new OpenAI({
  baseURL: "${base}/v1",
  apiKey: process.env.PRISM_API_KEY ?? "YOUR_PRISM_API_KEY",
  defaultHeaders: {
    "X-Prism-Agent-ID": "support-bot-acme", // optional: registered agent slug or ID
  },
});

const completion = await prism.chat.completions.create({
  model: "prism-auto",
  messages: [{ role: "user", content: "Summarize this support ticket." }],
});
console.log(completion.choices[0]?.message?.content);`,
    expectedResponse: 'HTTP 200 ChatCompletion payload with routed model slug recorded in X-Prism-Model and X-Prism-Routing-Policy.',
    failureCases: '401 API_KEY_INVALID (revoked or unknown key) · 403 TENANT_SECURITY_ERROR (spoofed workspace header) · 429 max_concurrent_streams_exceeded.',
  },
  {
    id: 'curl',
    label: 'cURL',
    note: 'Direct HTTP call to /v1/chat/completions. Pass X-Routing-Policy only if you have created that named policy in /api/routing-policies; omit it to use the default balanced policy.',
    prerequisites: 'Active provider credential in Prism + gateway key YOUR_PRISM_API_KEY (prism_sk_...).',
    code: `curl -i ${base}/v1/chat/completions \\
  -H "Authorization: Bearer YOUR_PRISM_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "prism-auto",
    "messages": [{"role": "user", "content": "Hello Prism"}]
  }'`,
    expectedResponse: 'HTTP/1.1 200 OK with headers X-Request-ID, X-Prism-Model, X-Prism-Provider, X-Prism-Routing-Policy, X-Prism-Failovers and OpenAI JSON body.',
    failureCases: '404 routing_policy_not_found (if X-Routing-Policy names a policy not created for your account) · 503 service unavailable (when Redis is down and REDIS_FAIL_CLOSED=true).',
  },
  {
    id: 'opencode',
    label: 'OpenCode CLI',
    note: 'Point OpenCode CLI at Prism using the OpenAI-compatible provider block in ~/.config/opencode/opencode.jsonc.',
    prerequisites: 'OpenCode CLI installed + a workspace-scoped prism_sk_* gateway key.',
    code: `{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "prism": {
      "options": {
        "baseURL": "${base}/v1",
        "apiKey": "YOUR_PRISM_API_KEY"
      },
      "models": {
        "prism-auto": {
          "name": "Prism Auto Smart Router"
        }
      }
    }
  }
}`,
    expectedResponse: 'OpenCode routes chat and coding prompts through /v1/chat/completions with SSE streaming and model attribution in Request Logs.',
    failureCases: '401 API_KEY_MISSING · 403 FORBIDDEN (if key is bound to an agent whose allowedModels excludes the requested model).',
  },
  {
    id: 'claude-code',
    label: 'Claude Code (MCP)',
    note: 'Connects Claude Code to Prism as a governed HTTP MCP server at /v1/mcp. (Anthropic-compatible POST /v1/messages also exists in apps/api, currently marked experimental for Claude Code inference.)',
    prerequisites: 'Claude Code CLI + registered agent claude-code in /api/agents with allowedMcpServers configured.',
    code: `claude mcp add --transport http prism ${base}/v1/mcp \\
  --header "Authorization: Bearer YOUR_PRISM_API_KEY" \\
  --header "X-Prism-Agent-ID: claude-code"`,
    expectedResponse: 'JSON-RPC 2.0 initialize and tools/list return only the MCP tools permitted for agent claude-code plus delegate_ai_task.',
    failureCases: 'JSON-RPC error -32001 (missing gateway key, unknown X-Prism-Agent-ID, or tool blocked by !deny rule).',
  },
  {
    id: 'claude-desktop',
    label: 'Claude Desktop (MCP)',
    note: 'Uses the mcp-remote bridge in claude_desktop_config.json to connect Claude Desktop to POST /v1/mcp.',
    prerequisites: 'Node.js (npx mcp-remote) + registered agent claude-desktop in /api/agents.',
    code: `{
  "mcpServers": {
    "prism": {
      "command": "npx",
      "args": [
        "mcp-remote", "${base}/v1/mcp",
        "--header", "Authorization: Bearer \${PRISM_KEY}",
        "--header", "X-Prism-Agent-ID: claude-desktop"
      ],
      "env": { "PRISM_KEY": "YOUR_PRISM_API_KEY" }
    }
  }
}`,
    expectedResponse: 'Claude Desktop lists allowed upstream MCP tools and records every tool invocation in Prism Audit Trail.',
    failureCases: 'JSON-RPC error -32001 on tools/call when tool matches an explicit !deny rule or agent maxBudgetCents is exhausted.',
  },
];
