// Samples target a self-hosted gateway on the default port from docker-compose.
// Verified against apps/api routes: /v1/chat/completions, /v1/mcp, X-Prism-Agent-ID, X-Routing-Policy.

const base = 'http://localhost:8080';

export const integrations = [
  {
    id: 'python',
    label: 'Python',
    note: 'Any OpenAI SDK works. Change the base URL and key.',
    code: `from openai import OpenAI

client = OpenAI(
    base_url="${base}/v1",
    api_key="prism_sk_...",  # gateway key scoped to one client workspace
)

response = client.chat.completions.create(
    model="prism-auto",
    messages=[{"role": "user", "content": "Summarize this support ticket."}],
    extra_headers={"X-Prism-Agent-ID": "support-bot-acme"},
)
print(response.choices[0].message.content)`,
  },
  {
    id: 'typescript',
    label: 'TypeScript',
    note: 'Any OpenAI SDK works. Change the base URL and key.',
    code: `import OpenAI from "openai";

const prism = new OpenAI({
  baseURL: "${base}/v1",
  apiKey: process.env.PRISM_KEY,
  defaultHeaders: { "X-Prism-Agent-ID": "support-bot-acme" },
});

const completion = await prism.chat.completions.create({
  model: "prism-auto",
  messages: [{ role: "user", content: "Summarize this support ticket." }],
});`,
  },
  {
    id: 'curl',
    label: 'cURL',
    note: 'Pick a routing policy per request with X-Routing-Policy.',
    code: `curl ${base}/v1/chat/completions \\
  -H "Authorization: Bearer $PRISM_KEY" \\
  -H "X-Prism-Agent-ID: support-bot-acme" \\
  -H "X-Routing-Policy: cost-saver" \\
  -H "Content-Type: application/json" \\
  -d '{"model": "prism-auto", "messages": [{"role": "user", "content": "Hello"}]}'`,
  },
  {
    id: 'claude-code',
    label: 'Claude Code (MCP)',
    note: 'Claude Code connects to Prism as an MCP server. Its own model calls do not go through Prism yet.',
    code: `claude mcp add --transport http prism ${base}/v1/mcp \\
  --header "Authorization: Bearer $PRISM_KEY" \\
  --header "X-Prism-Agent-ID: claude-code"`,
  },
  {
    id: 'claude-desktop',
    label: 'Claude Desktop (MCP)',
    note: 'Uses the mcp-remote bridge in claude_desktop_config.json. Model selection stays in Claude Desktop.',
    code: `{
  "mcpServers": {
    "prism": {
      "command": "npx",
      "args": [
        "mcp-remote", "${base}/v1/mcp",
        "--header", "Authorization: Bearer \${PRISM_KEY}",
        "--header", "X-Prism-Agent-ID: claude-desktop"
      ],
      "env": { "PRISM_KEY": "prism_sk_..." }
    }
  }
}`,
  },
];
