# Prism Landing Page (`apps/web`) — Production Readiness Report

**Repository:** `roozylabs/prism`  
**Baseline Commit SHA:** `775eb2460d2676745ea0517219a0cd08a4a5dc45` (`775eb24`)  
**Target:** `apps/web` (`https://prism.roozylabs.com`)  
**Final Readiness Verdict:** **Ready with limitations** (`apps/web` is 100% production-ready; external repo items such as root `LICENSE` file and legal pages before hosted SaaS billing remain open owner decisions)  
**Date:** 2026-11-21

---

## 1. Readiness Summary

| Dimension | Verdict | Notes |
|---|---|---|
| **Content & Positioning Readiness** | **READY** | Clear control-plane positioning; zero AI marketing buzzwords; zero em dashes (`—`); explicit boundary between governed and ungoverned calls. |
| **Product Claim Verification** | **READY** | All unverified claims (`MIT Licensed`, hardcoded `v0.4.0`) removed; streaming failover and internal spend metering accurately qualified. |
| **Integration & Quickstart Accuracy** | **READY** | 6 verified tabs using `YOUR_PRISM_API_KEY`, consistent `default_headers`/`defaultHeaders`, prerequisites, expected response shapes, and HTTP/JSON-RPC error codes. |
| **Pricing & Commercial Accuracy** | **READY** | Hosted billing (`FEATURE_FLAG_BILLING`) is disabled in backend; landing page accurately offers only Self-host (Docker Compose) and Design-partner staging pilot, and separates Prism software ($0 self-hosted), infrastructure, and upstream provider token costs. |
| **CTA & Link Integrity** | **READY** | Every in-page anchor and external GitHub link verified (`HTTP 200 OK`); no dead buttons or misleading labels. |
| **SEO & Metadata** | **READY** | Clean `<title>`, `<meta name="description">`, canonical URL, Open Graph, Twitter tags, `robots.txt`, `sitemap.xml`, and `SoftwareApplication` JSON-LD. |
| **Accessibility (WCAG AA)** | **READY** | All light and dark token pairs pass WCAG AA (`5.22:1` to `16.60:1`); skip-to-content link; keyboard-operable tabs (`ArrowLeft`/`ArrowRight`/`Home`/`End`) and mobile drawer (`Escape` + focus return); `prefers-reduced-motion` respected. |
| **Responsive UX (`1440` / `1280` / `768` / `390`)** | **READY** | Browser-verified zero horizontal overflow (`scrollWidth <= innerWidth`) across all 4 viewports; all standalone interactive controls `>= 44px` height. |
| **Build & Deployment (Vercel + Docker)** | **READY** | Both Vercel static output (`VERCEL=1` → `apps/web/dist/index.html`) and Docker standalone Node output (`apps/web/dist/server/entry.mjs`) build with 0 errors. |

---

## 2. Integration Verification Matrix

| Tab | Target Endpoint / Config | Auth Header | Tested Behavior & Edge Cases | Status |
|---|---|---|---|---|
| **Python (OpenAI SDK)** | `http://localhost:8080/v1` (`/v1/chat/completions`) | `Authorization: Bearer YOUR_PRISM_API_KEY` | Uses `default_headers={"X-Prism-Agent-ID": "support-bot-acme"}`; documents `401 API_KEY_PREFIX_INVALID`, `403 AGENT_NOT_FOUND`, and `429 budget_exceeded_error`. | **VERIFIED** |
| **TypeScript (OpenAI SDK)** | `http://localhost:8080/v1` (`/v1/chat/completions`) | `Authorization: Bearer YOUR_PRISM_API_KEY` | Uses `defaultHeaders`; documents `X-Prism-Model`, `X-Prism-Routing-Policy`, `401 API_KEY_INVALID`, `403 TENANT_SECURITY_ERROR`, `429 max_concurrent_streams_exceeded`. | **VERIFIED** |
| **cURL** | `POST http://localhost:8080/v1/chat/completions` | `Authorization: Bearer YOUR_PRISM_API_KEY` | Works out of the box with `model: "prism-auto"` (falls back to built-in `balanced` policy); documents `404 routing_policy_not_found` if an uncreated `X-Routing-Policy` is passed. | **VERIFIED** |
| **OpenCode CLI** | `~/.config/opencode/opencode.jsonc` (`baseURL: http://localhost:8080/v1`) | `apiKey: "YOUR_PRISM_API_KEY"` | Matches schema from `prism-guide` (`$schema: https://opencode.ai/config.json`, `provider.prism.options.baseURL`). | **VERIFIED** |
| **Claude Code (MCP)** | `POST http://localhost:8080/v1/mcp` (`claude mcp add --transport http`) | `Authorization: Bearer YOUR_PRISM_API_KEY` + `X-Prism-Agent-ID: claude-code` | Verified against `MCPServerHandler` (`mcp_server.go`); documents JSON-RPC `-32001` error on policy/key denial and notes experimental `/v1/messages` support. | **VERIFIED** |
| **Claude Desktop (MCP)** | `claude_desktop_config.json` (`npx mcp-remote http://localhost:8080/v1/mcp`) | `Authorization: Bearer ${PRISM_KEY}` + `X-Prism-Agent-ID: claude-desktop` | Verified against `POST /v1/mcp`; documents JSON-RPC `-32001` error when a tool matches `!deny` or `maxBudgetCents` is exceeded. | **VERIFIED** |

---

## 3. Accessibility & WCAG AA Contrast Verification

Verified with `.agents/skills/antislop-human/contrast-check.py`:

| Theme | Foreground Token | Background Token | Contrast Ratio | WCAG AA Normal (`4.5:1`) | WCAG AA Large (`3.0:1`) |
|---|---|---|---|---|---|
| **Dark (Default)** | `--ink` (`#ECEAE4`) | `--paper` (`#111315`) | **15.48 : 1** | PASS | PASS |
| **Dark (Default)** | `--ink-muted` (`#A4A8AE`) | `--paper` (`#111315`) | **7.79 : 1** | PASS | PASS |
| **Dark (Default)** | `--ink-muted` (`#A4A8AE`) | `--paper-raised` (`#181B1E`) | **7.24 : 1** | PASS | PASS |
| **Dark (Default)** | `--accent` (`#B49CFB`) | `--paper` (`#111315`) | **8.06 : 1** | PASS | PASS |
| **Dark (Default)** | `--allow` (`#47CD89`) | `--paper` (`#111315`) | **9.19 : 1** | PASS | PASS |
| **Dark (Default)** | `--deny` (`#F97066`) | `--paper` (`#111315`) | **6.68 : 1** | PASS | PASS |
| **Light** | `--ink` (`#14161A`) | `--paper` (`#F6F5F1`) | **16.60 : 1** | PASS | PASS |
| **Light** | `--ink-muted` (`#555A61`) | `--paper` (`#F6F5F1`) | **6.37 : 1** | PASS | PASS |
| **Light** | `--ink-muted` (`#555A61`) | `--paper-raised` (`#FFFFFF`) | **6.95 : 1** | PASS | PASS |
| **Light** | `--accent` (`#6D28D9`) | `--paper` (`#F6F5F1`) | **6.51 : 1** | PASS | PASS |
| **Light** | `--allow` (`#067647`) | `--paper` (`#F6F5F1`) | **5.22 : 1** | PASS | PASS |
| **Light** | `--deny` (`#B42318`) | `--paper` (`#F6F5F1`) | **6.03 : 1** | PASS | PASS |

---

## 4. Responsive & Browser Verification (`1440px`, `1280px`, `768px`, `390px`)

| Viewport | `innerWidth` | `scrollWidth` | Horizontal Overflow | Verified Components & Interactions | Console Errors |
|---|---|---|---|---|---|
| **Desktop** | `1440px` | `1425px` | `false` (`0px`) | Hero 6/6 split, `PrismStage` 3-tab route inspector, `RequestPath` 3-column pipeline, 6-tab `CodeTabs` with `Expected response` / `Common failure cases` footer, `WaysToStart` 2-column tracks. | `0` |
| **Laptop** | `1280px` | `1265px` | `false` (`0px`) | Single-row desktop navigation (`min-h: 44px`), side-by-side `AgentPlane` definition list + JSON policy figure, full-width operator workflow table. | `0` |
| **Tablet** | `768px` | `753px` | `false` (`0px`) | `RequestPath` stages stack cleanly at full width; labeled `Menu` trigger (`>= 44px`) replaces desktop nav links; `WaysToStart` cards stack cleanly. | `0` |
| **Mobile** | `390px` (touch emulated) | `390px` | `false` (`0px`) | Zero horizontal scroll leak; code blocks and tablist scroll horizontally inside their own containers; mobile drawer opens/closes with `Menu`/`Close` label and `Escape` key; all standalone buttons/links `>= 44px` height. | `0` |

---

## 5. Build & Test Verification Results

- **Em-Dash Check (`apps/web/src/**/*`)**: `0` matches (`PASS`).
- **TypeScript Check (`pnpm --dir apps/web exec tsc --noEmit`)**: `0` errors (`PASS`).
- **Astro Node Standalone Build (`pnpm --dir apps/web build`)**: `dist/server/entry.mjs` and `dist/client/index.html` built cleanly (`PASS`).
- **Astro Vercel Static Build (`VERCEL=1 pnpm --dir apps/web build`)**: `dist/index.html`, `dist/_astro/*`, `dist/favicon.svg`, `dist/robots.txt`, and `dist/sitemap.xml` built cleanly at the root of `apps/web/dist/` (`PASS`).

---

## 6. Outstanding Repository / Business Blockers (Outside `apps/web`)

1. **Missing `LICENSE` file in repository root**: `README.md` displays an MIT license badge linking to `LICENSE`, which returns `404` on GitHub. (`apps/web` no longer claims `MIT Licensed` until `LICENSE` is added.)
2. **Broken `file:///` links in `docs/getting-started.md` (lines 119–121)**: `docs/getting-started.md` links to `file:///c:/me/projects/ai-gateway/docs/...` at the bottom of the file. Replace with relative Markdown paths in a docs commit.
3. **Legal & Compliance Pages (Privacy Policy, Terms of Service, Acceptable Use Policy)**: Not required while `prism.roozylabs.com` is an informational developer landing page linking to self-hosted GitHub code, but mandatory before enabling hosted SaaS signups or Stripe billing (`FEATURE_FLAG_BILLING`).
