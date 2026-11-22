// Core quickstart examples targeting a self-hosted gateway (http://localhost:8080).
// Verified against apps/api routes: /v1/chat/completions and /v1/mcp.

const base = 'http://localhost:8080';

export interface IntegrationExample {
  id: string;
  label: string;
  note: string;
  code: string;
  expectedResponse: string;
}

export const integrations: IntegrationExample[] = [
  {
    id: 'typescript',
    label: 'TypeScript',
    note: 'Uses the official openai package against /v1/chat/completions. Omit X-Prism-Agent-ID until you register the agent in /api/agents.',
    code: `import OpenAI from "openai";

const prism = new OpenAI({
  baseURL: "${base}/v1",
  apiKey: process.env.PRISM_API_KEY ?? "prism_sk_YOUR_KEY",
  defaultHeaders: {
    "X-Prism-Agent-ID": "support-bot-acme", // optional: registered agent
  },
});

const completion = await prism.chat.completions.create({
  model: "prism-auto",
  messages: [{ role: "user", content: "Summarize this support ticket." }],
});
console.log(completion.choices[0]?.message?.content);`,
    expectedResponse:
      'HTTP 200 JSON ChatCompletion with X-Request-ID, X-Prism-Model, X-Prism-Provider, and X-Prism-Routing-Policy headers.',
  },
  {
    id: 'curl',
    label: 'cURL',
    note: 'Direct request to /v1/chat/completions using the built-in balanced default policy. Pass X-Routing-Policy only after creating a custom policy.',
    code: `curl -i ${base}/v1/chat/completions \\
  -H "Authorization: Bearer prism_sk_YOUR_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "prism-auto",
    "messages": [{"role": "user", "content": "Hello Prism"}]
  }'`,
    expectedResponse:
      'HTTP/1.1 200 OK with routed model headers and OpenAI JSON body (401/403/429 on invalid key, policy denial, or quota cap).',
  },
  {
    id: 'claude-code',
    label: 'MCP (Claude Code)',
    note: 'Registers Prism as a governed HTTP MCP server at /v1/mcp (Anthropic /v1/messages endpoint also exists in apps/api as experimental for Claude Code inference).',
    code: `claude mcp add --transport http prism ${base}/v1/mcp \\
  --header "Authorization: Bearer prism_sk_YOUR_KEY" \\
  --header "X-Prism-Agent-ID: claude-code"`,
    expectedResponse:
      'JSON-RPC 2.0 tools/list returns only tools permitted for agent claude-code plus delegate_ai_task (-32001 on denial).',
  },
];
