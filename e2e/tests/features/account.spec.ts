import { test, expect } from '../../helpers/fixtures.js';
import { ADMIN_EMAIL, ADMIN_PASSWORD } from '../../helpers/test-data.js';

test.describe('Account Management', () => {
  test('updates display name', async ({ authenticatedPage: page }) => {
    await page.goto('/account');

    const nameInput = page.getByPlaceholder('Your display name');
    await nameInput.clear();
    await nameInput.fill('E2E Admin');
    await page.getByRole('button', { name: 'Save changes' }).click();

    // Reload and verify persisted
    await page.reload();
    await expect(page.getByPlaceholder('Your display name')).toHaveValue('E2E Admin');

    // Clean up
    await page.getByPlaceholder('Your display name').clear();
    await page.getByRole('button', { name: 'Save changes' }).click();
  });

  test('changes password and changes back', async ({ authenticatedPage: page }) => {
    await page.goto('/account');

    // The password inputs are type="password" within the page
    const pwInputs = page.locator('input[type="password"]');

    await pwInputs.nth(0).fill(ADMIN_PASSWORD);
    await pwInputs.nth(1).fill('NewPassword456!');
    await pwInputs.nth(2).fill('NewPassword456!');
    await page.getByRole('button', { name: 'Change password' }).click();

    // Wait for the fields to clear (indicates success)
    await expect(pwInputs.nth(0)).toHaveValue('', { timeout: 5000 });

    // Change back
    await pwInputs.nth(0).fill('NewPassword456!');
    await pwInputs.nth(1).fill(ADMIN_PASSWORD);
    await pwInputs.nth(2).fill(ADMIN_PASSWORD);
    await page.getByRole('button', { name: 'Change password' }).click();

    await expect(pwInputs.nth(0)).toHaveValue('', { timeout: 5000 });
  });

  test('shows account info', async ({ authenticatedPage: page }) => {
    await page.goto('/account');
    await expect(page.getByText(ADMIN_EMAIL)).toBeVisible();
  });
});
