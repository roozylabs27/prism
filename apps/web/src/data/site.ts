export const site = {
  name: 'Prism',
  org: 'RoozyLabs',
  version: '0.4.0',
  url: 'https://prism.roozylabs.com',
  title: 'Prism: the control layer for AI apps and agents',
  description:
    'Prism is a self-hostable gateway that gives each client its own keys, budgets, model routing and MCP tool permissions. OpenAI-compatible API, governed MCP endpoint, audit records.',
};

const repo = 'https://github.com/roozylabs27/prism';

export const links = {
  repo,
  quickstart: `${repo}#quick-start`,
  gettingStarted: `${repo}/blob/main/docs/getting-started.md`,
  docs: `${repo}/tree/main/docs`,
  changelog: `${repo}/blob/main/CHANGELOG.md`,
  issues: `${repo}/issues`,
  pilotProgram: `${repo}/blob/main/docs/commercial/design-partner-pilot-program.md`,
  pilotRequest: `${repo}/issues/new?title=${encodeURIComponent('Design partner pilot request')}`,
  roadmap: `${repo}/blob/main/docs/roadmap/prism-product-engineering-roadmap.md`,
};

export const nav = [
  { label: 'Product', href: '#boundary' },
  { label: 'Agencies', href: '#agencies' },
  { label: 'Integrate', href: '#integrate' },
  { label: 'Status', href: '#status' },
];

export const cta = {
  primary: { label: 'Self-host with Docker', href: links.quickstart },
  secondary: { label: 'Apply for a pilot', href: '#start' },
};
