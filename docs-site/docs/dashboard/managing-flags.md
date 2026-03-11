---
sidebar_position: 1
title: Managing Flags
---

# Managing Flags

This guide walks through the complete workflow for creating, configuring, and managing feature flags in the Togglerino dashboard.

## Creating a Flag

1. Navigate to your project from the **Projects** page.
2. Open the **Flags** tab (the default view).
3. Click **Create Flag**.
4. Fill in the following fields:

| Field | Description |
|-------|-------------|
| **Key** | A URL-safe slug used in your SDK code (e.g., `new-checkout-flow`). Cannot be changed after creation. |
| **Name** | A human-readable display name shown in the dashboard. |
| **Description** | Optional. Explains the flag's purpose for your team. |
| **Value type** | The data type of the flag's value: `boolean`, `string`, `number`, or `json`. |
| **Flag type** | The flag's purpose category: `release`, `experiment`, `operational`, `kill-switch`, or `permission`. This determines staleness thresholds — see [Flag Lifecycle](/core-concepts/flag-lifecycle). |
| **Default value** | The fallback value returned when no targeting rule matches. |
| **Tags** | Optional. Used for filtering and organizing flags in the list view. |

5. Click **Create** to save the flag.

## Per-Environment Configuration

Each flag has independent configuration per environment. When you first create a project, three environments are set up automatically: **Development**, **Staging**, and **Production**.

From the flag detail page, select an environment tab to configure that environment:

### Enabled/Disabled Toggle

The toggle at the top of the environment configuration controls whether the flag is active. When disabled, the flag returns the default value to all SDKs regardless of targeting rules.

### Default Variant

Choose which variant to serve when no targeting rule matches. Every flag starts with a single variant; you can add more to support multi-variate flags.

### Variants

Variants define the possible values a flag can return. Each variant has a key (name) and a value matching the flag's value type.

Click **Add Variant** to create additional options beyond the default. For example, a string flag might have variants `control`, `variant-a`, and `variant-b`.

### Targeting Rules

Targeting rules are an ordered list evaluated top-to-bottom — the first matching rule wins. Click **Add Rule** to create a new rule with:

- **Conditions**: One or more attribute checks combined with AND logic. Each condition specifies:
  - An **attribute** name (use the autocomplete dropdown, which suggests attributes previously seen in SDK evaluation contexts)
  - An **operator** (e.g., `equals`, `contains`, `in`, `segment_match` — see [Targeting](/core-concepts/targeting) for the full list)
  - A **value** to compare against
- **Variant**: Which variant to serve when all conditions match.
- **Percentage rollout** (optional): A value from 0-100% controlling what fraction of matching users receive this variant. Uses consistent hashing so a given user always gets the same result.

Drag rules to reorder their priority. Since the first matching rule wins, rule order matters.

## Locking a Flag

You can lock a flag's configuration in a specific environment to prevent accidental changes during critical periods like production freezes or incident response.

1. Open the flag detail page.
2. Expand the environment you want to lock.
3. Click the **Lock** button (visible to project admins only).
4. Optionally enter a reason (e.g., "Production freeze for launch").
5. Click **Confirm Lock**.

When a flag is locked:

- The environment's toggle, config editor, and promote actions are disabled.
- A lock banner shows who locked it, when, and the reason.
- Bulk enable/disable and archive operations skip locked flags and report errors.
- Scheduled changes for the locked environment are skipped until unlocked.

To unlock, click the **Unlock** button on the same environment.

:::note
Locking requires the `project:settings` permission (project admin role). The lock prevents changes to the flag's environment configuration but does not affect evaluation — SDKs continue to receive the current flag value.
:::

## Filtering Flags

The flag list supports two filtering mechanisms:

- **Search bar**: Type to filter flags by name or key.
- **Tag filter**: Use the tag dropdown to show only flags with a specific tag.

These filters can be combined to narrow down large flag lists.

## Archiving Flags

When a flag is no longer needed:

1. Open the flag detail page.
2. Click **Archive**.

Archived flags return their default value to all SDKs. They remain in the system for audit purposes and can be found in the [Lifecycle Dashboard](./lifecycle-dashboard.md).

:::tip
Use flag types to help manage flag lifetimes. For example, `release` flags are expected to be short-lived and will be flagged as stale after their configured lifetime expires. See [Flag Lifecycle](/core-concepts/flag-lifecycle) for details on staleness thresholds and how to configure them.
:::

## Further Reading

- [Core Concepts: Targeting](/core-concepts/targeting) — full reference on condition operators and evaluation logic
- [Core Concepts: Rollouts](/core-concepts/rollouts) — how percentage rollouts and consistent hashing work
- [Core Concepts: Segments](/core-concepts/segments) — reusable groups of targeting conditions
- [Lifecycle Dashboard](./lifecycle-dashboard.md) — managing flag health and cleanup
