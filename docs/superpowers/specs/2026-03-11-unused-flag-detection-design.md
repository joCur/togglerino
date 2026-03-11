# Unused Flag Detection via SDK Evaluation Tracking

**Issue**: #88
**Date**: 2026-03-11
**Status**: Approved

## Problem

The staleness checker only uses time-based heuristics (flag age vs configured lifetimes) to promote flags through lifecycle stages. This misses whether a flag is actually being evaluated by any SDK. Teams need to identify flags that can be cleaned up regardless of age.

## Design

### Part 1: Backend — Evaluation Tracking

#### Schema

Migration 026 adds `last_evaluated_at TIMESTAMPTZ NULL` to the `flags` table. NULL means never evaluated.

#### Tracker (`internal/evaluation/tracker.go`)

New `Tracker` struct:

- In-memory `sync.Mutex`-protected `map[string]time.Time` (flag ID to latest timestamp)
- `Track(flagID string)` — updates map entry to `time.Now()`, synchronous (cheap map write)
- `Start(ctx)` — launches a background goroutine that flushes the map to the database every 60 seconds
- Flush swaps the map for a fresh one, then performs a single batched UPDATE using `unnest` (same pattern as context attribute upsert)
- `Stop()` — performs a final flush, then returns

#### Flag Store

Add `UpdateLastEvaluatedAt(ctx context.Context, flagIDs []string) error` — batch UPDATE setting `last_evaluated_at = NOW()` for all provided flag IDs.

#### Flag Model

Add `LastEvaluatedAt *time.Time` to the `Flag` struct. Included in JSON serialization as `last_evaluated_at`.

#### Handler Integration

In `EvaluateHandler`, after evaluating flags, call `tracker.Track(fd.Flag.ID)` for each successfully evaluated flag (extracting the ID from the `FlagData` struct in the cache loop). No per-request goroutine — the tracker's background goroutine handles all DB writes.

#### Store Query Updates

All flag store queries that scan into the `Flag` struct must be updated to include `last_evaluated_at`:
- `ListByProject` (flag list API)
- `FindByKey` (flag detail API)
- `ListNonArchived` (staleness checker)

#### API Response

`last_evaluated_at` is included in flag list and flag detail API responses via the model struct. Note: `last_evaluated_at` is intentionally NOT cached in the evaluation cache — it's only relevant for management API reads, not the evaluation hot path.

#### Shutdown Ordering

`tracker.Stop()` must be called before `pool.Close()` in the graceful shutdown sequence to ensure the final flush can write to the database.

### Part 2: UI Signals

#### Flag List Filter

New filter dropdown "Evaluation" on the flag list page with options:
- All (default)
- Never evaluated
- Not evaluated in 7 days
- Not evaluated in 30 days
- Not evaluated in 90 days

Maps to a query parameter `unevaluated_days` (integer, or `never`) on the flag list API endpoint. Backend filters accordingly.

#### Flag Card

Row 4 of the flag card (alongside owner and purpose) shows last evaluation time:
- Never evaluated: muted amber/red text "Never evaluated"
- Evaluated: muted text with relative time, e.g. "Evaluated 3d ago"
- Hidden for archived flags

#### Flag Detail Page

Metadata chips area includes a badge for last evaluation:
- Never evaluated: amber badge "Never evaluated"
- Evaluated: muted badge "Last evaluated 2 hours ago"

### Part 3: Lifecycle Integration (Opt-in)

#### Project Settings

Add `unevaluated_stale_after_days` (`*int`, nullable) to `ProjectSettings`. NULL or 0 means disabled. When set, flags that haven't been evaluated within N days (or have never been evaluated) are promoted to `potentially_stale` by the staleness checker.

#### Schema

`unevaluated_stale_after_days` is stored as a key inside the existing `settings` JSONB column of the `project_settings` table (consistent with how `flag_lifetimes` and `environment_defaults` are stored). No schema migration needed for this field — it's handled by the Go struct serialization.

#### Staleness Checker

Adds a second criterion alongside the existing age-based check:
- Only applies when `UnevaluatedStaleAfterDays` is set for the project
- Only promotes `active` flags to `potentially_stale`
- A flag qualifies if `last_evaluated_at` is NULL or older than the threshold
- Existing age-based logic is unchanged and runs independently

#### Settings API

`PUT /api/v1/projects/{key}/settings/flags` accepts `unevaluated_stale_after_days` (integer or null).

#### Settings UI

New section on the Flag Lifetimes settings tab below existing inputs. Single row with:
- Label: "Mark unevaluated flags as potentially stale after"
- Number input (days) with enable/disable toggle (same pattern as permanent toggle)
- Description explaining the opt-in behavior

## Implementation Notes

- The evaluation path (`POST /api/v1/evaluate`) is performance-sensitive — the tracker avoids per-request DB writes by coalescing in memory and flushing periodically
- Tracking is per-flag (not per-flag-per-environment) to avoid false positives for flags not yet deployed to all environments
- The 60-second flush interval means `last_evaluated_at` can be up to 60 seconds stale, which is acceptable for a feature measuring days of inactivity
- The tracker follows existing async patterns (context attribute tracking, unknown flag tracking) but uses periodic flush instead of per-request goroutines for better efficiency
- The `EvaluateHandler` already has access to flag IDs through the cache — no additional DB lookups needed in the hot path
