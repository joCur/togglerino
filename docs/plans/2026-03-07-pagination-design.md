# Pagination for Management API Endpoints

## Problem

Most list endpoints return unbounded result sets. As organizations grow (hundreds of flags, users, projects), this causes performance issues and poor UX. Only audit log and flag history currently support pagination.

## Decision

Add offset-based pagination (`limit`/`offset`) to endpoints with unbounded growth potential. Keep naturally bounded endpoints (environments, SDK keys, members, invites, templates, schedules) as-is.

## API Contract

All paginated endpoints accept:

| Parameter | Type | Default | Constraints |
|-----------|------|---------|-------------|
| `limit`   | int  | 50      | 1-100       |
| `offset`  | int  | 0       | >= 0        |

All paginated endpoints return:

```json
{
  "data": [...],
  "total": 142,
  "limit": 50,
  "offset": 0
}
```

`total` is the filtered count (respects search/tag/status filters). Frontend uses `offset + limit < total` to determine if more pages exist.

## Endpoints to Paginate

| Endpoint | Frontend Pattern |
|----------|-----------------|
| `GET /api/v1/projects` | Infinite scroll |
| `GET /api/v1/projects/{key}/flags` | Infinite scroll |
| `GET /api/v1/management/users` | Load More button |
| `GET /api/v1/projects/{key}/segments` | Load More button |
| `GET /api/v1/projects/{key}/unknown-flags` | Load More button |

Existing paginated endpoints (audit log, flag history) will be migrated to the shared `PaginatedResponse` wrapper for consistency.

## Endpoints NOT Paginated (Naturally Bounded)

- Environments per project (typically 3-5)
- SDK keys per environment
- Project members (bounded by org users)
- User project assignments (bounded by project count)
- Pending invites (transient)
- Context attributes (already capped for search)
- Schedules per flag/environment
- Templates (global + project)
- OIDC identities

## Backend Implementation

### Shared helpers (`internal/handler/pagination.go`)

```go
func parsePagination(r *http.Request) (limit, offset int)
```

Extracts `limit`/`offset` from query params, applies defaults (50, 0) and clamps limit to 1-100. Replaces duplicated parsing in audit and history handlers.

```go
type PaginatedResponse struct {
    Data   any `json:"data"`
    Total  int `json:"total"`
    Limit  int `json:"limit"`
    Offset int `json:"offset"`
}
```

### Store layer

Each affected store method gains `limit, offset int` parameters and returns `([]T, int, error)` where `int` is the total count.

SQL pattern using window function:

```sql
SELECT *, COUNT(*) OVER() AS total
FROM flags
WHERE project_key = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3
```

`COUNT(*) OVER()` provides the total filtered count without a second query.

## Frontend Implementation

### API client

Generic paginated fetch helper returning `PaginatedResponse<T>`.

### TanStack Query

Use `useInfiniteQuery` for all paginated endpoints:
- `getNextPageParam` derives next offset from `offset + limit < total`
- Pages accumulate via `flatMap(page => page.data)`

### Infinite scroll (flags, projects)

`IntersectionObserver` on a sentinel element near the list bottom triggers `fetchNextPage()`. Shared `useInfiniteScroll` hook to avoid duplication.

### Load More (users, segments, unknown flags)

"Load More" button shown when `hasNextPage` is true. Matches existing audit log / flag history pattern.

### Filter interaction

When filters change (search, tag, status), reset to offset 0 and clear accumulated pages. TanStack Query handles this when filter params are part of the query key.

## Migration & Rollout

No database migration needed. Changes are query-level (`LIMIT`/`OFFSET`) and response shape only.

Response shape change (`{"data": [...]}` wrapper) affects management API only. SDK client API (`POST /api/v1/evaluate`, `GET /api/v1/stream`) is unaffected.

### PR order

1. Extract shared `parsePagination` + `PaginatedResponse` helpers
2. Flags list (highest value)
3. Projects list
4. Users list
5. Segments list
6. Unknown flags list
7. Migrate audit log + flag history to shared helpers

Each PR updates backend + frontend together so the response shape change is atomic.

## Testing

- Go tests: verify pagination params flow through to SQL, verify `total` count correctness with filters applied
- Frontend: verify infinite scroll triggers, "Load More" works, filter reset clears pages
