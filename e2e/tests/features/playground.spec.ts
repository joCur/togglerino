import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

test.describe('Playground', () => {
  test('evaluates a flag with context', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Playground Test',
      value_type: 'boolean',
      default_value: true,
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    await page.goto(`/projects/${testProject.key}/playground`);

    // Environment should auto-select (development is first)
    // Enter a user ID
    await page.getByPlaceholder('e.g. user-123').fill('test-user-1');

    // Click Evaluate
    await page.getByRole('button', { name: 'Evaluate' }).click();

    // Result should show our flag
    await expect(page.getByText(flagKey)).toBeVisible();
  });

  test('evaluates a specific flag', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Single Eval Test',
      value_type: 'string',
      default_value: 'hello',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, flagKey, 'development', { enabled: true });

    await page.goto(`/projects/${testProject.key}/playground`);

    // Enter specific flag key
    await page.getByPlaceholder(/leave empty/i).fill(flagKey);

    await page.getByRole('button', { name: 'Evaluate' }).click();

    // Should show the flag result with value "hello"
    await expect(page.getByText(flagKey)).toBeVisible();
    await expect(page.getByText('hello')).toBeVisible();
  });

  test('evaluates with custom context attributes', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Context Playground',
      value_type: 'string',
      default_value: 'default',
    });
    await apiContext.setFlagEnvConfig(testProject.key, flagKey, 'development', {
      enabled: true,
      default_variant: 'control',
      variants: [
        { key: 'control', value: 'default' },
        { key: 'vip', value: 'premium' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'plan', operator: 'equals', value: 'enterprise' }],
          variant: 'vip',
        },
      ],
    });

    await page.goto(`/projects/${testProject.key}/playground`);

    // Enter specific flag
    await page.getByPlaceholder(/leave empty/i).fill(flagKey);

    // Add context attribute
    await page.getByRole('button', { name: /add attribute/i }).click();
    await page.getByPlaceholder('Key').last().fill('plan');
    await page.getByPlaceholder('Value').last().fill('enterprise');

    await page.getByRole('button', { name: 'Evaluate' }).click();

    // Wait for results to render, then verify
    await expect(page.getByText('RESULTS')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByText('premium')).toBeVisible();
    await expect(page.getByText('rule match')).toBeVisible();
  });
});
