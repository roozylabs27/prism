# Prism Landing Page (`apps/web`) — Proposed Content Architecture

**Baseline Commit SHA:** `a66d45f5b1d8ab1ea440d08dd0a11f04994dd2fa` (`a66d45f5`)  
**Target:** Streamline from 7 overlapping sections (~1,800 words) to **5 distinct, non-repetitive sections (~750 words)** while keeping every claim 100% verified against `apps/api`.

---

## Section-by-Section Outline

### Header (`SiteHeader.astro`)
- **Left**: Prism geometric triangle mark + `Prism` wordmark (no redundant badge).
- **Center Nav (Desktop)**: `Architecture` (`#architecture`), `Capabilities` (`#capabilities`), `Quickstart` (`#quickstart`), `Docs` (external), `GitHub` (external).
- **Right Controls**: Theme toggle (`44px`), Primary CTA `Self-host with Docker` (`#start`), and labeled `Menu`/`Close` toggle (`<1024px`) opening a smooth CSS Grid overlay drawer.

---

### Hero (`Hero.astro`)
- **Purpose**: Answer *what Prism is*, *what problem it solves*, *where it fits*, and *what to do next* in under 5 seconds.
- **Left Column**:
  - Eyebrow: `Self-hosted AI gateway · OpenAI-compatible & MCP`
  - H1: `One gateway for your AI models and tools.`
  - Supporting Copy: `Route requests across OpenAI, Anthropic, Google Gemini, and MCP servers through a single API. Manage provider credentials, workspace spend limits, and agent permissions from one place.`
  - Primary CTA: `Self-host with Docker` (`href="#start"`)
  - Secondary CTA: `Read the docs` (`href={links.docs}`, external)
- **Right Column (`PrismStage.astro`)**:
  - Static, de-animated geometric Prism diagram (`Application / Agent → Governance Ring → Prism Gateway → Model / MCP Server / Standby Credential`) paired with the **3-tab Illustrative Route Inspector** (`01 · Route`, `02 · MCP Gate`, `03 · Failover`). Explicitly labeled `FIG. 01 · ILLUSTRATIVE ROUTE INSPECTOR`.

---

### `01 · Architecture` (`Architecture.astro`, `layout="stacked"`)
- **Purpose**: Explain visually where Prism sits in the request lifecycle (`Application → Prism → Providers & MCP Tools`) and where its enforcement boundary stops.
- **Top Visual (`RequestPath.astro`)**:
  - 3-stage pipeline:
    1. `01 · CALLER`: AI Client or Agent (`Bearer YOUR_PRISM_API_KEY · X-Prism-Agent-ID`)
    2. `02 · PRISM GATEWAY CHECKS (IN ORDER)`:
       - `1. Gateway key scope`: Resolves workspace/project; rejects spoofed tenant headers (`403`).
       - `2. Quotas & budget`: Enforces spend, request & stream caps before upstream (`429`).
       - `3. Agent policy`: Checks allowed models, tools & `!deny` overrides (`403`).
       - `4. Route & failover`: Evaluates routing rules or `prism-auto`; pre-stream failover on `429`/`5xx`.
    3. `03 · UPSTREAM`: Providers & MCP Tools (AES-256-GCM key decrypted on the fly; `X-Prism-Model`, `X-Prism-Provider`).
- **Bottom Split (`Governed vs. Outside` + `McpBoundary.astro`)**:
  - Left (`5 cols`): Concise list of what passes through Prism (`/v1/chat/completions`, `/v1/messages`, `/v1/mcp`, `/v1/tools/:name/execute`, `/v1/resources/:name/query`) vs. what runs outside Prism (local shell commands, direct provider keys, unproxied MCP servers).
  - Right (`7 cols`): `McpBoundary.astro` 3-call illustrative trace (`Allowed`, `Denied`, `Not seen`).

---

### `02 · Capabilities` (`Capabilities.astro`, `layout="stacked"`)
- **Purpose**: Replace 3 fragmented sections (`Agencies`, `InferencePlane`, `AgentPlane`) with **3 hairline-divided capability bands** (zero card grids, zero repeated feature bullets):
  1. **`01 · Routing & failover`**:
     - Wildcard routing rules (`/api/routing-rules`) + `prism-auto` policy scoring (`X-Routing-Policy`) + Redis `Retry-After` cooldowns and pre-stream credential failover (spending recorded only on the final completed response).
  2. **`02 · Governance & isolation`**:
     - Workspace/project separation with pinned `prism_sk_*` keys, AES-256-GCM encrypted provider keys, org/workspace/project quotas, and per-agent allowlists with `!deny` precedence and `maxBudgetCents` — paired directly with the compact `POST /api/agents` example JSON snippet.
  3. **`03 · Visibility & security`**:
     - `X-Request-ID` correlation, structured JSON logs with secret redaction, Prometheus `/metrics` (`METRICS_TOKEN`), CSV/JSON workspace outcome exports, HMAC-verified execution audit trails, admin audit logs, dial-time SSRF guards, and `REDIS_FAIL_CLOSED` protection.

---

### `03 · Quickstart` (`Integrate.astro` + `CodeTabs.astro`, `layout="split"`)
- **Purpose**: Show real, immediately understandable developer integration without documentation overload.
- **3 Core Tabs**:
  1. **`TypeScript`** (Official `openai` SDK pointed at `http://localhost:8080/v1`, `YOUR_PRISM_API_KEY`, `model: "prism-auto"`)
  2. **`cURL`** (`POST http://localhost:8080/v1/chat/completions`, `Authorization: Bearer YOUR_PRISM_API_KEY`)
  3. **`MCP (Claude Code)`** (`claude mcp add --transport http prism http://localhost:8080/v1/mcp` with `X-Prism-Agent-ID`)
- Each tab renders: concise 1-sentence setup note, copyable code block, and a clean 1-line `Expected response` summary. Python, OpenCode CLI, Claude Desktop, and error code troubleshooting link directly to `docs/getting-started.md` and `docs/`.

---

### `04 · Start` (`WaysToStart.astro`, `layout="stacked"`)
- **Purpose**: Give engineering teams and agencies a crisp, honest decision between self-hosting and the guided staging pilot.
- **2 Clean Columns**:
  1. **Self-host with Docker Compose**: Explicit software/provider cost split, prerequisites (`Docker + Docker Compose`, `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `ENCRYPTION_KEY (32+ chars)`, `APP_ENV=development` for local runs), copyable `docker compose up -d --build` command, and `Open Quick Start on GitHub` link (`links.quickstart`).
  2. **Design-partner staging pilot**: For agencies with 5–30 client accounts and 2+ staging workflows ready for a 7–14 day guided setup, clear 3-step involvement, and `Apply for a design-partner pilot` CTA (`links.pilotRequest`, noting GitHub account requirement + link to pilot charter).

---

### Footer (`SiteFooter.astro`)
- Concise 2-line product summary + stack note (`Self-hosted Go gateway · PostgreSQL · Redis`), 4 section anchor links (`Architecture`, `Capabilities`, `Quickstart`, `Start`), and verified GitHub resource links (`Repository`, `Getting started`, `Documentation`, `Pilot charter`, `Changelog`, `Issues`).
