import { test as base, type Page, type APIRequestContext } from '@playwright/test';
import { ApiHelper, type Project } from './api.js';
import { ADMIN_EMAIL, ADMIN_PASSWORD, uniqueProjectKey } from './test-data.js';

type Fixtures = {
  authenticatedPage: Page;
  apiContext: ApiHelper;
  testProject: Project;
};

export const test = base.extend<Fixtures>({
  /**
   * A page with the admin session pre-loaded via storageState (from auth.setup.ts).
   */
  authenticatedPage: async ({ page }, use) => {
    await use(page);
  },

  /**
   * API helper with an authenticated session.
   * Each worker gets its own context (rate limiting is disabled in E2E env).
   */
  apiContext: async ({ playwright }, use) => {
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
    await use(api);
    await context.dispose();
  },

  testProject: async ({ apiContext }, use) => {
    const key = uniqueProjectKey();
    const project = await apiContext.createProject({
      key,
      name: `Test Project ${key}`,
    });

    await use(project);

    try {
      await apiContext.deleteProject(project.key);
    } catch {
      // Ignore cleanup errors (project may already be deleted by test)
    }
  },
});

export { expect } from '@playwright/test';
