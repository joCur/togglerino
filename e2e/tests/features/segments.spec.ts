import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

function uniqueSegmentKey(): string {
  return `test-seg-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

test.describe('Segments', () => {
  test('creates a segment and uses it in targeting via segment_match', async ({ apiContext, testProject }) => {
    // Create a reusable segment: enterprise users
    const segmentKey = uniqueSegmentKey();
    const segment = await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'Enterprise Users',
      description: 'Users on enterprise plan',
      conditions: [
        { attribute: 'plan', operator: 'equals', value: 'enterprise' },
      ],
    });
    expect(segment.key).toBe(segmentKey);

    // Create a flag that uses the segment via segment_match
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Segment Test',
      value_type: 'string',
      default_value: 'basic',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'basic',
      variants: [
        { key: 'basic', value: 'basic' },
        { key: 'premium', value: 'premium-feature' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: '', operator: 'segment_match', value: segmentKey }],
          variant: 'premium',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'segment-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Enterprise user matches segment → premium variant
    const matched = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise' },
    });
    expect(matched.value).toBe('premium-feature');
    expect(matched.variant).toBe('premium');
    expect(matched.reason).toBe('rule_match');

    // Free user doesn't match segment → basic variant
    const unmatched = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'free' },
    });
    expect(unmatched.value).toBe('basic');
    expect(unmatched.variant).toBe('basic');
    expect(unmatched.reason).toBe('default');
  });

  test('segment with multiple conditions uses AND logic', async ({ apiContext, testProject }) => {
    const segmentKey = uniqueSegmentKey();
    await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'US Enterprise',
      conditions: [
        { attribute: 'plan', operator: 'equals', value: 'enterprise' },
        { attribute: 'country', operator: 'equals', value: 'US' },
      ],
    });

    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Multi-Condition Segment',
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
          conditions: [{ attribute: '', operator: 'segment_match', value: segmentKey }],
          variant: 'on',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'multi-seg');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Matches both conditions → true
    const both = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise', country: 'US' },
    });
    expect(both.value).toBe(true);

    // Only matches one condition → false
    const partial = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise', country: 'DE' },
    });
    expect(partial.value).toBe(false);
  });

  test('segment can be shared across multiple flags', async ({ apiContext, testProject }) => {
    const segmentKey = uniqueSegmentKey();
    await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'Beta Users',
      conditions: [
        { attribute: 'beta', operator: 'equals', value: true },
      ],
    });

    // Two flags using the same segment
    const flag1Key = uniqueFlagKey();
    const flag2Key = uniqueFlagKey();

    for (const flagKey of [flag1Key, flag2Key]) {
      await apiContext.createFlag(testProject.key, {
        key: flagKey,
        name: `Shared Segment Flag`,
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
            conditions: [{ attribute: '', operator: 'segment_match', value: segmentKey }],
            variant: 'on',
          },
        ],
      });
    }

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'shared-seg');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Both flags should match for beta users
    const flags = await client.evaluateAll({ attributes: { beta: true } });
    expect(flags[flag1Key].value).toBe(true);
    expect(flags[flag2Key].value).toBe(true);

    // Neither should match for non-beta users
    const nonBeta = await client.evaluateAll({ attributes: { beta: false } });
    expect(nonBeta[flag1Key].value).toBe(false);
    expect(nonBeta[flag2Key].value).toBe(false);
  });
});
