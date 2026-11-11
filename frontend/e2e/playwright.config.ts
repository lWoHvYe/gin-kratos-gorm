import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: process.env.WEB_BASE_URL || 'http://mall.local',
    channel: 'chrome',
    headless: true,
    trace: 'retain-on-failure',
    launchOptions: {
      args: ['--host-resolver-rules=MAP mall.local 10.211.55.29'],
    },
  },
  reporter: [['list']],
})
