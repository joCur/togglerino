import type { Page } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from './test-data.js';

/**
 * Idempotent setup — creates the first admin user if the database is fresh.
 * Checks /api/v1/auth/status first to avoid 409 on subsequent calls.
 */
export async function ensureSetup(page: Page): Promise<void> {
  const res = await page.request.get('/api/v1/auth/status');
  const { setup_required } = await res.json();
  if (!setup_required) return;

  await page.goto('/');
  await page.getByPlaceholder('admin@example.com').fill(ADMIN_EMAIL);
  await page.getByPlaceholder('At least 8 characters').fill(ADMIN_PASSWORD);
  await page.getByPlaceholder('Confirm your password').fill(ADMIN_PASSWORD);
  await page.getByRole('button', { name: 'Create Account' }).click();

  // Wait for redirect to dashboard
  await page.waitForURL('**/projects');
}

/**
 * Login via the UI login page.
 */
export async function login(page: Page, email = ADMIN_EMAIL, password = ADMIN_PASSWORD): Promise<void> {
  await page.goto('/login');
  await page.getByPlaceholder('you@example.com').fill(email);
  await page.getByPlaceholder('Your password').fill(password);
  await page.getByRole('button', { name: 'Sign In' }).click();

  // Wait for redirect to dashboard
  await page.waitForURL('**/projects');
}

/**
 * Logout via the topbar dropdown menu.
 */
export async function logout(page: Page): Promise<void> {
  // Open the user dropdown in the topbar
  await page.locator('header').getByRole('button').last().click();
  await page.getByRole('menuitem', { name: 'Log out' }).click();

  // Wait for redirect to login page
  await page.waitForURL('**/login');
}
