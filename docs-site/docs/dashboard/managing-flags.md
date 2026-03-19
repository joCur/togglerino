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

From the flag detail page, select an environment tab to configure that environment.

### Enabled/Disabled Toggle

The toggle at the top of the environment configuration controls whether the flag is active. When disabled, the flag returns its **off variant** to all SDKs regardless of targeting rules.

### Off Variant

The off variant is the value returned when the flag's targeting is disabled. For boolean flags this is always `false`. For string, number, and JSON flags you choose which variant acts as the safe fallback when the flag is switched off.

### Fallthrough Variant

The fallthrough variant is returned when the flag is enabled but no targeting rule matches the current user. This is what most users see until you configure targeting rules. For boolean flags this defaults to `true`.

### Variants

Variants define the possible values a flag can return. Each variant has a name and a value matching the flag's value type. Boolean flags have implicit `true` and `false` variants that cannot be modified.

Use the variant tag chips on the flag detail page to add, edit, or remove variants. For example, a string flag might have variants `control`, `variant-a`, and `variant-b`.

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
2. Select the environment tab you want to lock.
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

## Comparing and Reviewing Flag History

The flag detail page header includes two actions for tracking changes:

- **Compare** — opens a dialog showing a side-by-side diff of two history entries, letting you see exactly what changed between any two points in time.
- **History** — opens a dialog listing all change events for the flag, with timestamps and the actor who made each change.

Both are accessible directly from the flag detail header without leaving the page.

## Archiving Flags

When a flag is no longer needed:

1. Open the flag detail page.
2. Click **Archive**.

Archived flags return the flag's `default_value` (set at creation time) to all SDKs. They remain in the system for audit purposes and can be found in the [Lifecycle Dashboard](./lifecycle-dashboard.md).

:::tip
Use flag types to help manage flag lifetimes. For example, `release` flags are expected to be short-lived and will be flagged as stale after their configured lifetime expires. See [Flag Lifecycle](/core-concepts/flag-lifecycle) for details on staleness thresholds and how to configure them.
:::

## Further Reading

- [Core Concepts: Targeting](/core-concepts/targeting) — full reference on condition operators and evaluation logic
- [Core Concepts: Rollouts](/core-concepts/rollouts) — how percentage rollouts and consistent hashing work
- [Core Concepts: Segments](/core-concepts/segments) — reusable groups of targeting conditions
- [Lifecycle Dashboard](./lifecycle-dashboard.md) — managing flag health and cleanup
