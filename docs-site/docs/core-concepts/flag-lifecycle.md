---
sidebar_position: 7
title: Flag Lifecycle & Staleness
---

# Flag Lifecycle & Staleness

Feature flags are meant to be temporary (with a few exceptions). Flags that outlive their purpose become tech debt — dead code paths, confusing configurations, and potential sources of bugs. Togglerino's lifecycle system helps teams identify and clean up flags that are no longer needed.

## Lifecycle States

Every flag moves through four states:

```
active  -->  potentially_stale  -->  stale  -->  archived
```

| State | Meaning |
|-------|---------|
| `active` | The flag is in use and within its expected lifetime. All new flags start here. |
| `potentially_stale` | The flag has exceeded its expected lifetime. It may still be needed, but the team should review it. |
| `stale` | The flag has been in `potentially_stale` for 14 days without action. It's likely no longer needed and should be removed or archived. |
| `archived` | The flag is retired. Archived flags return their default value to SDKs and are hidden from the main flags list. |

## Staleness Checker

Togglerino runs a background process (the staleness checker) that evaluates all non-archived flags every hour. For each flag, it:

1. Looks up the expected lifetime for the flag's type (from project settings or global defaults).
2. If the flag type is permanent (`kill-switch` or `permission`), it is skipped entirely.
3. Compares the flag's age (time since creation) to the expected lifetime.
4. Promotes the flag to the next lifecycle state if thresholds are exceeded.

### Transition Rules

- **`active` to `potentially_stale`** — when the flag's age exceeds its expected lifetime. For example, a `release` flag older than 40 days.
- **`potentially_stale` to `stale`** — when the flag has been in `potentially_stale` for more than 14 days (the grace period). This gives teams two weeks to review the flag before it's marked stale.
- **`stale` to `archived`** — this transition is **manual only**. The staleness checker never archives a flag automatically.

## Default Lifetimes

Each flag type has a default expected lifetime:

| Flag Type | Default Lifetime | Staleness Behavior |
|-----------|-----------------|-------------------|
| `release` | 40 days | Marked potentially stale after 40 days |
| `experiment` | 40 days | Marked potentially stale after 40 days |
| `operational` | 7 days | Marked potentially stale after 7 days |
| `kill-switch` | Permanent | Never marked stale |
| `permission` | Permanent | Never marked stale |

## Per-Project Configuration

Project admins can customize the expected lifetime for each flag type in **Project > Settings > Flag Lifetimes**. For example, if your team's release cycles are longer, you might increase the `release` lifetime to 60 days.

Custom lifetimes override the defaults only for that project. Other projects continue using the global defaults.

## Manual Overrides

A flag's lifecycle status can be overridden manually through the dashboard or API. Common scenarios:

- **Reset to active** — a `potentially_stale` flag that is intentionally long-lived can be reset to `active`. This resets the staleness timer.
- **Mark as stale** — force a flag to `stale` to signal the team that it needs cleanup, even if the automatic threshold hasn't been reached.

Manual overrides are useful for flags that don't fit neatly into the default lifecycle — for example, a `release` flag for a feature with a long beta period.

## Archiving

Archiving moves a flag to the `archived` state. Archived flags:

- **Return their default value** to SDKs. Evaluation short-circuits immediately with reason `archived`.
- **Are hidden** from the main flags list in the dashboard (you can still view them with a filter).
- **Can be unarchived** if needed, restoring the flag to its previous configuration.

Archive a flag through the dashboard or the API (`PUT /api/v1/projects/{key}/flags/{flag}/archive`).

## Why It Matters

Stale flags are technical debt:

- **Code complexity** — every flag adds a conditional branch. Flags that are always on (or always off) are dead code waiting to confuse a new team member.
- **Testing burden** — each flag multiplies the number of code paths to test.
- **Configuration risk** — an old flag with outdated targeting rules could be accidentally re-enabled with unintended consequences.

The lifecycle system makes flag hygiene visible. Use the lifecycle board in the dashboard to see all flags grouped by state, identify cleanup opportunities, and track your project's overall flag health score.
