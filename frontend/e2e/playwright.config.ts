import { defineConfig, devices } from '@playwright/test';

/**
 * RealWorld e2e tests
 * Assumes:
 *   - Frontend at http://localhost:4100
 *   - Backend  at http://localhost:8080
 */
export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 8_000 },
  fullyParallel: false,   // tests share state (same DB), run sequentially
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { open: 'never' }]],

  use: {
    baseURL: 'http://localhost:3001',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    // Give SPA routes time to load data
    navigationTimeout: 15_000,
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
