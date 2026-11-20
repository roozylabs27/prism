/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{astro,html,js,ts,md}'],
  darkMode: 'class',
  theme: {
    extend: {
      fontFamily: {
        sans: ['"IBM Plex Sans"', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'ui-monospace', 'SFMono-Regular', 'Consolas', 'monospace'],
      },
      colors: {
        paper: 'rgb(var(--paper) / <alpha-value>)',
        raised: 'rgb(var(--paper-raised) / <alpha-value>)',
        ink: 'rgb(var(--ink) / <alpha-value>)',
        muted: 'rgb(var(--ink-muted) / <alpha-value>)',
        rule: 'rgb(var(--rule) / <alpha-value>)',
        accent: 'rgb(var(--accent) / <alpha-value>)',
        allow: 'rgb(var(--allow) / <alpha-value>)',
        deny: 'rgb(var(--deny) / <alpha-value>)',
      },
      borderRadius: {
        DEFAULT: '2px',
        sm: '2px',
      },
      maxWidth: {
        page: '75rem',
        prose: '40rem',
      },
    },
  },
  plugins: [],
};
