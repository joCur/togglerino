# Batch Environment Configs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `include=environment_configs` query param to the flags list endpoint so the kill-switch dashboard can fetch all data in a single request instead of N+1.

**Architecture:** New store method does a batch query (`WHERE flag_id = ANY($1)`) to fetch all environment configs for a set of flags. The handler parses the `include` param and attaches configs to flags before serializing. Frontend switches from N individual fetches to the single enriched list endpoint.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React (TanStack Query), TypeScript

---

### Task 1: Add `EnvironmentConfigs` field to Flag model

**Files:**
- Modify: `internal/model/flag.go:64-80`

**Step 1: Add the field**

Add to the `Flag` struct after the `Owner` field (line 79):

```go
EnvironmentConfigs []FlagEnvironmentConfig `json:"environment_configs,omitempty"`
```

**Step 2: Commit**

```bash
git add internal/model/flag.go
git commit -m "feat(model): add EnvironmentConfigs field to Flag struct"
```

---

### Task 2: Add `GetEnvironmentConfigsByFlagIDs` store method — test first

**Files:**
- Test: `internal/store/flag_store_test.go`
- Modify: `internal/store/flag_store.go`

**Step 1: Write the failing test**

Append to `internal/store/flag_store_test.go`:

```go
func TestFlagStore_GetEnvironmentConfigsByFlagIDs(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("batchcfg")
	project, err := ps.Create(ctx, projKey, "Batch Config Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env1, err := es.Create(ctx, project.ID, "development", "Development")
	if err != nil {
		t.Fatalf("creating env1: %v", err)
	}
	env2, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env2: %v", err)
	}

	flag1, err := fs.Create(ctx, project.ID, "ks-one", "KS One", "first", model.ValueTypeBoolean, model.FlagTypeKillSwitch, json.RawMessage(`false`), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating flag1: %v", err)
	}
	flag2, err := fs.Create(ctx, project.ID, "ks-two", "KS Two", "second", model.ValueTypeBoolean, model.FlagTypeKillSwitch, json.RawMessage(`false`), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating flag2: %v", err)
	}

	// Fetch configs for both flags in one call
	configsMap, err := fs.GetEnvironmentConfigsByFlagIDs(ctx, []string{flag1.ID, flag2.ID})
	if err != nil {
		t.Fatalf("GetEnvironmentConfigsByFlagIDs: %v", err)
	}

	// Each flag should have 2 environment configs
	if len(configsMap[flag1.ID]) != 2 {
		t.Errorf("flag1 configs: got %d, want 2", len(configsMap[flag1.ID]))
	}
	if len(configsMap[flag2.ID]) != 2 {
		t.Errorf("flag2 configs: got %d, want 2", len(configsMap[flag2.ID]))
	}

	// Verify environment IDs are present
	envIDs := map[string]bool{}
	for _, cfg := range configsMap[flag1.ID] {
		envIDs[cfg.EnvironmentID] = true
	}
	if !envIDs[env1.ID] || !envIDs[env2.ID] {
		t.Errorf("expected configs for both environments, got envIDs: %v", envIDs)
	}

	// Empty input should return empty map, no error
	emptyMap, err := fs.GetEnvironmentConfigsByFlagIDs(ctx, []string{})
	if err != nil {
		t.Fatalf("empty input: %v", err)
	}
	if len(emptyMap) != 0 {
		t.Errorf("expected empty map for empty input, got %d entries", len(emptyMap))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestFlagStore_GetEnvironmentConfigsByFlagIDs -v`
Expected: FAIL — `fs.GetEnvironmentConfigsByFlagIDs` does not exist.

**Step 3: Write minimal implementation**

Add to `internal/store/flag_store.go`:

```go
// GetEnvironmentConfigsByFlagIDs returns environment configs for multiple flags in a single query.
// The returned map is keyed by flag ID.
func (s *FlagStore) GetEnvironmentConfigsByFlagIDs(ctx context.Context, flagIDs []string) (map[string][]model.FlagEnvironmentConfig, error) {
	result := make(map[string][]model.FlagEnvironmentConfig)
	if len(flagIDs) == 0 {
		return result, nil
	}

	rows, err := s.pool.Query(ctx,
		`SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant,
		        fec.variants, fec.targeting_rules, fec.updated_at, fec.updated_by,
		        u.id, u.email, u.display_name
		 FROM flag_environment_configs fec
		 LEFT JOIN users u ON fec.updated_by = u.id
		 WHERE fec.flag_id = ANY($1)
		 ORDER BY fec.flag_id, fec.updated_at`,
		flagIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("querying environment configs by flag IDs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cfg model.FlagEnvironmentConfig
		var variantsJSON, rulesJSON json.RawMessage
		var updatedByUserID, updatedByEmail *string
		var updatedByDisplayName *string
		if err := rows.Scan(&cfg.ID, &cfg.FlagID, &cfg.EnvironmentID, &cfg.Enabled,
			&cfg.DefaultVariant, &variantsJSON, &rulesJSON, &cfg.UpdatedAt, &cfg.UpdatedBy,
			&updatedByUserID, &updatedByEmail, &updatedByDisplayName); err != nil {
			return nil, fmt.Errorf("scanning environment config: %w", err)
		}
		json.Unmarshal(variantsJSON, &cfg.Variants)
		json.Unmarshal(rulesJSON, &cfg.TargetingRules)
		if cfg.Variants == nil {
			cfg.Variants = []model.Variant{}
		}
		if cfg.TargetingRules == nil {
			cfg.TargetingRules = []model.TargetingRule{}
		}
		if updatedByUserID != nil {
			cfg.UpdatedByUser = &model.FlagOwner{ID: *updatedByUserID, Email: *updatedByEmail, DisplayName: updatedByDisplayName}
		}
		result[cfg.FlagID] = append(result[cfg.FlagID], cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environment configs: %w", err)
	}
	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestFlagStore_GetEnvironmentConfigsByFlagIDs -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/store/flag_store.go internal/store/flag_store_test.go
git commit -m "feat(store): add GetEnvironmentConfigsByFlagIDs batch method"
```

---

### Task 3: Wire `include=environment_configs` in the handler — test first

**Files:**
- Modify: `internal/handler/flag_handler.go:165-199`

Note: The handler tests require a real database (the handler uses concrete `*store.FlagStore`, not an interface). Test this through the store test + manual verification, or write an integration test. Since the handler is thin glue code (parse param, call store, attach to flags), the critical logic is already tested in Task 2. Below we add a focused handler-level integration test.

**Step 1: Write the failing test**

Create or append to a handler test file. Since there are no existing handler tests using httptest (handlers use concrete stores), create `internal/handler/flag_handler_integration_test.go`:

```go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/togglerino/togglerino/internal/evaluation"
	"github.com/togglerino/togglerino/internal/handler"
	"github.com/togglerino/togglerino/internal/model"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/internal/stream"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://togglerino:togglerino@localhost:5432/togglerino?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connecting to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestFlagHandler_List_IncludeEnvironmentConfigs(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	as := store.NewAuditStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	hub := stream.NewHub()
	cache := evaluation.NewCache()
	pss := store.NewProjectSettingsStore(pool)

	h := handler.NewFlagHandler(fs, ps, es, as, hub, cache, pool, ufs, nil, pss)

	// Create project with environments
	projKey := "incltest-" + t.Name()
	project, err := ps.Create(ctx, projKey, "Include Test", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	// Create a flag
	_, err = fs.Create(ctx, project.ID, "my-flag", "My Flag", "test", model.ValueTypeBoolean, model.FlagTypeKillSwitch, json.RawMessage(`false`), nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("creating flag: %v", err)
	}

	// Request WITHOUT include — should NOT have environment_configs
	req := httptest.NewRequest("GET", "/api/v1/projects/"+projKey+"/flags", nil)
	req.SetPathValue("key", projKey)
	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}

	var respWithout struct {
		Data []json.RawMessage `json:"data"`
	}
	json.NewDecoder(w.Body).Decode(&respWithout)
	if len(respWithout.Data) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(respWithout.Data))
	}
	// Check that environment_configs key is absent
	var flagMap map[string]any
	json.Unmarshal(respWithout.Data[0], &flagMap)
	if _, exists := flagMap["environment_configs"]; exists {
		t.Error("environment_configs should NOT be present without include param")
	}

	// Request WITH include=environment_configs — should have environment_configs
	req2 := httptest.NewRequest("GET", "/api/v1/projects/"+projKey+"/flags?include=environment_configs", nil)
	req2.SetPathValue("key", projKey)
	w2 := httptest.NewRecorder()
	h.List(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w2.Code)
	}

	var respWith struct {
		Data []json.RawMessage `json:"data"`
	}
	json.NewDecoder(w2.Body).Decode(&respWith)
	if len(respWith.Data) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(respWith.Data))
	}
	var flagMap2 map[string]any
	json.Unmarshal(respWith.Data[0], &flagMap2)
	configs, exists := flagMap2["environment_configs"]
	if !exists {
		t.Fatal("environment_configs should be present with include param")
	}
	configSlice, ok := configs.([]any)
	if !ok {
		t.Fatalf("environment_configs should be an array, got %T", configs)
	}
	if len(configSlice) != 1 {
		t.Errorf("expected 1 environment config, got %d", len(configSlice))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/... -run TestFlagHandler_List_IncludeEnvironmentConfigs -v`
Expected: FAIL — either compilation error (NewFlagHandler signature) or the include param is not handled yet.

Note: Check the `NewFlagHandler` constructor signature before writing the test. Adjust the arguments if needed. The test should compile but the second assertion (with `include`) should fail because configs won't be attached.

**Step 3: Implement the include logic in the handler**

In `internal/handler/flag_handler.go`, modify the `List` method. After the `ListByProject` call and before `writeJSON`, add:

```go
// Check for include=environment_configs
if include := r.URL.Query().Get("include"); include == "environment_configs" {
	flagIDs := make([]string, len(flags))
	for i, f := range flags {
		flagIDs[i] = f.ID
	}
	configsMap, err := h.flags.GetEnvironmentConfigsByFlagIDs(r.Context(), flagIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get environment configs")
		return
	}
	for i := range flags {
		configs := configsMap[flags[i].ID]
		if configs == nil {
			configs = []model.FlagEnvironmentConfig{}
		}
		flags[i].EnvironmentConfigs = configs
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/handler/... -run TestFlagHandler_List_IncludeEnvironmentConfigs -v`
Expected: PASS

**Step 5: Run all tests to check for regressions**

Run: `go test ./internal/... -v`
Expected: All pass. The `omitempty` tag ensures existing responses are unchanged.

**Step 6: Commit**

```bash
git add internal/handler/flag_handler.go internal/handler/flag_handler_integration_test.go
git commit -m "feat(handler): support include=environment_configs on flags list endpoint"
```

---

### Task 4: Update frontend — API client and kill-switch dashboard

**Files:**
- Modify: `web/src/api/client.ts:44-55`
- Modify: `web/src/api/types.ts:55-71`
- Modify: `web/src/pages/KillSwitchDashboardPage.tsx`

**Step 1: Add `include` param to the API client**

In `web/src/api/client.ts`, update the `flags.list` params type and URL builder:

```typescript
list: (projectKey: string, params?: { search?: string; tag?: string; lifecycle_status?: string; flag_type?: string; include?: string; limit?: number; offset?: number }) => {
  const search = new URLSearchParams()
  if (params?.search) search.set('search', params.search)
  if (params?.tag) search.set('tag', params.tag)
  if (params?.lifecycle_status) search.set('lifecycle_status', params.lifecycle_status)
  if (params?.flag_type) search.set('flag_type', params.flag_type)
  if (params?.include) search.set('include', params.include)
  if (params?.limit !== undefined) search.set('limit', String(params.limit))
  if (params?.offset !== undefined) search.set('offset', String(params.offset))
  const qs = search.toString()
  return request<PaginatedResponse<Flag>>(`/projects/${projectKey}/flags${qs ? `?${qs}` : ''}`)
},
```

**Step 2: Add `environment_configs` to the Flag type**

In `web/src/api/types.ts`, add to the `Flag` interface:

```typescript
environment_configs?: FlagEnvironmentConfig[]
```

**Step 3: Simplify the kill-switch dashboard**

Replace the separate configs query in `web/src/pages/KillSwitchDashboardPage.tsx`. The key change: instead of two queries (flags list + N detail fetches), use one query with `include=environment_configs`.

Replace the `useInfiniteQuery` call (lines 63-78) to add `include: 'environment_configs'`:

```typescript
queryFn: ({ pageParam = 0 }) => api.flags.list(key!, { flag_type: 'kill-switch', include: 'environment_configs', limit: PAGE_SIZE, offset: pageParam }),
```

Remove the separate `configsMap` `useQuery` block (lines 94-115) entirely.

Replace the `configsMap` derivation with a `useMemo` that builds it from the flags data:

```typescript
const configsMap = useMemo(() => {
  if (!flags) return undefined
  const map: Record<string, Record<string, FlagEnvironmentConfig>> = {}
  for (const flag of flags) {
    if (!flag.environment_configs) continue
    const flagConfigs: Record<string, FlagEnvironmentConfig> = {}
    for (const config of flag.environment_configs) {
      flagConfigs[config.environment_id] = config
    }
    map[flag.key] = flagConfigs
  }
  return map
}, [flags])
```

Remove the `activeFlagKeys` memo (line 92) — no longer needed.

Update the `useInfiniteQuery` to add `refetchInterval: 30_000` (which was previously on the configs query):

```typescript
refetchInterval: 30_000,
```

Update `toggleMutation.onSuccess` to only invalidate `['projects', key, 'flags']` (remove the `kill-switch-configs` invalidation since that query no longer exists).

**Step 4: Verify the frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds with no type errors.

**Step 5: Commit**

```bash
git add web/src/api/client.ts web/src/api/types.ts web/src/pages/KillSwitchDashboardPage.tsx
git commit -m "feat(frontend): use include=environment_configs for kill-switch dashboard"
```

---

### Task 5: Final verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: All pass.

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 3: Verify no regressions with a quick manual check**

Start `./dev.sh`, open the kill-switch dashboard in a browser, verify:
- Kill switches load with environment toggles
- Network tab shows a single flags request with `include=environment_configs` instead of N+1

**Step 4: Final commit if any fixups needed, then done**
