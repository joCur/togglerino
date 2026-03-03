# Error Pages for Deleted Resources — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show user-friendly "Not Found" pages when navigating to deleted projects, flags, or environments via direct URL, and stop TanStack Query from retrying 404s.

**Architecture:** Enrich the API client to throw `ApiError` with HTTP status codes, configure TanStack Query to skip retries on 4xx errors, create a reusable `NotFoundState` component, and add 404 checks to `ProjectLayout`, `FlagDetailPage`, and `SDKKeysPage`.

**Tech Stack:** React 19, TypeScript, TanStack Query v5, React Router v7, shadcn/ui, Tailwind CSS v4

---

### Task 1: Add `ApiError` class to API client

**Files:**
- Modify: `web/src/api/client.ts`

**Step 1: Add the `ApiError` class above the `request` function**

Add this at the top of `web/src/api/client.ts`, after the imports and before the `API_BASE` constant:

```typescript
export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}
```

**Step 2: Update the `request` function to throw `ApiError`**

Replace line 16:
```typescript
    throw new Error(error.error || res.statusText)
```
with:
```typescript
    throw new ApiError(res.status, error.error || res.statusText)
```

**Step 3: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat(web): add ApiError class with HTTP status code"
```

---

### Task 2: Configure TanStack Query to not retry on 4xx

**Files:**
- Modify: `web/src/App.tsx`

**Step 1: Add `ApiError` import**

Add to the imports at the top of `App.tsx`:
```typescript
import { ApiError } from './api/client.ts'
```

**Step 2: Replace the `QueryClient` instantiation**

Replace:
```typescript
const queryClient = new QueryClient()
```
with:
```typescript
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
          return false
        }
        return failureCount < 3
      },
    },
  },
})
```

**Step 3: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/App.tsx
git commit -m "feat(web): disable TanStack Query retries on 4xx errors"
```

---

### Task 3: Create `NotFoundState` component

**Files:**
- Create: `web/src/components/NotFoundState.tsx`

**Step 1: Create the component**

Create `web/src/components/NotFoundState.tsx`:

```tsx
import { Link } from 'react-router-dom'

interface NotFoundStateProps {
  title: string
  description: string
  backTo: string
  backLabel: string
}

export default function NotFoundState({ title, description, backTo, backLabel }: NotFoundStateProps) {
  return (
    <div className="text-center py-16 animate-[fadeIn_300ms_ease]">
      <div className="text-[15px] font-medium text-foreground mb-1.5">{title}</div>
      <div className="text-[13px] text-muted-foreground/60 mb-6">{description}</div>
      <Link
        to={backTo}
        className="text-[13px] text-[#d4956a] hover:text-[#e0a87a] transition-colors"
      >
        &larr; Back to {backLabel}
      </Link>
    </div>
  )
}
```

**Step 2: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add web/src/components/NotFoundState.tsx
git commit -m "feat(web): add reusable NotFoundState component"
```

---

### Task 4: Add project 404 handling to `ProjectLayout`

**Files:**
- Modify: `web/src/components/ProjectLayout.tsx`

The layout currently does not fetch any project data — it just reads `key` from params and renders navigation. We need to add a lightweight project existence check so that all child routes under `/projects/:key/*` see a "not found" state when the project doesn't exist, rather than each child failing independently.

**Step 1: Add imports**

Add these imports to the top of `ProjectLayout.tsx`:

```typescript
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import { ApiError } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import NotFoundState from './NotFoundState.tsx'
```

**Step 2: Add project query inside the component**

Inside the `ProjectLayout` function, after the existing hooks (`useParams`, `useIsMobile`, `useState`, `useAuth`, `useNavigate`, `useFlag`), add:

```typescript
const { error: projectError, isLoading: projectLoading } = useQuery({
  queryKey: ['projects', key],
  queryFn: () => api.get<Project>(`/projects/${key}`),
  enabled: !!key,
})
```

**Step 3: Add loading and 404 checks before the return**

After the `closeDrawer` line and before the `navLinks` function, add:

```typescript
if (projectLoading) {
  return (
    <div className="flex items-center justify-center h-screen text-muted-foreground/60 text-[13px] animate-pulse">
      Loading project...
    </div>
  )
}

if (projectError instanceof ApiError && projectError.status === 404) {
  return (
    <div className="flex flex-col min-h-screen">
      <Topbar onMenuClick={() => setDrawerOpen(true)} />
      <div className="flex flex-1">
        <main className="flex-1 p-4 md:p-9">
          <NotFoundState
            title="Project not found"
            description={`The project "${key}" could not be found. It may have been deleted.`}
            backTo="/projects"
            backLabel="Projects"
          />
        </main>
      </div>
    </div>
  )
}
```

**Step 4: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 5: Commit**

```bash
git add web/src/components/ProjectLayout.tsx
git commit -m "feat(web): show not-found page for deleted projects in ProjectLayout"
```

---

### Task 5: Add flag 404 handling to `FlagDetailPage`

**Files:**
- Modify: `web/src/pages/FlagDetailPage.tsx`

**Step 1: Add imports**

Add to the imports of `FlagDetailPage.tsx`:

```typescript
import { ApiError } from '../api/client.ts'
import NotFoundState from '../components/NotFoundState.tsx'
```

**Step 2: Add 404 check before the generic error handler**

In `FlagDetailPage.tsx`, the current error handling is at lines 149-157. Insert a 404-specific check BEFORE the existing `if (error || !data)` block (i.e., between the loading check and the generic error check):

```typescript
if (error instanceof ApiError && error.status === 404) {
  return (
    <NotFoundState
      title="Flag not found"
      description={`The flag "${flagKey}" could not be found. It may have been deleted.`}
      backTo={`/projects/${key}`}
      backLabel="Flags"
    />
  )
}
```

**Step 3: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/pages/FlagDetailPage.tsx
git commit -m "feat(web): show not-found page for deleted flags"
```

---

### Task 6: Add environment 404 handling to `SDKKeysPage`

**Files:**
- Modify: `web/src/pages/SDKKeysPage.tsx`

**Step 1: Add imports**

Add to the imports of `SDKKeysPage.tsx`:

```typescript
import { ApiError } from '../api/client.ts'
import NotFoundState from '../components/NotFoundState.tsx'
```

**Step 2: Add 404 check before the generic error handler**

In `SDKKeysPage.tsx`, the current error handling is at lines 83-91. Insert a 404-specific check BEFORE the existing `if (error)` block:

```typescript
if (error instanceof ApiError && error.status === 404) {
  return (
    <NotFoundState
      title="Environment not found"
      description={`The environment "${env}" could not be found. It may have been deleted.`}
      backTo={`/projects/${key}/environments`}
      backLabel="Environments"
    />
  )
}
```

**Step 3: Run lint to verify**

Run: `cd web && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add web/src/pages/SDKKeysPage.tsx
git commit -m "feat(web): show not-found page for deleted environments"
```

---

### Task 7: Final verification

**Step 1: Run full lint**

Run: `cd web && npm run lint`
Expected: No errors

**Step 2: Run TypeScript build check**

Run: `cd web && npx tsc -b --noEmit`
Expected: No type errors

**Step 3: Build the frontend**

Run: `cd web && npm run build`
Expected: Build succeeds
