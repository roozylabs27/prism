export const site = {
  name: 'Prism',
  org: 'RoozyLabs',
  url: 'https://prism.roozylabs.com',
  title: 'Prism: AI Gateway, Model Routing & MCP Tool Control',
  description:
    'One gateway for your AI models and tools. Route requests across OpenAI, Anthropic, Google Gemini, and MCP servers while managing credentials, spend limits, and agent permissions.',
};

const repo = 'https://github.com/roozylabs27/prism';

export const links = {
  repo,
  quickstart: `${repo}#quick-start`,
  gettingStarted: `${repo}/blob/main/docs/getting-started.md`,
  apiReference: `${repo}/blob/main/docs/api-reference.md`,
  docs: `${repo}/tree/main/docs`,
  changelog: `${repo}/blob/main/CHANGELOG.md`,
  issues: `${repo}/issues`,
  pilotProgram: `${repo}/blob/main/docs/commercial/design-partner-pilot-program.md`,
  pilotRequest: `${repo}/issues/new?title=${encodeURIComponent('Design partner pilot request')}`,
};

export const nav = [
  { label: 'Architecture', href: '#architecture' },
  { label: 'Capabilities', href: '#capabilities' },
  { label: 'Quickstart', href: '#quickstart' },
];

export const cta = {
  primary: { label: 'Self-host with Docker', href: '#start' },
  secondary: { label: 'Read the docs', href: links.docs },
};
