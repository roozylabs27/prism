import { defineConfig } from 'astro/config';
import tailwind from '@astrojs/tailwind';
import node from '@astrojs/node';

// Server output keeps the existing Dockerfile entry (dist/server/entry.mjs) working.
// The landing page itself is prerendered to static HTML.
export default defineConfig({
  site: 'https://prism.roozylabs.com',
  output: 'server',
  adapter: node({
    mode: 'standalone',
  }),
  integrations: [
    tailwind({
      applyBaseStyles: false,
    }),
  ],
  server: {
    port: 3001,
    host: '0.0.0.0',
  },
});
