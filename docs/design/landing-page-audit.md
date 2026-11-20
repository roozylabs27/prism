# Landing Page Audit (apps/web)

Date: 2026-11-19. Audited file: `apps/web/src/pages/index.astro` (1,012 lines) plus `Layout.astro`, `BackgroundAnimation.astro`, `globals.css`, `astro.config.mjs`, `tailwind.config.mjs`.

Every claim was checked against code in `apps/api` and the docs in `docs/`. Status of the redesign decisions: approved by the product owner on 2026-11-19 (AntiSlop mode: DURING).

## 1. Positioning

| # | Finding | Evidence |
|---|---|---|
| P1 | The Agent Operations Control Plane is absent. MCP Gateway, agent identity, tool/resource allowlists, audit records are built and tested but not shown. | Page sections: hero, stats, routing, failover, governance, pricing, providers, CTA |
| P2 | The strongest buyer (agencies running 5 to 30 client accounts) is never addressed, though workspaces per client, scoped gateway keys, per-client budgets and usage export exist. | `docs/product/agency-use-cases.md`, `/api/workspaces`, `/api/gateway-keys`, `/api/analytics/pilot-outcomes` |
| P3 | The hero sells a commodity proxy ("One API endpoint for every LLM"), which puts Prism next to generic routers. | `index.astro` L257-260 |
| P4 | The page never states where Prism's enforcement boundary ends, which is the product's most credible differentiator. | n/a |

## 2. Unsupported or false claims

| # | Claim | Reality | Decision |
|---|---|---|---|
| C1 | "Zero downtime, automatic failover" | Absolute. When every credential fails, the client receives an error. | Removed |
| C2 | "Cut token bills up to 70%" | No measurement exists | Removed |
| C3 | Routing simulator "Live Decision Engine", "100% summary accuracy" | Hardcoded JS object (L970-989) | Removed |
| C4 | "Fails 3 times, 60-second quarantine" | Cooldown is per credential and derived from `Retry-After` (`proxy/engine.go`, `determineCooldownDuration`) | Rewritten to match code |
| C5 | "Stream re-established in 1.96ms", "client never receives a 50x", "$0.00 retry billing" guarantee | Invented or absolute | Removed |
| C6 | Claude Code via `ANTHROPIC_BASE_URL` | No `POST /v1/messages` endpoint exists | Replaced with MCP setup against `/v1/mcp` |
| C7 | OpenCode JSON config | Schema not verified against OpenCode | Removed |
| C8 | Stats banner: 1.96 ms p95, 3,585 req/s, "0 Data Races", "100% RLS" | Single internal load test (`docs/production-hardening-load-chaos.md`); RLS enabled in 11 migrations, "100%" unverified | Banner removed |
| C9 | "Sub-2ms AI Gateway Engine • Active" badge, "sub-2ms" meta description | Same single test; "Active" is a fake status | Removed |
| C10 | GPT-4o, Claude 3.7 Sonnet, Gemini 2.5, o1 | Outdated relative to the model catalog | Providers named without model versions |
| C11 | Four paid tiers with limits, SSO, dedicated instance, 24/7 support | `FEATURE_FLAG_BILLING=false`; `UpgradeSubscription` is an unpersisted stub; features disagree with `repository/billing.go` | Replaced with "Ways to start" |
| C12 | "Open-source", links to `github.com/roozylabs/prism` | Repo is `roozylabs27/prism`; README badge says MIT but no `LICENSE` file exists | Links fixed; no licence claim until a LICENSE file is committed |
| C13 | `docs.prism.roozylabs.com`, `/health`, `/ready` production links | Hosts not verified | Docs link to repository `docs/` |
| C14 | "Tamper-proof audit trail" (README wording, implied by governance copy) | `docs/audit/system-wide-product-integrity-audit.md` #2: audit delivery is in-memory and hashes do not cover bodies | Page says "audit records", no tamper-proof claim |

## 3. UX and conversion

- Four CTAs read "Launch Console" and all point to a hosted sign-in that is not confirmed public. No low-commitment path.
- Three consecutive 5/7 split sections with identical structure.
- Pricing appears before the visitor knows what Prism connects to.

## 4. AntiSlop violations (core + UI + copy)

- Violet glow shadows on CTAs and the highlighted pricing card.
- Uppercase pill eyebrow on every section; emerald checkmark list on every section.
- Fake terminal and dashboard costumes with invented data (`acme-corp`, `$184.20 / $500.00`).
- Full-viewport canvas with continuous "micro-glitch" animation; the rAF loop keeps running under `prefers-reduced-motion`.
- Generic headings: "Transparent plans for every scale", "Connect to any provider or framework".
- "Recommended for Devs" badge.

## 5. Accessibility

- Body copy at `text-xs`; widespread `text-[10px]` and `text-[11px]`.
- Touch targets at 32px.
- Code tabs lack `role="tab"` and `aria-selected`; copy feedback lacks `aria-live`.
- Mobile menu has no Escape handling or focus return.
- Theme init bug: `|| !storedTheme` forces dark and ignores a light system preference.

## 6. Technical

- Single 1,012-line page mixing data, markup and four scripts.
- `@astrojs/react`, `react`, `react-dom`, `lucide-react` used only to render icons.
- No Open Graph, Twitter, canonical, robots or sitemap.
- Playwright not installed; verification uses Chrome DevTools MCP.
