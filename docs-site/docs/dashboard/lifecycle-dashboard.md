---
sidebar_position: 2
title: Lifecycle Dashboard
---

# Lifecycle Dashboard

The Lifecycle Dashboard provides a high-level view of flag health across your project, helping teams identify technical debt and manage flag cleanup.

## What It Is

A dashboard organizing all flags by their lifecycle status. Each flag falls into one of four statuses:

| Status | Meaning |
|--------|---------|
| **Active** | The flag is in normal use and within its expected lifetime. |
| **Potentially Stale** | The flag has exceeded its expected lifetime and may need attention. |
| **Stale** | The flag has significantly outlived its expected lifetime and should be reviewed for cleanup. |
| **Archived** | The flag has been retired. It returns its default value to all SDKs. |

The dashboard displays summary cards with counts for each status, a **health score** (percentage reflecting the proportion of non-stale flags), and a **staleness trends chart** showing how flag health has changed over time.

## Navigation

From any project, click **Lifecycle** in the sidebar to open the dashboard.

## Using the Action Queue

Below the summary cards and trends chart is the **Action Queue** — a filterable table of flags that need attention.

### Filtering

Use the dropdown filters to narrow the view:

- **Status filter**: Show flags by lifecycle status. The default view is "Needs Attention", which combines potentially stale and stale flags. You can also filter to a single status or view all flags.
- **Type filter**: Filter by flag type (release, experiment, operational, kill-switch, permission) or show all types.

### Taking Action on Individual Flags

- **Click any flag row** to open its detail page for full configuration access.
- **Mark Stale**: For potentially stale flags, click the "Mark Stale" button to explicitly transition the flag to stale status if you've confirmed it should be cleaned up.
- **Archive**: For stale flags, click the "Archive" button to retire the flag.

### Bulk Actions

Select multiple flags using the checkboxes, then click **Archive selected** to archive them all at once. This is useful for batch cleanup of flags that have been reviewed and confirmed as no longer needed.

## Configuring Staleness Lifetimes

The staleness checker runs automatically in the background (every hour), comparing each flag's age to the configured lifetime for its flag type.

To configure lifetimes:

1. Navigate to **Project Settings** (gear icon or Settings in the sidebar).
2. Open the **Flag Lifetimes** tab.
3. Set per-flag-type thresholds. The defaults are:
   - **Release**: 40 days
   - **Experiment**: 40 days
   - **Operational**: 7 days
   - **Kill-switch**: Permanent (never stale)
   - **Permission**: Permanent (never stale)

See [Flag Lifecycle](/core-concepts/flag-lifecycle) for a detailed explanation of how the staleness system works, including the transition from active to potentially stale to stale.

## Best Practices

- **Review the dashboard weekly** to catch flags drifting toward staleness before they accumulate.
- **Use flag types accurately** when creating flags — the type determines the staleness threshold, so a `release` flag marked as `operational` will have a much shorter expected lifetime.
- **Archive promptly** once a flag's rollout is complete and the code path is permanent. Archived flags still return their default value, so SDKs continue to function while you remove the flag checks from your codebase.
