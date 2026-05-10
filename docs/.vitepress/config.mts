import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Helix',
  base: '/helix/',
  description: 'The Spring Boot-inspired Go backend framework. DI/IoC, convention routing, auto-configuration, JWT security, Prometheus metrics — production-ready defaults, zero magic.',
  lang: 'en-US',

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
          text: 'Building Applications',
          items: [
            { text: 'Web & Routing', link: '/guide/web' },
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
