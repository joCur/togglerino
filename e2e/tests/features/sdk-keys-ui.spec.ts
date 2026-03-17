import { test, expect } from '../../helpers/fixtures.js';

test.describe('SDK Keys UI', () => {
  test('creates and displays an SDK key', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/environments/development/sdk-keys`);

    await page.getByRole('button', { name: 'Generate New Key' }).click();
    await page.getByPlaceholder('e.g. Backend Service Key').fill('E2E Test Key');
    await page.getByRole('button', { name: 'Generate', exact: true }).click();

    // Key should appear in the table with Active status
    await expect(page.getByText('E2E Test Key')).toBeVisible();
    await expect(page.getByText('Active')).toBeVisible();
  });

  test('revokes an SDK key', async ({ authenticatedPage: page, testProject }) => {
    await page.request.post(`/api/v1/projects/${testProject.key}/environments/development/sdk-keys`, {
      data: { name: 'Revoke Me' },
    });

    await page.goto(`/projects/${testProject.key}/environments/development/sdk-keys`);

    // Set up dialog handler before clicking revoke
    page.on('dialog', dialog => dialog.accept());

    const row = page.locator('tr').filter({ hasText: 'Revoke Me' });
    await row.getByRole('button', { name: /revoke/i }).click();

    // Status should change to Revoked
    await expect(row.getByText('Revoked')).toBeVisible();
  });

  test('different environments have separate keys', async ({ authenticatedPage: page, testProject }) => {
    await page.request.post(`/api/v1/projects/${testProject.key}/environments/development/sdk-keys`, {
      data: { name: 'Dev Only Key' },
    });
    await page.request.post(`/api/v1/projects/${testProject.key}/environments/staging/sdk-keys`, {
      data: { name: 'Staging Only Key' },
    });

    // Dev page shows only dev key
    await page.goto(`/projects/${testProject.key}/environments/development/sdk-keys`);
    await expect(page.getByText('Dev Only Key')).toBeVisible();
    await expect(page.getByText('Staging Only Key')).not.toBeVisible();

    // Staging page shows only staging key
    await page.goto(`/projects/${testProject.key}/environments/staging/sdk-keys`);
    await expect(page.getByText('Staging Only Key')).toBeVisible();
    await expect(page.getByText('Dev Only Key')).not.toBeVisible();
  });
});
