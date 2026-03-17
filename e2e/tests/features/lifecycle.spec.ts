import { test, expect } from '../../helpers/fixtures.js';
import { uniqueFlagKey } from '../../helpers/test-data.js';

test.describe('Flag Lifecycle Board', () => {
  test('displays lifecycle summary cards', async ({ authenticatedPage: page, testProject, apiContext }) => {
    const flagKey = uniqueFlagKey();
    await apiContext.createFlag(testProject.key, {
      key: flagKey,
      name: 'Lifecycle Test',
      value_type: 'boolean',
    });

    await page.goto(`/projects/${testProject.key}/lifecycle`);

    // Summary cards should be visible (use exact match to avoid collisions)
    await expect(page.getByText('Active', { exact: true })).toBeVisible();
    await expect(page.getByText('Potentially Stale', { exact: true })).toBeVisible();
    await expect(page.getByText('Stale', { exact: true })).toBeVisible();
    await expect(page.getByText('Archived', { exact: true })).toBeVisible();

    // Active count should be at least 1
    await expect(page.getByText('Action Queue')).toBeVisible();
  });

  test('lifecycle summary via API', async ({ apiContext, testProject }) => {
    // Create flags in different states
    const activeKey = uniqueFlagKey();
    const archivedKey = uniqueFlagKey();

    await apiContext.createFlag(testProject.key, { key: activeKey, name: 'Active', value_type: 'boolean' });
    await apiContext.createFlag(testProject.key, { key: archivedKey, name: 'Archived', value_type: 'boolean' });
    await apiContext.archiveFlag(testProject.key, archivedKey, true);

    // Verify via lifecycle API
    const summary = await apiContext.getLifecycleSummary(testProject.key);
    expect(summary.active).toBeGreaterThanOrEqual(1);
    expect(summary.archived).toBeGreaterThanOrEqual(1);
  });
});
