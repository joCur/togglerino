import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

test.describe('Flags', () => {
  test('creates a boolean flag', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}`);

    await page.getByRole('button', { name: 'Create Flag' }).click();

    // Wait for the template chooser dialog to appear, then click "Blank"
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await dialog.getByText('Blank', { exact: true }).click();

    const key = uniqueFlagKey();
    await dialog.getByPlaceholder('Dark Mode').fill(`E2E Flag ${key}`);
    await dialog.getByPlaceholder('dark-mode').clear();
    await dialog.getByPlaceholder('dark-mode').fill(key);

    await dialog.getByRole('button', { name: 'Create Flag' }).click();

    // Dialog closes and flag appears in the list (no redirect to detail page)
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(key, { exact: true }).first()).toBeVisible();
  });

  test('toggles flag on', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Toggle Test ${key}` });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Wait for the environment config to load
    await expect(page.getByRole('switch').first()).toBeVisible();

    // Click the first toggle switch (development environment)
    const toggle = page.getByRole('switch').first();
    await toggle.click();

    // Wait for the API response to complete
    await page.waitForResponse(resp =>
      resp.url().includes(`/flags/${key}/environments/`) && resp.request().method() === 'PUT'
    );

    // Verify toggle is now checked
    await expect(toggle).toBeChecked();
  });

  test('persists toggle state', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Persist Test ${key}` });
    // Enable via API
    await apiContext.updateFlagEnvConfig(testProject.key, key, 'development', { enabled: true });

    // Load the flag page
    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Verify toggle shows enabled state
    const toggle = page.getByRole('switch').first();
    await expect(toggle).toBeChecked();

    // Refresh and verify still enabled
    await page.reload();
    await expect(page.getByRole('switch').first()).toBeChecked();
  });

  test('creates different value types', async ({ authenticatedPage: page, testProject, apiContext }) => {
    // Create flags of different types via API and verify they appear
    const stringKey = uniqueFlagKey();
    const numberKey = uniqueFlagKey();
    const jsonKey = uniqueFlagKey();

    await apiContext.createFlag(testProject.key, {
      key: stringKey,
      name: `String Flag`,
      value_type: 'string',
      default_value: 'hello',
    });
    await apiContext.createFlag(testProject.key, {
      key: numberKey,
      name: `Number Flag`,
      value_type: 'number',
      default_value: 42,
    });
    await apiContext.createFlag(testProject.key, {
      key: jsonKey,
      name: `JSON Flag`,
      value_type: 'json',
      default_value: { feature: true },
    });

    // Verify all flags appear in the project flag list
    await page.goto(`/projects/${testProject.key}`);
    await expect(page.getByText(stringKey, { exact: true })).toBeVisible();
    await expect(page.getByText(numberKey, { exact: true })).toBeVisible();
    await expect(page.getByText(jsonKey, { exact: true })).toBeVisible();
  });

  test('toggles flag off', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Off Test ${key}` });
    await apiContext.updateFlagEnvConfig(testProject.key, key, 'development', { enabled: true });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Verify it's on first
    const toggle = page.getByRole('switch').first();
    await expect(toggle).toBeChecked();

    // Toggle it off
    await toggle.click();

    // Wait for the API response
    await page.waitForResponse(resp =>
      resp.url().includes(`/flags/${key}/environments/`) && resp.request().method() === 'PUT'
    );

    // Reload page and verify UI persists the off state
    await page.reload();
    await expect(page.getByRole('switch').first()).not.toBeChecked();
  });

  test('archives a flag', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Archive Test ${key}` });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Wait for the page to load — use the heading which contains the flag key
    await expect(page.getByRole('heading', { name: key })).toBeVisible();

    // Open the settings gear dropdown menu
    await page.locator('button:has(svg.lucide-settings)').click();

    // Click "Archive" in the dropdown menu
    await page.getByRole('menuitem', { name: 'Archive' }).click();

    // Confirm in the archive dialog
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Archive' }).click();

    // Verify the flag shows archived status
    await expect(page.getByText('archived')).toBeVisible();
  });
});
