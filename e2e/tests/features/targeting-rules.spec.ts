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
    // Use a STRING flag for AND logic test (boolean flags always return true when enabled)
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'AND Logic Test',
      value_type: 'string',
      default_value: 'default',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'control',
      variants: [
        { key: 'control', value: 'default' },
        { key: 'matched', value: 'both-matched' },
      ],
      targeting_rules: [
        {
          conditions: [
            { attribute: 'country', operator: 'equals', value: 'US' },
            { attribute: 'plan', operator: 'equals', value: 'enterprise' },
          ],
          variant: 'matched',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'and-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Both conditions match → matched variant
    const both = await client.evaluateFlag(flagKey, {
      attributes: { country: 'US', plan: 'enterprise' },
    });
    expect(both.value).toBe('both-matched');
    expect(both.reason).toBe('rule_match');

    // Only one condition → default variant
    const partial = await client.evaluateFlag(flagKey, {
      attributes: { country: 'US', plan: 'free' },
    });
    expect(partial.value).toBe('default');
    expect(partial.reason).toBe('default');
  });

  test('supports in operator for list matching', async ({ apiContext, testProject }) => {
    // Use string flag so non-matching returns a different value
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'In Operator Test',
      value_type: 'string',
      default_value: 'blocked',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'default',
      variants: [
        { key: 'default', value: 'blocked' },
        { key: 'allowed', value: 'allowed' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'country', operator: 'in', value: ['US', 'CA', 'UK'] }],
          variant: 'allowed',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'in-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Country in list → allowed
    const matched = await client.evaluateFlag(flagKey, { attributes: { country: 'CA' } });
    expect(matched.value).toBe('allowed');

    // Country not in list → blocked
    const unmatched = await client.evaluateFlag(flagKey, { attributes: { country: 'DE' } });
    expect(unmatched.value).toBe('blocked');
  });

  test('percentage rollout produces stable results', async ({ apiContext, testProject }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Rollout Test',
      value_type: 'boolean',
      default_value: false,
    });

    // 50% rollout — use "false" variant so rollout users get false, others get true (enabled default)
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: '',
      variants: [],
      targeting_rules: [
        {
          conditions: [{ attribute: 'plan', operator: 'exists', value: '' }],
          variant: 'false',
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
