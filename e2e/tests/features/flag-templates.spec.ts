import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

function uniqueTemplateKey(): string {
  return `test-tmpl-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
}

test.describe('Flag Templates', () => {
  test('creates a project template via UI', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/templates`);

    await page.getByRole('button', { name: 'Create Template' }).click();

    const dialog = page.getByRole('dialog');
    await dialog.getByPlaceholder('e.g. Feature Flag').fill('E2E Template');
    // Key auto-generates from name
    await expect(dialog.getByPlaceholder('feature-flag')).toHaveValue('e2e-template');

    // Set flag type and value type
    await dialog.locator('select').first().selectOption('release');
    await dialog.locator('select').nth(1).selectOption('boolean');

    await dialog.getByRole('button', { name: /create template/i }).click();

    await expect(dialog).not.toBeVisible();
    await expect(page.getByText('E2E Template')).toBeVisible();
  });

  test('uses a template when creating a flag', async ({ authenticatedPage: page, testProject, apiContext }) => {
    // Create a template via API
    await page.request.post(`/api/v1/projects/${testProject.key}/templates`, {
      data: {
        key: uniqueTemplateKey(),
        name: 'Quick Release',
        flag_type: 'release',
        value_type: 'boolean',
        default_value: false,
        tags: ['release'],
      },
    });

    // Go to project flags and create a flag
    await page.goto(`/projects/${testProject.key}`);
    await page.getByRole('button', { name: 'Create Flag' }).click();

    // Template chooser should show our template
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Quick Release')).toBeVisible();

    // Click the template
    await dialog.getByText('Quick Release').click();

    // Form should be pre-filled with template values
    const flagKey = uniqueFlagKey();
    await dialog.getByPlaceholder('Dark Mode').fill(`Template Flag ${flagKey}`);
    await dialog.getByPlaceholder('dark-mode').clear();
    await dialog.getByPlaceholder('dark-mode').fill(flagKey);

    await dialog.getByRole('button', { name: 'Create Flag' }).click();
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(flagKey, { exact: true }).first()).toBeVisible();
  });

  test('deletes a project template', async ({ authenticatedPage: page, testProject }) => {
    const tmplKey = uniqueTemplateKey();
    await page.request.post(`/api/v1/projects/${testProject.key}/templates`, {
      data: { key: tmplKey, name: 'Delete Me', flag_type: 'release', value_type: 'boolean' },
    });

    await page.goto(`/projects/${testProject.key}/templates`);
    await expect(page.getByText(tmplKey, { exact: true })).toBeVisible();

    // Click the row to edit
    const row = page.locator('tr').filter({ hasText: tmplKey });
    await row.click();

    // Click Delete in the edit dialog
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Delete' }).click();
    // Confirm delete
    await dialog.getByRole('button', { name: /confirm delete/i }).click();

    // Wait for dialog to close, then verify template is gone from table
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(tmplKey, { exact: true })).not.toBeVisible();
  });
});
