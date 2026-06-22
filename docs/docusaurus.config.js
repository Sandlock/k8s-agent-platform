// @ts-check
import {themes as prismThemes} from 'prism-react-renderer';

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Sandlock',
  tagline: 'Isolated Kubernetes sandboxes for Claude Code agents',
  favicon: 'img/favicon.ico',

  url: 'https://sandlock.dev',
  baseUrl: '/k8s-agent-platform/',

  organizationName: 'Sandlock',
  projectName: 'k8s-agent-platform',

  onBrokenLinks: 'throw',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: './sidebars.js',
          editUrl: 'https://github.com/Sandlock/k8s-agent-platform/edit/main/docs/',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      image: 'img/sandlock-social.png',
      colorMode: {
        defaultMode: 'dark',
        disableSwitch: false,
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Sandlock',
        logo: {
          alt: 'Sandlock Logo',
          src: 'img/logo.svg',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'docs',
            position: 'left',
            label: 'Docs',
          },
          {
            to: '/docs/cli/overview',
            position: 'left',
            label: 'CLI',
          },
          {
            to: '/docs/api/overview',
            position: 'left',
            label: 'API',
          },
          {
            href: 'https://github.com/Sandlock/k8s-agent-platform',
            label: 'GitHub',
            position: 'right',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [
              {label: 'Getting Started', to: '/docs/intro'},
              {label: 'CLI Reference', to: '/docs/cli/overview'},
              {label: 'API Reference', to: '/docs/api/overview'},
            ],
          },
          {
            title: 'Architecture',
            items: [
              {label: 'How It Works', to: '/docs/architecture/overview'},
              {label: 'Security Model', to: '/docs/architecture/security'},
              {label: 'Deployment', to: '/docs/deployment/helm'},
            ],
          },
          {
            title: 'More',
            items: [
              {label: 'GitHub', href: 'https://github.com/Sandlock/k8s-agent-platform'},
              {label: 'agent-sandbox', href: 'https://github.com/kubernetes-sigs/agent-sandbox'},
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} Sandlock. Built with Docusaurus.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
        additionalLanguages: ['bash', 'yaml', 'go', 'sql', 'json', 'docker'],
      },
      algolia: undefined,
    }),
};

export default config;