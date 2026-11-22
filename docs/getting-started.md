# Prism Quickstart & Getting Started Guide

Welcome to **RoozyLabs Prism** — the Universal AI Infrastructure Control Plane & Gateway.

This guide will walk you through setting up Prism, acquiring API keys, and making your first request via cURL, the TypeScript SDK, and the Prism CLI.

---

## 1. Gateway Core Architecture Overview

Prism sits between your client applications, AI agents, and upstream LLM providers (OpenAI, Anthropic, Google Gemini, OpenCode, etc.):

```text
[Your App / Agent] ──(Bearer prism_sk_*)──> [Prism Gateway] ──(Smart Router)──> [Upstream Provider]
```

### Base Endpoints
* **API Gateway (`/v1`)**: OpenAI-compatible endpoint (`/v1/chat/completions`, `/v1/models`, `/v1/tools/:name/execute`, `/v1/resources/:name/query`). Authenticated using `prism_sk_*` Bearer tokens.
* **Control Plane (`/api`)**: Management API for credentials, agents, routing policies, budgets, and telemetry.

---

## 2. Self-Hosting with Docker Compose

### Step 1: Clone and create `.env`

```bash
git clone https://github.com/roozylabs27/prism.git
cd prism
cp .env.example .env
```

`.env.example` intentionally leaves required secrets blank and defaults `APP_ENV=production`. Compose and the API server (`apps/api/internal/config/config.go`) refuse to start with empty or default credentials. Never commit `.env` to Git.

### Step 2: Generate required secrets

Generate distinct random values and set them in `.env`:

```bash
# Run once per secret and paste the output into .env
openssl rand -hex 24   # -> POSTGRES_PASSWORD
openssl rand -hex 24   # -> REDIS_PASSWORD
openssl rand -hex 32   # -> ENCRYPTION_KEY (minimum 32 chars, high variety)
```

Also set a database name matching `.env.example` (for example `POSTGRES_DB=prism` or `POSTGRES_DB=ai_gateway`).

### Step 3: Choose Local Development vs. Production settings

**Local evaluation (`http://localhost:3000`):** update these keys in `.env` so local browser logins and CORS checks work without Cloudflare Turnstile or HTTPS domains:

```ini
APP_ENV=development
TURNSTILE_DISABLED=true
APP_PUBLIC_URL=http://localhost:3000
PUBLIC_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000
```

**Production deployment:** do not reuse development settings or dev secrets on a public host:

- Keep `APP_ENV=production` (enforces strict validation on `ENCRYPTION_KEY`, datastore passwords, CORS origins, and Redis fail-closed behavior).
- Set HTTPS origins in `ALLOWED_ORIGINS`, `APP_PUBLIC_URL`, and `PUBLIC_URL`.
- Set `CLOUDFLARE_SECRET_KEY` and `NEXT_PUBLIC_CLOUDFLARE_SITE_KEY` (or intentionally opt out with `TURNSTILE_DISABLED=true`).
- Set `METRICS_TOKEN` (`openssl rand -hex 24`) if scraping `/metrics`, and follow [Configuration Runbook](./operations/configuration.md) and [Deployment Runbook](./operations/deployment.md) for runtime RLS (`POSTGRES_APP_USER=prism_app`) and TLS setup.

### Step 4: Start the stack and verify health

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:8080/ready
```

All core services (`postgres`, `redis`, `migrator` completed cleanly, `api`, `app`, `web`) should report healthy/running, and `/ready` should return HTTP `200`.

### Step 5: Open the dashboard and mint a gateway key

1. Open `http://127.0.0.1:3000` (or `http://localhost:3000`) and sign up for your initial organization account.
2. Add at least one upstream provider credential (OpenAI, Anthropic, Google Gemini, OpenRouter, etc.).
3. Mint a Gateway API key (`prism_sk_*`) scoped to your workspace/project for the examples below.

> **Routing policy note:** When calling `/v1/chat/completions` with `"model": "prism-auto"`, omitting `X-Routing-Policy` uses Prism's built-in `balanced` fallback policy and returns `X-Prism-Routing-Policy: balanced`. Passing `X-Routing-Policy: <name-or-id>` performs an owner-scoped database lookup and returns `404 routing_policy_not_found` unless you have created and enabled that policy first.

---

## 3. Making Your First Request via cURL

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer prism_sk_your_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "prism-auto",
    "messages": [
      {
        "role": "user",
        "content": "Hello Prism!"
      }
    ]
  }'
```

---

## 4. Getting Started with `@roozylabs/prism` TypeScript SDK

### Installation

```bash
npm install @roozylabs/prism
# or
pnpm add @roozylabs/prism
```

### Usage Example

```typescript
import { Prism } from "@roozylabs/prism";

const prism = new Prism({
  baseURL: "http://localhost:8080",
  apiKey: process.env.PRISM_API_KEY,
  agentId: "backend-engineer", // Optional agent identity attribution
});

async function main() {
  // Non-streaming completion
  const response = await prism.chat.create({
    model: "prism-auto",
    messages: [{ role: "user", content: "Explain vector search in 2 sentences." }],
  });

  console.log("Response:", response.choices[0].message.content);

  // Streaming completion
  const stream = await prism.chat.create({
    model: "prism-auto",
    messages: [{ role: "user", content: "Write a quick hello world in Go." }],
    stream: true,
  });

  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || "");
  }
}

main();
```

---

## 5. Getting Started with `prism` CLI

### Installation

```bash
npm install -g @roozylabs/prism-cli
# or
pnpm add -g @roozylabs/prism-cli
```

### Commands

```bash
# 1. Login & save config
prism login --key prism_sk_12345 --url http://localhost:8080

# 2. Check Gateway Health
prism health

# 3. List active models
prism model list

# 4. Simulate Smart Routing
prism routing simulate --model prism-auto --prompt "Refactor this SQL query"
```

---

## Next Steps

- Explore the [TypeScript SDK Guide](file:///c:/me/projects/ai-gateway/docs/sdk-typescript.md)
- Learn about [Prism CLI Commands](file:///c:/me/projects/ai-gateway/docs/cli.md)
- Inspect the full [OpenAPI API Reference](file:///c:/me/projects/ai-gateway/docs/api-reference.md)

