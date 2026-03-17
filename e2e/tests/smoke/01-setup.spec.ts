import { test, expect } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from '../../helpers/test-data.js';

test.describe('Initial Setup', () => {
  test('setup has been completed', async ({ page }) => {
    // The auth.setup.ts project already created the admin user.
    // Verify the API reports setup is no longer required.
    const res = await page.request.get('/api/v1/auth/status');
    const body = await res.json();
    expect(body.setup_required).toBe(false);
  });

  test('rejects duplicate setup', async ({ page }) => {
    // Attempting setup via API should return 409
    const setupRes = await page.request.post('/api/v1/auth/setup', {
      data: { email: 'another@example.com', password: 'AnotherPass123!' },
    });
    expect(setupRes.status()).toBe(409);
  });

  test('navigating to app shows dashboard (not setup page)', async ({ page }) => {
    // With storage state from setup, navigating to root should show dashboard
    await page.goto('/');
    await page.waitForURL('**/projects');
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();
  });
});
