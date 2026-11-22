# Prism Marketing Site: Design Direction

Scope: `apps/web` only. The console (`apps/console`) follows `.agents/skills/antislop/DESIGN.md` (with sharp `2px` radius unified across both apps).
Updated 2026-11-22.

## Identity: "Technical control ledger"

The page reads like a carefully typeset engineering architecture brief: precise, quiet, specific. It earns trust by showing how requests flow through Prism, what controls operators enforce, and where the boundary stops. No console costumes, no glow halos, no invented numbers, no infinite motion.

## Audience

1. Engineering teams running multi-provider AI applications and MCP tools.
2. AI automation agencies isolating workspaces, keys, and budgets across client accounts.

## Color

| Token | Light | Dark (Default) | Use |
|---|---|---|---|
| `--paper` | `#F6F5F1` | `#111315` | Page background |
| `--paper-raised` | `#FFFFFF` | `#181B1E` | Code blocks, raised panels |
| `--ink` | `#14161A` | `#ECEAE4` | Headings, body, primary button |
| `--ink-muted` | `#555A61` | `#A4A8AE` | Secondary text |
| `--rule` | `#D9D6CE` | `#2A2E33` | 1px borders, dividers |
| `--accent` | `#6D28D9` | `#B49CFB` | Links, focus ring, active tab, diagram emphasis. Never a fill for large areas. |
| `--deny` | `#B42318` | `#F97066` | Denied path in diagrams only |
| `--allow` | `#067647` | `#47CD89` | Allowed path in diagrams only |

One accent. No radial background orbs. No drop shadows beyond 1px rules.

## Type

- IBM Plex Sans (`400`, `500`, `600`) for prose and headings.
- IBM Plex Mono (`400`, `500`) for indices, endpoints, HTTP headers, and code.
- Body `17px / 1.6`. Fluid display headings via `clamp()`.

## Dials

ENERGY 1 · RHYTHM 2 · MOTION 1

## Layout

- `1200px` (`75rem`) max container, `12`-column grid, generous vertical padding (`py-16 sm:py-20 lg:py-24`).
- 5 focused sections: Hero, `01 · Architecture`, `02 · Capabilities`, `03 · Quickstart`, `04 · Start`.
- Section rhythm alternates between split (`Hero`, `03 · Quickstart`) and full-width stacked/banded compositions (`01 · Architecture`, `02 · Capabilities`, `04 · Start`).
- Radius `2px` (`rounded-sm`) on controls, code blocks, and architectural frames.

## Visual demonstration

- **Hero architectural stage (`PrismStage.astro`):** Crisp geometric Prism crystal and Governance & Audit Boundary ring paired with an interactive 3-tab **Illustrative Route Inspector** (`01 · Route`, `02 · MCP Gate`, `03 · Failover`). Static crystal with zero glow filters and zero infinite animation loops.
- **Request path ledger (`RequestPath.astro`):** Ordered 4-step admission sequence (`Gateway key scope`, `Quotas & budget`, `Agent policy`, `Route & failover`) inside `01 · Architecture`.
- **MCP boundary trace (`McpBoundary.astro`):** Illustrative 3-call agent trace distinguishing Allowed, Denied, and Outside-Prism calls.
- **Real code (`integrations.ts`):** 3 core verified quickstarts (`TypeScript`, `cURL`, `MCP (Claude Code)`) using `YOUR_PRISM_API_KEY`.

## Motion

- Strict `MOTION 1`: hover, focus, active-tab opacity transitions (150ms–180ms), and the CSS Grid (`0fr` -> `1fr`) mobile navigation drawer.
- Zero continuous loops, zero pulsing animations, zero pointer-tilt parallax, zero scroll-reveal animations. Everything respects `prefers-reduced-motion`.

## Experience

- Mobile: single column (`grid-cols-1`), code blocks scroll horizontally inside `min-w-0` containers, `44px` minimum tap targets, smooth overlay menu drawer.
- Navigation: sticky header with 3 concise anchor links (`Architecture`, `Capabilities`, `Quickstart`) + `Docs` + `GitHub`; labeled `Menu`/`Close` mobile drawer with `Escape` and focus return.
- Themes: dark editorial interface by default, full light theme supported, manual toggle persisted.
