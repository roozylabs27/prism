# Prism Marketing Site: Design Direction

Scope: `apps/web` only. The dashboard (`apps/app`) follows `.agents/skills/antislop/DESIGN.md`.
Approved 2026-11-19.

## Identity: "Operations ledger"

The page reads like a carefully set technical document: precise, quiet, specific. It earns trust by showing exactly what Prism does and where it stops. No console costumes, no glow, no invented numbers.

## Audience

1. AI automation agencies running 5 to 30 client accounts (primary).
2. Platform teams running several providers and agents (secondary).

## Color

| Token | Light | Dark | Use |
|---|---|---|---|
| `--paper` | `#F6F5F1` | `#111315` | Page background |
| `--paper-raised` | `#FFFFFF` | `#181B1E` | Code blocks, tables |
| `--ink` | `#14161A` | `#ECEAE4` | Headings, body, primary button |
| `--ink-muted` | `#555A61` | `#A4A8AE` | Secondary text |
| `--rule` | `#D9D6CE` | `#2A2E33` | Borders, dividers |
| `--accent` | `#6D28D9` | `#B49CFB` | Links, focus ring, active tab. Never a fill for large areas. |
| `--deny` | `#B42318` | `#F97066` | Denied path in diagrams only |
| `--allow` | `#067647` | `#47CD89` | Allowed path in diagrams only |

One accent. No gradients. No shadows beyond a 1px rule.

## Type

- IBM Plex Sans (400, 500, 600) for prose and headings. Chosen for its engineering heritage and because it separates the site from the Inter-based dashboard.
- IBM Plex Mono (400, 500) for code, endpoints, header names.
- Body 17px / 1.6. Minimum text size 14px. Headings left-aligned, tight tracking only above 32px.

## Dials

ENERGY 1 · RHYTHM 2 · MOTION 1

## Layout

- 1200px max container, 12-column grid, generous left margin for section labels on desktop.
- Section rhythm alternates between prose, diagram, table and code. No two adjacent sections share a structure.
- No bento grids, no stat banners, no card walls.
- Radius 2px on controls and code blocks.

## Visual demonstration

- **Hero architectural stage (`PrismStage.astro`):** Adapted from the RoozyLabs `company-profile` Prism Control Plane showcase (`PrismControlPlaneStage` + `ContinuumArtifact` fine-pointer tilt + `ContinuumBridge` section spline). Encodes the product name ("Prism") and its architectural boundary: an incoming `/v1/chat/completions` or `/v1/mcp` request enters the governance & audit boundary ring, refracts through the central Prism crystal, and routes to primary model, governed MCP tool server, or failover credential standby. Uses a single localized radial atmosphere behind the stage focal point (`rgb(var(--accent) / 0.08)`) and adapts all labels and node fills to both Light and Dark themes.
- **Request path ledger (`RequestPath.astro`):** Ordered 4-step inspection sequence (gateway key, budget and quota, agent policy, route) below the Hero split.
- **MCP boundary diagram (`McpBoundary.astro`):** An agent's tool call allowed or denied by Prism, with a direct shell call shown as ungoverned.
- **Real code (`integrations.ts`):** Runs against the current API. No screenshots until real dashboard captures exist.

## Motion

- No full-screen background canvas animation.
- Hero `PrismStage` includes a slow 36s orbital ring drift, route node pulse, and subtle fine-pointer 3D tilt (`max 4.5deg`); all three are disabled under `prefers-reduced-motion: reduce` and on coarse pointers.
- 150ms color transitions on interactive states only. Everything respects `prefers-reduced-motion`.

## Experience

- Mobile: single column, diagrams switch to a vertical layout, code blocks scroll horizontally inside their own box, 44px targets.
- Navigation: sticky header with anchor links; mobile drawer with Escape and focus return.
- Conversion: primary CTA "Self-host with Docker" (GitHub quickstart), secondary "Apply for a design-partner pilot".
- Themes: dark editorial interface by default, full light theme supported, manual toggle persisted.
