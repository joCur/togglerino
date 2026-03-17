import { test, expect } from '../../helpers/fixtures.js';
import { uniqueProjectKey } from '../../helpers/test-data.js';

test.describe('Projects', () => {
  test('creates a project', async ({ authenticatedPage: page }) => {
    await page.goto('/projects');
    await expect(page.getByRole('heading', { name: 'Projects' })).toBeVisible();

    await page.getByRole('button', { name: 'Create Project' }).click();

    const key = uniqueProjectKey();
    await page.getByPlaceholder('My Project').fill(`E2E Project ${key}`);
    // Clear the auto-generated key and type our own
    await page.getByPlaceholder('my-project').clear();
    await page.getByPlaceholder('my-project').fill(key);

    // Click the submit button inside the dialog
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: 'Create Project' }).click();

    // Wait for dialog to close and project to appear in list
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(key, { exact: true }).first()).toBeVisible();
  });

  test('shows default environments', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/environments`);

    // Verify the 3 default environments exist
    await expect(page.getByText('development', { exact: true })).toBeVisible();
    await expect(page.getByText('staging', { exact: true })).toBeVisible();
    await expect(page.getByText('production', { exact: true })).toBeVisible();
  });

  test('lists multiple projects', async ({ authenticatedPage: page, apiContext }) => {
    const key1 = uniqueProjectKey();
    const key2 = uniqueProjectKey();
    await apiContext.createProject({ key: key1, name: `Project ${key1}` });
    await apiContext.createProject({ key: key2, name: `Project ${key2}` });

    await page.goto('/projects');
    await expect(page.getByText(key1, { exact: true }).first()).toBeVisible();
    await expect(page.getByText(key2, { exact: true }).first()).toBeVisible();

    // Cleanup
    await apiContext.deleteProject(key1);
    await apiContext.deleteProject(key2);
  });

  test('deletes a project', async ({ authenticatedPage: page, apiContext }) => {
    const key = uniqueProjectKey();
    await apiContext.createProject({ key, name: `Delete Me ${key}` });

    // Navigate into the project
    await page.goto(`/projects/${key}`);
    await expect(page.getByRole('heading', { name: key })).toBeVisible();

    // Navigate to project settings via sidebar
    await page.getByRole('link', { name: 'Settings' }).click();

    // Click "Delete Project" to reveal the confirmation input
    await page.getByRole('button', { name: 'Delete Project' }).click();

    // Type the project key in the confirmation input
    await page.getByPlaceholder(key).fill(key);

    // Click the "Delete" confirmation button
    await page.getByRole('button', { name: 'Delete', exact: true }).click();

    // Should redirect back to projects list
    await page.waitForURL('**/projects');
    await expect(page.getByText(key)).not.toBeVisible();
  });
});
