export default defineNuxtConfig({
  compatibilityDate: '2026-11-11',
  devtools: { enabled: false },
  modules: ['@pinia/nuxt'],
  css: ['vant/lib/index.css', '~/assets/main.css'],
  app: {
    head: {
      title: 'Mall Store',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1, viewport-fit=cover' },
        { name: 'theme-color', content: '#2563eb' },
      ],
    },
  },
  runtimeConfig: {
    public: {
      apiBase: process.env.NUXT_PUBLIC_API_BASE || '',
      ssrApiBase: process.env.NUXT_PUBLIC_SSR_API_BASE || 'http://127.0.0.1:18080',
    },
  },
  nitro: {
    devProxy: {
      '/api': { target: 'http://127.0.0.1:18080', changeOrigin: true },
    },
  },
})
