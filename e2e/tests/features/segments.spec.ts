import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

function uniqueSegmentKey(): string {
  return `test-seg-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

test.describe('Segments', () => {
  test('creates a segment and uses it in targeting via segment_match', async ({ apiContext, testProject }) => {
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

    // Use a string flag so we can distinguish matched vs default
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Segment Test',
      value_type: 'string',
      default_value: 'basic',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      fallthrough_variant: 'basic',
      off_variant: 'basic',
      variants: [
        { name: 'basic', value: 'basic' },
        { name: 'premium', value: 'premium-feature' },
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

    const matched = await client.evaluateFlag(flagKey, { attributes: { plan: 'enterprise' } });
    expect(matched.value).toBe('premium-feature');
    expect(matched.reason).toBe('rule_match');

    const unmatched = await client.evaluateFlag(flagKey, { attributes: { plan: 'free' } });
    expect(unmatched.value).toBe('basic');
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

    // Use string flag to distinguish values
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Multi-Condition Segment',
      value_type: 'string',
      default_value: 'no',
    });

    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      fallthrough_variant: 'default',
      off_variant: 'default',
      variants: [
        { name: 'default', value: 'no' },
        { name: 'matched', value: 'yes' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: '', operator: 'segment_match', value: segmentKey }],
          variant: 'matched',
        },
      ],
    });

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'multi-seg');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const both = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise', country: 'US' },
    });
    expect(both.value).toBe('yes');

    const partial = await client.evaluateFlag(flagKey, {
      attributes: { plan: 'enterprise', country: 'DE' },
    });
    expect(partial.value).toBe('no');
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

    // Use boolean flags with canonical variants
    const flag1Key = uniqueFlagKey();
    const flag2Key = uniqueFlagKey();

    for (const flagKey of [flag1Key, flag2Key]) {
      await apiContext.createFlag(testProject.key, {
        key: flagKey,
        name: `Shared Segment Flag`,
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
            conditions: [{ attribute: '', operator: 'segment_match', value: segmentKey }],
            variant: 'true',
          },
        ],
      });
    }

    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'shared-seg');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    // Both flags should return true for beta users (rule match)
    const flags = await client.evaluateAll({ attributes: { beta: true } });
    expect(flags[flag1Key].value).toBe(true);
    expect(flags[flag1Key].reason).toBe('rule_match');
    expect(flags[flag2Key].value).toBe(true);
    expect(flags[flag2Key].reason).toBe('rule_match');

    // Non-beta users: fallthrough = true
    const nonBeta = await client.evaluateAll({ attributes: { beta: false } });
    expect(nonBeta[flag1Key].value).toBe(true);
    expect(nonBeta[flag1Key].reason).toBe('default');
  });
});
