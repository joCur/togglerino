import { test, expect } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Authentication', () => {
  test('session is valid from setup', async ({ page }) => {
    // Storage state provides the session cookie
    await page.goto('/projects');
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();
  });

  test('rejects invalid password', async ({ browser }) => {
    // Use a fresh context WITHOUT the stored session
    const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await context.newPage();

    await page.goto(`${BASE_URL}/login`);
    await page.getByPlaceholder('you@example.com').fill(ADMIN_EMAIL);
    await page.getByPlaceholder('Your password').fill('WrongPassword123!');
    await page.getByRole('button', { name: 'Sign In' }).click();

    // Should show error, stay on login page
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page).toHaveURL(/login/);
    await context.close();
  });

  test('persists session across refresh', async ({ page }) => {
    await page.goto('/projects');
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();

    // Refresh and verify still authenticated
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();
    await expect(page).not.toHaveURL(/login/);
  });

  test('redirects unauthenticated to login', async ({ browser }) => {
    // Use a fresh context WITHOUT the stored session
    const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await context.newPage();
    await page.goto(`${BASE_URL}/projects`);

    // Should redirect to login page
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
    await context.close();
  });
});
