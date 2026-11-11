import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const gateway = env.VITE_DEV_GATEWAY_URL || 'http://127.0.0.1:18080'
  const admin = env.VITE_DEV_ADMIN_URL || 'http://127.0.0.1:18081'
  return {
    base: env.VITE_ADMIN_BASE || '/admin/',
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      port: 5173,
      proxy: {
        '/api/admin': { target: admin, changeOrigin: true },
        '/api/v1': { target: gateway, changeOrigin: true },
      },
    },
  }
})
