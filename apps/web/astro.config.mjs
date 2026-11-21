import { defineConfig } from 'astro/config';
import tailwind from '@astrojs/tailwind';
import node from '@astrojs/node';

// On Vercel (process.env.VERCEL is set), output pure static HTML into dist/
// so Vercel serves dist/index.html directly from its edge CDN without 404s.
// Outside Vercel, server output keeps the Dockerfile entry (dist/server/entry.mjs) working.
const isVercel = Boolean(process.env.VERCEL);

export default defineConfig({
  site: 'https://prism.roozylabs.com',
  output: isVercel ? 'static' : 'server',
  ...(isVercel
    ? {}
    : {
        adapter: node({
          mode: 'standalone',
        }),
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
