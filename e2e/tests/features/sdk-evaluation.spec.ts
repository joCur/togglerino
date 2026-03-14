import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('SDK Evaluation', () => {
  test('creates SDK key and evaluates a boolean flag', async ({ apiContext, testProject }) => {
    // Create a flag with default_value: true and enable it in development
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: `Eval Test ${flagKey}`,
      value_type: 'boolean',
      default_value: true,
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    // Create an SDK key for the development environment
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-test-key');
    expect(sdkKey.key).toBeTruthy();

    // Evaluate using the SDK key
    const client = new SDKClient(BASE_URL, sdkKey.key);
    const result = await client.evaluateFlag(flagKey);

    expect(result.value).toBe(true);
    expect(result.reason).toBe('default');
  });

  test('evaluates all flags at once', async ({ apiContext, testProject }) => {
    const flag1 = uniqueFlagKey();
    const flag2 = uniqueFlagKey();

    await apiContext.createFlag(testProject.key, {
      key: flag1,
      name: `All Eval 1`,
      value_type: 'boolean',
      default_value: true,
    });
    await apiContext.createFlag(testProject.key, {
      key: flag2,
      name: `All Eval 2`,
      value_type: 'string',
      default_value: 'hello',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flag1, 'development', { enabled: true });
    await apiContext.updateFlagEnvConfig(testProject.key, flag2, 'development', { enabled: true });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-all-flags');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const flags = await client.evaluateAll();

    expect(flags[flag1]).toBeDefined();
    expect(flags[flag1].value).toBe(true);
    expect(flags[flag2]).toBeDefined();
    expect(flags[flag2].value).toBe('hello');
  });

  test('disabled flag returns default value with disabled reason', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: `Disabled Test`,
      value_type: 'string',
      default_value: 'fallback',
    });
    // Do NOT enable the flag — leave it disabled in development
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: false });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-disabled');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const result = await client.evaluateFlag(flagKey);

    // Disabled flags return the flag's default_value with reason "disabled"
    expect(result.value).toBe('fallback');
    expect(result.reason).toBe('disabled');
  });

  test('evaluates string, number, and json flag types', async ({ apiContext, testProject }) => {
    const stringFlag = uniqueFlagKey();
    const numberFlag = uniqueFlagKey();
    const jsonFlag = uniqueFlagKey();

    await apiContext.createFlag(testProject.key, {
      key: stringFlag,
      name: 'String Flag',
      value_type: 'string',
      default_value: 'variant-a',
    });
    await apiContext.createFlag(testProject.key, {
      key: numberFlag,
      name: 'Number Flag',
      value_type: 'number',
      default_value: 42,
    });
    await apiContext.createFlag(testProject.key, {
      key: jsonFlag,
      name: 'JSON Flag',
      value_type: 'json',
      default_value: { theme: 'dark', version: 2 },
    });

    // Enable all flags
    for (const key of [stringFlag, numberFlag, jsonFlag]) {
      await apiContext.updateFlagEnvConfig(testProject.key, key, 'development', { enabled: true });
    }

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-types');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const stringResult = await client.evaluateFlag(stringFlag);
    expect(stringResult.value).toBe('variant-a');

    const numberResult = await client.evaluateFlag(numberFlag);
    expect(numberResult.value).toBe(42);

    const jsonResult = await client.evaluateFlag(jsonFlag);
    expect(jsonResult.value).toEqual({ theme: 'dark', version: 2 });
  });

  test('invalid SDK key is rejected', async ({ testProject }) => {
    const client = new SDKClient(BASE_URL, 'invalid-key-that-does-not-exist');

    const res = await fetch(`${BASE_URL}/api/v1/evaluate`, {
      method: 'POST',
      headers: {
        Authorization: 'Bearer invalid-key-that-does-not-exist',
        'Content-Type': 'application/json',
      },
      body: '{}',
    });

    expect(res.status).toBe(401);
  });

  test('SDK key is scoped to its environment', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Env Scope Test',
      value_type: 'boolean',
      default_value: true,
    });

    // Enable in development, leave disabled in staging
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'staging', { enabled: false });

    // Create SDK keys for both environments
    const devKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-dev');
    const stagingKey = await apiContext.createSDKKey(testProject.key, 'staging', 'e2e-staging');

    const devClient = new SDKClient(BASE_URL, devKey.key);
    const stagingClient = new SDKClient(BASE_URL, stagingKey.key);

    // Development should return enabled with reason "default" (evaluation ran)
    const devResult = await devClient.evaluateFlag(flagKey);
    expect(devResult.value).toBe(true);
    expect(devResult.reason).toBe('default');

    // Staging should return the flag's default value but with reason "disabled"
    // (disabled means evaluation is skipped, not that the value is false)
    const stagingResult = await stagingClient.evaluateFlag(flagKey);
    expect(stagingResult.value).toBe(true); // default_value is true
    expect(stagingResult.reason).toBe('disabled');
  });

  test('evaluation with context passes user_id', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Context Test',
      value_type: 'boolean',
      default_value: true,
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-context');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Evaluate with context — should still return the flag value
    const result = await client.evaluateFlag(flagKey, {
      user_id: 'user-123',
      attributes: { plan: 'enterprise', country: 'US' },
    });

    expect(result.value).toBe(true);
  });

  test('non-existent flag returns 404', async ({ apiContext, testProject }) => {
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-404');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const { status } = await client.evaluateFlagRaw('flag-that-does-not-exist');
    expect(status).toBe(404);
  });
});
