import { defineConfig } from '@playwright/test'

const baseURL = process.env.WEB_BASE_URL || 'http://mall.local'
const host = new URL(baseURL).hostname
const resolver = process.env.WEB_HOST_RESOLVER || '10.211.55.29'

export default defineConfig({
  testDir: '.',
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL,
    channel: 'chrome',
    headless: true,
    trace: 'retain-on-failure',
    launchOptions: {
      args: [`--host-resolver-rules=MAP ${host} ${resolver}`],
    },
  },
  reporter: [['list']],
})
