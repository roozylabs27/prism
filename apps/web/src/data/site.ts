export const site = {
  name: 'Prism',
  org: 'RoozyLabs',
  version: '0.4.0',
  url: 'https://prism.roozylabs.com',
  title: 'Prism: the control layer between AI agents, models, and tools',
  description:
    'Prism is a self-hostable AI gateway that gives each workspace and project its own keys, spend quotas, model routing, and MCP tool permissions.',
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
  { label: 'Boundary', href: '#boundary' },
  { label: 'Workflows', href: '#agencies' },
  { label: 'Routing', href: '#inference' },
  { label: 'Agents & MCP', href: '#agents' },
  { label: 'Integrate', href: '#integrate' },
  { label: 'Start', href: '#start' },
];

export const cta = {
  primary: { label: 'Self-host with Docker', href: links.quickstart },
  secondary: { label: 'Apply for a design-partner pilot', href: '#start' },
};
