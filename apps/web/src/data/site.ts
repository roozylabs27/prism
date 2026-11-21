export const site = {
  name: 'Prism',
  org: 'RoozyLabs',
  url: 'https://prism.roozylabs.com',
  title: 'Prism: AI Gateway, Model Routing & MCP Tool Control',
  description:
    'Prism is a self-hostable AI gateway for routing requests across supported model providers and MCP tool servers. Manage provider credentials, routing policies, agent permissions, and workspace spend controls from one place.',
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
