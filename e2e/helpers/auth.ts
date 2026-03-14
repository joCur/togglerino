import type { Page } from '@playwright/test';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from './test-data.js';

/**
 * Idempotent setup — creates the first admin user if the database is fresh.
 * Uses the API to check status and the UI to create the account (since
 * the setup form is only shown once).
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
 * Login via the API to get a session cookie, then navigate to the app.
 * Uses the API instead of the UI form to avoid rate limit pressure on
 * the auth endpoints (10 req/60s per IP).
 */
export async function login(page: Page, email = ADMIN_EMAIL, password = ADMIN_PASSWORD): Promise<void> {
  const res = await page.request.post('/api/v1/auth/login', {
    data: { email, password },
  });
  if (!res.ok()) {
    throw new Error(`Login failed: ${res.status()} ${await res.text()}`);
  }
  // Navigate to the dashboard — the session cookie is already set
  await page.goto('/projects');
}

/**
 * Logout via the topbar dropdown menu.
 */
export async function logout(page: Page): Promise<void> {
  // Open the user dropdown by clicking the element containing the user email
  await page.getByText(ADMIN_EMAIL).click();
  await page.getByRole('menuitem', { name: 'Log out' }).click();

  // Wait for redirect to login page
  await page.waitForURL('**/login');
}
