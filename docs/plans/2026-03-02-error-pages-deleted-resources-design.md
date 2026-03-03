# Error Pages for Deleted Resources

Issue: #65

## Problem

When navigating to a direct URL for a deleted resource (project, flag, environment, segment), the app shows no proper error page. TanStack Query retries the failed 404 request 3 times (default), the console gets spammed with errors, and the user sees either an indefinite loading state or a generic "Failed to load" error alert.

## Design

### Approach: API client error enrichment + global retry config + NotFoundState component

### 1. Custom `ApiError` class (`web/src/api/client.ts`)

Add an `ApiError` class that preserves the HTTP status code. The existing `request()` function throws this instead of a plain `Error`.

### 2. TanStack Query retry config (`web/src/App.tsx`)

Configure `QueryClient` with a global `retry` function that returns `false` for 4xx errors (client errors should never be retried) and preserves the default 3-retry behavior for 5xx/network errors.

### 3. `NotFoundState` component (`web/src/components/NotFoundState.tsx`)

Reusable inline component displayed in the content area (sidebar remains visible). Props: `title`, `description`, `backTo`, `backLabel`. Styled to match existing empty-state patterns.

### 4. Affected pages

Each page that loads a resource by key adds a 404-specific check before the generic error alert:

| Page | Resource | Back link |
|------|----------|-----------|
| `ProjectLayout` | Project | `/projects` |
| `FlagDetailPage` | Flag | `/projects/:key` |
| `SDKKeysPage` | Environment | `/projects/:key/environments` |

`ProjectLayout` is the parent layout — a 404 on the project replaces `<Outlet />` entirely, preventing broken child routes.

### 5. Out of scope

- Full-page 404 for unknown routes (current redirect to `/projects` is fine)
- React Error Boundary
- 403/500 specific pages
- Backend changes
