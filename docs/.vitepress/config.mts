import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Helix',
  base: '/helix/',
  description: 'The Spring Boot-inspired Go backend framework. DI/IoC, convention routing, auto-configuration, JWT security, Prometheus metrics — production-ready defaults, zero magic.',
  lang: 'en-US',

  locales: {
    root: {
      label: 'English',
      lang: 'en-US',
    },
    fr: {
      label: 'Français',
      lang: 'fr-FR',
      description: 'Le framework backend Go inspiré de Spring Boot — DI/IoC, routage par convention, auto-configuration, sécurité JWT, métriques Prometheus.',
      themeConfig: {
        nav: [
          { text: 'Guide', link: '/fr/guide/introduction', activeMatch: '/fr/guide/' },
          { text: 'Référence', link: '/fr/reference/cli', activeMatch: '/fr/reference/' },
          { text: 'Exemples', link: '/fr/examples/crud-api', activeMatch: '/fr/examples/' },
          {
            text: 'v0.1',
            items: [
              { text: 'Changelog', link: 'https://github.com/enokdev/helix/releases' },
              { text: 'Contribuer', link: 'https://github.com/enokdev/helix/blob/main/CONTRIBUTING.md' },
            ],
          },
        ],
        sidebar: {
          '/fr/guide/': [
            {
              text: 'Démarrage',
              items: [
                { text: 'Introduction', link: '/fr/guide/introduction' },
                { text: 'Installation', link: '/fr/guide/installation' },
                { text: 'Démarrage rapide', link: '/fr/guide/quick-start' },
              ],
            },
            {
              text: 'Concepts fondamentaux',
              items: [
                { text: 'Injection de dépendances', link: '/fr/guide/dependency-injection' },
                { text: 'Configuration', link: '/fr/guide/configuration' },
                { text: 'Cycle de vie', link: '/fr/guide/lifecycle' },
              ],
            },
            {
              text: 'CLI',
              items: [
                { text: 'Utiliser le CLI', link: '/fr/guide/cli' },
              ],
            },
            {
              text: 'Concepts avancés',
              items: [
                { text: 'DI avancé', link: '/fr/guide/advanced-di' },
              ],
            },
            {
              text: 'Construire une application',
              items: [
                { text: 'Web & Routage', link: '/fr/guide/web' },
                { text: 'Gestion des erreurs', link: '/fr/guide/error-handling' },
                { text: 'Base de données & Repository', link: '/fr/guide/database' },
                { text: 'Sécurité', link: '/fr/guide/security' },
                { text: 'Observabilité', link: '/fr/guide/observability' },
                { text: 'Planification (Cron)', link: '/fr/guide/scheduling' },
                { text: 'Tests', link: '/fr/guide/testing' },
              ],
            },
          ],
          '/fr/reference/': [
            {
              text: 'Référence',
              items: [
                { text: 'CLI', link: '/fr/reference/cli' },
                { text: 'Clés de configuration', link: '/fr/reference/configuration-keys' },
                { text: 'Starters', link: '/fr/reference/starters' },
                { text: 'Déploiement', link: '/fr/reference/deployment' },
              ],
            },
          ],
          '/fr/examples/': [
            {
              text: 'Exemples',
              items: [
                { text: 'API CRUD', link: '/fr/examples/crud-api' },
                { text: 'API sécurisée (JWT + RBAC)', link: '/fr/examples/secured-api' },
              ],
            },
          ],
        },
        editLink: {
          pattern: 'https://github.com/enokdev/helix/edit/main/docs/:path',
          text: 'Modifier cette page sur GitHub',
        },
        search: { provider: 'local' },
        footer: {
          message: 'Publié sous licence MIT.',
          copyright: 'Copyright © 2024-present enokdev',
        },
      },
    },
  },

  head: [
    ['link', { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.googleapis.com' }],
    ['link', { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }],
    ['link', { rel: 'stylesheet', href: 'https://fonts.googleapis.com/css2?family=Plus+Jakarta+Sans:ital,wght@0,200..800;1,200..800&family=JetBrains+Mono:ital,wght@0,100..800;1,100..800&display=swap' }],
    ['meta', { name: 'theme-color', content: '#4f6ef7' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:site_name', content: 'Helix' }],
    ['meta', { property: 'og:image', content: '/og-image.png' }],
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'Helix',

    nav: [
      { text: 'Guide', link: '/guide/introduction', activeMatch: '/guide/' },
      { text: 'Reference', link: '/reference/cli', activeMatch: '/reference/' },
      { text: 'Examples', link: '/examples/crud-api', activeMatch: '/examples/' },
      {
        text: 'v0.1',
        items: [
          { text: 'Changelog', link: 'https://github.com/enokdev/helix/releases' },
          { text: 'Contributing', link: 'https://github.com/enokdev/helix/blob/main/CONTRIBUTING.md' },
        ],
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Introduction', link: '/guide/introduction' },
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Quick Start', link: '/guide/quick-start' },
          ],
        },
        {
          text: 'Core Concepts',
          items: [
            { text: 'Dependency Injection', link: '/guide/dependency-injection' },
            { text: 'Configuration', link: '/guide/configuration' },
            { text: 'Lifecycle', link: '/guide/lifecycle' },
          ],
        },
        {
          text: 'CLI',
          items: [
            { text: 'Using the CLI', link: '/guide/cli' },
          ],
        },
        {
          text: 'Core Concepts (Advanced)',
          items: [
            { text: 'Advanced DI', link: '/guide/advanced-di' },
          ],
        },
        {
          text: 'Building Applications',
          items: [
            { text: 'Web & Routing', link: '/guide/web' },
            { text: 'Error Handling', link: '/guide/error-handling' },
            { text: 'Database & Repository', link: '/guide/database' },
            { text: 'Security', link: '/guide/security' },
            { text: 'Observability', link: '/guide/observability' },
            { text: 'Scheduling', link: '/guide/scheduling' },
            { text: 'Testing', link: '/guide/testing' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Reference',
          items: [
            { text: 'CLI', link: '/reference/cli' },
            { text: 'Configuration Keys', link: '/reference/configuration-keys' },
            { text: 'Starters', link: '/reference/starters' },
            { text: 'Deployment', link: '/reference/deployment' },
          ],
        },
      ],
      '/examples/': [
        {
          text: 'Examples',
          items: [
            { text: 'CRUD API', link: '/examples/crud-api' },
            { text: 'Secured API (JWT + RBAC)', link: '/examples/secured-api' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/enokdev/helix' },
    ],

    search: {
      provider: 'local',
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2024-present enokdev',
    },

    editLink: {
      pattern: 'https://github.com/enokdev/helix/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },
  },
})
