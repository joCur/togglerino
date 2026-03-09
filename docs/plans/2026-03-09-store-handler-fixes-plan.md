# Store & Handler Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix silent json.Unmarshal errors (#104), extract shared row scanner (#106), and add error logging before 500 responses (#105).

**Architecture:** Three independent fixes in Go backend. #106 and #104 are applied together in `internal/store/flag_store.go`. #105 is a mechanical sweep across all handler files in `internal/handler/`.

**Tech Stack:** Go, pgx/v5, log/slog

---

### Task 1: Extract shared row scanner and fix unmarshal errors (#106 + #104)

**Files:**
- Modify: `internal/store/flag_store.go:360-387` (GetAllEnvironmentConfigs)
- Modify: `internal/store/flag_store.go:412-433` (GetEnvironmentConfigsByFlagIDs loop body)
- Modify: `internal/store/flag_store.go:494-511` (scanFlagEnvConfig)
- Modify: `internal/store/flag_store.go:513-536` (scanFlagEnvConfigWithUser)
- Test: `internal/store/flag_store_test.go`

**Step 1: Write failing test for unmarshal error propagation**

Add to `internal/store/flag_store_test.go`. We can't easily inject bad JSON into the DB for a true integration test, but we can verify that valid JSON with variants/targeting rules round-trips correctly through the new scanner (this tests the refactored code path). The existing tests already cover the happy path, so we add a test that creates a flag with known variants and targeting rules via `UpdateEnvironmentConfig`, then reads them back via both `GetAllEnvironmentConfigs` and `GetEnvironmentConfigsByFlagIDs`, verifying the parsed structs match exactly:

```go
func TestFlagStore_EnvironmentConfigScannerRoundTrip(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("scannerrt")
	project, err := ps.Create(ctx, projKey, "Scanner Roundtrip", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "scanner-flag", "Scanner Flag", "test",
		model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Set known variants and targeting rules
	variants := json.RawMessage(`[{"key":"on","value":true},{"key":"off","value":false}]`)
	rules := json.RawMessage(`[{"conditions":[{"attribute":"country","operator":"equals","value":"US"}],"variant":"on","percentage_rollout":50}]`)

	_, err = fs.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, true, "on", variants, rules, nil)
	if err != nil {
		t.Fatalf("UpdateEnvironmentConfig: %v", err)
	}

	// Read back via GetAllEnvironmentConfigs (uses new shared scanner)
	configs, err := fs.GetAllEnvironmentConfigs(ctx, flag.ID)
	if err != nil {
		t.Fatalf("GetAllEnvironmentConfigs: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	cfg := configs[0]
	if len(cfg.Variants) != 2 {
		t.Errorf("GetAll Variants: got %d, want 2", len(cfg.Variants))
	}
	if cfg.Variants[0].Key != "on" {
		t.Errorf("GetAll Variants[0].Key: got %q, want %q", cfg.Variants[0].Key, "on")
	}
	if len(cfg.TargetingRules) != 1 {
		t.Errorf("GetAll TargetingRules: got %d, want 1", len(cfg.TargetingRules))
	}
	if cfg.TargetingRules[0].Variant != "on" {
		t.Errorf("GetAll TargetingRules[0].Variant: got %q, want %q", cfg.TargetingRules[0].Variant, "on")
	}
	if cfg.TargetingRules[0].PercentageRollout != 50 {
		t.Errorf("GetAll TargetingRules[0].PercentageRollout: got %v, want 50", cfg.TargetingRules[0].PercentageRollout)
	}

	// Read back via GetEnvironmentConfigsByFlagIDs (uses same shared scanner)
	configsMap, err := fs.GetEnvironmentConfigsByFlagIDs(ctx, []string{flag.ID})
	if err != nil {
		t.Fatalf("GetEnvironmentConfigsByFlagIDs: %v", err)
	}
	cfgByID := configsMap[flag.ID]
	if len(cfgByID) != 1 {
		t.Fatalf("expected 1 config from ByFlagIDs, got %d", len(cfgByID))
	}
	if len(cfgByID[0].Variants) != 2 {
		t.Errorf("ByFlagIDs Variants: got %d, want 2", len(cfgByID[0].Variants))
	}
	if len(cfgByID[0].TargetingRules) != 1 {
		t.Errorf("ByFlagIDs TargetingRules: got %d, want 1", len(cfgByID[0].TargetingRules))
	}

	// Read back via GetEnvironmentConfig (uses scanFlagEnvConfigWithUser)
	singleCfg, err := fs.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentConfig: %v", err)
	}
	if len(singleCfg.Variants) != 2 {
		t.Errorf("Single Variants: got %d, want 2", len(singleCfg.Variants))
	}
	if len(singleCfg.TargetingRules) != 1 {
		t.Errorf("Single TargetingRules: got %d, want 1", len(singleCfg.TargetingRules))
	}
}
```

**Step 2: Run test to verify it passes (baseline)**

Run: `go test ./internal/store/... -run TestFlagStore_EnvironmentConfigScannerRoundTrip -v`
Expected: PASS (this validates the happy path before refactoring)

**Step 3: Extract shared scanner and fix unmarshal errors**

In `internal/store/flag_store.go`:

1. Create `scanEnvironmentConfigRowWithUser(rows pgx.Rows) (model.FlagEnvironmentConfig, error)`:

```go
func scanEnvironmentConfigRowWithUser(rows pgx.Rows) (model.FlagEnvironmentConfig, error) {
	var cfg model.FlagEnvironmentConfig
	var variantsJSON, rulesJSON json.RawMessage
	var updatedByUserID, updatedByEmail *string
	var updatedByDisplayName *string
	if err := rows.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
		&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
		&updatedByUserID, &updatedByEmail, &updatedByDisplayName); err != nil {
		return model.FlagEnvironmentConfig{}, fmt.Errorf("scanning environment config: %w", err)
	}
	if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
		return model.FlagEnvironmentConfig{}, fmt.Errorf("unmarshalling variants: %w", err)
	}
	if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
		return model.FlagEnvironmentConfig{}, fmt.Errorf("unmarshalling targeting rules: %w", err)
	}
	if cfg.Variants == nil {
		cfg.Variants = []model.Variant{}
	}
	if cfg.TargetingRules == nil {
		cfg.TargetingRules = []model.TargetingRule{}
	}
	if updatedByUserID != nil {
		cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
	}
	return cfg, nil
}
```

2. Replace the inline scan logic in `GetAllEnvironmentConfigs` loop body (lines 360-381) with:

```go
for rows.Next() {
    cfg, err := scanEnvironmentConfigRowWithUser(rows)
    if err != nil {
        return nil, err
    }
    configs = append(configs, cfg)
}
```

3. Replace the inline scan logic in `GetEnvironmentConfigsByFlagIDs` loop body (lines 412-433) with:

```go
for rows.Next() {
    cfg, err := scanEnvironmentConfigRowWithUser(rows)
    if err != nil {
        return nil, err
    }
    result[cfg.FlagID] = append(result[cfg.FlagID], cfg)
}
```

4. Fix unmarshal error handling in `scanFlagEnvConfig` (lines 502-503):

```go
if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
    return nil, fmt.Errorf("unmarshalling variants: %w", err)
}
if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
    return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
}
```

5. Fix unmarshal error handling in `scanFlagEnvConfigWithUser` (lines 524-525):

```go
if err := json.Unmarshal(variantsJSON, &cfg.Variants); err != nil {
    return nil, fmt.Errorf("unmarshalling variants: %w", err)
}
if err := json.Unmarshal(rulesJSON, &cfg.TargetingRules); err != nil {
    return nil, fmt.Errorf("unmarshalling targeting rules: %w", err)
}
```

**Step 4: Run all store tests to verify nothing broke**

Run: `go test ./internal/store/... -v`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/store/flag_store.go internal/store/flag_store_test.go
git commit -m "fix: handle json.Unmarshal errors and extract shared row scanner (#104, #106)"
```

---

### Task 2: Add error logging before 500 responses (#105)

**Files to modify** (all in `internal/handler/`):
- `auth_handler.go`
- `audit_handler.go`
- `context_attribute_handler.go`
- `environment_handler.go`
- `flag_handler.go`
- `history_handler.go`
- `lifecycle_handler.go`
- `oidc_handler.go`
- `org_settings_handler.go`
- `project_handler.go`
- `project_member_handler.go`
- `project_settings_handler.go`
- `schedule_handler.go`
- `sdk_key_handler.go`
- `segment_handler.go`
- `template_handler.go`
- `unknown_flag_handler.go`
- `user_handler.go`
- `user_search_handler.go`
- `role_handler.go`
- `environment_access_handler.go`

**Skip:** `override_handler.go` (already has slog.Error before all 500 responses)

**Step 1: Add slog.Error before every writeError 500 that lacks it**

For every pattern like:
```go
if err != nil {
    writeError(w, http.StatusInternalServerError, "failed to X")
    return
}
```

Add `slog.Error` with the actual error:
```go
if err != nil {
    slog.Error("failed to X", "error", err)
    writeError(w, http.StatusInternalServerError, "failed to X")
    return
}
```

For cases where `err` is not in scope (e.g., inline error checks), use the appropriate variable name.

Ensure each modified file imports `"log/slog"` if it doesn't already.

**Step 2: Verify the code compiles**

Run: `go build ./internal/handler/...`
Expected: Success (no compilation errors)

**Step 3: Run existing handler tests**

Run: `go test ./internal/handler/... -v`
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/handler/
git commit -m "chore: add error logging before 500 responses in handlers (#105)"
```
