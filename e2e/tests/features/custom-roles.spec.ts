import { test, expect } from '../../helpers/fixtures.js';
import { uniqueEmail, uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';
const MEMBER_PASSWORD = 'MemberPass123!';

function uniqueRoleName(): string {
  return `test-role-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
}

test.describe('Custom Roles', () => {
  test('creates a custom role via UI', async ({ authenticatedPage: page }) => {
    await page.goto('/settings/roles');

    // Verify built-in roles are listed
    await expect(page.getByText('admin', { exact: true })).toBeVisible();
    await expect(page.getByText('editor', { exact: true })).toBeVisible();
    await expect(page.getByText('viewer', { exact: true })).toBeVisible();

    // Create a new custom role
    await page.getByRole('button', { name: 'Create Role' }).click();

    const dialog = page.getByRole('dialog');
    const roleName = uniqueRoleName();
    await dialog.getByPlaceholder('e.g. reviewer').fill(roleName);
    await dialog.getByPlaceholder('A brief description').fill('E2E test custom role');

    // Select specific permissions by clicking the label text
    await dialog.getByText('View flags').click();

    await dialog.getByRole('button', { name: 'Create Role' }).click();

    // Dialog should close and role should appear in the table
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText(roleName)).toBeVisible();
  });

  test('custom role with read-only access restricts flag creation', async ({ apiContext, testProject, playwright }) => {
    const roleName = uniqueRoleName();
    const role = await apiContext.createRole({
      name: roleName,
      description: 'Read-only flag access',
      permissions: ['flags:read'],
    });
    expect(role.name).toBe(roleName);
    expect(role.is_built_in).toBe(false);

    // Create a member and assign the custom role
    const memberEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(memberEmail, 'member');
    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();

    await apiContext.addProjectMember(testProject.key, { email: memberEmail, role: roleName });

    // Login as member
    const memberCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await memberCtx.post('/api/v1/auth/login', {
      data: { email: memberEmail, password: MEMBER_PASSWORD },
    });

    // Can read flags (has flags:read)
    const listRes = await memberCtx.get(`/api/v1/projects/${testProject.key}/flags`);
    expect(listRes.ok()).toBeTruthy();

    // CANNOT create flags (lacks flags:write)
    const createRes = await memberCtx.post(`/api/v1/projects/${testProject.key}/flags`, {
      data: { key: uniqueFlagKey(), name: 'Should Fail', value_type: 'boolean' },
    });
    expect(createRes.status()).toBe(403);

    // CANNOT create environments (lacks environments:write)
    const envRes = await memberCtx.post(`/api/v1/projects/${testProject.key}/environments`, {
      data: { key: 'test-env', name: 'Test' },
    });
    expect(envRes.status()).toBe(403);

    await memberCtx.dispose();
  });

  test('custom role with write permissions allows flag creation but not settings', async ({ apiContext, testProject, playwright }) => {
    const roleName = uniqueRoleName();
    await apiContext.createRole({
      name: roleName,
      description: 'Flag writer without settings',
      permissions: ['flags:read', 'flags:write'],
    });

    const memberEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(memberEmail, 'member');
    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();

    await apiContext.addProjectMember(testProject.key, { email: memberEmail, role: roleName });

    const memberCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await memberCtx.post('/api/v1/auth/login', {
      data: { email: memberEmail, password: MEMBER_PASSWORD },
    });

    // CAN create flags
    const flagKey = uniqueFlagKey();
    const createRes = await memberCtx.post(`/api/v1/projects/${testProject.key}/flags`, {
      data: { key: flagKey, name: 'Custom Role Flag', value_type: 'boolean' },
    });
    expect(createRes.ok()).toBeTruthy();

    // CANNOT access project settings
    const settingsRes = await memberCtx.get(`/api/v1/projects/${testProject.key}/members`);
    expect(settingsRes.status()).toBe(403);

    await memberCtx.dispose();
  });

  test('custom role takes effect in the UI — hides write controls', async ({ apiContext, testProject, browser }) => {
    // Create a read-only role
    const roleName = uniqueRoleName();
    await apiContext.createRole({
      name: roleName,
      description: 'View-only for UI test',
      permissions: ['flags:read', 'environments:read'],
    });

    // Create a member and assign the role
    const memberEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(memberEmail, 'member');
    const acceptCtx = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const acceptPage = await acceptCtx.newPage();
    await acceptPage.goto(`${BASE_URL}/invite/${invite.token}`);
    await acceptPage.getByPlaceholder('At least 8 characters').fill(MEMBER_PASSWORD);
    await acceptPage.getByPlaceholder('Confirm your password').fill(MEMBER_PASSWORD);
    await acceptPage.getByRole('button', { name: 'Create Account' }).click();
    await expect(acceptPage.getByText('account has been created')).toBeVisible();
    await acceptCtx.close();

    await apiContext.addProjectMember(testProject.key, { email: memberEmail, role: roleName });

    // Create a flag so there's something to see
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Visible Flag',
      value_type: 'boolean',
    });

    // Login as the member
    const memberCtx = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await memberCtx.newPage();
    await page.goto(`${BASE_URL}/login`);
    await page.getByPlaceholder('you@example.com').fill(memberEmail);
    await page.getByPlaceholder('Your password').fill(MEMBER_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();
    await page.waitForURL('**/projects');

    // Navigate to project — should see flags
    await page.goto(`${BASE_URL}/projects/${testProject.key}`);
    await expect(page.getByText(flagKey, { exact: true })).toBeVisible();

    // "Create Flag" button should NOT be visible (no flags:write)
    await expect(page.getByRole('button', { name: 'Create Flag' })).not.toBeVisible();

    // "Settings" should NOT be in the sidebar (no project:settings)
    await expect(page.getByRole('link', { name: 'Settings' })).not.toBeVisible();

    await memberCtx.close();
  });

  test('cannot delete built-in roles', async ({ apiContext }) => {
    const roles = await apiContext.listRoles();
    const builtIn = roles.filter(r => r.is_built_in);
    expect(builtIn.length).toBeGreaterThanOrEqual(3);

    try {
      await apiContext.deleteRole('admin');
      expect(true).toBe(false);
    } catch (e: any) {
      expect(e.message).toContain('403');
    }
  });

  test('cannot delete a role that is in use', async ({ apiContext, testProject, playwright }) => {
    const roleName = uniqueRoleName();
    await apiContext.createRole({
      name: roleName,
      description: 'Role in use test',
      permissions: ['flags:read'],
    });

    const memberEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(memberEmail, 'member');
    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();
    await apiContext.addProjectMember(testProject.key, { email: memberEmail, role: roleName });

    // Delete should fail — role is in use
    try {
      await apiContext.deleteRole(roleName);
      expect(true).toBe(false);
    } catch (e: any) {
      expect(e.message).toContain('409');
    }
  });
});
