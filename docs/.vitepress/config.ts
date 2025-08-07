import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Project Arachnet',
  description: 'SPYDER probe and Arachnet docs',
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Ops', link: '/guide/ops' },
      { text: 'SPYDER Architecture', link: '/architecture/spyder' }
    ],
    sidebar: {
      '/guide/': [
        { text: 'Ops Guide', link: '/guide/ops' }
      ],
      '/architecture/': [
        { text: 'SPYDER', link: '/architecture/spyder' }
      ]
    }
  }
})
