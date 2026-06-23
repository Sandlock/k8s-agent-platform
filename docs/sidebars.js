// @ts-check

/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  docs: [
    {
      type: 'doc',
      id: 'intro',
      label: 'Introduction',
    },
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/quickstart',
      ],
    },
    {
      type: 'category',
      label: 'CLI Reference',
      collapsed: false,
      items: [
        'cli/overview',
        'cli/create',
        'cli/attach',
        'cli/stop-list',
        'cli/auth',
        'cli/keys',
        'cli/github',
        'cli/skills',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      collapsed: false,
      items: [
        'api/overview',
        'api/authentication',
        'api/sandboxes',
        'api/skills',
        'api/keys',
        'api/users',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      collapsed: false,
      items: [
        'architecture/overview',
        'architecture/lifecycle',
        'architecture/security',
        'architecture/database',
      ],
    },
    {
      type: 'category',
      label: 'Deployment',
      collapsed: false,
      items: [
        'deployment/helm',
        'deployment/configuration',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/building',
      ],
    },
  ],
};

export default sidebars;
