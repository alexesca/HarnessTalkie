import { defineConfig, devices } from '@playwright/test'

// Start HarnessTalkie separately for E2E runs. This keeps the test pointed at
// the same production-shaped Go server that operators and agents use.
// Example: go run . --addr 127.0.0.1:18080 --data /tmp/ht-e2e.events
//          HT_E2E_URL=http://127.0.0.1:18080 npm run test:e2e
export default defineConfig({
  testDir: './e2e',
  testMatch: '**/*.pw.ts',
  outputDir: process.env.HT_E2E_OUTPUT || '/tmp/harnesstalkie-playwright-results',
  timeout: 30_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? 'line' : 'list',
  use: {
    baseURL: process.env.HT_E2E_URL || 'http://127.0.0.1:18080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    ...devices['Desktop Chrome'],
  },
})
