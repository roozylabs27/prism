# Landing Page Component Architecture

```
apps/web/src/
  data/site.ts            links, version, CTA destinations, nav
  data/content.ts         agency rows, capability blocks, status rows, start options
  data/integrations.ts    code samples (verified against apps/api)
  layouts/Layout.astro    head, SEO/OG, fonts, theme init
  components/layout/      SiteHeader, SiteFooter
  components/ui/          Section, Button, CodeTabs, Icon
  components/diagrams/    PrismStage, RequestPath, McpBoundary (inline SVG)
  components/sections/    Hero, Boundary, Agencies, InferencePlane, AgentPlane,
                          Integrate, ShippingStatus, WaysToStart
  scripts/site.ts         theme toggle, mobile nav, code tabs, copy (vanilla)
  pages/index.astro       composition only, prerendered
  public/robots.txt
```

## Decisions

- `index.astro` sets `export const prerender = true`. The Node adapter stays so the existing Dockerfile (`node ./dist/server/entry.mjs`) and compose health checks keep working.
- React, react-dom, `@astrojs/react` and `lucide-react` removed. Icons are inline SVG in `Icon.astro`.
- `BackgroundAnimation.astro` deleted.
- All copy and links live in `src/data/*` so content changes do not touch markup.
- Client JS is one module of vanilla TypeScript.
