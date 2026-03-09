import type { SidebarsConfig } from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/introduction',
        'getting-started/quick-start',
        'getting-started/first-flag-in-code',
      ],
      collapsed: false,
    },
    {
      type: 'category',
      label: 'Self-Hosting',
      items: [
        'self-hosting/installation',
        'self-hosting/configuration',
        'self-hosting/production',
        'self-hosting/upgrading',
      ],
    },
    {
      type: 'category',
      label: 'Core Concepts',
      items: [
        'core-concepts/what-are-feature-flags',
        'core-concepts/projects-and-environments',
        'core-concepts/flags',
        'core-concepts/targeting',
        'core-concepts/segments',
        'core-concepts/rollouts',
        'core-concepts/flag-lifecycle',
      ],
    },
    {
      type: 'category',
      label: 'Dashboard Guide',
      items: [
        'dashboard/managing-flags',
        'dashboard/lifecycle-dashboard',
        'dashboard/kill-switch-dashboard',
        'dashboard/audit-log',
        'dashboard/team-management',
        'dashboard/sso-oidc',
      ],
    },
    {
      type: 'category',
      label: 'SDKs',
      items: [
        'sdks/overview',
        'sdks/javascript',
        'sdks/react',
        'sdks/go',
        'sdks/dotnet',
      ],
    },
    {
      type: 'category',
      label: 'API Reference',
      items: [
        'api-reference/authentication',
        'api-reference/management-api',
        'api-reference/client-api',
      ],
    },
  ],
};

export default sidebars;
