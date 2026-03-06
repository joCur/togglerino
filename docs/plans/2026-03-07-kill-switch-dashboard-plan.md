# Kill Switch Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a dedicated kill switch dashboard page for fast incident response toggling.

**Architecture:** Add `updated_by` column to `flag_environment_configs` via migration, update the Go store/handler to persist who toggled, extend the frontend `FlagEnvironmentConfig` type, and build a new React page with per-environment inline toggles and confirmation dialogs.

**Tech Stack:** Go (stdlib net/http, pgx/v5), React 19, TypeScript, TanStack Query, shadcn/ui, Tailwind CSS v4.

---

### Task 1: Migration — Add `updated_by` to `flag_environment_configs`

**Files:**
- Create: `migrations/019_flag_config_updated_by.up.sql`
- Create: `migrations/019_flag_config_updated_by.down.sql`

**Step 1: Write the up migration**

```sql
-- migrations/019_flag_config_updated_by.up.sql
ALTER TABLE flag_environment_configs
    ADD COLUMN updated_by UUID REFERENCES users(id) ON DELETE SET NULL;
```

**Step 2: Write the down migration**

```sql
-- migrations/019_flag_config_updated_by.down.sql
ALTER TABLE flag_environment_configs DROP COLUMN IF EXISTS updated_by;
```

**Step 3: Verify migration applies**

Run: `./dev.sh` (rebuilds and runs migrations)
Expected: Container starts without migration errors.

**Step 4: Commit**

```
feat: add updated_by column to flag_environment_configs (#52)
```

---

### Task 2: Backend — Update model and store to read/write `updated_by`

**Files:**
- Modify: `internal/model/flag.go:88-97` (FlagEnvironmentConfig struct)
- Modify: `internal/store/flag_store.go:315-429` (queries that read/write flag_environment_configs)
- Test: `internal/store/flag_store_test.go`

**Step 1: Write the failing test**

Add to `internal/store/flag_store_test.go`:

```go
func TestFlagStore_UpdateEnvironmentConfig_UpdatedBy(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("updatedby")
	project, err := ps.Create(ctx, projKey, "UpdatedBy Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}

	env, err := es.Create(ctx, project.ID, "production", "Production")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "ks-flag", "KS Flag", "test",
		model.ValueTypeBoolean, model.FlagTypeKillSwitch, json.RawMessage(`false`), []string{}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Initial config should have nil UpdatedBy
	cfg, err := fs.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentConfig: %v", err)
	}
	if cfg.UpdatedBy != nil {
		t.Error("expected nil UpdatedBy for new config")
	}

	// Create a user to be the updater
	email := uniqueEmail("updater")
	user, err := us.Create(ctx, email, "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	// Update with updated_by
	updated, err := fs.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, true, "", json.RawMessage(`[]`), json.RawMessage(`[]`), &user.ID)
	if err != nil {
		t.Fatalf("UpdateEnvironmentConfig: %v", err)
	}
	if updated.UpdatedBy == nil || *updated.UpdatedBy != user.ID {
		t.Errorf("UpdatedBy: got %v, want %q", updated.UpdatedBy, user.ID)
	}

	// Read back and verify
	readCfg, err := fs.GetEnvironmentConfig(ctx, flag.ID, env.ID)
	if err != nil {
		t.Fatalf("GetEnvironmentConfig after update: %v", err)
	}
	if readCfg.UpdatedBy == nil || *readCfg.UpdatedBy != user.ID {
		t.Errorf("UpdatedBy after re-read: got %v, want %q", readCfg.UpdatedBy, user.ID)
	}
	if readCfg.UpdatedByUser == nil {
		t.Fatal("expected UpdatedByUser to be populated")
	}
	if readCfg.UpdatedByUser.Email != email {
		t.Errorf("UpdatedByUser.Email: got %q, want %q", readCfg.UpdatedByUser.Email, email)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/... -run TestFlagStore_UpdateEnvironmentConfig_UpdatedBy -v`
Expected: FAIL — `UpdatedBy` field doesn't exist on `FlagEnvironmentConfig`.

**Step 3: Update the model**

In `internal/model/flag.go`, add fields to `FlagEnvironmentConfig`:

```go
type FlagEnvironmentConfig struct {
	ID             string          `json:"id"`
	FlagID         string          `json:"flag_id"`
	EnvironmentID  string          `json:"environment_id"`
	Enabled        bool            `json:"enabled"`
	DefaultVariant string          `json:"default_variant"`
	Variants       []Variant       `json:"variants"`
	TargetingRules []TargetingRule `json:"targeting_rules"`
	UpdatedAt      time.Time       `json:"updated_at"`
	UpdatedBy      *string         `json:"updated_by,omitempty"`
	UpdatedByUser  *FlagOwner      `json:"updated_by_user,omitempty"`
}
```

Note: Reuse `FlagOwner` struct (has `id`, `email`, `display_name`) — it works for any user reference.

**Step 4: Update store queries**

In `internal/store/flag_store.go`:

1. Update `scanFlagEnvConfig` to accept an optional user row (use a new helper or overload).
2. Update `GetEnvironmentConfig` query to LEFT JOIN users and scan `updated_by` + user fields.
3. Update `GetAllEnvironmentConfigs` query similarly.
4. Update `UpdateEnvironmentConfig` signature to accept `updatedBy *string` parameter, include it in the UPDATE SET clause, and return it.

**`GetEnvironmentConfig`** — change query to:
```sql
SELECT fec.id, fec.flag_id, fec.environment_id, fec.enabled, fec.default_variant,
       fec.variants, fec.targeting_rules, fec.updated_at, fec.updated_by,
       u.id, u.email, u.display_name
FROM flag_environment_configs fec
LEFT JOIN users u ON fec.updated_by = u.id
WHERE fec.flag_id = $1 AND fec.environment_id = $2
```

**`GetAllEnvironmentConfigs`** — same LEFT JOIN pattern.

**`UpdateEnvironmentConfig`** — add `updated_by` parameter:
```go
func (s *FlagStore) UpdateEnvironmentConfig(ctx context.Context, flagID, environmentID string, enabled bool, defaultVariant string, variants json.RawMessage, targetingRules json.RawMessage, updatedBy *string) (*model.FlagEnvironmentConfig, error) {
```

Update SQL to:
```sql
UPDATE flag_environment_configs
SET enabled=$3, default_variant=$4, variants=$5, targeting_rules=$6, updated_at=NOW(), updated_by=$7
WHERE flag_id=$1 AND environment_id=$2
RETURNING id, flag_id, environment_id, enabled, default_variant, variants, targeting_rules, updated_at, updated_by
```

After scanning, do a separate query or in-line set the `UpdatedByUser` from the `updatedBy` param (or re-query with JOIN). Simplest: return without user join from UPDATE, caller can populate if needed. OR: use a CTE/subquery. Simplest approach: just scan `updated_by` from RETURNING, and don't populate `UpdatedByUser` on write (it's populated on read via JOIN).

**Step 5: Fix all callers of `UpdateEnvironmentConfig`**

The signature change (adding `updatedBy *string`) will break callers. Update:

- `internal/handler/flag_handler.go:523` — pass user ID from session context:
  ```go
  var updatedBy *string
  if user := auth.UserFromContext(r.Context()); user != nil {
      updatedBy = &user.ID
  }
  cfg, err := h.flags.UpdateEnvironmentConfig(r.Context(), flag.ID, env.ID, req.Enabled, req.DefaultVariant, req.Variants, req.TargetingRules, updatedBy)
  ```

- `internal/handler/flag_handler.go:774` (bulkEnableDisable) — pass user ID:
  ```go
  cfg, err := h.flags.UpdateEnvironmentConfig(ctx, flag.ID, env.ID, enable, oldConfig.DefaultVariant,
      marshalJSON(oldConfig.Variants), marshalJSON(oldConfig.TargetingRules), userID)
  ```
  Where `userID` is derived from the `user` parameter (may be nil).

- Search for any other callers (schedule checker, etc.) and pass `nil` for system-initiated updates.

**Step 6: Run test to verify it passes**

Run: `go test ./internal/store/... -run TestFlagStore_UpdateEnvironmentConfig_UpdatedBy -v`
Expected: PASS

**Step 7: Run all tests to check nothing broke**

Run: `go test ./internal/store/... -v`
Expected: All PASS (existing tests updated for new signature).

**Step 8: Commit**

```
feat: track updated_by on flag environment config changes (#52)
```

---

### Task 3: Frontend — Update types and API client

**Files:**
- Modify: `web/src/api/types.ts:86-95` (FlagEnvironmentConfig interface)

**Step 1: Update `FlagEnvironmentConfig` type**

In `web/src/api/types.ts`, add `updated_by` and `updated_by_user` fields:

```typescript
export interface FlagEnvironmentConfig {
  id: string
  flag_id: string
  environment_id: string
  enabled: boolean
  default_variant: string
  variants: Variant[]
  targeting_rules: TargetingRule[]
  updated_at: string
  updated_by?: string
  updated_by_user?: FlagOwner
}
```

**Step 2: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds (new optional fields don't break existing code).

**Step 3: Commit**

```
feat: add updated_by fields to FlagEnvironmentConfig type (#52)
```

---

### Task 4: Frontend — Add route and nav link

**Files:**
- Modify: `web/src/App.tsx:98` (add route)
- Modify: `web/src/components/ProjectLayout.tsx:66` (add nav link)
- Create: `web/src/pages/KillSwitchDashboardPage.tsx` (placeholder)

**Step 1: Create placeholder page**

Create `web/src/pages/KillSwitchDashboardPage.tsx`:

```tsx
import { useParams } from 'react-router-dom'

export default function KillSwitchDashboardPage() {
  const { key } = useParams<{ key: string }>()
  return (
    <div>
      <h1 className="text-xl font-semibold tracking-tight mb-6">Kill Switches</h1>
      <p className="text-muted-foreground text-sm">Project: {key}</p>
    </div>
  )
}
```

**Step 2: Add route in `App.tsx`**

After the lifecycle route (line 98), add:

```tsx
import KillSwitchDashboardPage from './pages/KillSwitchDashboardPage.tsx'
// ...
<Route path="kill-switches" element={<KillSwitchDashboardPage />} />
```

**Step 3: Add nav link in `ProjectLayout.tsx`**

After the "Lifecycle" NavLink (line 66), add:

```tsx
<NavLink to={`/projects/${key}/kill-switches`} className={navLinkClass} onClick={onNavigate}>Kill Switches</NavLink>
```

**Step 4: Verify frontend builds**

Run: `cd web && npm run build`
Expected: Build succeeds.

**Step 5: Commit**

```
feat: add kill switch dashboard route and nav link (#52)
```

---

### Task 5: Frontend — Build the kill switch dashboard page

**Files:**
- Modify: `web/src/pages/KillSwitchDashboardPage.tsx` (full implementation)

**Step 1: Implement the full page**

Replace the placeholder with the full implementation. Key elements:

```tsx
import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig } from '../api/types.ts'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { useCanWrite } from '@/hooks/usePermissions'
import { cn } from '@/lib/utils'

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

export default function KillSwitchDashboardPage() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const canWrite = useCanWrite(key)

  const [confirmDialog, setConfirmDialog] = useState<{
    flagKey: string
    flagName: string
    envKey: string
    envName: string
    config: FlagEnvironmentConfig
  } | null>(null)

  // Fetch kill-switch flags
  const { data: flags, isLoading: flagsLoading } = useQuery({
    queryKey: ['projects', key, 'flags', { flag_type: 'kill-switch' }],
    queryFn: () => api.flags.list(key!, { flag_type: 'kill-switch' }),
    enabled: !!key,
  })

  // Fetch environments
  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  // Fetch env configs for each flag (same pattern as ProjectDetailPage)
  const flagDetailQueries = useQuery({
    queryKey: ['projects', key, 'kill-switch-configs'],
    queryFn: async () => {
      if (!flags || flags.length === 0) return {}
      const results: Record<string, FlagEnvironmentConfig[]> = {}
      await Promise.all(
        flags.map(async (flag) => {
          const detail = await api.get<FlagDetailResponse>(`/projects/${key}/flags/${flag.key}`)
          results[flag.key] = detail.environment_configs
        })
      )
      return results
    },
    enabled: !!key && !!flags && flags.length > 0,
  })

  // Toggle mutation
  const toggleMutation = useMutation({
    mutationFn: ({ flagKey, envKey, config }: { flagKey: string; envKey: string; config: FlagEnvironmentConfig }) =>
      api.put(`/projects/${key}/flags/${flagKey}/environments/${envKey}`, {
        enabled: !config.enabled,
        default_variant: config.default_variant,
        variants: config.variants,
        targeting_rules: config.targeting_rules,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'kill-switch-configs'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
      setConfirmDialog(null)
    },
  })

  const nonArchivedFlags = flags?.filter(f => f.lifecycle_status !== 'archived') ?? []

  // Summary counts
  const summaryText = () => {
    if (!environments || !flagDetailQueries.data) return null
    let enabledCount = 0
    let totalCount = 0
    for (const flag of nonArchivedFlags) {
      const configs = flagDetailQueries.data[flag.key] ?? []
      for (const cfg of configs) {
        totalCount++
        if (cfg.enabled) enabledCount++
      }
    }
    const disabledCount = totalCount - enabledCount
    return `${nonArchivedFlags.length} kill switches — ${enabledCount} enabled, ${disabledCount} disabled across ${environments.length} environments`
  }

  // Helper: find config for a flag+environment
  const getConfig = (flagKey: string, envId: string): FlagEnvironmentConfig | undefined => {
    return flagDetailQueries.data?.[flagKey]?.find(c => c.environment_id === envId)
  }

  // Helper: format relative time
  const timeAgo = (dateStr: string) => {
    const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000)
    if (seconds < 60) return 'just now'
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}m ago`
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return `${hours}h ago`
    const days = Math.floor(hours / 24)
    return `${days}d ago`
  }

  if (flagsLoading) {
    return (
      <div className="text-muted-foreground/60 text-sm animate-pulse">Loading kill switches...</div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Kill Switches</h1>
          {summaryText() && (
            <p className="text-muted-foreground text-sm mt-1">{summaryText()}</p>
          )}
        </div>
      </div>

      {nonArchivedFlags.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground text-sm">
          <p>No kill switches found.</p>
          <p className="mt-1">Create a flag with type "kill-switch" to see it here.</p>
        </div>
      ) : (
        <div className="border rounded-lg overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/30">
                <th className="text-left p-3 font-medium">Flag</th>
                {environments?.map(env => (
                  <th key={env.id} className="text-center p-3 font-medium min-w-[140px]">
                    {env.name}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {nonArchivedFlags.map(flag => (
                <tr key={flag.id} className="border-b last:border-b-0 hover:bg-muted/20">
                  <td className="p-3">
                    <Link
                      to={`/projects/${key}/flags/${flag.key}`}
                      className="font-medium hover:text-[#d4956a] transition-colors"
                    >
                      {flag.name}
                    </Link>
                    <div className="text-xs text-muted-foreground font-mono">{flag.key}</div>
                  </td>
                  {environments?.map(env => {
                    const config = getConfig(flag.key, env.id)
                    if (!config) return <td key={env.id} className="p-3 text-center text-muted-foreground/40">—</td>
                    return (
                      <td key={env.id} className="p-3 text-center">
                        <div className="flex flex-col items-center gap-1">
                          <div className="flex items-center gap-2">
                            <Switch
                              checked={config.enabled}
                              disabled={!canWrite || toggleMutation.isPending}
                              onCheckedChange={() => {
                                setConfirmDialog({
                                  flagKey: flag.key,
                                  flagName: flag.name,
                                  envKey: env.key,
                                  envName: env.name,
                                  config,
                                })
                              }}
                              className={cn(
                                config.enabled
                                  ? 'data-[state=checked]:bg-emerald-600'
                                  : 'data-[state=unchecked]:bg-red-900/40'
                              )}
                            />
                            <Badge variant={config.enabled ? 'default' : 'secondary'} className={cn(
                              'text-[10px] px-1.5',
                              config.enabled ? 'bg-emerald-600/20 text-emerald-400 border-emerald-600/30' : 'bg-red-900/20 text-red-400 border-red-900/30'
                            )}>
                              {config.enabled ? 'ON' : 'OFF'}
                            </Badge>
                          </div>
                          <div className="text-[10px] text-muted-foreground/60">
                            {timeAgo(config.updated_at)}
                            {config.updated_by_user && (
                              <> by {config.updated_by_user.display_name || config.updated_by_user.email}</>
                            )}
                          </div>
                        </div>
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Confirmation Dialog */}
      <Dialog open={!!confirmDialog} onOpenChange={(open) => !open && setConfirmDialog(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {confirmDialog?.config.enabled ? 'Disable' : 'Enable'} kill switch
            </DialogTitle>
            <DialogDescription>
              {confirmDialog?.config.enabled
                ? `Disable "${confirmDialog.flagName}" in ${confirmDialog.envName}? This will turn off the kill switch.`
                : `Enable "${confirmDialog?.flagName}" in ${confirmDialog?.envName}? This will activate the kill switch.`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmDialog(null)}>
              Cancel
            </Button>
            <Button
              variant={confirmDialog?.config.enabled ? 'destructive' : 'default'}
              disabled={toggleMutation.isPending}
              onClick={() => {
                if (!confirmDialog) return
                toggleMutation.mutate({
                  flagKey: confirmDialog.flagKey,
                  envKey: confirmDialog.envKey,
                  config: confirmDialog.config,
                })
              }}
            >
              {toggleMutation.isPending ? 'Updating...' : confirmDialog?.config.enabled ? 'Disable' : 'Enable'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

**Step 2: Verify frontend builds and lint passes**

Run: `cd web && npm run build && npm run lint`
Expected: Both pass.

**Step 3: Manual smoke test**

Run: `./dev.sh` and `cd web && npm run dev`
Navigate to `/projects/<key>/kill-switches`. Verify:
- Page loads, nav link is highlighted
- Kill switch flags shown in table with environment toggles
- Clicking toggle shows confirmation dialog
- Confirming toggle updates the switch state
- "Last changed" info shows after toggling

**Step 4: Commit**

```
feat: kill switch dashboard with inline toggles and confirmation (#52)
```

---

### Task 6: Backend — Run full test suite

**Files:** None (verification only)

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: All PASS.

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors.

**Step 3: Commit if any fixups needed**

---

### Task 7: Final commit and PR

**Step 1: Verify git status is clean**

Run: `git status`

**Step 2: Create PR**

```
feat: kill switch dashboard for incident response (#52)

## Summary
- Add dedicated `/projects/:key/kill-switches` dashboard page
- Inline per-environment toggles with confirmation dialogs
- Track `updated_by` on flag environment config changes
- Show "last changed" time and user on each toggle

## Test plan
- [ ] Go tests pass (`go test ./...`)
- [ ] Frontend builds (`cd web && npm run build`)
- [ ] Frontend lint passes (`cd web && npm run lint`)
- [ ] Manual: navigate to kill switches page, verify toggles work
- [ ] Manual: verify confirmation dialog appears before toggle
- [ ] Manual: verify "last changed" info updates after toggle
```
