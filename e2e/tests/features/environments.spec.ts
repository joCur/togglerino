import { test, expect } from '../../helpers/fixtures.js';

test.describe('Environment Management', () => {
  test('creates a new environment', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/environments`);

    await page.getByRole('button', { name: 'Create Environment' }).click();

    // The form uses labels KEY and NAME (uppercase)
    await page.getByPlaceholder('e.g. staging', { exact: true }).fill('qa');
    await page.getByPlaceholder('e.g. Staging', { exact: true }).fill('QA Environment');

    // Click the Create button (not the "Create Environment" header button)
    await page.getByRole('button', { name: 'Create', exact: true }).click();

    // Wait for the form to close and new environment to appear
    await expect(page.getByRole('cell', { name: 'qa', exact: true })).toBeVisible();
  });

  test('default environments exist in correct order', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/environments`);

    await expect(page.getByRole('cell', { name: 'development', exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: 'staging', exact: true })).toBeVisible();
    await expect(page.getByRole('cell', { name: 'production', exact: true })).toBeVisible();
  });

  test('deletes an environment', async ({ authenticatedPage: page, testProject }) => {
    // Create an extra environment via API
    await page.request.post(`/api/v1/projects/${testProject.key}/environments`, {
      data: { key: 'temp-env', name: 'Temporary' },
    });

    await page.goto(`/projects/${testProject.key}/environments`);
    await expect(page.locator('td').filter({ hasText: 'temp-env' })).toBeVisible();

    // Click delete button in the temp-env row
    const row = page.locator('tr').filter({ hasText: 'temp-env' });
    await row.locator('button').last().click();

    // Confirm in dialog
    const dialog = page.getByRole('dialog', { name: /delete environment/i });
    await dialog.getByRole('button', { name: 'Delete' }).click();

    await expect(page.locator('td').filter({ hasText: 'temp-env' })).not.toBeVisible();
  });

  test('navigates to SDK keys page', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/environments`);

    const devRow = page.locator('tr').filter({ hasText: 'development' });
    await devRow.getByRole('link', { name: /manage sdk keys/i }).click();

    await page.waitForURL(`**/environments/development/sdk-keys`);
  });
});
