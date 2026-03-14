import { test as base, type Page } from '@playwright/test';
import { ApiHelper, type Project } from './api.js';
import { ensureSetup, login } from './auth.js';
import { ADMIN_EMAIL, ADMIN_PASSWORD, uniqueProjectKey } from './test-data.js';

type Fixtures = {
  authenticatedPage: Page;
  apiContext: ApiHelper;
  testProject: Project;
};

export const test = base.extend<Fixtures>({
  authenticatedPage: async ({ page }, use) => {
    // Ensure admin user exists
    await ensureSetup(page);

    // Check if we already have a valid session
    const meRes = await page.request.get('/api/v1/auth/me');
    if (!meRes.ok()) {
      await login(page);
    }

    await use(page);
  },

  apiContext: async ({ playwright }, use) => {
    // Create a fresh API context with its own cookie jar
    const context = await playwright.request.newContext({
      baseURL: process.env.E2E_BASE_URL || 'http://localhost:9091',
    });

    // Login via API to get session cookie
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

    // Cleanup
    try {
      await apiContext.deleteProject(project.key);
    } catch {
      // Ignore cleanup errors (project may already be deleted by test)
    }
  },
});

export { expect } from '@playwright/test';
