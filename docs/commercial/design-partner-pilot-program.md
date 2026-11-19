# Prism — Design Partner Pilot Program & Client #N+1 Onboarding Runbook (Recommendation 2)

> **Validation Status Notice:** This guide packages operational charters and technical API workflows enabled in `apps/api`. **Zero external design-partner pilots have been completed yet.** Do not pitch Prism as unattended multi-tenant production SaaS until remaining open P0 items (`P0-5`, `P0-6`, `P0-7` in [prism-product-engineering-roadmap.md](file:///c:/me/roozylabs/projects/prism/docs/roadmap/prism-product-engineering-roadmap.md)) are closed.

**Pilot Cohort Target:** **3–5 AI Automation Agencies** running **>= 2 isolated client workspaces** each for **7–14 days** on a staging workflow.

---

## 1. Staging Pre-Flight Boundary (Honest Capabilities vs Open Gates)

| Workflow | Ready to Pilot on Staging Today | Open Constraint / Gate |
|---|---|---|
| **Client Workspaces & Projects (`POST /api/workspaces`, `/projects`)** | **Yes (Backend API Live)** | Frontend `TenantSelector` still shows organizations only; onboard Client #2+ via API / [onboard-client-workspace.sh](file:///c:/me/roozylabs/projects/prism/scripts/pilot/onboard-client-workspace.sh). |
| **Client-Pinned Gateway Keys (`workspaceId` + `projectId`)** | **Yes** (`POST /api/gateway-keys` validates org ownership) | Spoofed `X-Prism-Workspace-ID` / `X-Prism-Project-ID` headers reject with `403`. |
| **OpenAI-Compatible Workflows (n8n, LangChain/LangGraph, CrewAI, Custom)** | **Yes** (`/v1/chat/completions`, `prism-auto`, `X-Routing-Policy`) | Native Anthropic `/v1/messages` (Claude Code `ANTHROPIC_BASE_URL`) is **P1-2 open**. |
| **Governed Inbound MCP (`POST /v1/mcp`) + `delegate_ai_task`** | **Yes** (Streamable HTTP JSON-RPC 2.0) | Client apps using native vendor chats keep their own main chat model; MCP governs exposed tools + delegated tasks. |
| **Workspace/Project Quotas, Daily Caps & Agent Boundaries** | **Yes** (`PUT /api/quotas/workspace/:id`, `!deny`, `MaxBudgetCents`) | Gateway key does not bind `agent_id` yet (`P0-5`); caller must pass `X-Prism-Agent-ID` header. Human approvals (`P1-4`) not built. |
| **Team Collaboration & Multi-Member Shared Views** | **Constrained** (`user_id`-scoped repos `P0-6`) | During staging pilot, operate each agency org under one primary operator account rather than relying on multi-member resource sharing. |
| **Client Outcomes Attribution & CSV/JSON Export** | **Yes** (`GET /api/analytics/pilot-outcomes`, `/export`) | Cost savings report modeled `$3/$15` flagship delta and requires operator quality sign-off. |

---

## 2. Design Partner Pilot Charter Template

Use this lightweight 1-page agreement (unpaid 14-day staging pilot) to align expectations honestly:

1. **Scope:** Pilot Partner connects **2 isolated client workspaces** (e.g., `Client-Alpha-Staging`, `Client-Beta-Staging`) running real workflow prompts (n8n, LangGraph, CrewAI, custom app, or HTTP MCP client -> `POST /v1/mcp`).
2. **Boundary Disclosure:** Prism acts strictly on traffic routed through `/v1/*` and `/v1/mcp` on dedicated staging/isolated environments. Partner retains zero-downtime fallback to direct endpoints if staging requires maintenance.
3. **Commitments (Founder Side):** Dedicated onboarding call (<30 min), Slack/WhatsApp support channel, weekly cost & denial review using `GET /api/analytics/pilot-outcomes/export?format=csv`.
4. **Commitments (Agency Partner Side):**
   - Log baseline weekly ops time spent on key management & client cost spreadsheets prior to Day 1 ([pilot-outcomes-scorecard.md](file:///c:/me/roozylabs/projects/prism/docs/commercial/pilot-outcomes-scorecard.md)).
   - Run real staging traffic for >=7 days across both client workspaces.
   - Deliberately trigger >=1 test hard budget cap (`429`) and >=1 blocked tool/model (`403` / `-32001`) to verify policy enforcement end-to-end.
   - Complete a 20-minute Week-2 readout reviewing savings, quality delta, and WTP decision (`\$49` / `\$129` / `\$199/mo`).

---

## 3. <10-Minute Client #N+1 Onboarding Runbook

Target: **< 10 minutes** from "new pilot client signed" to an isolated, quota-capped workflow sending traffic through Prism.

### Option A — Automated CLI Onboarding (~60 seconds)
Run [onboard-client-workspace.sh](file:///c:/me/roozylabs/projects/prism/scripts/pilot/onboard-client-workspace.sh):
```bash
export PRISM_API_URL="http://localhost:8080"
export PRISM_SESSION_COOKIE="better-auth.session_token=..."
export PRISM_ORG_ID="org_..."
export CLIENT_NAME="Acme Retail Staging"
export PROJECT_NAME="Support Triage Bot"
export DAILY_SPEND_LIMIT_USD="5.00"
export MONTHLY_SPEND_LIMIT_USD="100.00"
export DAILY_REQUEST_LIMIT="1000"

bash scripts/pilot/onboard-client-workspace.sh
```
This script atomically creates:
1. Isolated Client Workspace (`POST /api/workspaces`)
2. Client Bot Project (`POST /api/workspaces/:workspaceId/projects`)
3. Hard Client Quotas (`PUT /api/quotas/workspace/:id`)
4. Workspace+Project Pinned Gateway Key (`POST /api/gateway-keys` returning the one-time `prism_sk_*` secret)

### Option B — Step-by-Step API Calls
1. **Create Client Workspace:**
   `POST /api/workspaces` with `X-Prism-Org-ID: <org_id>` and `{"name": "Client Beta"}` -> capture `id` (`ws_...`).
2. **Create Project Inside Workspace:**
   `POST /api/workspaces/<ws_id>/projects` with `{"name": "Invoice Extractor"}` -> capture `id` (`proj_...`).
3. **Set Workspace Quota Caps:**
   `PUT /api/quotas/workspace/<ws_id>` with `{"monthlySpendLimitUsd": 100, "dailySpendLimitUsd": 5, "dailyRequestLimit": 1000}`.
4. **Mint Pinned Client Gateway Key:**
   `POST /api/gateway-keys` with `{"name": "client-beta-prod", "workspaceId": "<ws_id>", "projectId": "<proj_id>", "rateLimit": 60}`.
5. **(Optional) Register Restricted Agent:**
   `POST /api/agents` with `allowedModels`, `allowedTools` (e.g. `["*", "!delete_customer"]`), `allowedMcpServers`, and `maxBudgetCents`.

---

## 4. Client Runtime Wiring Examples

### n8n / OpenAI Python or TypeScript SDK
Point `base_url` to `<PRISM_URL>/v1` with the client-pinned `prism_sk_*` key:
```python
from openai import OpenAI

client = OpenAI(
    base_url="https://your-staging-prism.example.com/v1",
    api_key="prism_sk_client_beta...",
    default_headers={
        "X-Prism-Agent-ID": "ag_invoice_extractor",
        "X-Routing-Policy": "cost-saver",
    },
)
```
*Attribution & isolation check:* Even if a developer sends a conflicting `X-Prism-Workspace-ID: ws_other`, Prism rejects it with `403 TENANT_SECURITY_ERROR`.

### MCP Clients (`POST /v1/mcp`)
Configure the MCP HTTP endpoint (`https://your-staging-prism.example.com/v1/mcp`) with:
- `Authorization: Bearer prism_sk_client_beta...`
- `X-Prism-Agent-ID: ag_invoice_extractor`

Exposed tools automatically filter to the agent's allowed MCP servers/tools plus `delegate_ai_task`, returning explicit `executed_model`, `executed_provider`, and `cost_usd` in every delegated result.
