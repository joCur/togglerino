import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Definitions Endpoint', () => {
  test('returns flag definitions with variants and targeting rules', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: `Definitions Test ${flagKey}`,
      value_type: 'string',
      default_value: 'default-val',
    });

    // Set up variants and a targeting rule
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'off',
      variants: [
        { key: 'off', value: 'default-val' },
        { key: 'pro', value: 'pro-features' },
      ],
      targeting_rules: [
        {
          variant: 'pro',
          conditions: [{ attribute: 'plan', operator: 'equals', value: 'pro' }],
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-definitions');
    const client = new SDKClient(BASE_URL, sdkKey.key);
    const defs = await client.getDefinitions();

    // Find our flag in the response
    const flag = defs.flags.find(f => f.key === flagKey);
    expect(flag).toBeDefined();
    expect(flag!.valueType).toBe('string');
    expect(flag!.status).toBe('active');
    expect(flag!.config.enabled).toBe(true);
    expect(flag!.config.defaultVariant).toBe('off');
    expect(flag!.config.variants).toHaveLength(2);
    expect(flag!.config.variants.map(v => v.key).sort()).toEqual(['off', 'pro']);

    // Targeting rule should be present with condition
    expect(flag!.config.targetingRules).toHaveLength(1);
    expect(flag!.config.targetingRules[0].variant).toBe('pro');
    expect(flag!.config.targetingRules[0].conditions).toHaveLength(1);
    expect(flag!.config.targetingRules[0].conditions[0]).toEqual({
      attribute: 'plan',
      operator: 'equals',
      value: 'pro',
    });
  });

  test('returns empty arrays when no flags exist', async ({ apiContext, testProject }) => {
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-defs-empty');
    const client = new SDKClient(BASE_URL, sdkKey.key);
    const defs = await client.getDefinitions();

    expect(defs.flags).toEqual([]);
    expect(defs.segments).toEqual([]);
  });

  test('includes segments in the response', async ({ apiContext, testProject }) => {
    // Create a segment
    await apiContext.createSegment(testProject.key, {
      key: 'beta-users',
      name: 'Beta Users',
      conditions: [{ attribute: 'beta', operator: 'equals', value: 'true' }],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-defs-segments');
    const client = new SDKClient(BASE_URL, sdkKey.key);
    const defs = await client.getDefinitions();

    const segment = defs.segments.find(s => s.key === 'beta-users');
    expect(segment).toBeDefined();
    expect(segment!.conditions).toHaveLength(1);
    expect(segment!.conditions[0]).toEqual({
      attribute: 'beta',
      operator: 'equals',
      value: 'true',
    });
  });

  test('includes archived flags', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Archive Test',
      value_type: 'boolean',
      default_value: true,
    });
    await apiContext.archiveFlag(testProject.key, flagKey, true);

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-defs-archived');
    const client = new SDKClient(BASE_URL, sdkKey.key);
    const defs = await client.getDefinitions();

    const flag = defs.flags.find(f => f.key === flagKey);
    expect(flag).toBeDefined();
    expect(flag!.status).toBe('archived');
  });

  test('is scoped to the SDK key environment', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Env Scope Defs',
      value_type: 'boolean',
      default_value: true,
    });

    // Enable in development, disable in staging
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'staging', { enabled: false });

    const devKey = await apiContext.createSDKKey(testProject.key, 'development', 'e2e-defs-dev');
    const stagingKey = await apiContext.createSDKKey(testProject.key, 'staging', 'e2e-defs-staging');

    const devClient = new SDKClient(BASE_URL, devKey.key);
    const stagingClient = new SDKClient(BASE_URL, stagingKey.key);

    const devDefs = await devClient.getDefinitions();
    const stagingDefs = await stagingClient.getDefinitions();

    const devFlag = devDefs.flags.find(f => f.key === flagKey);
    const stagingFlag = stagingDefs.flags.find(f => f.key === flagKey);

    expect(devFlag!.config.enabled).toBe(true);
    expect(stagingFlag!.config.enabled).toBe(false);
  });

  test('invalid SDK key is rejected', async () => {
    const res = await fetch(`${BASE_URL}/api/v1/definitions`, {
      headers: { Authorization: 'Bearer invalid-key' },
    });
    expect(res.status).toBe(401);
  });
});
