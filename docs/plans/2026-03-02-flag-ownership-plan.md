# Flag Ownership Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add optional owner (user) assignment to flags with full CRUD, filtering, and UI display with Gravatar avatars.

**Architecture:** Nullable `owner_id` FK on the `flags` table pointing to `users`. LEFT JOIN in flag queries to populate owner info. Gravatar avatars via MD5 hash of email. No new endpoints — ownership is a field on existing flag CRUD.

**Tech Stack:** Go (pgx/v5), React 19 + TypeScript + TanStack Query, Tailwind CSS + shadcn/ui, MD5 for Gravatar hashing.

**Design doc:** `docs/plans/2026-03-02-flag-ownership-design.md`

---

### Task 1: Database Migration

**Files:**
- Create: `migrations/010_flag_ownership.up.sql`
- Create: `migrations/010_flag_ownership.down.sql`

**Step 1: Write up migration**

```sql
-- migrations/010_flag_ownership.up.sql
ALTER TABLE flags ADD COLUMN owner_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX idx_flags_owner_id ON flags(owner_id);
```

**Step 2: Write down migration**

```sql
-- migrations/010_flag_ownership.down.sql
DROP INDEX IF EXISTS idx_flags_owner_id;
ALTER TABLE flags DROP COLUMN IF EXISTS owner_id;
```

**Step 3: Verify migration applies**

Run: `docker compose up -d && go test ./internal/store/... -count=1 -run TestFlagStore_Create -v`
Expected: PASS (migration runs on startup, existing tests still work)

**Step 4: Commit**

```bash
git add migrations/010_flag_ownership.up.sql migrations/010_flag_ownership.down.sql
git commit -m "feat: add owner_id column to flags table (#40)"
```

---

### Task 2: Go Model — Add FlagOwner and Owner Fields

**Files:**
- Modify: `internal/model/flag.go:39-53` (Flag struct)

**Step 1: Add FlagOwner type and extend Flag struct**

Add `FlagOwner` struct after the `Flag` struct, and add two fields to `Flag`:

```go
// In Flag struct, after UpdatedAt:
OwnerID *string    `json:"owner_id,omitempty"`
Owner   *FlagOwner `json:"owner,omitempty"`
```

```go
// New type after Flag struct:
type FlagOwner struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
}
```

**Step 2: Verify compilation**

Run: `go build ./...`
Expected: SUCCESS

**Step 3: Commit**

```bash
git add internal/model/flag.go
git commit -m "feat: add OwnerID and FlagOwner to Flag model (#40)"
```

---

### Task 3: Store — Update Flag Queries to JOIN Owner

**Files:**
- Modify: `internal/store/flag_store.go`

This is the largest backend task. We need to:
1. Update `Create` to accept and insert `ownerID`
2. Update `ListByProject` to LEFT JOIN users, scan owner fields, accept `owner` filter param
3. Update `FindByKey` to LEFT JOIN users, scan owner fields
4. Update `Update` to accept and update `ownerID`
5. Update `SetLifecycleStatus` to return `owner_id` (scan it)
6. Update `ListNonArchived` to scan `owner_id`

**Step 1: Write failing tests for owner in Create and FindByKey**

Add to `internal/store/flag_store_test.go`:

```go
func TestFlagStore_CreateWithOwner(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagowner")
	project, err := ps.Create(ctx, projKey, "Owner Test Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	email := uniqueEmail("owner")
	user, err := us.Create(ctx, email, "hash", model.RoleAdmin)
	if err != nil {
		t.Fatalf("creating user: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "owned-flag", "Owned Flag", "desc",
		model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`),
		[]string{}, nil, &user.ID)
	if err != nil {
		t.Fatalf("Create with owner: %v", err)
	}
	if flag.OwnerID == nil || *flag.OwnerID != user.ID {
		t.Errorf("OwnerID: got %v, want %q", flag.OwnerID, user.ID)
	}

	// FindByKey should return owner info
	found, err := fs.FindByKey(ctx, project.ID, "owned-flag")
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}
	if found.Owner == nil {
		t.Fatal("expected Owner to be populated")
	}
	if found.Owner.Email != email {
		t.Errorf("Owner.Email: got %q, want %q", found.Owner.Email, email)
	}
}

func TestFlagStore_CreateWithoutOwner(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagnoowner")
	project, err := ps.Create(ctx, projKey, "No Owner Project", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	flag, err := fs.Create(ctx, project.ID, "no-owner-flag", "No Owner", "desc",
		model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`),
		[]string{}, nil, nil)
	if err != nil {
		t.Fatalf("Create without owner: %v", err)
	}
	if flag.OwnerID != nil {
		t.Errorf("expected nil OwnerID, got %v", flag.OwnerID)
	}

	found, err := fs.FindByKey(ctx, project.ID, "no-owner-flag")
	if err != nil {
		t.Fatalf("FindByKey: %v", err)
	}
	if found.Owner != nil {
		t.Errorf("expected nil Owner, got %+v", found.Owner)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/store/... -count=1 -run TestFlagStore_CreateWith -v`
Expected: FAIL (Create signature doesn't accept ownerID yet)

**Step 3: Update `Create` method**

In `internal/store/flag_store.go`, update the `Create` signature to add `ownerID *string` as the last parameter. Update the INSERT SQL to include `owner_id` and the RETURNING clause to include `owner_id`. Scan into `f.OwnerID`.

```go
func (s *FlagStore) Create(ctx context.Context, projectID, key, name, description string, valueType model.ValueType, flagType model.FlagType, defaultValue json.RawMessage, tags []string, envEnabled map[string]bool, ownerID *string) (*model.Flag, error) {
```

SQL becomes:
```sql
INSERT INTO flags (project_id, key, name, description, value_type, flag_type, default_value, tags, owner_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, project_id, key, name, description, value_type, flag_type, default_value, tags, lifecycle_status, lifecycle_status_changed_at, created_at, updated_at, owner_id
```

Add `ownerID` to args and scan `&f.OwnerID` in the Scan call.

**Step 4: Update `FindByKey` to LEFT JOIN users**

```sql
SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id,
       u.id, u.email, u.display_name
FROM flags f
LEFT JOIN users u ON f.owner_id = u.id
WHERE f.project_id = $1 AND f.key = $2
```

Scan owner columns into nullable variables, populate `f.Owner` when non-null:

```go
var ownerUserID, ownerEmail *string
var ownerDisplayName *string
// ... scan these ...
if ownerUserID != nil {
    f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
}
```

**Step 5: Update `ListByProject` — LEFT JOIN + owner filter**

Update the query to use `f.` prefix on all columns, LEFT JOIN users, add owner filter:

```go
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string) ([]model.Flag, error) {
```

Add to WHERE clause:
```go
if owner != "" {
    query += fmt.Sprintf(" AND f.owner_id = $%d", argIdx)
    args = append(args, owner)
    argIdx++
}
```

Scan owner columns for each row, populate `f.Owner` when non-null.

**Step 6: Update `Update` to accept ownerID**

```go
func (s *FlagStore) Update(ctx context.Context, flagID, name, description string, tags []string, flagType model.FlagType, ownerID *string) (*model.Flag, error) {
```

SQL: add `owner_id=$6` to SET clause, add `owner_id` to RETURNING, scan `&f.OwnerID`.

Note: To support explicitly clearing the owner (setting to NULL), the handler will need to distinguish "not provided" from "set to null". Use a `**string` or a sentinel. Simplest approach: always pass the value — if the client sends `"owner_id": null`, pass `nil`. If the client sends `"owner_id": "uuid"`, pass `&uuid`.

**Step 7: Update `SetLifecycleStatus` and `ListNonArchived`**

Add `owner_id` to the RETURNING/SELECT columns and scan it. These don't need the JOIN (owner_id alone is sufficient for these internal queries).

**Step 8: Fix all callers of Create and Update**

All existing callers of `Create` need to pass `nil` as the new `ownerID` parameter. All callers of `Update` need to pass the owner ID. All callers of `ListByProject` need to pass `""` as the owner filter. Search for call sites:

- `internal/handler/flag_handler.go`: `Create` handler (line 122), `Update` handler (line 270), `List` handler (line 174)
- `internal/staleness/checker.go`: calls `ListNonArchived` (no signature change needed)

**Step 9: Run tests**

Run: `go test ./internal/store/... -count=1 -run TestFlagStore -v`
Expected: ALL PASS

**Step 10: Commit**

```bash
git add internal/store/flag_store.go internal/store/flag_store_test.go
git commit -m "feat: add owner support to flag store with LEFT JOIN (#40)"
```

---

### Task 4: Store — Owner Filter and Update Tests

**Files:**
- Modify: `internal/store/flag_store_test.go`

**Step 1: Write test for owner filter**

```go
func TestFlagStore_ListByProject_OwnerFilter(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("ownerfilter")
	project, _ := ps.Create(ctx, projKey, "Owner Filter Project", "test")
	es.Create(ctx, project.ID, "dev", "Development")

	user, _ := us.Create(ctx, uniqueEmail("filter"), "hash", model.RoleAdmin)

	// Create one flag with owner, one without
	fs.Create(ctx, project.ID, "owned", "Owned", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, &user.ID)
	fs.Create(ctx, project.ID, "unowned", "Unowned", "", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil)

	// Filter by owner
	flags, err := fs.ListByProject(ctx, project.ID, "", "", "", "", user.ID)
	if err != nil {
		t.Fatalf("ListByProject with owner filter: %v", err)
	}
	if len(flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(flags))
	}
	if flags[0].Key != "owned" {
		t.Errorf("expected key 'owned', got %q", flags[0].Key)
	}
}
```

**Step 2: Write test for updating owner**

```go
func TestFlagStore_UpdateOwner(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("updateowner")
	project, _ := ps.Create(ctx, projKey, "Update Owner Project", "test")
	es.Create(ctx, project.ID, "dev", "Development")

	user, _ := us.Create(ctx, uniqueEmail("updateowner"), "hash", model.RoleAdmin)

	flag, _ := fs.Create(ctx, project.ID, "update-owner-flag", "Flag", "",
		model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil)

	// Set owner
	updated, err := fs.Update(ctx, flag.ID, "Flag", "", []string{}, model.FlagTypeRelease, &user.ID)
	if err != nil {
		t.Fatalf("Update to set owner: %v", err)
	}
	if updated.OwnerID == nil || *updated.OwnerID != user.ID {
		t.Errorf("expected OwnerID %q, got %v", user.ID, updated.OwnerID)
	}

	// Clear owner
	updated, err = fs.Update(ctx, flag.ID, "Flag", "", []string{}, model.FlagTypeRelease, nil)
	if err != nil {
		t.Fatalf("Update to clear owner: %v", err)
	}
	if updated.OwnerID != nil {
		t.Errorf("expected nil OwnerID, got %v", updated.OwnerID)
	}
}
```

**Step 3: Run tests**

Run: `go test ./internal/store/... -count=1 -run "TestFlagStore_ListByProject_OwnerFilter|TestFlagStore_UpdateOwner" -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/store/flag_store_test.go
git commit -m "test: add owner filter and update owner store tests (#40)"
```

---

### Task 5: Handler — Wire Owner Through Create, Update, List

**Files:**
- Modify: `internal/handler/flag_handler.go:52-153` (Create handler)
- Modify: `internal/handler/flag_handler.go:155-183` (List handler)
- Modify: `internal/handler/flag_handler.go:226-294` (Update handler)

**Step 1: Update Create handler**

Add `OwnerID *string` to the create request struct (line 66-75):

```go
var req struct {
    Key                  string                              `json:"key"`
    Name                 string                              `json:"name"`
    Description          string                              `json:"description"`
    ValueType            model.ValueType                     `json:"value_type"`
    FlagType             model.FlagType                      `json:"flag_type"`
    DefaultValue         json.RawMessage                     `json:"default_value"`
    Tags                 []string                            `json:"tags"`
    OwnerID              *string                             `json:"owner_id"`
    EnvironmentOverrides map[string]model.EnvironmentDefault `json:"environment_overrides"`
}
```

Pass `req.OwnerID` to `h.flags.Create(...)` call (line 122).

**Step 2: Update List handler**

Read `owner` query param and pass to store (lines 169-174):

```go
owner := r.URL.Query().Get("owner")
flags, err := h.flags.ListByProject(r.Context(), project.ID, tag, search, lifecycleStatus, flagType, owner)
```

**Step 3: Update Update handler**

Add `OwnerID *string` to the update request struct (lines 252-257). Handle the distinction between "not sent" and "explicitly null":

Since Go's JSON unmarshalling sets pointer fields to `nil` for both missing and `null`, and we want to always pass the value through (the client must always send the full update), the simplest approach is: always include `owner_id` in the UPDATE SQL. The client sends the current owner_id to keep it, `null` to clear it.

```go
var req struct {
    Name        string         `json:"name"`
    Description string         `json:"description"`
    Tags        []string       `json:"tags"`
    FlagType    model.FlagType `json:"flag_type"`
    OwnerID     *string        `json:"owner_id"`
}
```

Pass `req.OwnerID` to `h.flags.Update(...)` (line 270).

**Step 4: Verify all tests pass**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/handler/flag_handler.go
git commit -m "feat: wire owner_id through flag create/update/list handlers (#40)"
```

---

### Task 6: Frontend Types and Gravatar Utility

**Files:**
- Modify: `web/src/api/types.ts:40-54` (Flag interface)
- Create: `web/src/lib/gravatar.ts`

**Step 1: Add owner types to Flag interface**

In `web/src/api/types.ts`, add `FlagOwner` interface and extend `Flag`:

```typescript
export interface FlagOwner {
  id: string
  email: string
  display_name?: string
}

// In Flag interface, add:
  owner_id?: string
  owner?: FlagOwner
```

**Step 2: Create Gravatar utility**

```typescript
// web/src/lib/gravatar.ts
import { md5 } from './md5'

export function gravatarUrl(email: string, size = 32): string {
  const hash = md5(email.trim().toLowerCase())
  return `https://gravatar.com/avatar/${hash}?d=mp&s=${size}`
}
```

For MD5, use a lightweight implementation. Install `md5` package or use the Web Crypto API (which doesn't support MD5 directly). Simplest: install the `blueimp-md5` npm package (tiny, no dependencies):

```bash
cd web && npm install blueimp-md5 && npm install -D @types/blueimp-md5
```

Then:
```typescript
// web/src/lib/gravatar.ts
import md5 from 'blueimp-md5'

export function gravatarUrl(email: string, size = 32): string {
  const hash = md5(email.trim().toLowerCase())
  return `https://gravatar.com/avatar/${hash}?d=mp&s=${size}`
}
```

**Step 3: Commit**

```bash
git add web/src/api/types.ts web/src/lib/gravatar.ts web/package.json web/package-lock.json
git commit -m "feat: add FlagOwner type and Gravatar utility (#40)"
```

---

### Task 7: Frontend — Flag Card Owner Display

**Files:**
- Modify: `web/src/components/FlagCard.tsx`

**Step 1: Add owner display to FlagCard**

After the environment status pills (Row 3) and before the purpose row (Row 4), add owner info. If `flag.owner` exists, show a small Gravatar + name:

```tsx
import { gravatarUrl } from '@/lib/gravatar'

// In the component, between Row 3 (env pills) and Row 4 (purpose):
{/* Row 4: Owner + Purpose */}
<div className="flex items-center justify-between">
  {flag.owner ? (
    <div className="flex items-center gap-1.5">
      <img
        src={gravatarUrl(flag.owner.email, 20)}
        alt=""
        className="w-5 h-5 rounded-full"
      />
      <span className="text-[11px] text-muted-foreground/60 truncate max-w-[140px]">
        {flag.owner.display_name ?? flag.owner.email}
      </span>
    </div>
  ) : (
    <span />
  )}
  <span className="text-[11px] text-muted-foreground/50 capitalize">{flag.flag_type}</span>
</div>
```

**Step 2: Verify lint passes**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/components/FlagCard.tsx
git commit -m "feat: show flag owner with Gravatar on flag cards (#40)"
```

---

### Task 8: Frontend — Flag Detail Owner Display and Edit

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Fetch users list for the owner selector**

Add a query for users (needed for the owner combobox):

```tsx
import type { Flag, Environment, FlagEnvironmentConfig, User } from '../api/types.ts'

const { data: users } = useQuery({
  queryKey: ['users'],
  queryFn: () => api.get<User[]>('/management/users'),
})
```

**Step 2: Add owner mutation**

```tsx
const ownerMutation = useMutation({
  mutationFn: (ownerId: string | null) =>
    api.put<Flag>(`/projects/${key}/flags/${flagKey}`, {
      name: flag.name,
      description: flag.description,
      tags: flag.tags,
      flag_type: flag.flag_type,
      owner_id: ownerId,
    }),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
    queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags'] })
  },
})
```

**Step 3: Add owner display and selector in metadata section**

After the tags in the metadata chips section (after line 228), add an owner section:

```tsx
import { gravatarUrl } from '@/lib/gravatar'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

{/* Owner */}
<div className="flex items-center gap-2 mb-6">
  <span className="text-[11px] text-muted-foreground/50 uppercase tracking-wider font-mono">Owner</span>
  <Select
    value={flag.owner_id ?? 'unassigned'}
    onValueChange={(value) => ownerMutation.mutate(value === 'unassigned' ? null : value)}
  >
    <SelectTrigger className="w-[220px] h-8 text-[13px]">
      <SelectValue>
        {flag.owner ? (
          <span className="flex items-center gap-2">
            <img src={gravatarUrl(flag.owner.email, 20)} alt="" className="w-5 h-5 rounded-full" />
            {flag.owner.display_name ?? flag.owner.email}
          </span>
        ) : (
          <span className="text-muted-foreground/60">Unassigned</span>
        )}
      </SelectValue>
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="unassigned">Unassigned</SelectItem>
      {users?.map((u) => (
        <SelectItem key={u.id} value={u.id}>
          <span className="flex items-center gap-2">
            <img src={gravatarUrl(u.email, 20)} alt="" className="w-5 h-5 rounded-full" />
            {u.display_name ?? u.email}
          </span>
        </SelectItem>
      ))}
    </SelectContent>
  </Select>
</div>
```

**Step 4: Verify lint passes**

Run: `cd web && npm run lint`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat: add owner selector to flag detail page (#40)"
```

---

### Task 9: Frontend — Owner Filter on Flag List

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Add owner filter state and users query**

```tsx
import type { Flag, Environment, FlagEnvironmentConfig, UnknownFlag, FlagPurpose, LifecycleStatus, User } from '../api/types.ts'

const [ownerFilter, setOwnerFilter] = useState('')

const { data: users } = useQuery({
  queryKey: ['users'],
  queryFn: () => api.get<User[]>('/management/users'),
})
```

**Step 2: Add owner to client-side filtering**

In the `filtered` useMemo (line 101-113), add:

```tsx
const matchesOwner = !ownerFilter ||
  (ownerFilter === 'unassigned' ? !f.owner_id : f.owner_id === ownerFilter)
return matchesSearch && matchesTag && matchesPurpose && matchesStatus && matchesOwner
```

Add `ownerFilter` to the dependency array.

**Step 3: Add owner filter dropdown to the filter bar**

After the status filter select (line 203-213), add:

```tsx
<select
  className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer w-full md:w-auto md:min-w-[130px]"
  value={ownerFilter}
  onChange={(e) => setOwnerFilter(e.target.value)}
>
  <option value="">All Owners</option>
  <option value="unassigned">Unassigned</option>
  {users?.map((u) => (
    <option key={u.id} value={u.id}>{u.display_name ?? u.email}</option>
  ))}
</select>
```

**Step 4: Verify lint passes**

Run: `cd web && npm run lint`
Expected: PASS

**Step 5: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat: add owner filter to flag list page (#40)"
```

---

### Task 10: Frontend — Owner in Create Flag Modal

**Files:**
- Modify: `web/src/components/CreateFlagModal.tsx`

**Step 1: Add owner selector to create modal**

Add a users query and owner state:

```tsx
const { data: users } = useQuery({
  queryKey: ['users'],
  queryFn: () => api.get<User[]>('/management/users'),
  enabled: open,
})
const [ownerId, setOwnerId] = useState<string>('')
```

Add owner to the mutation data (in `handleSubmit`):

```tsx
mutation.mutate({
  // ... existing fields ...
  owner_id: ownerId || undefined,
})
```

Add the owner select field in the form, after description and before flag purpose:

```tsx
<div className="space-y-1.5">
  <Label>Owner</Label>
  <Select value={ownerId || 'none'} onValueChange={(v) => setOwnerId(v === 'none' ? '' : v)}>
    <SelectTrigger>
      <SelectValue placeholder="No owner" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="none">No owner</SelectItem>
      {users?.map((u) => (
        <SelectItem key={u.id} value={u.id}>
          {u.display_name ?? u.email}
        </SelectItem>
      ))}
    </SelectContent>
  </Select>
</div>
```

Reset `ownerId` in `resetAndClose`.

**Step 2: Verify lint passes**

Run: `cd web && npm run lint`
Expected: PASS

**Step 3: Commit**

```bash
git add web/src/components/CreateFlagModal.tsx
git commit -m "feat: add owner selector to create flag modal (#40)"
```

---

### Task 11: Full Integration Test

**Step 1: Build frontend**

Run: `cd web && npm install && npm run build`
Expected: SUCCESS

**Step 2: Build Go binary**

Run: `go build -o togglerino ./cmd/togglerino`
Expected: SUCCESS

**Step 3: Run all Go tests**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 4: Run frontend lint**

Run: `cd web && npm run lint`
Expected: PASS

**Step 5: Final commit if any fixups needed**

---

### Task 12: Manual Smoke Test (Optional)

**Step 1: Start the app**

```bash
docker compose up -d
```

**Step 2: Verify in browser**

- Create a flag with an owner assigned
- Verify owner appears on flag card with Gravatar
- Open flag detail, verify owner displayed, change owner via dropdown
- Filter flag list by owner
- Create a flag without owner, verify "Unassigned" state
