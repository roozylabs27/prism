# Prism Landing Page (`apps/web`) — Content & UX Remediation Log

**Repository:** `roozylabs/prism`  
**Baseline Commit SHA:** `775eb2460d2676745ea0517219a0cd08a4a5dc45` (`775eb24`)  
**Scope:** `apps/web` and `docs/landing-page-*.md`  
**Date:** 2026-11-21

---

## 1. Before / After Copy & Component Changes

| Area / File | Before | After | Reason for Change |
|---|---|---|---|
| **SEO Title & Description** (`src/data/site.ts`, `src/layouts/Layout.astro`) | *Title:* `Prism: the control layer between AI agents, models, and tools`<br>*Description:* `Prism is a self-hostable AI gateway that gives each workspace and project its own keys, spend quotas, model routing, and MCP tool permissions.` | *Title:* `Prism: AI Gateway, Model Routing & MCP Tool Control`<br>*Description:* `Prism is a self-hostable AI gateway for routing requests across supported model providers and MCP tool servers. Manage provider credentials, routing policies, agent permissions, and workspace spend controls from one place.` + `SoftwareApplication` JSON-LD | Aligns title and meta description with verified gateway, routing, credential, and MCP capabilities; adds structured data without duplicating page headline prose. |
| **Header & Footer Version / License** (`SiteHeader.astro`, `SiteFooter.astro`, `content.ts`) | Header badge: `v0.4.0`<br>Footer: `MIT Licensed · Go + PostgreSQL + Redis` and `© 2026 RoozyLabs. Prism v0.4.0.`<br>Self-host tag: `MIT Licensed · Local or VPS` | Header badge: `AI Gateway`<br>Footer: `Self-hosted Go gateway · PostgreSQL · Redis` and `© 2026 RoozyLabs. Source and release history on GitHub.`<br>Self-host tag: `Self-hosted · Local Docker or VPS` | Removes unverified `"MIT Licensed"` claim (`LICENSE` file is missing from the repo and returns `404` on GitHub) and removes stale hardcoded `v0.4.0` version string (Section 11). |
| **Hero Failover Qualification** (`Hero.astro`) | `...route across encrypted provider credentials with automatic failover, and keep raw API keys off caller machines.` | `...route across encrypted provider credentials with pre-stream failover, and keep raw API keys off caller machines.` | Clarifies that streaming requests only fail over before the first token is sent to the caller (`engine.go:980-987`). |
| **Section 01 — Governed Endpoints** (`src/data/content.ts`) | `Model inference sent to /v1/chat/completions (sync or SSE stream) and /v1/chat/completions/async (polled via /v1/jobs/:jobId)` | `Model inference sent to /v1/chat/completions (sync or SSE stream), /v1/messages (Anthropic format), and /v1/chat/completions/async (polled via /v1/jobs/:jobId)` | Reflects the implemented `POST /v1/messages` route (`main.go:621`) alongside `/v1/chat/completions`. |
| **Section 03 — Failover & Billing Accounting** (`src/data/content.ts`) | `When an upstream credential returns 429 or 5xx, Prism places it in a Redis cooldown honoring Retry-After and retries the next eligible credential. Only the final successful attempt is billed; if all candidates fail, the caller receives the error.` | `When an upstream credential returns 429 or 5xx (or fails before the first token on an SSE stream), Prism places it in a Redis cooldown honoring Retry-After and retries the next eligible credential. Prism only records token spend for the final completed response; if all candidates fail, the caller receives the error.` | Distinguishes pre-first-token SSE failover from mid-stream interruptions, and clarifies that zero-cost retries refer to Prism's internal spend ledger rather than upstream vendor invoices. |
| **Illustrative Data & Example Labels** (`PrismStage.astro`, `McpBoundary.astro`, `AgentPlane.astro`) | `FIG. 01 · INSPECT ROUTE PATH`<br>`Agent support-bot-acme makes three calls`<br>`POST /api/agents · JSON policy` | `FIG. 01 · ILLUSTRATIVE ROUTE INSPECTOR`<br>`Illustrative trace: agent support-bot-acme makes three calls`<br>`POST /api/agents · Example configuration` | Explicitly distinguishes static illustrative examples from live telemetry or a live routing simulator (Section 11). |
| **Section 05 — Integration Quickstarts** (`src/data/integrations.ts`, `CodeTabs.astro`, `Integrate.astro`) | 5 tabs (`Python`, `TypeScript`, `cURL`, `Claude Code (MCP)`, `Claude Desktop (MCP)`) using `prism_sk_...` or `$PRISM_KEY`, mixed `extra_headers` vs `defaultHeaders`, `-H "X-Routing-Policy: cost-saver"` in `cURL`, and no response/error documentation. | 6 tabs (`Python (OpenAI SDK)`, `TypeScript (OpenAI SDK)`, `cURL`, `OpenCode CLI`, `Claude Code (MCP)`, `Claude Desktop (MCP)`) using `YOUR_PRISM_API_KEY`, consistent `default_headers`/`defaultHeaders`, a base `cURL` request that works without pre-creating a `cost-saver` policy, plus per-tab **Prerequisites**, **Expected response**, and **Common failure cases**. | Meets all P0 Section 6 requirements and prevents new users from hitting `403 AGENT_NOT_FOUND` or `404 routing_policy_not_found` on their first test call. |
| **Section 06 — Self-Host & Commercial Clarity** (`src/data/content.ts`, `WaysToStart.astro`) | `For engineering teams evaluating Prism on their own Docker host...` + `Clone the repository, copy .env.example to .env, and set your database/encryption secrets.` | Explicitly distinguishes commercial costs (`No Prism software fee; you run the containers and pay your upstream AI providers directly for model usage`), names the exact `.env` keys required by `docker-compose.yml` and `config.go` (`POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `ENCRYPTION_KEY (32+ chars)`, and `APP_ENV=development`), and notes that the pilot request link opens a GitHub issue template (requires a GitHub account). | Prevents `docker compose up` startup failure from blank `.env.example` secrets and satisfies P0 Section 5 commercial cost separation. |

---

## 2. Removed or Qualified Claims

1. **Removed:** `"MIT Licensed"` in `SiteFooter.astro` and `content.ts` (contradicted by missing `LICENSE` file in repository).
2. **Removed:** Hardcoded `v0.4.0` version badges in `SiteHeader.astro` and `SiteFooter.astro`.
3. **Removed:** Unseeded `-H "X-Routing-Policy: cost-saver"` from the default `cURL` snippet (which caused `404 routing_policy_not_found` on fresh setups); documented in the tab note that `X-Routing-Policy` selects user-created policies while omitting it uses the built-in `balanced` policy.
4. **Qualified:** Automatic failover qualified to `pre-stream failover` / `before the first token on an SSE stream`.
5. **Qualified:** `"Only the final successful attempt is billed"` replaced with `"Prism only records token spend for the final completed response"`.
6. **Qualified:** Claude Code `POST /v1/messages` support noted as available in `apps/api` and currently marked experimental for full Claude Code CLI inference while `/v1/mcp` is the primary governed MCP integration.

---

## 3. CTA Audit & Remediation

| CTA Label | Location | Destination | Auth Requirement | Expected User Outcome | Target Route Verified? |
|---|---|---|---|---|---|
| `Self-host with Docker` | `SiteHeader.astro`, `Hero.astro`, `WaysToStart.astro` | `https://github.com/roozylabs27/prism#quick-start` | None (Public GitHub repo, HTTP `200`) | Jumps to the Quick Start section in the public GitHub repository README. | **Yes (`200 OK`)** |
| `Apply for a design-partner pilot` | `Hero.astro`, `SiteHeader.astro` (mobile) | `#start` (in-page anchor to Section `06 · Start`) | None | Scrolls to Section `06 · Start` where pilot prerequisites, steps, and deliverables are explained before opening GitHub. | **Yes (In-page `#start`)** |
| `Apply for a design-partner pilot` | `WaysToStart.astro` (Pilot card CTA) | `https://github.com/roozylabs27/prism/issues/new?title=Design%20partner%20pilot%20request` | GitHub account required to submit issue (disclosed in `expectAfterClick`) | Opens a pre-titled GitHub Issue on `roozylabs27/prism` to submit pilot workload details. | **Yes (`200 OK`)** |
| `Docs` / `documentation` | `SiteHeader.astro`, `Integrate.astro`, `SiteFooter.astro` | `https://github.com/roozylabs27/prism/tree/main/docs` | None (Public GitHub repo) | Opens the `docs/` directory on GitHub. | **Yes (`200 OK`)** |
| `getting started guide` | `Integrate.astro`, `SiteFooter.astro` | `https://github.com/roozylabs27/prism/blob/main/docs/getting-started.md` | None (Public GitHub repo) | Opens `docs/getting-started.md` on GitHub. | **Yes (`200 OK`)** |
| `design-partner pilot program charter` | `WaysToStart.astro`, `SiteFooter.astro` | `https://github.com/roozylabs27/prism/blob/main/docs/commercial/design-partner-pilot-program.md` | None (Public GitHub repo) | Opens the 1-page pilot charter and runbook on GitHub. | **Yes (`200 OK`)** |
| `GitHub` / `Changelog` / `Issues` | `SiteHeader.astro`, `SiteFooter.astro` | `https://github.com/roozylabs27/prism` (`/blob/main/CHANGELOG.md`, `/issues`) | None (Public GitHub repo) | Opens the repository, changelog, or issue tracker on GitHub. | **Yes (`200 OK`)** |

---

## 4. Files Modified

- `apps/web/src/data/site.ts`
- `apps/web/src/data/content.ts`
- `apps/web/src/data/integrations.ts`
- `apps/web/src/layouts/Layout.astro`
- `apps/web/src/components/layout/SiteHeader.astro`
- `apps/web/src/components/layout/SiteFooter.astro`
- `apps/web/src/components/sections/Hero.astro`
- `apps/web/src/components/sections/AgentPlane.astro`
- `apps/web/src/components/sections/Integrate.astro`
- `apps/web/src/components/ui/CodeTabs.astro`
- `apps/web/src/components/diagrams/PrismStage.astro`
- `apps/web/src/components/diagrams/McpBoundary.astro`
- `docs/landing-page-content-audit.md`
- `docs/landing-page-remediation.md`
- `docs/landing-page-production-readiness.md`

---

## 5. Remaining Product-Owner Decisions

1. **Commit a `LICENSE` file at the repository root**: `README.md` links to `LICENSE` (MIT), which currently returns `404` on GitHub. Once a `LICENSE` file is added to the repository, `"MIT Licensed"` can be restored to `apps/web`.
2. **Fix 3 broken `file:///` links in `docs/getting-started.md` (lines 119–121)**: Those lines currently point to `file:///c:/me/projects/ai-gateway/docs/...` instead of relative links (`./sdk-typescript.md`, `./cli.md`, `./api-reference.md`).
3. **Direct contact / email alternative for Design-Partner Pilot applications**: Currently `Apply for a design-partner pilot` opens a public GitHub Issue (`links.pilotRequest`), which requires a GitHub account and posts the agency's inquiry publicly. Decide whether to add a `mailto:` or private intake form for agencies that prefer not to post client counts in a public issue.
4. **Legal pages before enabling hosted SaaS (`app.prism.roozylabs.com`)**: Before turning on `FEATURE_FLAG_BILLING=true` or promoting public self-serve signups on the landing page, publish verified Privacy Policy, Terms of Service, and Acceptable Use Policy pages.
