import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

test.describe('Targeting Rules', () => {
  test('matches a simple equals condition', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Targeting Test',
      value_type: 'string',
      default_value: 'default',
    });

    // Configure with a targeting rule: plan=enterprise → "enterprise-value"
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'control',
      variants: [
        { key: 'control', value: 'default' },
        { key: 'enterprise', value: 'enterprise-value' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'plan', operator: 'equals', value: 'enterprise' }],
          variant: 'enterprise',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'targeting-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Matching context → enterprise variant
    const matched = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise' },
    });
    expect(matched.value).toBe('enterprise-value');
    expect(matched.variant).toBe('enterprise');
    expect(matched.reason).toBe('rule_match');

    // Non-matching context → default variant
    const unmatched = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'free' },
    });
    expect(unmatched.value).toBe('default');
    expect(unmatched.variant).toBe('control');
    expect(unmatched.reason).toBe('default');
  });

  test('evaluates rules in order (first match wins)', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Rule Order Test',
      value_type: 'string',
      default_value: 'fallback',
    });

    // Two rules: country=US → "us-value", plan=enterprise → "ent-value"
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'fallback',
      variants: [
        { key: 'fallback', value: 'fallback' },
        { key: 'us', value: 'us-value' },
        { key: 'ent', value: 'ent-value' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'country', operator: 'equals', value: 'US' }],
          variant: 'us',
        },
        {
          conditions: [{ attribute: 'plan', operator: 'equals', value: 'enterprise' }],
          variant: 'ent',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'order-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Matches both rules → first rule wins
    const result = await client.evaluateFlag(flagKey, {
      attributes: { country: 'US', plan: 'enterprise' },
    });
    expect(result.value).toBe('us-value');
    expect(result.variant).toBe('us');
  });

  test('multiple conditions in a rule use AND logic', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'AND Logic Test',
      value_type: 'boolean',
      default_value: false,
    });

    // Rule requires BOTH country=US AND plan=enterprise
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'off',
      variants: [
        { key: 'off', value: false },
        { key: 'on', value: true },
      ],
      targeting_rules: [
        {
          conditions: [
            { attribute: 'country', operator: 'equals', value: 'US' },
            { attribute: 'plan', operator: 'equals', value: 'enterprise' },
          ],
          variant: 'on',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'and-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Both conditions match → true
    const both = await client.evaluateFlag(flagKey, {
      attributes: { country: 'US', plan: 'enterprise' },
    });
    expect(both.value).toBe(true);

    // Only one condition matches → false (AND not satisfied)
    const partial = await client.evaluateFlag(flagKey, {
      attributes: { country: 'US', plan: 'free' },
    });
    expect(partial.value).toBe(false);
    expect(partial.reason).toBe('default');
  });

  test('supports in operator for list matching', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'In Operator Test',
      value_type: 'boolean',
      default_value: false,
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'off',
      variants: [
        { key: 'off', value: false },
        { key: 'on', value: true },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'country', operator: 'in', value: ['US', 'CA', 'UK'] }],
          variant: 'on',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'in-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Country in list → true
    const matched = await client.evaluateFlag(flagKey, { attributes: { country: 'CA' } });
    expect(matched.value).toBe(true);

    // Country not in list → false
    const unmatched = await client.evaluateFlag(flagKey, { attributes: { country: 'DE' } });
    expect(unmatched.value).toBe(false);
  });

  test('percentage rollout produces stable results', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Rollout Test',
      value_type: 'boolean',
      default_value: false,
    });

    // 50% rollout — consistent hashing should give same result for same user
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'off',
      variants: [
        { key: 'off', value: false },
        { key: 'on', value: true },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'plan', operator: 'exists', value: '' }],
          variant: 'on',
          percentage_rollout: 50,
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'rollout-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Same user should get consistent results across multiple evaluations
    const result1 = await client.evaluateFlag(flagKey, {
      user_id: 'user-stable-123',
      attributes: { plan: 'pro' },
    });
    const result2 = await client.evaluateFlag(flagKey, {
      user_id: 'user-stable-123',
      attributes: { plan: 'pro' },
    });
    expect(result1.value).toBe(result2.value);
    expect(result1.variant).toBe(result2.variant);

    // Different users may get different results (probabilistic, but with enough users
    // we should see both variants). Test with 20 users.
    const results = new Set<boolean>();
    for (let i = 0; i < 20; i++) {
      const r = await client.evaluateFlag(flagKey, {
        user_id: `rollout-user-${i}`,
        attributes: { plan: 'pro' },
      });
      results.add(r.value as boolean);
    }
    // With 50% rollout and 20 users, extremely likely we'll see both true and false
    expect(results.size).toBe(2);
  });
});
