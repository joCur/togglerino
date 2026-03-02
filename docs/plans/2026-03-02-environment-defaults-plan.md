# Environment Defaults Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** When creating a new flag, automatically configure it as enabled/disabled per environment based on project-level defaults, with override capability at creation time.

**Architecture:** Extend the existing `project_settings` JSONB column with an `environment_defaults` key. Add GET/PUT endpoints for managing these defaults. Modify the flag creation handler and store to accept per-environment enabled overrides. Update the frontend project settings page and create flag modal.

**Tech Stack:** Go stdlib, pgx/v5, React 19, TanStack Query, shadcn/ui, Tailwind CSS v4

---

### Task 1: Add EnvironmentDefault model and helpers

**Files:**
- Modify: `internal/model/project_settings.go`

**Step 1: Add the EnvironmentDefault type and default factory**

Add to `internal/model/project_settings.go`:

```go
// EnvironmentDefault holds the default flag configuration for an environment.
type EnvironmentDefault struct {
	Enabled bool `json:"enabled"`
}

// DefaultEnvironmentDefaults returns the hardcoded fallback defaults.
// "development" is enabled; all others are disabled.
func DefaultEnvironmentDefaults() map[string]EnvironmentDefault {
	return map[string]EnvironmentDefault{
		"development": {Enabled: true},
		"staging":     {Enabled: false},
		"production":  {Enabled: false},
	}
}
```

**Step 2: Add EnvironmentDefaults field to ProjectSettings**

Add field to the `ProjectSettings` struct:

```go
EnvironmentDefaults map[string]EnvironmentDefault `json:"environment_defaults,omitempty"`
```

**Step 3: Add a helper to resolve defaults for a set of environments**

```go
// ResolveEnvironmentDefaults merges hardcoded fallbacks with project-level
// settings and optional per-request overrides. Returns a map of env key → enabled.
func (ps *ProjectSettings) ResolveEnvironmentDefaults(envKeys []string, overrides map[string]EnvironmentDefault) map[string]bool {
	result := make(map[string]bool, len(envKeys))
	hardcoded := DefaultEnvironmentDefaults()

	for _, key := range envKeys {
		// Layer 1: hardcoded fallback (development=true, everything else=false)
		if hc, ok := hardcoded[key]; ok {
			result[key] = hc.Enabled
		} else {
			result[key] = false
		}

		// Layer 2: project-level setting
		if ps != nil && ps.EnvironmentDefaults != nil {
			if pd, ok := ps.EnvironmentDefaults[key]; ok {
				result[key] = pd.Enabled
			}
		}

		// Layer 3: per-request override
		if overrides != nil {
			if ov, ok := overrides[key]; ok {
				result[key] = ov.Enabled
			}
		}
	}
	return result
}
```

**Step 4: Commit**

```bash
git add internal/model/project_settings.go
git commit -m "feat: add environment default model and resolution helpers"
```

---

### Task 2: Update ProjectSettingsStore to read/write environment defaults

**Files:**
- Modify: `internal/store/project_settings_store.go`

**Step 1: Update the raw settings struct used for JSON marshaling**

In both `Get()`, `Upsert()`, and `GetAll()`, the local `raw` struct currently only has `FlagLifetimes`. Update all three to include:

```go
var raw struct {
	FlagLifetimes       map[model.FlagType]*int               `json:"flag_lifetimes"`
	EnvironmentDefaults map[string]model.EnvironmentDefault    `json:"environment_defaults,omitempty"`
}
```

And after unmarshaling, set `ps.EnvironmentDefaults = raw.EnvironmentDefaults`.

**Step 2: Add UpsertEnvironmentDefaults method**

This method reads the existing settings, merges only the `environment_defaults` key, and writes back — preserving `flag_lifetimes`.

```go
// UpsertEnvironmentDefaults creates or updates just the environment_defaults
// portion of project settings, preserving other settings (e.g., flag_lifetimes).
func (s *ProjectSettingsStore) UpsertEnvironmentDefaults(ctx context.Context, projectID string, envDefaults map[string]model.EnvironmentDefault) (*model.ProjectSettings, error) {
	// Read existing settings JSON (or empty object)
	var existingJSON []byte
	err := s.pool.QueryRow(ctx,
		`SELECT settings FROM project_settings WHERE project_id = $1`,
		projectID,
	).Scan(&existingJSON)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("reading existing settings: %w", err)
	}

	var full map[string]json.RawMessage
	if len(existingJSON) > 0 {
		if err := json.Unmarshal(existingJSON, &full); err != nil {
			return nil, fmt.Errorf("unmarshaling existing settings: %w", err)
		}
	}
	if full == nil {
		full = make(map[string]json.RawMessage)
	}

	envJSON, err := json.Marshal(envDefaults)
	if err != nil {
		return nil, fmt.Errorf("marshaling environment defaults: %w", err)
	}
	full["environment_defaults"] = envJSON

	mergedJSON, err := json.Marshal(full)
	if err != nil {
		return nil, fmt.Errorf("marshaling merged settings: %w", err)
	}

	var ps model.ProjectSettings
	var returnedJSON []byte
	err = s.pool.QueryRow(ctx,
		`INSERT INTO project_settings (project_id, settings)
		 VALUES ($1, $2)
		 ON CONFLICT (project_id) DO UPDATE SET settings = $2, updated_at = NOW()
		 RETURNING id, project_id, settings, updated_at`,
		projectID, mergedJSON,
	).Scan(&ps.ID, &ps.ProjectID, &returnedJSON, &ps.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("upserting environment defaults: %w", err)
	}

	var raw struct {
		FlagLifetimes       map[model.FlagType]*int            `json:"flag_lifetimes"`
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults,omitempty"`
	}
	if err := json.Unmarshal(returnedJSON, &raw); err != nil {
		return nil, fmt.Errorf("unmarshaling upserted settings: %w", err)
	}
	ps.FlagLifetimes = raw.FlagLifetimes
	ps.EnvironmentDefaults = raw.EnvironmentDefaults
	return &ps, nil
}
```

**Step 3: Commit**

```bash
git add internal/store/project_settings_store.go
git commit -m "feat: update settings store to read/write environment defaults"
```

---

### Task 3: Add environment defaults API endpoints

**Files:**
- Modify: `internal/handler/project_settings_handler.go`
- Modify: `cmd/togglerino/main.go`

**Step 1: Add EnvironmentStore dependency to ProjectSettingsHandler**

Update the handler struct and constructor to also accept `*store.EnvironmentStore`:

```go
type ProjectSettingsHandler struct {
	settings     *store.ProjectSettingsStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
}

func NewProjectSettingsHandler(settings *store.ProjectSettingsStore, projects *store.ProjectStore, environments *store.EnvironmentStore) *ProjectSettingsHandler {
	return &ProjectSettingsHandler{settings: settings, projects: projects, environments: environments}
}
```

**Step 2: Add GetEnvironmentDefaults handler**

```go
// GetEnvironmentDefaults handles GET /api/v1/projects/{key}/settings/environments
func (h *ProjectSettingsHandler) GetEnvironmentDefaults(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}

	settings, err := h.settings.Get(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get project settings")
		return
	}

	envKeys := make([]string, len(envs))
	for i, e := range envs {
		envKeys[i] = e.Key
	}

	resolved := settings.ResolveEnvironmentDefaults(envKeys, nil)

	type envDefault struct {
		Key     string `json:"key"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	result := make([]envDefault, len(envs))
	for i, e := range envs {
		result[i] = envDefault{Key: e.Key, Name: e.Name, Enabled: resolved[e.Key]}
	}

	writeJSON(w, http.StatusOK, map[string]any{"environment_defaults": result})
}
```

**Step 3: Add UpdateEnvironmentDefaults handler**

```go
// UpdateEnvironmentDefaults handles PUT /api/v1/projects/{key}/settings/environments
func (h *ProjectSettingsHandler) UpdateEnvironmentDefaults(w http.ResponseWriter, r *http.Request) {
	projectKey := r.PathValue("key")
	if projectKey == "" {
		writeError(w, http.StatusBadRequest, "project key is required")
		return
	}

	project, err := h.projects.FindByKey(r.Context(), projectKey)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var req struct {
		EnvironmentDefaults map[string]model.EnvironmentDefault `json:"environment_defaults"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EnvironmentDefaults == nil {
		writeError(w, http.StatusBadRequest, "environment_defaults is required")
		return
	}

	_, err = h.settings.UpsertEnvironmentDefaults(r.Context(), project.ID, req.EnvironmentDefaults)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update environment defaults")
		return
	}

	// Return the resolved view (same as GET)
	h.GetEnvironmentDefaults(w, r)
}
```

**Step 4: Update main.go — constructor call and routes**

In `cmd/togglerino/main.go`, update the `NewProjectSettingsHandler` call:

```go
projectSettingsHandler := handler.NewProjectSettingsHandler(projectSettingsStore, projectStore, environmentStore)
```

Add routes after the existing project settings routes (line ~184):

```go
mux.Handle("GET /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.GetEnvironmentDefaults, sessionAuth))
mux.Handle("PUT /api/v1/projects/{key}/settings/environments", wrap(projectSettingsHandler.UpdateEnvironmentDefaults, sessionAuth))
```

**Step 5: Commit**

```bash
git add internal/handler/project_settings_handler.go cmd/togglerino/main.go
git commit -m "feat: add GET/PUT endpoints for environment defaults"
```

---

### Task 4: Modify flag creation to use environment defaults

**Files:**
- Modify: `internal/store/flag_store.go:24-81` (Create method)
- Modify: `internal/handler/flag_handler.go:52-134` (Create handler)

**Step 1: Update FlagStore.Create to accept per-environment enabled map**

Change the `Create` method signature to accept an optional `envEnabled` parameter:

```go
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool) (*model.Flag, error) {
```

In the environment config creation loop (lines 42-70), change the query to also select the environment key, and use the `envEnabled` map:

```go
	// Get all environments for this project
	rows, err := tx.Query(ctx, `SELECT id, key FROM environments WHERE project_id = $1`, projectID)
	if err != nil {
		return nil, fmt.Errorf("querying environments: %w", err)
	}
	defer rows.Close()

	type envInfo struct {
		ID  string
		Key string
	}
	var envs []envInfo
	for rows.Next() {
		var e envInfo
		if err := rows.Scan(&e.ID, &e.Key); err != nil {
			return nil, fmt.Errorf("scanning environment: %w", err)
		}
		envs = append(envs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating environments: %w", err)
	}

	// Create a FlagEnvironmentConfig for each environment
	for _, env := range envs {
		enabled := false
		if envEnabled != nil {
			if v, ok := envEnabled[env.Key]; ok {
				enabled = v
			}
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO flag_environment_configs (flag_id, environment_id, enabled) VALUES ($1, $2, $3)`,
			f.ID, env.ID, enabled,
		)
		if err != nil {
			return nil, fmt.Errorf("creating flag environment config for env %s: %w", env.ID, err)
		}
	}
```

**Step 2: Update FlagHandler.Create to load defaults and pass them to the store**

Add `settings *store.ProjectSettingsStore` to the `FlagHandler` struct and constructor.

In `internal/handler/flag_handler.go`, update:

```go
type FlagHandler struct {
	flags        *store.FlagStore
	projects     *store.ProjectStore
	environments *store.EnvironmentStore
	audit        *store.AuditStore
	hub          *stream.Hub
	cache        *evaluation.Cache
	pool         *pgxpool.Pool
	unknownFlags *store.UnknownFlagStore
	schedules    *store.ScheduleStore
	settings     *store.ProjectSettingsStore
}

func NewFlagHandler(flags *store.FlagStore, projects *store.ProjectStore, environments *store.EnvironmentStore, audit *store.AuditStore, hub *stream.Hub, cache *evaluation.Cache, pool *pgxpool.Pool, unknownFlags *store.UnknownFlagStore, schedules *store.ScheduleStore, settings *store.ProjectSettingsStore) *FlagHandler {
	return &FlagHandler{flags: flags, projects: projects, environments: environments, audit: audit, hub: hub, cache: cache, pool: pool, unknownFlags: unknownFlags, schedules: schedules, settings: settings}
}
```

In the `Create` handler, add `EnvironmentOverrides` to the request struct:

```go
var req struct {
	Key                  string                                 `json:"key"`
	Name                 string                                 `json:"name"`
	Description          string                                 `json:"description"`
	ValueType            model.ValueType                        `json:"value_type"`
	FlagType             model.FlagType                         `json:"flag_type"`
	DefaultValue         json.RawMessage                        `json:"default_value"`
	Tags                 []string                               `json:"tags"`
	EnvironmentOverrides map[string]model.EnvironmentDefault    `json:"environment_overrides"`
}
```

After validation and before calling `h.flags.Create`, resolve the environment defaults:

```go
	// Resolve environment defaults
	envs, err := h.environments.ListByProject(r.Context(), project.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}
	envKeys := make([]string, len(envs))
	for i, e := range envs {
		envKeys[i] = e.Key
	}

	projectSettings, err := h.settings.Get(r.Context(), project.ID)
	if err != nil {
		slog.Warn("failed to load project settings for env defaults, using fallbacks", "error", err)
	}
	envEnabled := projectSettings.ResolveEnvironmentDefaults(envKeys, req.EnvironmentOverrides)

	flag, err := h.flags.Create(r.Context(), project.ID, req.Key, req.Name, req.Description, req.ValueType, req.FlagType, req.DefaultValue, req.Tags, envEnabled)
```

**Step 3: Update main.go constructor call**

```go
flagHandler := handler.NewFlagHandler(flagStore, projectStore, environmentStore, auditStore, hub, cache, pool, unknownFlagStore, scheduleStore, projectSettingsStore)
```

**Step 4: Verify it compiles**

Run: `go build ./cmd/togglerino/... 2>&1 || true` (will fail on embed, that's OK — check for type errors only)

Run: `go vet ./internal/...`

**Step 5: Commit**

```bash
git add internal/store/flag_store.go internal/handler/flag_handler.go cmd/togglerino/main.go
git commit -m "feat: apply environment defaults when creating flags"
```

---

### Task 5: Add Environment Defaults section to Project Settings frontend

**Files:**
- Modify: `web/src/pages/ProjectSettingsPage.tsx`

**Step 1: Add EnvironmentDefaultsSettings component**

Add a new component before the `ProjectSettingsPage` export. Follow the same pattern as `FlagLifetimesSettings`:

```tsx
function EnvironmentDefaultsSettings({ projectKey }: { projectKey: string }) {
  const queryClient = useQueryClient()
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [defaults, setDefaults] = useState<{ key: string; name: string; enabled: boolean }[]>([])
  const [initialized, setInitialized] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', projectKey, 'settings', 'environments'],
    queryFn: () => api.get<{ environment_defaults: { key: string; name: string; enabled: boolean }[] }>(
      `/projects/${projectKey}/settings/environments`
    ),
  })

  if (data && !initialized) {
    setDefaults(data.environment_defaults)
    setInitialized(true)
  }

  const updateMutation = useMutation({
    mutationFn: (envDefaults: Record<string, { enabled: boolean }>) =>
      api.put(`/projects/${projectKey}/settings/environments`, { environment_defaults: envDefaults }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'settings', 'environments'] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleToggle = (envKey: string) => {
    setDefaults(prev => prev.map(d => d.key === envKey ? { ...d, enabled: !d.enabled } : d))
  }

  const handleSave = () => {
    const payload: Record<string, { enabled: boolean }> = {}
    for (const d of defaults) {
      payload[d.key] = { enabled: d.enabled }
    }
    updateMutation.mutate(payload)
  }

  if (isLoading) return null

  return (
    <Card className="mb-6">
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-1">
          Environment Defaults
        </div>
        <div className="text-xs text-muted-foreground mb-4">
          Default enabled state for new flags per environment.
        </div>

        <div className="flex flex-col gap-3">
          {defaults.map((env) => (
            <div key={env.key} className="flex items-center justify-between gap-4">
              <div>
                <div className="text-[13px] font-medium text-foreground">{env.name}</div>
                <div className="text-[11px] text-muted-foreground font-mono">{env.key}</div>
              </div>
              <div className="flex items-center gap-2">
                <Switch checked={env.enabled} onCheckedChange={() => handleToggle(env.key)} />
                <span className="text-xs text-muted-foreground w-[52px]">
                  {env.enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
            </div>
          ))}
        </div>

        <div className="flex items-center gap-3 mt-4">
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save Defaults'}
          </Button>
          {saveSuccess && (
            <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">Saved</span>
          )}
          {updateMutation.error && (
            <span className="text-[13px] text-destructive">
              {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to save'}
            </span>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
```

**Step 2: Add component to the page**

In the `ProjectSettingsPage` return, add after `<FlagLifetimesSettings>`:

```tsx
{/* Environment Defaults Section */}
<EnvironmentDefaultsSettings projectKey={key!} />
```

**Step 3: Add Switch to imports**

Add `Switch` to the imports from `@/components/ui/switch`.

**Step 4: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/pages/ProjectSettingsPage.tsx
git commit -m "feat: add environment defaults section to project settings page"
```

---

### Task 6: Add environment overrides to Create Flag Modal

**Files:**
- Modify: `web/src/components/CreateFlagModal.tsx`

**Step 1: Add environment defaults query and state**

Add a query to fetch environment defaults when the modal opens. Add state for overrides:

```tsx
const { data: envDefaultsData } = useQuery({
  queryKey: ['projects', projectKey, 'settings', 'environments'],
  queryFn: () => api.get<{ environment_defaults: { key: string; name: string; enabled: boolean }[] }>(
    `/projects/${projectKey}/settings/environments`
  ),
  enabled: open,
})

const [envOverrides, setEnvOverrides] = useState<Record<string, boolean>>({})
```

**Step 2: Add Collapsible import**

```tsx
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
```

Also add `useQuery` to the TanStack imports:

```tsx
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
```

**Step 3: Add collapsible environment section to the form**

Insert after the Tags field and before the button row. Compute the effective state by merging defaults with overrides:

```tsx
{envDefaultsData && (
  <Collapsible>
    <CollapsibleTrigger asChild>
      <button type="button" className="flex items-center justify-between w-full text-left py-2 text-[13px] font-medium text-foreground hover:text-foreground/80 transition-colors">
        <span>Environment Configuration</span>
        <span className="text-[11px] text-muted-foreground font-normal">
          {envDefaultsData.environment_defaults.map(e => {
            const enabled = envOverrides[e.key] ?? e.enabled
            return `${e.key}: ${enabled ? 'on' : 'off'}`
          }).join(', ')}
        </span>
      </button>
    </CollapsibleTrigger>
    <CollapsibleContent>
      <div className="flex flex-col gap-2.5 pt-1 pb-2">
        {envDefaultsData.environment_defaults.map((env) => {
          const enabled = envOverrides[env.key] ?? env.enabled
          return (
            <div key={env.key} className="flex items-center justify-between">
              <div>
                <span className="text-[13px] text-foreground">{env.name}</span>
                <span className="text-[11px] text-muted-foreground ml-2 font-mono">{env.key}</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={enabled}
                  onCheckedChange={(checked) =>
                    setEnvOverrides(prev => ({ ...prev, [env.key]: checked }))
                  }
                />
                <span className="text-xs text-muted-foreground w-[52px]">
                  {enabled ? 'Enabled' : 'Disabled'}
                </span>
              </div>
            </div>
          )
        })}
      </div>
    </CollapsibleContent>
  </Collapsible>
)}
```

**Step 4: Include overrides in the mutation payload**

Update `handleSubmit` to include environment overrides:

```tsx
const handleSubmit = (e: React.FormEvent) => {
  e.preventDefault()
  const parsedTags = tags.split(',').map((tag) => tag.trim()).filter(Boolean)

  // Build environment_overrides from any user changes
  const environmentOverrides: Record<string, { enabled: boolean }> | undefined =
    Object.keys(envOverrides).length > 0
      ? Object.fromEntries(
          Object.entries(envOverrides).map(([key, enabled]) => [key, { enabled }])
        )
      : undefined

  mutation.mutate({
    key, name, description,
    value_type: flagType,
    flag_type: flagPurpose,
    default_value: getDefaultValueParsed(),
    tags: parsedTags,
    environment_overrides: environmentOverrides,
  })
}
```

Update the mutation type to include environment_overrides:

```tsx
const mutation = useMutation({
  mutationFn: (data: {
    key: string; name: string; description: string
    value_type: string; flag_type: string; default_value: unknown; tags: string[]
    environment_overrides?: Record<string, { enabled: boolean }>
  }) => api.post<Flag>(`/projects/${projectKey}/flags`, data),
  ...
})
```

**Step 5: Reset envOverrides in resetAndClose**

Add `setEnvOverrides({})` to the `resetAndClose` function.

**Step 6: Run lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 7: Commit**

```bash
git add web/src/components/CreateFlagModal.tsx
git commit -m "feat: show environment defaults with overrides in create flag modal"
```

---

### Task 7: Final verification and cleanup

**Step 1: Run Go vet on all internal packages**

Run: `go vet ./internal/...`
Expected: No errors

**Step 2: Run Go tests (non-DB)**

Run: `go test $(go list ./internal/... | grep -v /store)`
Expected: All pass

**Step 3: Run frontend lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Run frontend build**

Run: `cd web && npm run build`
Expected: Build succeeds

**Step 5: Commit any remaining fixes if needed**
