import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL || 'http://localhost:9091';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI
    ? [['html', { open: 'never' }]]
    : 'list',
  use: {
    baseURL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  globalSetup: './global-setup.ts',
  globalTeardown: './global-teardown.ts',
  projects: [
    // Setup project: creates admin user and saves authenticated session
    {
      name: 'setup',
      testDir: './tests/setup',
      testMatch: /.*\.setup\.ts/,
    },
    // Smoke tests: run serially with pre-authenticated session
    {
      name: 'chromium',
      use: {
        browserName: 'chromium',
        // Reuse the authenticated session from setup
        storageState: 'test-results/.auth/user.json',
      },
      dependencies: ['setup'],
      testDir: './tests/smoke',
    },
  ],
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
});
