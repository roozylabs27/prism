# Prism Landing Page (`apps/web`) — Design Direction & Reference Analysis

**Repository:** `roozylabs/prism`  
**Scope:** `apps/web` only (`apps/app` dashboard follows `.agents/skills/antislop/DESIGN.md`; commit `d523b9bb` unified `apps/app` to sharp `2px` radii to match `apps/web`).  
**Baseline Commit SHA:** `a66d45f5b1d8ab1ea440d08dd0a11f04994dd2fa` (`a66d45f5`)  
**Design Read:** *"Reading this as: developer infrastructure control-plane landing page for engineering teams and AI agencies, in Prism's sharp-2px dark editorial operations-ledger language inspired by Paperclip's restraint and Resend's code-first clarity, dial ENERGY 1 / RHYTHM 2 / MOTION 1."*

---

## 1. Design Reference Analysis (Inspected via Firecrawl)

### Primary Reference: Paperclip (`https://paperclip.ing/`)
- **Observed strengths**:
  - **Restraint and whitespace**: Dark canvas (`#0A0A0A`), crisp `1px` hairlines, generous vertical breathing room, zero heavy drop shadows or glowing hero blobs.
  - **Hero discipline**: 7-word H1 headline (`A team of agents for every person.`) + single 12-word subtitle + clear primary/secondary CTAs.
  - **Structured operational artifacts**: Communicates complex orchestration through concrete artifacts (numbered sequence `01 / 02 / 03`, budget usage rows, short trace steps, single CLI install command) instead of prose walls.
- **Adopted for Prism**:
  - Crisp `1px` hairline structural rhythm, dark-first restraint, concise 8-word Hero headline + 2-sentence explanation, and concrete architectural artifacts (`RequestPath`, `McpBoundary`, `POST /api/agents` policy example).
- **Explicitly rejected (do not clone)**:
  - Pill buttons (`border-radius: 9999px`) — Prism uses sharp `2px` (`rounded-sm`), matching `d523b9bb`.
  - `#22C55E` green brand palette and `Inter Tight` — Prism keeps its own restrained violet accent (`#B49CFB` dark / `#6D28D9` light) and `IBM Plex Sans` + `IBM Plex Mono`.
  - Customer testimonial cards and long FAQ — Prism has zero verified public customer quotes (`R-18`).

### Secondary Reference: Resend (`https://resend.com/`)
- **Observed strengths**:
  - Immediate developer positioning (`Email for developers`).
  - Short section leads paired directly with ONE runnable SDK snippet and its exact `HTTP 200` response body (`{ "id": "..." }`).
- **Adopted for Prism**:
  - Streamlined `03 · Quickstart` section focused on 3 crisp tabs (`TypeScript`, `cURL`, `MCP (Claude Code)`) with a clean 1-line `Expected response` readout instead of dumping every SDK and error code onto the homepage.
- **Explicitly rejected**:
  - Multi-card testimonial grid and marketing analytics/editor screenshots that do not apply to Prism's gateway boundary.

### Secondary Reference: Linear (`https://linear.app/`)
- **Observed strengths**:
  - Groups broad product capabilities into 3–4 narrative pillars (`Intake`, `Planning`, `AI and automations`, `Build, review, and ship`) with subtle `Fig 0.x` annotations and steady editorial pacing.
- **Adopted for Prism**:
  - Consolidating 4 overlapping capability sections into **`02 · Capabilities`** with 3 hairline-divided horizontal bands (`01 · Routing & failover`, `02 · Governance & isolation`, `03 · Visibility & security`), avoiding repetitive card grids.

### Secondary Reference: Vercel (`https://vercel.com/`)
- **Observed strengths**:
  - Frames technical infrastructure (`AI Model Gateway`, `Tenant Isolation`) around clear architectural control points without turning primary copy into an environment-variable manual.
- **Adopted for Prism**:
  - Explaining *what control Prism gives operators* (routing rules, tenant isolation, credential encryption, observability) in clear prose while keeping exhaustive env var flags and internal package details in documentation.

---

## 2. Prism Design System (`apps/web`)

### Identity: "Technical Control Ledger"
The page reads like a carefully typeset engineering architecture brief: calm (`ENERGY 1`), structurally varied without clutter (`RHYTHM 2`), and free of decorative animation (`MOTION 1`).

### Color Tokens (`src/styles/globals.css`)
- **Dark Default**: `#111315` (`--paper`), `#181B1E` (`--paper-raised`), `#ECEAE4` (`--ink`), `#A4A8AE` (`--ink-muted`), `#2A2E33` (`--rule`), `#B49CFB` (`--accent`), `#47CD89` (`--allow`), `#F97066` (`--deny`).
- **Light Mode (toggled & persisted)**: `#F6F5F1` (`--paper`), `#FFFFFF` (`--paper-raised`), `#14161A` (`--ink`), `#555A61` (`--ink-muted`), `#D9D6CE` (`--rule`), `#6D28D9` (`--accent`), `#067647` (`--allow`), `#B42318` (`--deny`).
- **WCAG AA Verified**: All text and semantic tokens meet `>= 5.22:1` contrast in both themes.
- **Strict Accent Rule**: `--accent` is reserved for active tab indicators, focus rings, inline links, and route emphasis — never used as large gradient backgrounds or decorative glows.

### Typography
- **Sans**: `IBM Plex Sans` (`400`, `500`, `600`) for all UI, headings, and prose.
- **Mono**: `IBM Plex Mono` (`400`, `500`) for section indices (`01 · Architecture`), endpoints (`/v1/chat/completions`), HTTP headers (`X-Prism-Model`), and code blocks.
- **Fluid Scale**:
  - Hero H1: `clamp(2.25rem, 4.5vw, 3.5rem)` (`leading-[1.06]`, `tracking-[-0.025em]`)
  - Section H2: `clamp(1.625rem, 2.6vw, 2.25rem)` (`leading-tight`, `tracking-tight`)
  - Body: `1.0625rem` (`17px`), `line-height: 1.6`, `max-w-prose` (`40rem`).

### Geometry, Whitespace & Motion (`MOTION 1`)
- **Radius**: Crisp `2px` (`rounded-sm`) across buttons, code frames, diagrams, and inputs — unified with `apps/app` (`d523b9bb`).
- **No decorative gradients, glows, or continuous loops**: Zero `<feGaussianBlur>` glow halos, zero ambient radial hero blobs, zero infinite CSS `@keyframes`, zero `Continuum Bridge` divider graphics, and zero pointer-tilt parallax.
- **Permitted Motion**: 150ms–250ms state transitions on interactive tabs, focus states, hover color changes, and the CSS Grid (`0fr` ↔ `1fr`) mobile/tablet navigation drawer, all honoring `prefers-reduced-motion: reduce`.
