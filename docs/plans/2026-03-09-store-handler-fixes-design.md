# Store & Handler Fixes Design

Addresses issues #104, #105, #106.

## Issue #106: Extract shared row scanner for FlagEnvironmentConfig

`GetAllEnvironmentConfigs` and `GetEnvironmentConfigsByFlagIDs` have ~20 lines of identical row-scanning logic. Extract a shared helper:

```go
func scanEnvironmentConfigRowWithUser(rows pgx.Rows) (model.FlagEnvironmentConfig, error)
```

Both methods call this helper instead of inlining the scan logic. The existing `scanFlagEnvConfig(row pgx.Row)` and `scanFlagEnvConfigWithUser(row pgx.Row)` remain since they operate on `pgx.Row` (single-row queries).

## Issue #104: Handle json.Unmarshal errors in flag environment config scanning

Fix all 4 locations where `json.Unmarshal` return values are discarded:

1. `scanEnvironmentConfigRowWithUser` (new shared helper from #106)
2. `scanFlagEnvConfig`
3. `scanFlagEnvConfigWithUser`

Each `json.Unmarshal` call gets its error checked and returned with a descriptive `fmt.Errorf` wrapper. Since the JSON comes from the database, corruption here indicates a serious data integrity issue that should surface as an error rather than silently defaulting to empty slices.

## Issue #105: Add error logging before 500 responses in handlers

Add `slog.Error(msg, "error", err)` before every `writeError(w, http.StatusInternalServerError, ...)` that lacks it. `override_handler.go` already logs — all other handler files need it.

The log message matches the user-facing error string for easy correlation (e.g., `slog.Error("failed to list flags", "error", err)`).

## Implementation Order

1. #106 — extract shared scanner
2. #104 — fix unmarshal errors (in all scanner functions including the new shared one)
3. #105 — add slog.Error logging across handlers
