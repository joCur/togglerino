import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Boolean Flag Evaluation', () => {
  test('enabled boolean flag returns true', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Enabled Bool',
      value_type: 'boolean',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-enabled');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const result = await client.evaluateFlag(flagKey);
    expect(result.value).toBe(true);
    expect(result.reason).toBe('default');
  });

  test('disabled boolean flag returns false', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Disabled Bool',
      value_type: 'boolean',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: false });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-disabled');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const result = await client.evaluateFlag(flagKey);
    expect(result.value).toBe(false);
    expect(result.reason).toBe('disabled');
  });

  test('archived boolean flag returns false', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Archived Bool',
      value_type: 'boolean',
    });
    await apiContext.archiveFlag(testProject.key, flagKey, true);

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-archived');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const result = await client.evaluateFlag(flagKey);
    expect(result.value).toBe(false);
    expect(result.reason).toBe('archived');
  });

  test('targeting rule can serve false on a boolean flag', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Rule False Bool',
      value_type: 'boolean',
    });

    // Rule: blocked users get false, everyone else gets true (fallthrough)
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      fallthrough_variant: 'true',
      off_variant: 'false',
      variants: [
        { name: 'true', value: true },
        { name: 'false', value: false },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'blocked', operator: 'equals', value: true }],
          variant: 'false',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-rule');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Blocked user → false (rule match)
    const blocked = await client.evaluateFlag(flagKey, { attributes: { blocked: true } });
    expect(blocked.value).toBe(false);
    expect(blocked.reason).toBe('rule_match');

    // Normal user → true (fallthrough)
    const normal = await client.evaluateFlag(flagKey, { attributes: { blocked: false } });
    expect(normal.value).toBe(true);
    expect(normal.reason).toBe('default');
  });

  test('targeting rule can serve true on a boolean flag', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Rule True Bool',
      value_type: 'boolean',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      fallthrough_variant: 'true',
      off_variant: 'false',
      variants: [
        { name: 'true', value: true },
        { name: 'false', value: false },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'plan', operator: 'equals', value: 'enterprise' }],
          variant: 'true',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-rule-true');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Enterprise → true (rule match)
    const enterprise = await client.evaluateFlag(flagKey, { attributes: { plan: 'enterprise' } });
    expect(enterprise.value).toBe(true);
    expect(enterprise.reason).toBe('rule_match');

    // Free → true (fallthrough, same value but different reason)
    const free = await client.evaluateFlag(flagKey, { attributes: { plan: 'free' } });
    expect(free.value).toBe(true);
    expect(free.reason).toBe('default');
  });

  test('boolean flag toggle in UI directly controls evaluation', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Toggle Eval Test',
      value_type: 'boolean',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: false });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'bool-toggle');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Initially disabled → false
    const before = await client.evaluateFlag(flagKey);
    expect(before.value).toBe(false);

    // Toggle ON via UI — click the "Targeting is OFF" button, then save
    await page.goto(`/projects/${testProject.key}/flags/${flagKey}`);
    const toggleBtn = page.getByRole('button', { name: /^OFF$/i });
    await expect(toggleBtn).toBeVisible();
    await toggleBtn.click();
    await expect(page.getByRole('button', { name: /^ON$/i })).toBeVisible();

    // Save the change
    await page.getByRole('button', { name: /save/i }).click();
    await page.waitForResponse(resp =>
      resp.url().includes(`/flags/${flagKey}/environments/`) && resp.request().method() === 'PUT'
    );

    // Now evaluates to true
    const after = await client.evaluateFlag(flagKey);
    expect(after.value).toBe(true);
  });
});
