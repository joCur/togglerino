---
sidebar_position: 3
title: Kill Switch Dashboard
---

# Kill Switch Dashboard

The Kill Switch Dashboard is a purpose-built view for rapid toggling of `kill-switch` type flags during incidents and operational emergencies.

## What It Is

A dedicated page showing all flags with the `kill-switch` flag type in a single table. Each row is a kill switch flag, and each column is an environment, with toggle switches for instant enable/disable control.

## Purpose

During an outage or degraded service, you need to act fast. The Kill Switch Dashboard puts every kill switch in one place so you can:

- Quickly disable features to reduce load or isolate a failing dependency
- See the current state of all kill switches across all environments at a glance
- Toggle flags without navigating through the full flag list or opening individual flag detail pages

## Navigation

From any project, click **Kill Switches** in the sidebar.

## How to Use

### Viewing State

The dashboard displays:

- A **summary bar** at the top showing the total number of kill switches and how many are currently enabled vs. disabled across all environments.
- A **table** with one row per kill switch flag and one column per environment (e.g., Development, Staging, Production). Each cell shows:
  - A toggle switch (ON/OFF)
  - A status badge (green for enabled, red for disabled)
  - When the flag was last updated and by whom

### Toggling a Kill Switch

1. Find the flag and environment you want to change.
2. Click the toggle switch.
3. A **confirmation dialog** appears asking you to confirm the change (e.g., "Disable 'disable-payment-processing' in Production?").
4. Click **Enable** or **Disable** to confirm.

Changes take effect immediately. Connected SDK clients receive the update via SSE within seconds.

### Viewing Flag Details

Click any flag name in the table to navigate to its full detail page, where you can configure targeting rules, variants, and other settings.

## Creating Kill Switch Flags

Kill switch flags are created through the normal flag creation flow:

1. Go to **Flags** and click **Create Flag**.
2. Set the **flag type** to `kill-switch`.
3. Use a `boolean` value type (recommended for simple on/off behavior).
4. The flag will automatically appear in the Kill Switch Dashboard.

## Best Practices

- **Create kill switches proactively** for critical external dependencies and high-risk features before you need them.
- **Use clear, descriptive names** that make sense during a high-stress incident. Prefix with `disable-` for clarity (e.g., `disable-payment-processing`, `disable-email-notifications`, `disable-search-indexing`).
- **Keep kill switches simple** — use boolean values with no targeting rules. The goal is a fast, global on/off switch.
- **Kill-switch flags are permanent by default** — they never transition to stale status in the [Lifecycle Dashboard](./lifecycle-dashboard.md), since they are expected to remain available indefinitely.
