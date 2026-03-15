import { test as base, type Page } from '@playwright/test';
import { ApiHelper, type Project } from './api.js';
import { ADMIN_EMAIL, ADMIN_PASSWORD, uniqueProjectKey } from './test-data.js';

/** Shared API context to avoid multiple logins (rate limit: 10 req/60s). */
let sharedApiContext: { context: any; api: ApiHelper } | null = null;

type Fixtures = {
  authenticatedPage: Page;
  apiContext: ApiHelper;
  testProject: Project;
};

export const test = base.extend<Fixtures>({
  /**
   * A page with the admin session pre-loaded via storageState (from auth.setup.ts).
   * No login needed — just navigate and go.
   */
  authenticatedPage: async ({ page }, use) => {
    await use(page);
  },

  /**
   * API helper with an authenticated session. Reuses a single login across
   * all tests in the worker to avoid rate limiting.
   */
  apiContext: async ({ playwright }, use) => {
    // Reuse existing API context if session is still valid
    if (sharedApiContext) {
      const meRes = await sharedApiContext.context.get('/api/v1/auth/me');
      if (meRes.ok()) {
        await use(sharedApiContext.api);
        return;
      }
      // Session expired — clean up and create new one
      await sharedApiContext.context.dispose();
      sharedApiContext = null;
    }

    const context = await playwright.request.newContext({
      baseURL: process.env.E2E_BASE_URL || 'http://localhost:9091',
    });

    const loginRes = await context.post('/api/v1/auth/login', {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    if (!loginRes.ok()) {
      throw new Error(`API login failed: ${loginRes.status()}`);
    }

    const api = new ApiHelper(context);
    sharedApiContext = { context, api };
    await use(api);
    // Don't dispose — reuse across tests
  },

  testProject: async ({ apiContext }, use) => {
    const key = uniqueProjectKey();
    const project = await apiContext.createProject({
      key,
      name: `Test Project ${key}`,
    });

    await use(project);

    // Cleanup
    try {
      await apiContext.deleteProject(project.key);
    } catch {
      // Ignore cleanup errors (project may already be deleted by test)
    }
  },
});

export { expect } from '@playwright/test';
