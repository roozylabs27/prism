# Prism Landing Page (`apps/web`) — Strategic Redesign & Anti-Slop Audit

**Repository:** `roozylabs/prism` (`https://github.com/roozylabs27/prism`)  
**Baseline Commit SHA:** `a66d45f5b1d8ab1ea440d08dd0a11f04994dd2fa` (`a66d45f5`, branch `main`)  
**Recent `apps/web` Commits:** `b080f2ea`, `8dd47b41`, `7c7334b9`, `fc5bf78c`, `622c9f11`  
**Audit Date:** 2026-11-22

---

## 1. Executive Summary & Current File Density

While earlier remediation (`b080f2ea`, `7c7334b9`) removed fabricated claims (`MIT Licensed`, hardcoded `v0.4.0`) and added error codes to quickstarts, inspecting the live `apps/web/src` source at baseline `a66d45f5` reveals **cognitive overload, structural repetition, decorative motion that contradicts `MOTION 1`, and over-documented homepage copy**:

- `apps/web/src/data/content.ts`: **8,245 chars (~1,125 tokens)**
- `apps/web/src/data/integrations.ts`: **6,229 chars (~686 tokens)**
- `apps/web/src/components/diagrams/PrismStage.astro`: **14,269 chars (~1,223 tokens)**
- Total `apps/web/src`: **7 section blocks** (`Hero`, `Boundary`, `Agencies`, `InferencePlane`, `AgentPlane`, `Integrate`, `WaysToStart`) explaining the same mechanisms (`prism-auto`, `prism_sk_*`, `!deny`, `/v1/mcp`, failover) repeatedly across sections.

The goal of this strategic redesign is **clarity by subtraction**: compress 7 overlapping sections into **5 focused narrative sections** (`Hero`, `01 · Architecture`, `02 · Capabilities`, `03 · Quickstart`, `04 · Start`), eliminate decorative motion, remove repeated prose, and move reference documentation where it belongs (GitHub `docs/`).

---

## 2. Detailed Redesign Audit Findings

| ID | Location | Current behavior | Problem | Anti-Slop rule | Severity | Recommendation |
|---|---|---|---|---|---|---|
| **AUD-01** | `Hero.astro` | 52-word supporting paragraph + 3-column spec strip (`GATEWAY SURFACES`, `ENFORCEMENT`, `STACK`) + `PrismStage` (with 3 tabs) + `RequestPath` 4-gate box all stacked before Section 01. | Above-the-fold and Hero span attempts to explain every endpoint, deployment stack, and enforcement primitive simultaneously before the visitor finishes reading what Prism is. | `C-3`, `R-05` | **P0** | **Rewrite & simplify**: Keep Hero to 1 concise headline (`One gateway for your AI models and tools.`), 2 short sentences, 2 CTAs (`Self-host with Docker` → `#start`, `Read the docs` → `links.docs`), and `PrismStage`. Move `RequestPath` into `01 · Architecture`. |
| **AUD-02** | `PrismStage.astro`, `Hero.astro`, `site.ts` | Infinite 36s `@keyframes pr-orbital-drift`, infinite `@keyframes pr-pulse-glow`, SVG `<feGaussianBlur>` halos (`#pr-blur-lg`, `#pr-blur-sm`), Hero radial accent aura (`ellipse_at_74%_36%`), `Continuum Bridge` SVG divider, and mouse-move 3D tilt parallax (`setupPrismStageParallax`). | Endless animations and glowing/floating effects contradict the declared `MOTION 1` dial (*Hover/state transitions only*) and rely on decorative blur/glow tropes rather than clarity. | `R-01`, `R-13`, `R-19`, `antislop-ui` (*Endless Pulses and Loops*) | **P0** | **Redesign**: Remove infinite CSS animations, SVG Gaussian glow filters, Hero radial aura, `Continuum Bridge` divider, and JS pointer parallax. Keep the crisp geometric Prism crystal, boundary ring, and 3-mode Interactive Illustrative Route Inspector with clean 180ms opacity transitions. |
| **AUD-03** | `Boundary.astro`, `Agencies.astro`, `InferencePlane.astro`, `AgentPlane.astro`, `content.ts` | Same capabilities (`prism_sk_*`, `prism-auto`, `!deny`, `/v1/mcp`, failover) repeated 5–8 times across 4 separate sections (`01` through `04`). | Visitors read the same concepts in 4 different formats (2-col list, 5-row problem table, 2x2 grid, 6-term `<dl>` + JSON), creating fatigue instead of progressive understanding. | `C-3`, `R-05`, `antislop-copywriting` | **P0** | **Consolidate**: Merge `RequestPath` + `Boundary` + `McpBoundary` into **`01 · Architecture`** (`Application -> Prism -> Providers & Tools`), and merge `Agencies` + `InferencePlane` + `AgentPlane` into **`02 · Capabilities`** (3 concise editorial pillars divided by 1px rules). |
| **AUD-04** | `CodeTabs.astro`, `integrations.ts`, `Integrate.astro` | 6 dense tabs (`Python`, `TypeScript`, `cURL`, `OpenCode CLI`, `Claude Code (MCP)`, `Claude Desktop (MCP)`) each embedding multi-line prerequisites, full config files, expected responses, and error-code tables (`API_KEY_PREFIX_INVALID`, `AGENT_NOT_FOUND`, `routing_policy_not_found`, etc.). | Turns the homepage Quickstart section into inline reference documentation. | `C-3`, `R-05` | **P1** | **Simplify**: Keep 3 verified core tabs (`TypeScript`, `cURL`, `MCP (Claude Code)`) with `YOUR_PRISM_API_KEY`, one crisp `Expected response` summary, and a direct link to `docs/getting-started.md` for Python, OpenCode CLI, Claude Desktop, and error codes. |
| **AUD-05** | `WaysToStart.astro`, `content.ts` | Two ~150-word cards including a 5-item checkmark list (`Available during pilot`) that repeats features already shown earlier on the page, plus multi-line post-click prose. | Repeats the feature list a fifth time right above the footer and makes the final action visually heavy. | `C-3`, `R-14`, `R-15` | **P1** | **Simplify**: Keep both honest tracks (`Self-host with Docker Compose` with copyable command + explicit `.env` prerequisites, and `Design-partner staging pilot`) in clean, parallel hairline columns without repeating the capability list. |
| **AUD-06** | `SiteHeader.astro`, `site.ts` | 8 links (`Boundary`, `Workflows`, `Routing`, `Agents & MCP`, `Integrate`, `Status`/`Start`, `Docs`, `GitHub`) + theme toggle + CTA button. | Too many top-level anchors for a single-page site, crowding `1024px–1279px` header bars and echoing the multi-section sprawl. | `R-24`, `C-3` | **P1** | **Simplify**: Trim in-page nav anchors to 3 (`Architecture`, `Capabilities`, `Quickstart`) alongside `Docs`, `GitHub`, theme toggle, and `Self-host with Docker` CTA (`href="#start"`). |
| **AUD-07** | `SiteHeader.astro` (`cta.primary`) | Header primary button `Self-host with Docker` opens an external GitHub `#quick-start` URL in a new tab even when the user is already on the landing page with `#start` right below. | Surprises users clicking the primary page CTA by kicking them off-site before they see the Docker command on the page. | `R-15`, `C-2` | **P1** | **Fix CTA flow**: Route header & Hero primary CTA `Self-host with Docker` to `#start` (where the copyable Docker Compose command and `.env` setup live, with a link out to GitHub Quick Start), and make Hero secondary CTA `Read the docs` (`links.docs`). |
| **AUD-08** | `Layout.astro`, external repo | No `og:image` asset exists in `apps/web/public/`; root `LICENSE` file returns `404` on GitHub; `docs/getting-started.md:119-121` contains broken `file:///` links. | Social link previews lack an `og:image`; repo-level links remain external blockers until fixed outside `apps/web`. | `R-35`, `C-5` | **P2** | **Document as blockers**: Keep `apps/web` free of unverified `MIT Licensed` or fake `og:image` URLs; record in `landing-page-production-readiness.md`. |

---

## 3. Unnecessary Information Removed From Homepage

To achieve Paperclip-level restraint without losing substance:
1. **Removed from Hero**: The 3-column spec strip (`GATEWAY SURFACES`, `ENFORCEMENT`, `STACK`) and inline `RequestPath` box (moved cleanly to `01 · Architecture`).
2. **Removed redundant section splits**: Collapsed 4 repetitive explanation sections (`01 Boundary`, `02 Operator workflows`, `03 Model routing`, `04 Agent permissions & MCP`) into 2 clear sections (`01 · Architecture` and `02 · Capabilities`).
3. **Removed inline documentation sprawl**: Moved secondary client configs (`Python`, `OpenCode CLI`, `Claude Desktop`) and per-endpoint HTTP/JSON-RPC error enumerations out of homepage tabs (linked to `docs/getting-started.md`), keeping `TypeScript`, `cURL`, and `MCP (Claude Code)` as crisp, runnable examples.
4. **Removed redundant pilot feature checklist**: Cut the 5-bullet `"Available during pilot"` checklist from `WaysToStart.astro` because those exact capabilities are already covered in `01 · Architecture` and `02 · Capabilities`.
5. **Removed purely decorative visual noise**: Cut the radial glow orb, SVG `<feGaussianBlur>` glow filters, 36s orbital spin, infinite pulse keyframes, pointer-tilt parallax, and `Continuum Bridge` wave hairline.
