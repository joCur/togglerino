import type { Config } from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Togglerino',
  tagline: 'Self-hosted feature flag management',
  favicon: 'img/favicon.ico',

  url: 'https://togglerino.github.io',
  baseUrl: '/togglerino/',

  organizationName: 'togglerino',
  projectName: 'togglerino',

  onBrokenLinks: 'throw',

  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.ts',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    navbar: {
      title: 'Togglerino',
      items: [
        {
          href: 'https://github.com/togglerino/togglerino',
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
            { label: 'Getting Started', to: '/' },
            { label: 'Self-Hosting', to: '/self-hosting/installation' },
            { label: 'SDKs', to: '/sdks/overview' },
          ],
        },
        {
          title: 'More',
          items: [
            { label: 'GitHub', href: 'https://github.com/togglerino/togglerino' },
          ],
        },
      ],
      copyright: `Copyright \u00a9 ${new Date().getFullYear()} Togglerino`,
    },
    colorMode: {
      defaultMode: 'light',
      respectPrefersColorScheme: true,
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
