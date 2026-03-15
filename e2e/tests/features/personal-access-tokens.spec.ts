import { test, expect } from '../../helpers/fixtures.js';
import { uniqueProjectKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Personal Access Tokens', () => {
  test('creates a PAT and uses it for API access', async ({ authenticatedPage: page }) => {
    await page.goto('/account');

    // Create a new token
    await page.getByRole('button', { name: 'Create token' }).click();

    const dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('e.g. CI deploy token').fill('E2E Test Token');
    await dialog.getByRole('button', { name: 'Create token' }).click();

    // Token should be displayed (shown once)
    await expect(dialog.getByText('pat_')).toBeVisible();

    // Copy the token value for later use
    const tokenText = await dialog.locator('code, .font-mono').filter({ hasText: 'pat_' }).textContent();
    expect(tokenText).toMatch(/^pat_[a-f0-9]+$/);

    // Close the dialog
    await dialog.getByRole('button', { name: /close|done|copy/i }).first().click();

    // Token should appear in the tokens table
    await expect(page.getByText('E2E Test Token')).toBeVisible();

    // Use the PAT for API access
    const res = await fetch(`${BASE_URL}/api/v1/auth/me`, {
      headers: { Authorization: `Bearer ${tokenText}` },
    });
    expect(res.ok).toBeTruthy();
    const me = await res.json();
    expect(me.email).toBeTruthy();
  });

  test('revokes a PAT', async ({ authenticatedPage: page }) => {
    // Create a token via API
    const createRes = await page.request.post('/api/v1/auth/tokens', {
      data: { name: 'Revoke Me Token' },
    });
    expect(createRes.ok()).toBeTruthy();

    await page.goto('/account');
    await expect(page.getByText('Revoke Me Token')).toBeVisible();

    // Click revoke button
    const row = page.locator('tr').filter({ hasText: 'Revoke Me Token' });
    await row.getByRole('button', { name: /revoke/i }).click();

    // Token should be removed from the list
    await expect(page.getByText('Revoke Me Token')).not.toBeVisible();
  });
});
