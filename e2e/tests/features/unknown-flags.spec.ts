import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Unknown Flags', () => {
  test('evaluating a non-existent flag records it as unknown', async ({ authenticatedPage: page, testProject, apiContext }) => {
    // Create an SDK key
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'unknown-flag-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Evaluate a flag that doesn't exist — this should record it as unknown
    const ghostKey = uniqueFlagKey();
    const { status } = await client.evaluateFlagRaw(ghostKey);
    expect(status).toBe(404);

    // Navigate to the project page and check the Unknown Flags tab
    await page.goto(`/projects/${testProject.key}`);

    // Click the "Unknown Flags" tab
    await page.getByRole('tab', { name: /unknown flags/i }).click();

    // The unknown flag should appear in the list
    await expect(page.getByText(ghostKey, { exact: true })).toBeVisible();
  });

  test('dismissing an unknown flag removes it from the list', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'dismiss-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Evaluate a non-existent flag
    const ghostKey = uniqueFlagKey();
    await client.evaluateFlagRaw(ghostKey);

    await page.goto(`/projects/${testProject.key}`);
    await page.getByRole('tab', { name: /unknown flags/i }).click();
    await expect(page.getByText(ghostKey, { exact: true })).toBeVisible();

    // Dismiss the unknown flag
    const row = page.locator('tr').filter({ hasText: ghostKey });
    await row.getByRole('button', { name: /dismiss/i }).click();

    // Should be removed from the list
    await expect(page.getByText(ghostKey, { exact: true })).not.toBeVisible();
  });
});
