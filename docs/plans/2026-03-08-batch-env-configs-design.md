# Batch Environment Configs via `include` Query Param

**Issue**: #92
**Date**: 2026-03-08
**Status**: Approved

## Problem

The kill-switch dashboard fetches each flag's environment configs individually (N+1 pattern via `Promise.allSettled`). For an incident response tool, this adds unnecessary latency and network load.

## Solution

Add `include=environment_configs` query param to the existing flags list endpoint:

```
GET /api/v1/projects/{key}/flags?flag_type=kill-switch&include=environment_configs
```

When present, each flag in the response includes an `environment_configs` array. When absent, the response is unchanged.

## API Response

With `include=environment_configs`:

```json
{
  "data": [
    {
      "id": "...", "key": "kill-db-writes", "flag_type": "kill-switch",
      "environment_configs": [
        {
          "id": "...", "environment_id": "...", "enabled": false,
          "updated_at": "...", "updated_by_user": { "id": "...", "email": "...", "display_name": "..." }
        }
      ]
    }
  ],
  "total": 5, "limit": 50, "offset": 0
}
```

Without the param, `environment_configs` is omitted from the JSON (nil slice + `omitempty`).

## Backend Changes

### Model (`internal/model/flag.go`)

Add field to `Flag`:
```go
EnvironmentConfigs []FlagEnvironmentConfig `json:"environment_configs,omitempty"`
```

### Store (`internal/store/flag_store.go`)

New method:
```go
func (s *FlagStore) GetEnvironmentConfigsByFlagIDs(ctx context.Context, flagIDs []string) (map[string][]FlagEnvironmentConfig, error)
```

Single query: `WHERE fec.flag_id = ANY($1)` with LEFT JOIN on users for `updated_by_user`. Returns map keyed by flag ID.

### Handler (`internal/handler/flag_handler.go`)

In `List`: parse `include` query param. If it contains `environment_configs`, collect flag IDs, call batch store method, attach configs to each flag.

## Frontend Changes

Update kill-switch dashboard (`web/src/pages/KillSwitchDashboardPage.tsx`) to use `?flag_type=kill-switch&include=environment_configs` instead of N individual flag detail requests.

## What doesn't change

- Flag detail endpoint (`GET .../flags/{flag}`) unchanged
- Pagination, filtering, all existing query params work as before
- No new routes
