import { test, expect } from '../../helpers/fixtures.js';
import { uniqueProjectKey, uniqueEmail, uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';
const MEMBER_PASSWORD = 'MemberPass123!';

test.describe('Permissions & RBAC', () => {
  test('invite and accept flow (API + UI)', async ({ apiContext, browser, playwright }) => {
    // API flow: invite → accept → verify
    const email1 = uniqueEmail();
    const invite1 = await apiContext.inviteUser(email1, 'member');
    expect(invite1.token).toBeTruthy();

    const ctx = await playwright.request.newContext({ baseURL: BASE_URL });
    const acceptRes = await ctx.post('/api/v1/auth/accept-invite', {
      data: { token: invite1.token, password: MEMBER_PASSWORD },
    });
    expect(acceptRes.ok()).toBeTruthy();
    const body = await acceptRes.json();
    expect(body.email).toBe(email1);
    await ctx.dispose();

    // Duplicate acceptance returns 409
    const ctx2 = await playwright.request.newContext({ baseURL: BASE_URL });
    const dupeRes = await ctx2.post('/api/v1/auth/accept-invite', {
      data: { token: invite1.token, password: MEMBER_PASSWORD },
    });
    expect(dupeRes.status()).toBe(409);
    await ctx2.dispose();

    // UI flow: invite → accept via browser
    const email2 = uniqueEmail();
    const invite2 = await apiContext.inviteUser(email2, 'member');

    const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
    const page = await context.newPage();
    await page.goto(`${BASE_URL}/invite/${invite2.token}`);

    await page.getByPlaceholder('At least 8 characters').fill(MEMBER_PASSWORD);
    await page.getByPlaceholder('Confirm your password').fill(MEMBER_PASSWORD);
    await page.getByRole('button', { name: 'Create Account' }).click();
    await expect(page.getByText('account has been created')).toBeVisible();
    await context.close();
  });

  test('member role restrictions (cannot invite, create/delete projects; can read/write flags)', async ({ apiContext, testProject, playwright }) => {
    const memberEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(memberEmail, 'member');

    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();

    // Login as member
    const memberCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    const loginRes = await memberCtx.post('/api/v1/auth/login', {
      data: { email: memberEmail, password: MEMBER_PASSWORD },
    });
    expect(loginRes.ok()).toBeTruthy();

    // Verify member role
    const me = await (await memberCtx.get('/api/v1/auth/me')).json();
    expect(me.role).toBe('member');

    // Cannot invite users (requires org:users:manage)
    const inviteRes = await memberCtx.post('/api/v1/management/users/invite', {
      data: { email: 'x@example.com', role: 'member' },
    });
    expect(inviteRes.status()).toBe(403);

    // Cannot create projects (requires org:projects:create)
    const createRes = await memberCtx.post('/api/v1/projects', {
      data: { key: uniqueProjectKey(), name: 'Fail' },
    });
    expect(createRes.status()).toBe(403);

    // Can read flags on existing project (default base role = editor → flags:read)
    const flagsRes = await memberCtx.get(`/api/v1/projects/${testProject.key}/flags`);
    expect(flagsRes.ok()).toBeTruthy();

    // Can write flags (editor → flags:write)
    const flagKey = uniqueFlagKey();
    const flagRes = await memberCtx.post(`/api/v1/projects/${testProject.key}/flags`, {
      data: { key: flagKey, name: 'Member Flag', value_type: 'boolean' },
    });
    expect(flagRes.ok()).toBeTruthy();

    // Cannot access project settings (editor lacks project:settings)
    const membersRes = await memberCtx.get(`/api/v1/projects/${testProject.key}/members`);
    expect(membersRes.status()).toBe(403);

    // Cannot delete projects (requires org:projects:delete)
    const deleteRes = await memberCtx.delete(`/api/v1/projects/${testProject.key}`);
    expect(deleteRes.status()).toBe(403);

    await memberCtx.dispose();
  });

  test('viewer role can read but not write', async ({ apiContext, testProject, playwright }) => {
    const viewerEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(viewerEmail, 'member');

    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();

    // Assign viewer role on the test project
    await apiContext.addProjectMember(testProject.key, { email: viewerEmail, role: 'viewer' });

    // Login as viewer
    const viewerCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await viewerCtx.post('/api/v1/auth/login', {
      data: { email: viewerEmail, password: MEMBER_PASSWORD },
    });

    // Can read flags
    const listRes = await viewerCtx.get(`/api/v1/projects/${testProject.key}/flags`);
    expect(listRes.ok()).toBeTruthy();

    // Cannot create flags
    const createRes = await viewerCtx.post(`/api/v1/projects/${testProject.key}/flags`, {
      data: { key: uniqueFlagKey(), name: 'Fail', value_type: 'boolean' },
    });
    expect(createRes.status()).toBe(403);

    await viewerCtx.dispose();
  });

  test('project admin can manage members but not org settings', async ({ apiContext, testProject, playwright }) => {
    const paEmail = uniqueEmail();
    const invite = await apiContext.inviteUser(paEmail, 'member');

    const acceptCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await acceptCtx.post('/api/v1/auth/accept-invite', {
      data: { token: invite.token, password: MEMBER_PASSWORD },
    });
    await acceptCtx.dispose();

    // Make them project admin
    await apiContext.addProjectMember(testProject.key, { email: paEmail, role: 'admin' });

    // Login as project admin
    const paCtx = await playwright.request.newContext({ baseURL: BASE_URL });
    await paCtx.post('/api/v1/auth/login', { data: { email: paEmail, password: MEMBER_PASSWORD } });

    // Can list project members (project:settings permission)
    const membersRes = await paCtx.get(`/api/v1/projects/${testProject.key}/members`);
    expect(membersRes.ok()).toBeTruthy();

    // Cannot access org settings
    const orgRes = await paCtx.get('/api/v1/settings/base-project-role');
    expect(orgRes.status()).toBe(403);

    await paCtx.dispose();
  });
});
