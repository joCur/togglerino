import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

function uniqueSegmentKey(): string {
  return `test-seg-${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

test.describe('Segment Management UI', () => {
  test('creates a segment via the UI', async ({ authenticatedPage: page, testProject }) => {
    await page.goto(`/projects/${testProject.key}/segments`);

    await page.getByRole('button', { name: 'Create Segment' }).first().click();

    const dialog = page.getByRole('dialog', { name: 'Create Segment' });
    await expect(dialog).toBeVisible();

    await dialog.getByPlaceholder('e.g. Beta Users').fill('Enterprise Users');
    await expect(dialog.getByPlaceholder('e.g. beta-users')).toHaveValue('enterprise-users');
    await dialog.getByPlaceholder('Optional description').fill('Users on enterprise plan');

    // Set condition attribute — use the combobox
    await dialog.getByText('e.g. user_id, email, plan').click();
    await page.getByPlaceholder('Search or type attribute...').fill('plan');
    await page.getByText('Use "plan"').click();

    // Operator should default to equals
    // Set value
    await dialog.getByPlaceholder('Value').fill('enterprise');

    await dialog.getByRole('button', { name: 'Create Segment' }).click();

    // Wait for segment to appear in the list (dialog closes)
    await expect(page.getByText('enterprise-users')).toBeVisible();
  });

  test('creates a segment via API and displays in UI', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const segmentKey = uniqueSegmentKey();
    await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'Beta Testers',
      description: 'Users opted into beta',
      conditions: [
        { attribute: 'beta', operator: 'equals', value: true },
      ],
    });

    await page.goto(`/projects/${testProject.key}/segments`);
    await expect(page.getByText(segmentKey)).toBeVisible();
    await expect(page.getByText('Beta Testers')).toBeVisible();
    await expect(page.getByText('1 condition')).toBeVisible();
  });

  test('edits a segment name', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const segmentKey = uniqueSegmentKey();
    await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'Old Name',
      conditions: [
        { attribute: 'country', operator: 'equals', value: 'US' },
      ],
    });

    await page.goto(`/projects/${testProject.key}/segments`);

    // Click the row containing the segment key to open edit dialog
    const row = page.locator('tr').filter({ hasText: segmentKey });
    await row.click();

    // Wait for edit dialog to appear
    await expect(page.getByText('Edit Segment')).toBeVisible();

    // The edit dialog NAME input has the current value, no placeholder.
    // Find the input inside the dialog that has autoFocus (it's the name field).
    const nameInput = page.getByRole('textbox').first();
    await nameInput.clear();
    await nameInput.fill('Updated Name');

    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Wait for the name to update in the table
    await expect(page.getByText('Updated Name')).toBeVisible();

    await expect(page.getByText('Updated Name')).toBeVisible();
  });

  test('segment created in UI works with SDK evaluation', async ({ authenticatedPage: page, testProject, apiContext }) => {
    // Create segment via API (simpler than full UI flow for setup)
    const segmentKey = uniqueSegmentKey();
    await apiContext.createSegment(testProject.key, {
      key: segmentKey,
      name: 'VIP Users',
      conditions: [{ attribute: 'tier', operator: 'equals', value: 'vip' }],
    });

    // Create flag that uses the segment
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'VIP Feature',
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

    // Evaluate
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'seg-ui-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const vip = await client.evaluateFlag(flagKey, { attributes: { tier: 'vip' } });
    expect(vip.value).toBe(true);

    const regular = await client.evaluateFlag(flagKey, { attributes: { tier: 'free' } });
    expect(regular.value).toBe(false);

    // Verify segment shows in UI
    await page.goto(`/projects/${testProject.key}/segments`);
    await expect(page.getByText(segmentKey)).toBeVisible();
  });
});
