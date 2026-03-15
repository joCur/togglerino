import { test, expect } from '../../helpers/fixtures.js';
import { SDKClient } from '../../helpers/api.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:9091';

/**
 * Helper: expand an environment collapsible section on the flag detail page.
 * The collapsible trigger contains the env name + Lock/Promote/ON|OFF text.
 */
async function expandEnvSection(page: any, envName: string) {
  // Find the collapsible trigger button that contains this environment name
  const trigger = page.locator('[data-slot="collapsible-trigger"]').filter({ hasText: envName }).first();
  // Check if already expanded
  const expanded = await trigger.getAttribute('aria-expanded');
  if (expanded !== 'true') {
    await trigger.click();
  }
  // Wait for content to be visible
  await page.waitForTimeout(200);
}

test.describe('Flag Management', () => {
  test('deletes an archived flag', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Delete Test ${key}` });
    await apiContext.archiveFlag(testProject.key, key, true);

    await page.goto(`/projects/${testProject.key}/flags/${key}`);
    await expect(page.getByRole('heading', { name: key })).toBeVisible();

    // Open settings dropdown — should show "Delete permanently" for archived flags
    await page.locator('button:has(svg.lucide-settings)').click();
    await page.getByRole('menuitem', { name: /delete permanently/i }).click();

    // Confirm deletion in dialog
    const dialog = page.getByRole('dialog');
    await dialog.getByRole('button', { name: /delete permanently/i }).click();

    // Should redirect back to project flags list
    await page.waitForURL(`**/projects/${testProject.key}`);
  });

  test('assigns and changes flag owner', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, { key, name: `Owner Test ${key}` });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);
    await expect(page.getByRole('heading', { name: key })).toBeVisible();

    // Click the owner select trigger
    await page.locator('[data-slot="select-trigger"]').first().click();

    // Select the first user option (not "Unassigned")
    const options = page.getByRole('option');
    // Skip "Unassigned" (first option) and pick the next one
    await options.nth(1).click();

    // Verify owner changed (no longer shows "Unassigned")
    await expect(page.locator('[data-slot="select-trigger"]').first()).not.toHaveText('Unassigned');

    // Reload and verify persisted
    await page.reload();
    await expect(page.locator('[data-slot="select-trigger"]').first()).not.toHaveText('Unassigned');
  });

  test('configures variants via the UI', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key,
      name: `Variants Test`,
      value_type: 'string',
      default_value: 'control',
    });
    await apiContext.updateFlagEnvConfig(testProject.key, key, 'development', { enabled: true });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Expand the development environment section
    await expandEnvSection(page, 'Development');

    // Open the Variants collapsible — use the button that shows "Variants (0)"
    await page.getByRole('button', { name: /^Variants/ }).click();

    // Click "+ Add Variant"
    await page.getByRole('button', { name: '+ Add Variant' }).click();

    // Fill in the variant key and value
    await page.getByPlaceholder('Key').last().fill('treatment');
    await page.getByPlaceholder('Value').last().fill('new-experience');

    // Save the configuration
    await page.getByRole('button', { name: 'Save Configuration' }).click();
    await expect(page.getByText('Saved')).toBeVisible();

    // Verify via API that variants were saved
    const envs = await apiContext.listEnvironments(testProject.key);
    const devEnv = envs.find((e: any) => e.key === 'development');
    const { environment_configs } = await apiContext.getFlag(testProject.key, key);
    const devConfig = environment_configs.find((c: any) => c.environment_id === devEnv!.id);
    expect(devConfig?.variants).toContainEqual(
      expect.objectContaining({ key: 'treatment' })
    );
  });

  test('adds targeting rule via the UI and evaluates it', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key,
      name: `Rule Builder Test`,
      value_type: 'boolean',
      default_value: false,
    });

    // Boolean flags: no variants needed — just enable
    await apiContext.updateFlagEnvConfig(testProject.key, key, 'development', { enabled: true });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Expand development section
    await expandEnvSection(page, 'Development');

    // Open Targeting Rules section
    await page.getByRole('button', { name: /^Targeting Rules/ }).click();

    // Add a rule
    await page.getByRole('button', { name: '+ Add Rule' }).click();

    // Set attribute via combobox
    await page.getByText('e.g. user_id, email, plan').click();
    await page.getByPlaceholder('Search or type attribute...').fill('plan');
    await page.getByText('Use "plan"').click();

    // Set operator to "equals"
    await page.locator('select').filter({ has: page.locator('optgroup') }).first().selectOption('equals');

    // Set value
    await page.getByPlaceholder('Value').first().fill('enterprise');

    // For boolean flags, the "Serve value:" dropdown shows true/false instead of variants
    await page.locator('text=Serve value:').locator('..').locator('select').selectOption('true');

    // Save configuration
    await page.getByRole('button', { name: 'Save Configuration' }).click();
    await expect(page.getByText('Saved')).toBeVisible();

    // Verify targeting works via SDK evaluation
    const sdkKey = await apiContext.createSDKKey(testProject.key, 'development', 'rule-ui-test');
    const client = new SDKClient(BASE_URL, sdkKey.key);

    const matched = await client.evaluateFlag(key, { attributes: { plan: 'enterprise' } });
    expect(matched.value).toBe(true);
    expect(matched.reason).toBe('rule_match');

    // Boolean flags: enabled default = true (no rule match still returns true)
    const unmatched = await client.evaluateFlag(key, { attributes: { plan: 'free' } });
    expect(unmatched.value).toBe(true);
    expect(unmatched.reason).toBe('default');
  });

  test('promotes flag config between environments', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key,
      name: `Promote Test`,
      value_type: 'string',
      default_value: 'control',
    });

    // Configure development with variants and targeting rules
    await apiContext.setFlagEnvConfig(testProject.key, key, 'development', {
      enabled: true,
      default_variant: 'control',
      variants: [
        { key: 'control', value: 'control' },
        { key: 'treatment', value: 'new-feature' },
      ],
      targeting_rules: [
        {
          conditions: [{ attribute: 'beta', operator: 'equals', value: true }],
          variant: 'treatment',
        },
      ],
    });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // The Promote button is in the Development env header row (not inside the collapsible content)
    // Click the Promote dropdown trigger in the Development row
    const devRow = page.locator('[data-slot="collapsible-trigger"]').filter({ hasText: 'Development' });
    // The Promote button is a sibling within the collapsible trigger
    await devRow.getByText('Promote').click();

    // Select "Promote to Staging" from the dropdown
    await page.getByRole('menuitem', { name: /staging/i }).click();

    // Confirm in the dialog
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Promote Configuration')).toBeVisible();
    await dialog.getByRole('button', { name: 'Confirm Promotion' }).click();

    // Wait for dialog to close
    await expect(dialog).not.toBeVisible();

    // Verify staging now has the promoted config via API
    // Get staging environment ID first
    const envs = await apiContext.listEnvironments(testProject.key);
    const stagingEnv = envs.find((e: any) => e.key === 'staging');
    expect(stagingEnv).toBeDefined();

    const { environment_configs } = await apiContext.getFlag(testProject.key, key);
    const stagingConfig = environment_configs.find((c: any) => c.environment_id === stagingEnv!.id);
    expect(stagingConfig).toBeDefined();
    expect(stagingConfig!.variants?.length).toBe(2);
    expect(stagingConfig!.targeting_rules?.length).toBe(1);
  });

  test('locks and unlocks a flag environment', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const key = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key,
      name: `Lock Test`,
      value_type: 'boolean',
      default_value: false,
    });

    await page.goto(`/projects/${testProject.key}/flags/${key}`);

    // Click the Lock button specifically in the Development row
    const devRow = page.locator('[data-slot="collapsible-trigger"]').filter({ hasText: 'Development' });
    await devRow.getByText('Lock', { exact: true }).click();

    // Fill in the lock dialog
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Lock Flag in Environment')).toBeVisible();
    await dialog.getByPlaceholder('e.g. Holiday code freeze').fill('E2E test lock');
    await dialog.getByRole('button', { name: 'Lock Flag' }).click();

    // Wait for dialog to close and verify locked state
    await expect(dialog).not.toBeVisible();
    await expect(devRow.getByText('Locked')).toBeVisible();

    // Unlock the flag
    await devRow.getByText('Unlock').click();

    // Verify unlocked — "Lock" text should reappear (not "Locked" or "Unlock")
    await expect(devRow.getByText('Lock', { exact: true })).toBeVisible();
  });

  test('flag search and filtering', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const alphaKey = uniqueFlagKey();
    const betaKey = uniqueFlagKey();

    await apiContext.createFlag(testProject.key, {
      key: alphaKey,
      name: `Alpha Feature`,
      value_type: 'boolean',
      default_value: false,
    });
    await apiContext.createFlag(testProject.key, {
      key: betaKey,
      name: `Beta Feature`,
      value_type: 'string',
      default_value: 'off',
    });

    await page.goto(`/projects/${testProject.key}`);

    // Both flags should be visible
    await expect(page.getByText(alphaKey, { exact: true })).toBeVisible();
    await expect(page.getByText(betaKey, { exact: true })).toBeVisible();

    // Search for "Alpha"
    await page.getByPlaceholder('Search flags...').fill('Alpha');
    await expect(page.getByText(alphaKey, { exact: true })).toBeVisible();
    await expect(page.getByText(betaKey, { exact: true })).not.toBeVisible();

    // Clear search
    await page.getByPlaceholder('Search flags...').clear();
    await expect(page.getByText(alphaKey, { exact: true })).toBeVisible();
    await expect(page.getByText(betaKey, { exact: true })).toBeVisible();
  });
});
