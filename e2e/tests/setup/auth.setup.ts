import { test as setup, expect } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from '../../helpers/test-data.js';

const authFile = 'test-results/.auth/user.json';

/**
 * This setup test runs once before all other tests. It creates the admin
 * user (if needed) and saves the authenticated session to a storage state
 * file. All subsequent tests reuse this session, avoiding repeated logins
 * and rate limiting (10 req/60s on auth endpoints).
 */
setup('authenticate', async ({ page }) => {
  // Check if setup is needed
  const statusRes = await page.request.get('/api/v1/auth/status');
  const { setup_required } = await statusRes.json();

  if (setup_required) {
    // Create the first admin user via the UI
    await page.goto('/');
    await page.getByPlaceholder('admin@example.com').fill(ADMIN_EMAIL);
    await page.getByPlaceholder('At least 8 characters').fill(ADMIN_PASSWORD);
    await page.getByPlaceholder('Confirm your password').fill(ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Create Account' }).click();
    await page.waitForURL('**/projects');
  } else {
    // Login via API
    const loginRes = await page.request.post('/api/v1/auth/login', {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok()).toBeTruthy();
    await page.goto('/projects');
  }

  // Save the authenticated state
  await page.context().storageState({ path: authFile });
});
