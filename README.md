# RoozyLabs Prism

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?style=flat&logo=go)](https://golang.org)
[![Next.js 16](https://img.shields.io/badge/Next.js-16.3.3-black?style=flat&logo=next.js)](https://nextjs.org)
[![Astro](https://img.shields.io/badge/Astro-5.0-BC52EE?style=flat&logo=astro)](https://astro.build)
[![Docker](https://img.shields.io/badge/Docker-Enabled-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Version](https://img.shields.io/badge/Version-0.4.0-blue.svg)](CHANGELOG.md)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**RoozyLabs Prism** is an infrastructure-grade **Universal AI Control Plane** and intelligent model gateway. It unifies multiple AI providers (OpenAI, Anthropic, Google Gemini, OpenRouter, OpenCode Free) and credential pools behind a single, resilient execution layer.

Your client applications and AI coding tools (OpenCode CLI, Claude Code, Antigravity IDE, LangChain, or custom agents) connect to one **Gateway API Key** and one **Prism Gateway Endpoint** — Prism handles routing, retries, failover, budgets, and metering for you.

---

## Public Live Domains & Endpoints

- **Public Landing Page**: [https://prism.roozylabs.com](https://prism.roozylabs.com)
- **Admin Console Dashboard**: [https://app.prism.roozylabs.com](https://app.prism.roozylabs.com)
- **Model Gateway API Proxy**: [https://api.prism.roozylabs.com](https://api.prism.roozylabs.com)

---

## What Prism Does

- **One gateway key, many models**: Access all supported providers through a single OpenAI-compatible endpoint.
- **Smart routing**: Prism automatically picks the best model for each request based on quality, cost, speed, and provider health — with automatic fallback if a provider fails.
- **Resilience by default**: Circuit breaking, instant failover, and credential pooling keep your applications online during provider outages.
- **Governance & control**: Manage agents, tools, resources, budgets, and permissions from a single admin console, with a tamper-proof audit trail.
- **Cost visibility**: Track token usage and spend, set budget limits, and get burn-rate forecasts across your organization.

---

## System Architecture

```text
                                CLIENTS & AGENTS
               ┌───────────────────────┼───────────────────────┐
               │                       │                       │
         OpenCode CLI             Claude Code             Antigravity IDE
               │                       │                       │
               └───────────────────────┼───────────────────────┘
                                       │ (Authorization: Bearer prism_sk_xxx)
                                       │ (Header: X-Prism-Agent-ID: dev-agent)
                                       ▼
                              ┌─────────────────┐
                              │   Next.js App   │
                              │ Console & Web   │
                              └────────┬────────┘
                                       │ (/api proxy)
                                       ▼
                              ┌─────────────────┐
                              │ RoozyLabs Prism │
                              │ (Go API :8080)  │
                              └────────┬────────┘
                                       │
                     ┌─────────────────┼─────────────────┐
                     ▼                 ▼                 ▼
               OpenAI / Gemini   Tool/Resource Gwy    MCP Protocol
                 Inference          (REST/SQL)       (JSON-RPC 2.0)
```

For the detailed internal architecture, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Key Features

- **Universal Model Gateway**: One OpenAI-compatible endpoint for OpenAI, Anthropic, Google Gemini, OpenRouter, and OpenCode Free.
- **Smart Auto Router (`prism-auto`)**: Picks the best model per request using multi-factor scoring (quality, cost, speed, health) and falls back automatically when a provider is unavailable.
- **High Availability & Circuit Breaking**: Automatic error detection, credential quarantine, and instant failover keep your uptime high.
- **AI Agent Gateway**: Govern autonomous agents with per-agent model, tool, resource, and budget limits.
- **Tool & Resource Gateway**: Connect custom function calls, REST APIs, and PostgreSQL resources with sandboxed execution.
- **MCP Gateway**: Central discovery and routing for Model Context Protocol (MCP) servers, with per-server usage analytics (success rate, latency, per-tool breakdown) and the Next.js server detail page that also surfaces connected agent bindings.
- **Enterprise Identity & Permissions (RBAC)**: Declarative policies with allow/deny rules and strict cross-domain authorization.
- **Cryptographic Audit Trail**: Tamper-proof, verifiable log of every execution — who, what model, tools, cost, and outcome.
- **AI FinOps & Budget Manager**: Spend limits, velocity alerts, burn-rate forecasts, and model cost recommendations.
- **Multi-Tenant Isolation**: Organization → Workspace → Project → Agent hierarchy with row-level security and encrypted credential vaults.
- **Observability**: OpenTelemetry tracing and Prometheus metrics for requests, latency, token usage, and cost analytics.
- **Admin Console**: A high-density dashboard for managing providers, agents, budgets, routing, and telemetry.

---

## Monorepo & Technology Stack

- **`apps/api`**: Go 1.25 API proxy engine and control plane.
- **`apps/app`**: Next.js 16 admin console and dashboard.
- **`apps/web`**: Astro 5.0 marketing landing page.
- **`packages/sdk`**: Official TypeScript SDK (`@roozylabs/prism`) for chat, routing, workspaces, governed tools/MCP (`/v1/mcp`), and analytics.
- **`packages/cli`**: Official CLI (`@roozylabs/prism-cli`).
- **`packages/python-sdk`**: Python SDK (`roozylabs-prism`).
- **Database**: PostgreSQL 15
- **Cache & State Store**: Redis 7 (rate limiting, cooldown, distributed budget reservations, tenant keyspaces)
- **CI/CD**: GitHub Actions (linting, tests, backup/restore RLS drill, container registry, automated blue/green VPS deployment)

---

## Quick Start

### 1. Prerequisites

- [Docker & Docker Compose](https://docs.docker.com/get-docker/) installed on your system.
- `openssl` (or another secret generator) to generate `.env` credentials before first boot.
- [Go 1.25+](https://go.dev/dl/) and [pnpm 9+](https://pnpm.io/) (if running locally without Docker).

### 2. Connect a Client (OpenCode CLI example)

Add Prism to your `~/.config/opencode/opencode.jsonc`:

```jsonc
{
  "provider": "prism",
  "api_url": "https://api.prism.roozylabs.com",
  "api_key": "prism_sk_8f9a2b1c...",
  "model": "prism-auto", // Smart Router selects the best model dynamically!
  "headers": {
    "X-Prism-Agent-ID": "opencode-cli-agent"
  }
}
```

### 3. Run with Docker Compose

1. Clone the repository and create `.env` from the template:

```bash
git clone https://github.com/roozylabs27/prism.git
cd prism
cp .env.example .env
```

2. Populate required secrets and environment settings in `.env` **before** starting Compose (`.env.example` leaves secrets blank and defaults `APP_ENV=production`, so starting without editing `.env` fails immediately):

   - Generate random values for `POSTGRES_PASSWORD`, `REDIS_PASSWORD` (`openssl rand -hex 24`), and `ENCRYPTION_KEY` (`openssl rand -hex 32`).
   - **Local evaluation (`http://localhost:3000`)**: set `APP_ENV=development`, `TURNSTILE_DISABLED=true`, `ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000`, and `APP_PUBLIC_URL=http://localhost:3000` (plus `PUBLIC_URL=http://localhost:3000`).
   - **Production deployment**: keep `APP_ENV=production`, configure HTTPS origins in `ALLOWED_ORIGINS` / `APP_PUBLIC_URL`, configure Cloudflare Turnstile (`CLOUDFLARE_SECRET_KEY` and `NEXT_PUBLIC_CLOUDFLARE_SITE_KEY`), and review [docs/operations/configuration.md](docs/operations/configuration.md).

3. Build, start the stack, and verify gateway readiness:

```bash
docker compose up -d --build
docker compose ps
curl -fsS http://127.0.0.1:8080/ready
```

The stack binds on loopback by default:

- **Prism Gateway API**: `http://127.0.0.1:8080`
- **Admin Console Dashboard**: `http://127.0.0.1:3000`
- **Prism Landing Page**: `http://127.0.0.1:3001`

### 4. Next Steps

- Follow the full [Getting Started Guide](docs/getting-started.md) for complete `.env` walkthroughs (local vs. production), cURL, the [TypeScript SDK](docs/sdk-typescript.md), and the [Prism CLI](docs/cli.md).

---

## API Endpoints Summary

| Area | Endpoint Prefix | Description |
| :--- | :--- | :--- |
| **Health & Auth** | `/health`, `/ready`, `/api/auth/*` | Service readiness, login, session, OAuth |
| **Control Plane** | `/api/*` | Organizations, workspaces, projects, members, RBAC, approvals, onboarding |
| **MCP & Governance** | `/api/mcp/*`, `/api/approvals/*`, `/api/agent-templates/*` | MCP registry catalog, human approvals, agent templates |
| **Audit & Compliance** | `/api/audit-trail/*` | AI audit trails, admin action logs, HMAC verification & CSV/JSON export |
| **FinOps & Analytics** | `/api/budgets/*`, `/api/quotas/*`, `/api/analytics/*` | Budgets, quotas, alerts, anomalies, and client workspace pilot outcomes |
| **Model & Tool Gateway** | `/v1/chat/completions`, `/v1/messages`, `/v1/mcp`, `/v1/tools/*`, `/v1/resources/*` | OpenAI/Anthropic inference, inbound JSON-RPC MCP, governed tools/resources |

See the full [API Reference](docs/api-reference.md) for complete endpoint details, parameters, and authentication.

---

## Documentation

- [Getting Started](docs/getting-started.md) — first request with cURL, SDK, and CLI.
- [TypeScript SDK](docs/sdk-typescript.md) — `@roozylabs/prism` usage.
- [Prism CLI](docs/cli.md) — `@roozylabs/prism-cli` commands.
- [API Reference](docs/api-reference.md) — complete endpoint documentation.
- [Architecture](docs/ARCHITECTURE.md) — internal system design.

---

## License

Distributed under the [MIT License](LICENSE).
