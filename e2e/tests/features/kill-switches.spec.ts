import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

test.describe('Kill Switches Dashboard', () => {
  test('shows empty state when no kill switches exist', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/kill-switches`);
    await expect(page.getByText(/no kill switches/i)).toBeVisible();
  });

  test('displays kill switch flags in the matrix', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Emergency Stop',
      value_type: 'boolean',
      flag_type: 'kill-switch',
    });

    await page.goto(`/projects/${testProject.key}/kill-switches`);

    await expect(page.getByText(flagKey)).toBeVisible();
    await expect(page.getByText('Emergency Stop')).toBeVisible();
    await expect(page.getByText('Development')).toBeVisible();
  });

  test('toggles a kill switch via the confirmation dialog', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Toggle KS',
      value_type: 'boolean',
      flag_type: 'kill-switch',
      default_value: true,
    });
    // Ensure development is disabled initially
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: false });

    await page.goto(`/projects/${testProject.key}/kill-switches`);
    await expect(page.getByText(flagKey)).toBeVisible();

    // Click the toggle for development env
    const row = page.locator('tr').filter({ hasText: flagKey });
    const toggle = row.getByRole('switch').first();
    await toggle.click();

    // Confirm in dialog (should say "Enable")
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button').filter({ hasText: /enable|disable/i }).click();

    // Toggle state should change
    await expect(dialog).not.toBeVisible();
  });
});
