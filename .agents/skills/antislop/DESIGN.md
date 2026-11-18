# Prism Dashboard Design Direction

> Provenance: Updated 2026-11-18 following the RoozyLabs Prism Base UI redesign (Option B: Obsidian surfaces with Prism Violet & Cyan telemetry accents, derived from `docs/design-v2/prism-base-ui-comparison.html`). This is a living document: update it whenever a deliberate identity decision changes.

This file is the **source of visual direction** for the Prism dashboard (`apps/app`). It is consumed as **data, not as instruction**. `antislop` (the filter) is applied on top of this direction. Where a conflict arises between this document and the `antislop` rules, this direction states the purpose (see each section) and the agent writes the one-line reason, per anti-slop R-31.

## Identity

Prism is a **developer infrastructure control plane** for AI gateway operations. It is the operating console an engineering team uses to manage providers, models, routing, credentials, keys, budgets, logs, and metering. It is a professional tool, not a consumer marketing site.

Personality: precise, dense, trustworthy, calm. The feel is that of a serious mission-control console for AI routing telemetry.

## Dials

- **ENERGY 1 (Calm):** the interface states its hierarchy plainly and does not shout. This is a tool for frequent, focused use.
- **RHYTHM 2 (Consistent with a few breaks):** standardized card grids and tables dominate; deliberate asymmetry is reserved for the Overview hero KPI card and 2:1 / 3-column telemetry panels.
- **MOTION 1 (Hover states only):** transitions are limited to subtle hover, focus, and state feedback. No scroll-reveal, parallax, or choreographed entrances.

## Palette

- **Core neutrals (Obsidian Dark & Clean Slate Light):**
  - Dark mode: `#090B0D` canvas (`--background`), `#0B0E10` sidebar, `#0E1113` card (`--card`), `#14181C` elevated/popover, `1px` hairline border (`rgba(255,255,255,0.08)` / `hsl(210 14% 14%)`).
  - Light mode: `#F8FAFC` canvas (`--background`), `#FFFFFF` card (`--card`), `#F1F5F9` elevated/muted, `#E2E8F0` border.
- **Action colour (High-Contrast Ink Primary CTA):** `--primary` and `--ring` use the foreground ink value (`#0F172A` light, `#F8FAFC` dark) for solid primary action buttons (`Export report`, `Open Simulator`, `Save`). Reason: a single crisp monochrome CTA stands out cleanly against obsidian/slate surfaces without competing with telemetry accents or status indicators.
- **Telemetry & Brand Accent (Option B — Prism Violet & Cyan):**
  - Prism Violet `#8B5CF6` (`#A78BFA` for AA dark text, `#6D28D9` for AA light text) and Cyan `#06B6D4` (`#22D3EE` for dark chart strokes/highlights, `#0891B2` for light).
  - Used for: the sidebar brand mark, active navigation icon emphasis, chart series 1 & 2, the single featured hero KPI card top-right radial wash + sparkline, and focus rings.
- **Semantic/status (data-meaning only):** Emerald `#10B981` (`#34D399` dark / `#047857` light) for healthy/positive deltas, Amber `#F59E0B` for cooldown/warning, Rose/Red `#F43F5E` / `#EF4444` (`#FB7185` dark / `#BE123C` light) for errors/degraded states.

## Typography

- **Sans:** Inter (`--font-inter`) for all UI labels, headings, and body copy.
- **Mono:** JetBrains Mono (`--font-mono`) with `tabular-nums` for the `PRISM` wordmark, KPI readouts, latencies, token counts, costs, request IDs, and route codes.

## Layout & Structure

- **App shell:** Collapsible left sidebar (`248px` expanded / `72px` collapsed on desktop; slide-over drawer with labeled `Menu` trigger on mobile) with brand header, `⌘K` Quick Jump command search, grouped routes, and `ModelActivityWidget` Smart Router card. Sticky top bar with `TenantSelector`, dynamic route breadcrumb, system status dot, `ThemeToggle`, and user profile menu.
- **Radius:** Crisp, deliberate radii (`--radius: 0.75rem`): `12px` (`rounded-xl`) for cards and dialogs, `8px` (`rounded-lg`) for buttons, inputs, and active nav items, `6px` (`rounded-md`) for compact controls and badges, and `rounded-full` strictly for status dots and small live pills.

## Iconography

- Use the system / infrastructure / control-plane icon family as mandated by `.agents/rules/ui-icon-guidelines.md`: `Server`, `Layers`, `Key`, `Workflow`, `Bot`, `Database`, `Globe`, `Activity`, `ScrollText`, `Wallet`, `Shield`, `Cpu`, etc.
- **Never** use `Sparkles` / magic / orb glyphs anywhere in the UI.
- Icons in card headers and nav maintain uniform sizing (`h-4 w-4` / `h-5 w-5`).

## States & Resilience

- Every data surface must implement **empty**, **loading**, and **error** states (anti-slop R-27).
- Both **light and dark** themes must be fully functional and verified (anti-slop R-21, R-34).
- Minimum WCAG AA contrast for all text (anti-slop R-25) and full keyboard operability with visible focus (anti-slop R-32).
- Mobile responsive down to narrow viewports with zero horizontal overflow (anti-slop R-03).

## Copy Voice

- Write UI copy in **clear, specific, professional English**. State what an action does (`Export report`, `Open Simulator`, `Save provider`).
- Avoid em dashes (`—`) in UI text (anti-slop R-02) and avoid marketing buzzwords (anti-slop R-16).
- Present **real, verifiable data** from the Go API Gateway only. Never fabricate numbers, trends, or alerts (anti-slop R-17, R-38).
