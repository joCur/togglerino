---
sidebar_position: 4
title: Audit Log
---

# Audit Log

The Audit Log records every significant change within a project, providing a complete history for compliance, debugging, and team visibility.

## What's Tracked

The following actions are recorded in the audit log:

- **Flag events**: created, updated, deleted
- **Flag environment config changes**: targeting rules modified, variants changed, flag enabled/disabled per environment
- **Project events**: created, updated, deleted
- **Lifecycle transitions**: flag marked as potentially stale, stale, or archived
- **Bulk operations**: batch enable/disable, batch archive, batch tag changes, batch owner changes (grouped by batch ID)
- **Config promotions**: when configuration is promoted from one environment to another

## What Each Entry Contains

Every audit log entry includes:

| Field | Description |
|-------|-------------|
| **Time** | When the change occurred (displayed as relative time, with full timestamp on hover). |
| **User** | The email of the user who made the change. |
| **Action** | The type of change (e.g., "Created", "Updated", "Archive", "Promoted"). |
| **Entity type** | What was changed (e.g., `flag`, `flag_config`, `project`). |
| **Entity** | The ID of the affected entity. |
| **Details** | A summary or JSON snapshot of the new state. For promotions, shows the source environment. |

Entries store full JSON snapshots of the entity state before and after the change. This lets you reconstruct exactly what changed by comparing the old and new states.

## Navigation

From any project, click **Audit Log** in the sidebar.

## Pagination

Entries are displayed 50 per page, ordered newest first. Click **Load More** at the bottom to fetch additional entries.

## Important Notes

- **Best-effort recording**: Audit logging is designed to never block normal operations. If recording an audit entry fails (e.g., due to a transient database issue), the original action still succeeds. The failure is logged server-side but does not affect the user's request.
- **Project-scoped**: Each project has its own audit log. Navigate to the specific project to see its history.
- **Read-only**: Audit log entries cannot be modified or deleted through the dashboard.
