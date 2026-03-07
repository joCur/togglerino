# Pagination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add offset-based pagination to 5 management API list endpoints, with infinite scroll for flags/projects and "Load More" for users/segments/unknown flags.

**Architecture:** Shared `parsePagination` helper + `PaginatedResponse` wrapper in the Go handler layer. Store methods gain `limit, offset` params and return `([]T, total int, error)` using `COUNT(*) OVER()` window function. Frontend uses TanStack Query's `useInfiniteQuery` with a shared `useInfiniteScroll` hook for intersection-observer-based auto-loading.

**Tech Stack:** Go 1.25 (stdlib `net/http`), pgx/v5 (PostgreSQL), React 19, TanStack Query, TypeScript

**Design doc:** `docs/plans/2026-03-07-pagination-design.md`

---

### Task 1: Shared Pagination Helpers (Backend)

**Files:**
- Create: `internal/handler/pagination.go`
- Create: `internal/handler/pagination_test.go`

**Step 1: Write the test for `parsePagination`**

Create `internal/handler/pagination_test.go`:

```go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	limit, offset := parsePagination(r)
	if limit != 50 {
		t.Errorf("expected limit 50, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?limit=25&offset=100", nil)
	limit, offset := parsePagination(r)
	if limit != 25 {
		t.Errorf("expected limit 25, got %d", limit)
	}
	if offset != 100 {
		t.Errorf("expected offset 100, got %d", offset)
	}
}

func TestParsePagination_ClampsLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?limit=200", nil)
	limit, _ := parsePagination(r)
	if limit != 100 {
		t.Errorf("expected limit clamped to 100, got %d", limit)
	}
}

func TestParsePagination_RejectsNegativeLimit(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?limit=-5", nil)
	limit, _ := parsePagination(r)
	if limit != 50 {
		t.Errorf("expected default limit 50, got %d", limit)
	}
}

func TestParsePagination_RejectsNegativeOffset(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/test?offset=-10", nil)
	_, offset := parsePagination(r)
	if offset != 0 {
		t.Errorf("expected default offset 0, got %d", offset)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/handler/ -run TestParsePagination -v`
Expected: FAIL — `parsePagination` undefined

**Step 3: Write the implementation**

Create `internal/handler/pagination.go`:

```go
package handler

import (
	"net/http"
	"strconv"
)

const (
	defaultLimit = 50
	maxLimit     = 100
)

// parsePagination extracts limit and offset from query parameters.
// Defaults: limit=50, offset=0. Limit is clamped to 1-100.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultLimit
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

// PaginatedResponse wraps a list response with pagination metadata.
type PaginatedResponse struct {
	Data   any `json:"data"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/handler/ -run TestParsePagination -v`
Expected: all 5 tests PASS

**Step 5: Commit**

```bash
git add internal/handler/pagination.go internal/handler/pagination_test.go
git commit -m "feat: add shared parsePagination helper and PaginatedResponse type"
```

---

### Task 2: Paginate Flags Store

**Files:**
- Modify: `internal/store/flag_store.go:116-186` (ListByProject)
- Modify: `internal/store/flag_store_test.go` (add pagination test)

**Step 1: Write the failing test**

Add to `internal/store/flag_store_test.go`:

```go
func TestFlagStore_ListByProject_Pagination(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	fs := store.NewFlagStore(pool)
	ctx := context.Background()

	projKey := uniqueKey("flagpage")
	project, err := ps.Create(ctx, projKey, "Flag Pagination", "test")
	if err != nil {
		t.Fatalf("creating project: %v", err)
	}
	_, err = es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("creating env: %v", err)
	}

	// Create 5 flags
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("pflag-%d", i)
		_, err := fs.Create(ctx, project.ID, key, "Flag "+key, "desc", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{}, nil, nil, nil)
		if err != nil {
			t.Fatalf("Create %s: %v", key, err)
		}
	}

	// Page 1: limit=2, offset=0
	flags, total, err := fs.ListByProject(ctx, project.ID, "", "", "", "", "", 2, 0)
	if err != nil {
		t.Fatalf("ListByProject page1: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 flags on page1, got %d", len(flags))
	}

	// Page 2: limit=2, offset=2
	flags2, total2, err := fs.ListByProject(ctx, project.ID, "", "", "", "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListByProject page2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("expected total 5, got %d", total2)
	}
	if len(flags2) != 2 {
		t.Fatalf("expected 2 flags on page2, got %d", len(flags2))
	}

	// Page 3: limit=2, offset=4 — only 1 remaining
	flags3, total3, err := fs.ListByProject(ctx, project.ID, "", "", "", "", "", 2, 4)
	if err != nil {
		t.Fatalf("ListByProject page3: %v", err)
	}
	if total3 != 5 {
		t.Errorf("expected total 5, got %d", total3)
	}
	if len(flags3) != 1 {
		t.Fatalf("expected 1 flag on page3, got %d", len(flags3))
	}

	// No overlap between pages
	if flags[0].Key == flags2[0].Key {
		t.Error("page1 and page2 should not overlap")
	}

	// Total with filter: tag "ui" should give different total
	_, err = fs.Create(ctx, project.ID, "tagged-flag", "Tagged", "desc", model.ValueTypeBoolean, model.FlagTypeRelease, json.RawMessage(`false`), []string{"ui"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("Create tagged-flag: %v", err)
	}
	_, filteredTotal, err := fs.ListByProject(ctx, project.ID, "ui", "", "", "", "", 50, 0)
	if err != nil {
		t.Fatalf("ListByProject with tag filter: %v", err)
	}
	if filteredTotal != 1 {
		t.Errorf("expected filtered total 1, got %d", filteredTotal)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestFlagStore_ListByProject_Pagination -v`
Expected: FAIL — wrong number of return values (ListByProject currently returns 2 values, test expects 3)

**Step 3: Modify `ListByProject` to accept pagination and return total**

In `internal/store/flag_store.go`, change the signature at line 116 and update the implementation:

```go
// ListByProject returns flags for a project with pagination. Supports optional tag filter, search query,
// lifecycle status filter, flag type filter, and owner filter. Returns flags, total count, and error.
func (s *FlagStore) ListByProject(ctx context.Context, projectID string, tag string, search string, lifecycleStatus string, flagType string, owner string, limit, offset int) ([]model.Flag, int, error) {
	query := `SELECT f.id, f.project_id, f.key, f.name, f.description, f.value_type, f.flag_type, f.default_value, f.tags, f.lifecycle_status, f.lifecycle_status_changed_at, f.created_at, f.updated_at, f.owner_id,
	       u.id, u.email, u.display_name,
	       COUNT(*) OVER() AS total
		FROM flags f
		LEFT JOIN users u ON f.owner_id = u.id
		WHERE f.project_id = $1`
	args := []any{projectID}
	argIdx := 2

	if tag != "" {
		query += fmt.Sprintf(" AND $%d = ANY(f.tags)", argIdx)
		args = append(args, tag)
		argIdx++
	}

	if search != "" {
		query += fmt.Sprintf(" AND (f.key ILIKE '%%' || $%d || '%%' OR f.name ILIKE '%%' || $%d || '%%')", argIdx, argIdx)
		args = append(args, search)
		argIdx++
	}

	if lifecycleStatus != "" {
		values := strings.Split(lifecycleStatus, ",")
		query += fmt.Sprintf(" AND f.lifecycle_status = ANY($%d)", argIdx)
		args = append(args, values)
		argIdx++
	}

	if flagType != "" {
		values := strings.Split(flagType, ",")
		query += fmt.Sprintf(" AND f.flag_type = ANY($%d)", argIdx)
		args = append(args, values)
		argIdx++
	}

	if owner != "" {
		query += fmt.Sprintf(" AND f.owner_id = $%d", argIdx)
		args = append(args, owner)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY f.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("listing flags: %w", err)
	}
	defer rows.Close()

	var flags []model.Flag
	var total int
	for rows.Next() {
		var f model.Flag
		var ownerUserID, ownerEmail *string
		var ownerDisplayName *string
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.Key, &f.Name, &f.Description, &f.ValueType, &f.FlagType, &f.DefaultValue, &f.Tags, &f.LifecycleStatus, &f.LifecycleStatusChangedAt, &f.CreatedAt, &f.UpdatedAt, &f.OwnerID,
			&ownerUserID, &ownerEmail, &ownerDisplayName, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning flag: %w", err)
		}
		if ownerUserID != nil {
			f.Owner = &model.FlagOwner{ID: *ownerUserID, Email: *ownerEmail, DisplayName: ownerDisplayName}
		}
		if f.Tags == nil {
			f.Tags = []string{}
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating flags: %w", err)
	}
	return flags, total, nil
}
```

**Step 4: Fix all callers of `ListByProject`**

The signature change will break callers. Update each one:

1. `internal/handler/flag_handler.go:184` — change to:
   ```go
   limit, offset := parsePagination(r)
   flags, total, err := h.flags.ListByProject(r.Context(), project.ID, tag, search, lifecycleStatus, flagType, owner, limit, offset)
   ```
   And change the response at line 192 to:
   ```go
   writeJSON(w, http.StatusOK, PaginatedResponse{Data: flags, Total: total, Limit: limit, Offset: offset})
   ```

2. `internal/evaluation/cache.go` (or wherever `LoadAll` calls `ListByProject`) — find all other callers using `grep "ListByProject"` and pass large limit (e.g., `10000, 0`) or add a separate `ListAll` method. **Important:** check `internal/evaluation/cache.go` and `internal/staleness/checker.go` — these need ALL flags, not paginated. Add a convenience wrapper:

   In `internal/store/flag_store.go`, add after the modified `ListByProject`:
   ```go
   // ListAllByProject returns all flags for a project without pagination. Used by cache and staleness checker.
   func (s *FlagStore) ListAllByProject(ctx context.Context, projectID string) ([]model.Flag, error) {
   	flags, _, err := s.ListByProject(ctx, projectID, "", "", "", "", "", 1000000, 0)
   	return flags, err
   }
   ```

   Then update cache.go and staleness checker.go calls from `ListByProject(ctx, projectID, "", "", "", "", "")` to `ListAllByProject(ctx, projectID)`.

3. Update existing tests in `flag_store_test.go` that call `ListByProject` — add the two extra params (`50, 0`) and handle the third return value. Each call like:
   ```go
   flags, err := fs.ListByProject(ctx, project.ID, "", "", "", "", "")
   ```
   becomes:
   ```go
   flags, _, err := fs.ListByProject(ctx, project.ID, "", "", "", "", "", 50, 0)
   ```

**Step 5: Run all tests to verify**

Run: `go test ./internal/store/ -run TestFlagStore -v`
Run: `go test ./internal/... -v` (ensure cache/staleness callers compile)
Expected: all PASS

**Step 6: Commit**

```bash
git add internal/store/flag_store.go internal/store/flag_store_test.go internal/handler/flag_handler.go internal/evaluation/cache.go internal/staleness/checker.go
git commit -m "feat: add pagination to flags list endpoint (store + handler)"
```

---

### Task 3: Paginate Projects Store

**Files:**
- Modify: `internal/store/project_store.go:33-54` (List), `internal/store/project_store.go:56-79` (ListByIDs)
- Modify: `internal/store/project_store_test.go`
- Modify: `internal/handler/project_handler.go:75-132` (List)

**Step 1: Write the failing test**

Add to `internal/store/project_store_test.go`:

```go
func TestProjectStore_List_Pagination(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ctx := context.Background()

	// Create 5 projects
	for i := 0; i < 5; i++ {
		key := uniqueKey(fmt.Sprintf("projpage-%d", i))
		_, err := ps.Create(ctx, key, "Project "+key, "desc")
		if err != nil {
			t.Fatalf("Create %s: %v", key, err)
		}
	}

	// Page 1: limit=2
	projects, total, err := ps.List(ctx, 2, 0)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if total < 5 {
		t.Errorf("expected total >= 5, got %d", total)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects on page1, got %d", len(projects))
	}

	// Page 2 should not overlap
	projects2, _, err := ps.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("List page2: %v", err)
	}
	if len(projects2) != 2 {
		t.Fatalf("expected 2 projects on page2, got %d", len(projects2))
	}
	if projects[0].ID == projects2[0].ID {
		t.Error("page1 and page2 should not overlap")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestProjectStore_List_Pagination -v`
Expected: FAIL — wrong number of arguments/returns

**Step 3: Modify `List` and `ListByIDs`**

In `internal/store/project_store.go`:

```go
// List returns all projects with pagination.
func (s *ProjectStore) List(ctx context.Context, limit, offset int) ([]model.Project, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, name, description, created_at, updated_at, COUNT(*) OVER() AS total
		 FROM projects ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing projects: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	var total int
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating projects: %w", err)
	}
	return projects, total, nil
}

// ListByIDs returns projects matching the given IDs with pagination.
func (s *ProjectStore) ListByIDs(ctx context.Context, ids []string, limit, offset int) ([]model.Project, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, key, name, description, created_at, updated_at, COUNT(*) OVER() AS total
		 FROM projects WHERE id = ANY($1) ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		ids, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing projects by IDs: %w", err)
	}
	defer rows.Close()

	var projects []model.Project
	var total int
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(&p.ID, &p.Key, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning project: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating projects: %w", err)
	}
	return projects, total, nil
}
```

**Step 4: Update the handler**

In `internal/handler/project_handler.go`, update the `List` method. Each branch that calls `h.projects.List(r.Context())` or `h.projects.ListByIDs(r.Context(), projectIDs)` needs pagination:

```go
func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	limit, offset := parsePagination(r)

	if user != nil && user.Role == model.RoleAdmin {
		projects, total, err := h.projects.List(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		if projects == nil {
			projects = []model.Project{}
		}
		writeJSON(w, http.StatusOK, PaginatedResponse{Data: projects, Total: total, Limit: limit, Offset: offset})
		return
	}

	baseRole, _ := h.orgSettings.GetBaseProjectRole(r.Context())
	if baseRole != "" && baseRole != "none" {
		projects, total, err := h.projects.List(r.Context(), limit, offset)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list projects")
			return
		}
		if projects == nil {
			projects = []model.Project{}
		}
		writeJSON(w, http.StatusOK, PaginatedResponse{Data: projects, Total: total, Limit: limit, Offset: offset})
		return
	}

	if user == nil {
		writeJSON(w, http.StatusOK, PaginatedResponse{Data: []model.Project{}, Total: 0, Limit: limit, Offset: offset})
		return
	}
	projectIDs, err := h.members.ListAccessibleProjectIDs(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list accessible projects")
		return
	}
	if len(projectIDs) == 0 {
		writeJSON(w, http.StatusOK, PaginatedResponse{Data: []model.Project{}, Total: 0, Limit: limit, Offset: offset})
		return
	}

	projects, total, err := h.projects.ListByIDs(r.Context(), projectIDs, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}
	if projects == nil {
		projects = []model.Project{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Data: projects, Total: total, Limit: limit, Offset: offset})
}
```

**Step 5: Fix all callers and existing tests**

Search for all calls to `h.projects.List(` and `s.projects.List(` and `ps.List(` in tests — add `limit, offset` params and handle 3 return values. Update `internal/handler/project_handler_test.go` to decode `PaginatedResponse` instead of bare arrays.

**Step 6: Run tests**

Run: `go test ./internal/store/ -run TestProjectStore -v`
Run: `go test ./internal/handler/ -run TestProject -v`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/store/project_store.go internal/store/project_store_test.go internal/handler/project_handler.go internal/handler/project_handler_test.go
git commit -m "feat: add pagination to projects list endpoint"
```

---

### Task 4: Paginate Users Store

**Files:**
- Modify: `internal/store/user_store.go:65-86` (List)
- Modify: `internal/handler/user_handler.go:30-37` (List)

**Step 1: Write the failing test**

Add to `internal/store/user_store_test.go` (or create it if it doesn't exist):

```go
func TestUserStore_List_Pagination(t *testing.T) {
	pool := testPool(t)
	us := store.NewUserStore(pool)
	ctx := context.Background()

	// Create 3 users
	for i := 0; i < 3; i++ {
		email := fmt.Sprintf("userpage-%s-%d@test.dev", uniqueKey("u"), i)
		_, err := us.Create(ctx, email, "$2a$10$dummy", model.RoleMember)
		if err != nil {
			t.Fatalf("Create user %d: %v", i, err)
		}
	}

	users, total, err := us.List(ctx, 2, 0)
	if err != nil {
		t.Fatalf("List page1: %v", err)
	}
	if total < 3 {
		t.Errorf("expected total >= 3, got %d", total)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users on page1, got %d", len(users))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestUserStore_List_Pagination -v`
Expected: FAIL

**Step 3: Modify `List` in user_store.go**

```go
func (s *UserStore) List(ctx context.Context, limit, offset int) ([]model.User, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, email, display_name, password_hash, role, created_at, updated_at, COUNT(*) OVER() AS total
		 FROM users ORDER BY created_at LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	var total int
	for rows.Next() {
		var u model.User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating users: %w", err)
	}
	return users, total, nil
}
```

**Step 4: Update handler**

In `internal/handler/user_handler.go`:

```go
func (h *UserHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	users, total, err := h.users.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Data: users, Total: total, Limit: limit, Offset: offset})
}
```

**Step 5: Fix all callers and tests**

Search for all calls to `us.List(ctx)` or `h.users.List(` and update signatures. Fix tests to handle 3 return values.

**Step 6: Run tests**

Run: `go test ./internal/store/ -run TestUserStore -v`
Run: `go test ./internal/handler/ -run TestUser -v`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/store/user_store.go internal/handler/user_handler.go
git commit -m "feat: add pagination to users list endpoint"
```

---

### Task 5: Paginate Segments Store

**Files:**
- Modify: `internal/store/segment_store.go:63-94` (ListByProject)
- Modify: `internal/handler/segment_handler.go:67-90` (List)

**Step 1: Write the failing test**

Add to `internal/store/segment_store_test.go`:

```go
func TestSegmentStore_ListByProject_Pagination(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	ss := store.NewSegmentStore(pool)
	ctx := context.Background()

	project, err := ps.Create(ctx, uniqueKey("segpage"), "Seg Pagination", "test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}

	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("seg-%d-%s", i, uniqueKey("s"))
		_, err := ss.Create(ctx, project.ID, key, "Segment "+key, "desc", json.RawMessage(`[]`))
		if err != nil {
			t.Fatalf("Create segment %s: %v", key, err)
		}
	}

	segments, total, err := ss.ListByProject(ctx, project.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if total != 3 {
		t.Errorf("expected total 3, got %d", total)
	}
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestSegmentStore_ListByProject_Pagination -v`
Expected: FAIL

**Step 3: Modify `ListByProject` in segment_store.go**

```go
func (s *SegmentStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.Segment, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, key, name, description, conditions, created_at, updated_at, COUNT(*) OVER() AS total
		 FROM segments WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing segments: %w", err)
	}
	defer rows.Close()

	var segments []model.Segment
	var total int
	for rows.Next() {
		var seg model.Segment
		var conditionsJSON []byte
		if err := rows.Scan(&seg.ID, &seg.ProjectID, &seg.Key, &seg.Name, &seg.Description, &conditionsJSON, &seg.CreatedAt, &seg.UpdatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning segment: %w", err)
		}
		if err := json.Unmarshal(conditionsJSON, &seg.Conditions); err != nil {
			return nil, 0, fmt.Errorf("unmarshaling segment conditions: %w", err)
		}
		if seg.Conditions == nil {
			seg.Conditions = []model.Condition{}
		}
		segments = append(segments, seg)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating segments: %w", err)
	}
	return segments, total, nil
}
```

**Step 4: Update handler**

In `internal/handler/segment_handler.go`:

```go
func (h *SegmentHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	segments, total, err := h.segments.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list segments")
		return
	}
	if segments == nil {
		segments = []model.Segment{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Data: segments, Total: total, Limit: limit, Offset: offset})
}
```

**Step 5: Fix callers and tests**

Update all calls to `ss.ListByProject(ctx, projectID)` → `ss.ListByProject(ctx, projectID, 50, 0)` (or `1000000, 0` for cache-like callers). Handle 3 return values.

**Step 6: Run tests**

Run: `go test ./internal/store/ -run TestSegmentStore -v`
Run: `go test ./internal/handler/ -run TestSegment -v`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/store/segment_store.go internal/store/segment_store_test.go internal/handler/segment_handler.go
git commit -m "feat: add pagination to segments list endpoint"
```

---

### Task 6: Paginate Unknown Flags Store

**Files:**
- Modify: `internal/store/unknown_flag_store.go:38-73` (ListByProject)
- Modify: `internal/handler/unknown_flag_handler.go:21-42` (List)

**Step 1: Write the failing test**

Add to `internal/store/unknown_flag_store_test.go`:

```go
func TestUnknownFlagStore_ListByProject_Pagination(t *testing.T) {
	pool := testPool(t)
	ps := store.NewProjectStore(pool)
	es := store.NewEnvironmentStore(pool)
	ufs := store.NewUnknownFlagStore(pool)
	ctx := context.Background()

	project, err := ps.Create(ctx, uniqueKey("ufpage"), "UF Pagination", "test")
	if err != nil {
		t.Fatalf("Create project: %v", err)
	}
	env, err := es.Create(ctx, project.ID, "dev", "Development")
	if err != nil {
		t.Fatalf("Create env: %v", err)
	}

	for i := 0; i < 4; i++ {
		err := ufs.Upsert(ctx, project.ID, env.ID, fmt.Sprintf("unknown-%d", i))
		if err != nil {
			t.Fatalf("Upsert %d: %v", i, err)
		}
	}

	flags, total, err := ufs.ListByProject(ctx, project.ID, 2, 0)
	if err != nil {
		t.Fatalf("ListByProject: %v", err)
	}
	if total != 4 {
		t.Errorf("expected total 4, got %d", total)
	}
	if len(flags) != 2 {
		t.Fatalf("expected 2 flags, got %d", len(flags))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestUnknownFlagStore_ListByProject_Pagination -v`
Expected: FAIL

**Step 3: Modify `ListByProject` in unknown_flag_store.go**

```go
func (s *UnknownFlagStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.UnknownFlag, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT uf.id, uf.project_id, uf.environment_id, uf.flag_key,
		        uf.request_count, uf.first_seen_at, uf.last_seen_at,
		        e.key, e.name,
		        COUNT(*) OVER() AS total
		 FROM unknown_flags uf
		 JOIN environments e ON e.id = uf.environment_id
		 WHERE uf.project_id = $1 AND uf.dismissed_at IS NULL
		 ORDER BY uf.last_seen_at DESC
		 LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing unknown flags: %w", err)
	}
	defer rows.Close()

	var flags []model.UnknownFlag
	var total int
	for rows.Next() {
		var f model.UnknownFlag
		if err := rows.Scan(&f.ID, &f.ProjectID, &f.EnvironmentID, &f.FlagKey,
			&f.RequestCount, &f.FirstSeenAt, &f.LastSeenAt,
			&f.EnvironmentKey, &f.EnvironmentName, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning unknown flag: %w", err)
		}
		flags = append(flags, f)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating unknown flags: %w", err)
	}
	if flags == nil {
		flags = []model.UnknownFlag{}
	}
	return flags, total, nil
}
```

**Step 4: Update handler**

In `internal/handler/unknown_flag_handler.go`:

```go
func (h *UnknownFlagHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	flags, total, err := h.unknownFlags.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list unknown flags")
		return
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Data: flags, Total: total, Limit: limit, Offset: offset})
}
```

**Step 5: Fix callers and tests**

Update all calls to `ufs.ListByProject(ctx, projectID)` to include `limit, offset` and handle 3 return values.

**Step 6: Run tests**

Run: `go test ./internal/store/ -run TestUnknownFlagStore -v`
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/store/unknown_flag_store.go internal/store/unknown_flag_store_test.go internal/handler/unknown_flag_handler.go
git commit -m "feat: add pagination to unknown flags list endpoint"
```

---

### Task 7: Migrate Audit Log & Flag History to Shared Helpers

**Files:**
- Modify: `internal/store/audit_store.go:32-56` (ListByProject), `internal/store/audit_store.go:72-107` (ListByFlag)
- Modify: `internal/handler/audit_handler.go:20-58`
- Modify: `internal/handler/history_handler.go`

**Step 1: Update audit store to return total**

In `internal/store/audit_store.go`, modify `ListByProject`:

```go
func (s *AuditStore) ListByProject(ctx context.Context, projectID string, limit, offset int) ([]model.AuditEntry, int, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, project_id, user_id, user_email, environment_id, batch_id, action, entity_type, entity_id, old_value, new_value, created_at,
		        COUNT(*) OVER() AS total
		 FROM audit_log WHERE project_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		projectID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("listing audit entries: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	var total int
	for rows.Next() {
		var e model.AuditEntry
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.UserID, &e.UserEmail, &e.EnvironmentID, &e.BatchID, &e.Action, &e.EntityType, &e.EntityID, &e.OldValue, &e.NewValue, &e.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("scanning audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating audit entries: %w", err)
	}
	return entries, total, nil
}
```

Do the same for `ListByFlag` — add `COUNT(*) OVER() AS total` and return `([]model.AuditEntry, int, error)`.

**Step 2: Update audit handler to use shared helpers**

In `internal/handler/audit_handler.go`:

```go
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
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

	limit, offset := parsePagination(r)
	entries, total, err := h.audit.ListByProject(r.Context(), project.ID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit log")
		return
	}
	if entries == nil {
		entries = []model.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, PaginatedResponse{Data: entries, Total: total, Limit: limit, Offset: offset})
}
```

Do the same for `history_handler.go`.

**Step 3: Fix callers and tests**

Update `audit_store_test.go` — all calls return 3 values now.

**Step 4: Run tests**

Run: `go test ./internal/store/ -run TestAuditStore -v`
Run: `go test ./internal/handler/ -run TestAudit -v`
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/store/audit_store.go internal/store/audit_store_test.go internal/handler/audit_handler.go internal/handler/history_handler.go
git commit -m "refactor: migrate audit log and flag history to shared pagination helpers"
```

---

### Task 8: Frontend — Paginated API Types & Client Helper

**Files:**
- Modify: `web/src/api/types.ts`
- Modify: `web/src/api/client.ts`

**Step 1: Add PaginatedResponse type**

In `web/src/api/types.ts`, add:

```typescript
export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}
```

**Step 2: Update API client methods**

In `web/src/api/client.ts`, update `api.flags.list` to return paginated response and add pagination params:

```typescript
flags: {
  list: (projectKey: string, params?: { lifecycle_status?: string; flag_type?: string; limit?: number; offset?: number }) => {
    const search = new URLSearchParams()
    if (params?.lifecycle_status) search.set('lifecycle_status', params.lifecycle_status)
    if (params?.flag_type) search.set('flag_type', params.flag_type)
    if (params?.limit) search.set('limit', String(params.limit))
    if (params?.offset) search.set('offset', String(params.offset))
    const qs = search.toString()
    return request<PaginatedResponse<Flag>>(`/projects/${projectKey}/flags${qs ? `?${qs}` : ''}`)
  },
  // ... bulk stays the same
},
```

Update `api.segments.list`:

```typescript
segments: {
  list: (projectKey: string, params?: { limit?: number; offset?: number }) => {
    const search = new URLSearchParams()
    if (params?.limit) search.set('limit', String(params.limit))
    if (params?.offset) search.set('offset', String(params.offset))
    const qs = search.toString()
    return request<PaginatedResponse<Segment>>(`/projects/${projectKey}/segments${qs ? `?${qs}` : ''}`)
  },
  // ... get, create, update, delete stay the same
},
```

**Step 3: Commit**

```bash
git add web/src/api/types.ts web/src/api/client.ts
git commit -m "feat: add PaginatedResponse type and update API client for pagination"
```

---

### Task 9: Frontend — useInfiniteScroll Hook

**Files:**
- Create: `web/src/hooks/useInfiniteScroll.ts`

**Step 1: Create the hook**

```typescript
import { useEffect, useRef } from 'react'

export function useInfiniteScroll(options: {
  hasNextPage: boolean | undefined
  isFetchingNextPage: boolean
  fetchNextPage: () => void
}) {
  const sentinelRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const sentinel = sentinelRef.current
    if (!sentinel) return

    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && options.hasNextPage && !options.isFetchingNextPage) {
          options.fetchNextPage()
        }
      },
      { threshold: 0.1 }
    )

    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [options.hasNextPage, options.isFetchingNextPage, options.fetchNextPage])

  return sentinelRef
}
```

**Step 2: Commit**

```bash
git add web/src/hooks/useInfiniteScroll.ts
git commit -m "feat: add useInfiniteScroll hook with IntersectionObserver"
```

---

### Task 10: Frontend — Flags Page (Infinite Scroll)

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx`

**Step 1: Convert flags query to useInfiniteQuery**

Replace the `useQuery` for flags with `useInfiniteQuery`:

```typescript
import { useInfiniteQuery } from '@tanstack/react-query'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import type { PaginatedResponse } from '../api/types'

const PAGE_SIZE = 50

const {
  data: flagsData,
  isLoading: flagsLoading,
  error: flagsError,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
} = useInfiniteQuery({
  queryKey: ['projects', key, 'flags'],
  queryFn: ({ pageParam = 0 }) =>
    api.get<PaginatedResponse<Flag>>(
      `/projects/${key}/flags?limit=${PAGE_SIZE}&offset=${pageParam}`
    ),
  initialPageParam: 0,
  getNextPageParam: (lastPage) =>
    lastPage.offset + lastPage.limit < lastPage.total
      ? lastPage.offset + lastPage.limit
      : undefined,
  enabled: !!key,
})

const flags = flagsData?.pages.flatMap((page) => page.data)
```

Add the sentinel ref at the bottom of the flags list:

```tsx
const scrollRef = useInfiniteScroll({ hasNextPage, isFetchingNextPage, fetchNextPage })

// At the bottom of the flags grid, after the map:
<div ref={scrollRef} className="h-1" />
{isFetchingNextPage && (
  <div className="text-center py-4 text-muted-foreground/60 text-[13px] animate-pulse">
    Loading more flags...
  </div>
)}
```

**Step 2: Update dependent queries**

The `allConfigs` query depends on `flags` — update it to use the flattened `flags` array instead of `flagsData`.

**Step 3: Verify filters still work**

The `filtered` useMemo already filters the `flags` array client-side. Since `flags` now accumulates across pages, client-side filtering still works. When server-side filters are added (lifecycle_status, flag_type), include them in the queryKey to reset pagination.

**Step 4: Run frontend dev server and test**

Run: `cd web && npm run dev`
Verify: flags page loads, scrolling loads more, filters work.

**Step 5: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx
git commit -m "feat: convert flags page to infinite scroll with pagination"
```

---

### Task 11: Frontend — Projects Page (Infinite Scroll)

**Files:**
- Modify: `web/src/pages/ProjectsPage.tsx`

**Step 1: Convert to useInfiniteQuery**

```typescript
import { useInfiniteQuery } from '@tanstack/react-query'
import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import type { PaginatedResponse } from '../api/types'

const PAGE_SIZE = 50

const {
  data: projectsData,
  isLoading,
  error,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
} = useInfiniteQuery({
  queryKey: ['projects'],
  queryFn: ({ pageParam = 0 }) =>
    api.get<PaginatedResponse<Project>>(`/projects?limit=${PAGE_SIZE}&offset=${pageParam}`),
  initialPageParam: 0,
  getNextPageParam: (lastPage) =>
    lastPage.offset + lastPage.limit < lastPage.total
      ? lastPage.offset + lastPage.limit
      : undefined,
})

const projects = projectsData?.pages.flatMap((page) => page.data)

const scrollRef = useInfiniteScroll({ hasNextPage, isFetchingNextPage, fetchNextPage })
```

Add sentinel after the projects grid:

```tsx
<div ref={scrollRef} className="h-1" />
{isFetchingNextPage && (
  <div className="text-center py-4 text-muted-foreground/60 text-[13px] animate-pulse">
    Loading more projects...
  </div>
)}
```

**Step 2: Commit**

```bash
git add web/src/pages/ProjectsPage.tsx
git commit -m "feat: convert projects page to infinite scroll with pagination"
```

---

### Task 12: Frontend — Team Page (Load More)

**Files:**
- Modify: `web/src/pages/TeamPage.tsx`

**Step 1: Convert users query to useInfiniteQuery**

```typescript
const PAGE_SIZE = 50

const {
  data: membersData,
  isLoading: membersLoading,
  hasNextPage: membersHasNextPage,
  isFetchingNextPage: membersFetchingNext,
  fetchNextPage: fetchNextMembers,
} = useInfiniteQuery({
  queryKey: ['users'],
  queryFn: ({ pageParam = 0 }) =>
    api.get<PaginatedResponse<SafeUser>>(`/management/users?limit=${PAGE_SIZE}&offset=${pageParam}`),
  initialPageParam: 0,
  getNextPageParam: (lastPage) =>
    lastPage.offset + lastPage.limit < lastPage.total
      ? lastPage.offset + lastPage.limit
      : undefined,
})

const members = membersData?.pages.flatMap((page) => page.data)
```

Add Load More button after the members list:

```tsx
{membersHasNextPage && (
  <div className="text-center mt-4">
    <Button variant="outline" onClick={() => fetchNextMembers()} disabled={membersFetchingNext}>
      {membersFetchingNext ? 'Loading...' : 'Load More'}
    </Button>
  </div>
)}
```

**Step 2: Commit**

```bash
git add web/src/pages/TeamPage.tsx
git commit -m "feat: convert team page to load more pagination"
```

---

### Task 13: Frontend — Segments Page (Load More)

**Files:**
- Modify: `web/src/pages/SegmentsPage.tsx`

**Step 1: Convert to useInfiniteQuery**

Replace the `useQuery` with:

```typescript
const PAGE_SIZE = 50

const {
  data: segmentsData,
  isLoading,
  error,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
} = useInfiniteQuery({
  queryKey: ['segments', key],
  queryFn: ({ pageParam = 0 }) =>
    api.get<PaginatedResponse<Segment>>(`/projects/${key}/segments?limit=${PAGE_SIZE}&offset=${pageParam}`),
  initialPageParam: 0,
  getNextPageParam: (lastPage) =>
    lastPage.offset + lastPage.limit < lastPage.total
      ? lastPage.offset + lastPage.limit
      : undefined,
  enabled: !!key,
})

const segments = segmentsData?.pages.flatMap((page) => page.data)
```

Add Load More button after segments list:

```tsx
{hasNextPage && (
  <div className="text-center mt-4">
    <Button variant="outline" onClick={() => fetchNextPage()} disabled={isFetchingNextPage}>
      {isFetchingNextPage ? 'Loading...' : 'Load More'}
    </Button>
  </div>
)}
```

**Step 2: Commit**

```bash
git add web/src/pages/SegmentsPage.tsx
git commit -m "feat: convert segments page to load more pagination"
```

---

### Task 14: Frontend — Unknown Flags & Audit Log Migration

**Files:**
- Modify: `web/src/pages/ProjectDetailPage.tsx` (unknown flags query)
- Modify: `web/src/pages/AuditLogPage.tsx`
- Modify: `web/src/components/FlagHistory.tsx`

**Step 1: Update unknown flags query in ProjectDetailPage**

The unknown flags query in `ProjectDetailPage.tsx` (around line 95-99) needs to handle the new response shape:

```typescript
const { data: unknownFlagsData } = useQuery({
  queryKey: ['projects', key, 'unknown-flags'],
  queryFn: () => api.get<PaginatedResponse<UnknownFlag>>(`/projects/${key}/unknown-flags?limit=50&offset=0`),
  enabled: !!key && unknownFlagsEnabled,
})
const unknownFlags = unknownFlagsData?.data
```

Note: Unknown flags on this page is just a summary/count display — full pagination for this endpoint is lower priority. If a dedicated unknown flags page exists, convert that to Load More.

**Step 2: Migrate AuditLogPage to use PaginatedResponse**

In `web/src/pages/AuditLogPage.tsx`, update the fetch to use `useInfiniteQuery` with the new response format:

```typescript
const PAGE_SIZE = 50

const {
  data: auditData,
  isLoading,
  error,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
} = useInfiniteQuery({
  queryKey: ['projects', key, 'audit-log'],
  queryFn: ({ pageParam = 0 }) =>
    api.get<PaginatedResponse<AuditEntry>>(
      `/projects/${key}/audit-log?limit=${PAGE_SIZE}&offset=${pageParam}`
    ),
  initialPageParam: 0,
  getNextPageParam: (lastPage) =>
    lastPage.offset + lastPage.limit < lastPage.total
      ? lastPage.offset + lastPage.limit
      : undefined,
  enabled: !!key,
})

const allEntries = auditData?.pages.flatMap((page) => page.data) ?? []
```

Remove the manual `useState` for `offset`, `allEntries`, `hasMore`. Replace the Load More button:

```tsx
{hasNextPage && (
  <div className="text-center mt-4">
    <Button variant="outline" onClick={() => fetchNextPage()} disabled={isFetchingNextPage}>
      {isFetchingNextPage ? 'Loading...' : 'Load More'}
    </Button>
  </div>
)}
```

**Step 3: Migrate FlagHistory similarly**

Same pattern: replace manual state with `useInfiniteQuery`, use `PaginatedResponse<AuditEntry>`.

**Step 4: Run lint**

Run: `cd web && npm run lint`
Expected: no errors

**Step 5: Commit**

```bash
git add web/src/pages/ProjectDetailPage.tsx web/src/pages/AuditLogPage.tsx web/src/components/FlagHistory.tsx
git commit -m "feat: migrate unknown flags, audit log, and flag history to PaginatedResponse"
```

---

### Task 15: Final Verification

**Step 1: Run all Go tests**

Run: `go test ./...`
Expected: all PASS

**Step 2: Run frontend lint**

Run: `cd web && npm run lint`
Expected: no errors

**Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: build succeeds

**Step 4: Build Go binary**

Run: `go build -o togglerino ./cmd/togglerino`
Expected: build succeeds

**Step 5: Manual smoke test**

Run: `./dev.sh` and `cd web && npm run dev`

Verify:
- Flags page: scrolling loads more flags
- Projects page: scrolling loads more projects
- Team page: "Load More" button appears when >50 users
- Segments page: "Load More" button works
- Audit log: "Load More" still works with new response format
- Filters on flags page reset pagination correctly

**Step 6: Final commit (if any fixups needed)**

```bash
git add -A
git commit -m "fix: address pagination integration issues"
```
