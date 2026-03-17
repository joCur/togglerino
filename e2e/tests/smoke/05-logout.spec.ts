import { test, expect } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

/**
 * Logout test runs LAST in the smoke suite (05-*) because it invalidates a
 * server-side session. Uses its own browser context with a fresh login so
 * it doesn't affect the shared storageState session used by other tests.
 */
test.describe('Logout', () => {
  test('logs out successfully', async ({ browser }) => {
    // Create a completely independent context with NO stored session
    const context = await browser.newContext({
      baseURL: BASE_URL,
      storageState: { cookies: [], origins: [] },
    });
    const page = await context.newPage();

    // Login via API in this isolated context
    const loginRes = await page.request.post('/api/v1/auth/login', {
      data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok()).toBeTruthy();

    await page.goto('/projects');
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();

    // Open user dropdown — the DropdownMenuTrigger is the last button in the header
    await page.locator('header button').last().click();
    // Wait for dropdown to appear and click logout
    await page.getByRole('menuitem', { name: 'Log out' }).click();

    // Verify redirected to login
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
    await context.close();
  });
});
