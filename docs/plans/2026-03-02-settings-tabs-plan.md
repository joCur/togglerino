# Settings Page Tabbed Navigation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rework the project settings page from a single vertical page into tabbed sub-routes so each settings section has its own URL.

**Architecture:** Extract the three existing sub-components + danger zone from `ProjectSettingsPage.tsx` into separate tab files under `web/src/pages/settings/`. Convert the settings page into a layout component with breadcrumbs, header, route-driven tab bar, and `<Outlet />`. Update `App.tsx` with nested child routes.

**Tech Stack:** React 19, React Router v7 (NavLink, Outlet, Navigate), TypeScript, Tailwind CSS v4, shadcn/ui Card components.

---

### Task 1: Extract GeneralSettingsTab

**Files:**
- Create: `web/src/pages/settings/GeneralSettingsTab.tsx`
- Reference: `web/src/pages/ProjectSettingsPage.tsx:130-201` (GeneralSettings) and `380-434` (DangerZone)

**Step 1: Create `GeneralSettingsTab.tsx`**

This file combines the existing `GeneralSettings` component and the Danger Zone section. Copy them from `ProjectSettingsPage.tsx` with one change: the component reads `key` from `useParams` instead of receiving it as a prop.

```tsx
import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import type { Project } from '../../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'

export default function GeneralSettingsTab() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: project, isLoading, error } = useQuery({
    queryKey: ['projects', key],
    queryFn: () => api.get<Project>(`/projects/${key}`),
    enabled: !!key,
  })

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [initialized, setInitialized] = useState(false)

  if (project && !initialized) {
    setName(project.name)
    setDescription(project.description)
    setInitialized(true)
  }

  const hasChanges = project ? (name !== project.name || description !== project.description) : false

  const updateMutation = useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.put<Project>(`/projects/${key}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleSave = () => {
    if (!hasChanges) return
    updateMutation.mutate({ name: name.trim(), description: description.trim() })
  }

  // Danger Zone state
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('')

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${key}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate('/projects')
    },
  })

  const handleDelete = () => {
    if (deleteConfirmInput !== key) return
    deleteMutation.mutate()
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load project: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <>
      {/* General */}
      <Card className="mb-6">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-foreground mb-4">
            General
          </div>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
              <Textarea className="min-h-[80px]" value={description} onChange={(e) => setDescription(e.target.value)} />
            </div>
            <div className="flex items-center gap-3">
              <Button onClick={handleSave} disabled={!hasChanges || updateMutation.isPending}>
                {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
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
          </div>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border-destructive/25">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-destructive mb-3">
            Danger Zone
          </div>
          <div className="text-[13px] text-muted-foreground leading-relaxed mb-4">
            Deleting this project is permanent and cannot be undone. All flags, environments, and SDK keys associated with this project will be removed.
          </div>
          {!showDeleteConfirm ? (
            <Button
              variant="outline"
              className="border-destructive/50 text-destructive hover:bg-destructive/10"
              onClick={() => setShowDeleteConfirm(true)}
            >
              Delete Project
            </Button>
          ) : (
            <div className="flex flex-col gap-3 animate-[fadeIn_200ms_ease]">
              <div className="text-[13px] text-muted-foreground">
                Type <span className="font-mono text-destructive font-semibold">{key}</span> to confirm deletion:
              </div>
              <Input
                value={deleteConfirmInput}
                onChange={(e) => setDeleteConfirmInput(e.target.value)}
                placeholder={key}
                autoFocus
              />
              <div className="flex gap-3">
                <Button
                  variant="destructive"
                  disabled={deleteConfirmInput !== key || deleteMutation.isPending}
                  onClick={handleDelete}
                >
                  {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => { setShowDeleteConfirm(false); setDeleteConfirmInput('') }}
                >
                  Cancel
                </Button>
              </div>
              {deleteMutation.error && (
                <Alert variant="destructive">
                  <AlertDescription>
                    {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete project'}
                  </AlertDescription>
                </Alert>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </>
  )
}
```

**Step 2: Verify the file was created correctly**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/settings-tabs && npx tsc --noEmit 2>&1 | grep -i "GeneralSettingsTab" | head -5`

Note: There may be TypeScript errors until all files are wired up in Task 5. This is expected.

**Step 3: Commit**

```bash
git add web/src/pages/settings/GeneralSettingsTab.tsx
git commit -m "feat(web): extract GeneralSettingsTab with danger zone (#62)"
```

---

### Task 2: Extract FlagLifetimesTab

**Files:**
- Create: `web/src/pages/settings/FlagLifetimesTab.tsx`
- Reference: `web/src/pages/ProjectSettingsPage.tsx:14-128` (FLAG_PURPOSE_LABELS + FlagLifetimesSettings)

**Step 1: Create `FlagLifetimesTab.tsx`**

Copy the `FLAG_PURPOSE_LABELS` constant and `FlagLifetimesSettings` component. Change from receiving `projectKey` as prop to reading `key` from `useParams`.

```tsx
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'

const FLAG_PURPOSE_LABELS: Record<string, { label: string; description: string }> = {
  'release': { label: 'Release', description: 'Feature rollout flags' },
  'experiment': { label: 'Experiment', description: 'A/B testing flags' },
  'operational': { label: 'Operational', description: 'Technical migration flags' },
  'kill-switch': { label: 'Kill Switch', description: 'Graceful degradation flags' },
  'permission': { label: 'Permission', description: 'Access control flags' },
}

export default function FlagLifetimesTab() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [lifetimes, setLifetimes] = useState<Record<string, number | null>>({})
  const [initialized, setInitialized] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', key, 'settings', 'flags'],
    queryFn: () => api.get<{ flag_lifetimes: Record<string, number | null> }>(`/projects/${key}/settings/flags`),
  })

  if (data && !initialized) {
    setLifetimes(data.flag_lifetimes)
    setInitialized(true)
  }

  const updateMutation = useMutation({
    mutationFn: (flagLifetimes: Record<string, number | null>) =>
      api.put(`/projects/${key}/settings/flags`, { flag_lifetimes: flagLifetimes }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'flags'] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleSave = () => updateMutation.mutate(lifetimes)

  const handleChange = (purpose: string, value: string) => {
    if (value === '' || value === 'permanent') {
      setLifetimes(prev => ({ ...prev, [purpose]: null }))
    } else {
      const num = parseInt(value, 10)
      if (!isNaN(num) && num > 0) {
        setLifetimes(prev => ({ ...prev, [purpose]: num }))
      }
    }
  }

  if (isLoading) return null

  return (
    <Card>
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-1">
          Flag Lifetimes
        </div>
        <div className="text-xs text-muted-foreground mb-4">
          Expected lifetime per flag type. Flags exceeding their lifetime are marked as potentially stale.
        </div>

        <div className="flex flex-col gap-3">
          {Object.entries(FLAG_PURPOSE_LABELS).map(([purpose, { label, description }]) => (
            <div key={purpose} className="flex flex-col md:flex-row md:items-center gap-2 md:gap-4">
              <div className="md:w-[140px]">
                <div className="text-[13px] font-medium text-foreground">{label}</div>
                <div className="text-[11px] text-muted-foreground">{description}</div>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                {lifetimes[purpose] === null ? (
                  <Input className="w-full md:w-[120px]" value="Permanent" disabled />
                ) : (
                  <Input
                    className="w-full md:w-[120px]"
                    type="number"
                    min={1}
                    value={lifetimes[purpose] ?? ''}
                    onChange={(e) => handleChange(purpose, e.target.value)}
                  />
                )}
                <span className="text-xs text-muted-foreground">
                  {lifetimes[purpose] === null ? '' : 'days'}
                </span>
                <button
                  type="button"
                  className="text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                  onClick={() => handleChange(purpose, lifetimes[purpose] === null ? '40' : 'permanent')}
                >
                  {lifetimes[purpose] === null ? 'Set limit' : 'Make permanent'}
                </button>
              </div>
            </div>
          ))}
        </div>

        <div className="flex items-center gap-3 mt-4">
          <Button onClick={handleSave} disabled={updateMutation.isPending}>
            {updateMutation.isPending ? 'Saving...' : 'Save Lifetimes'}
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

Note: the outermost `<Card>` no longer has `className="mb-6"` since spacing between tabs is handled by the layout.

**Step 2: Commit**

```bash
git add web/src/pages/settings/FlagLifetimesTab.tsx
git commit -m "feat(web): extract FlagLifetimesTab (#62)"
```

---

### Task 3: Extract EnvironmentDefaultsTab

**Files:**
- Create: `web/src/pages/settings/EnvironmentDefaultsTab.tsx`
- Reference: `web/src/pages/ProjectSettingsPage.tsx:203-288` (EnvironmentDefaultsSettings)

**Step 1: Create `EnvironmentDefaultsTab.tsx`**

Same pattern: copy `EnvironmentDefaultsSettings`, use `useParams` for `key`.

```tsx
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../api/client.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Switch } from '@/components/ui/switch'

export default function EnvironmentDefaultsTab() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [saveSuccess, setSaveSuccess] = useState(false)
  const [defaults, setDefaults] = useState<{ key: string; name: string; enabled: boolean }[]>([])
  const [initialized, setInitialized] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['projects', key, 'settings', 'environments'],
    queryFn: () => api.get<{ environment_defaults: { key: string; name: string; enabled: boolean }[] }>(
      `/projects/${key}/settings/environments`
    ),
  })

  if (data && !initialized) {
    setDefaults(data.environment_defaults)
    setInitialized(true)
  }

  const updateMutation = useMutation({
    mutationFn: (envDefaults: Record<string, { enabled: boolean }>) =>
      api.put(`/projects/${key}/settings/environments`, { environment_defaults: envDefaults }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'settings', 'environments'] })
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
    <Card>
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

**Step 2: Commit**

```bash
git add web/src/pages/settings/EnvironmentDefaultsTab.tsx
git commit -m "feat(web): extract EnvironmentDefaultsTab (#62)"
```

---

### Task 4: Create MembersTab

**Files:**
- Create: `web/src/pages/settings/MembersTab.tsx`
- Reference: `web/src/pages/ProjectSettingsPage.tsx:368-378` (Members placeholder)

**Step 1: Create `MembersTab.tsx`**

```tsx
import { Card, CardContent } from '@/components/ui/card'

export default function MembersTab() {
  return (
    <Card>
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-3">
          Members
        </div>
        <div className="text-[13px] text-muted-foreground/60">
          Project-level member management coming soon.
        </div>
      </CardContent>
    </Card>
  )
}
```

**Step 2: Commit**

```bash
git add web/src/pages/settings/MembersTab.tsx
git commit -m "feat(web): add MembersTab placeholder (#62)"
```

---

### Task 5: Rewrite ProjectSettingsPage as layout + update App.tsx routing

This is the core task that wires everything together.

**Files:**
- Modify: `web/src/pages/ProjectSettingsPage.tsx` (full rewrite — becomes layout)
- Modify: `web/src/App.tsx:17,80` (import + route)

**Step 1: Rewrite `ProjectSettingsPage.tsx` as a layout component**

Replace the entire file with a layout that renders breadcrumbs, header, tab bar (NavLink styled like line-variant tabs), and `<Outlet />`.

The tab bar CSS is extracted from the existing `tabs.tsx` component's `line` variant:
- Container: same classes as `TabsList variant="line"` — `inline-flex items-center justify-center gap-1 bg-transparent text-muted-foreground`
- Links: same classes as `TabsTrigger` in line mode — the key active styles are the underline (`after:` pseudo-element) and `text-foreground`

```tsx
import { NavLink, Outlet, useParams, Link } from 'react-router-dom'
import { cn } from '@/lib/utils'

const settingsTabs = [
  { to: 'general', label: 'General' },
  { to: 'lifetimes', label: 'Flag Lifetimes' },
  { to: 'environments', label: 'Environments' },
  { to: 'members', label: 'Members' },
]

export default function ProjectSettingsPage() {
  const { key } = useParams<{ key: string }>()

  return (
    <div className="animate-[fadeIn_300ms_ease] max-w-[640px]">
      {/* Breadcrumbs */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Settings</span>
      </div>

      {/* Header */}
      <div className="mb-8">
        <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
          Project Settings
        </h1>
        <div className="text-[13px] text-muted-foreground/60">
          Manage settings for <span className="font-mono text-muted-foreground">{key}</span>
        </div>
      </div>

      {/* Tab bar — styled to match TabsList variant="line" / TabsTrigger */}
      <div className="inline-flex items-center justify-center gap-1 bg-transparent text-muted-foreground h-9 mb-6">
        {settingsTabs.map(({ to, label }) => (
          <NavLink
            key={to}
            to={to}
            className={({ isActive }) =>
              cn(
                'relative inline-flex h-[calc(100%-1px)] items-center justify-center gap-1.5 rounded-md border border-transparent px-2 py-1 text-sm font-medium whitespace-nowrap transition-all',
                'text-foreground/60 hover:text-foreground',
                'after:bg-foreground after:absolute after:inset-x-0 after:bottom-[-5px] after:h-0.5 after:opacity-0 after:transition-opacity',
                isActive && 'text-foreground after:opacity-100'
              )
            }
          >
            {label}
          </NavLink>
        ))}
      </div>

      {/* Active tab content */}
      <Outlet />
    </div>
  )
}
```

**Step 2: Update `App.tsx` routing**

Add imports for the four tab components and change the settings route to a parent with children.

In imports (after existing imports, around line 23):
```tsx
import GeneralSettingsTab from './pages/settings/GeneralSettingsTab.tsx'
import FlagLifetimesTab from './pages/settings/FlagLifetimesTab.tsx'
import EnvironmentDefaultsTab from './pages/settings/EnvironmentDefaultsTab.tsx'
import MembersTab from './pages/settings/MembersTab.tsx'
```

Replace line 80:
```tsx
<Route path="settings" element={<ProjectSettingsPage />} />
```

With:
```tsx
<Route path="settings" element={<ProjectSettingsPage />}>
  <Route index element={<Navigate to="general" replace />} />
  <Route path="general" element={<GeneralSettingsTab />} />
  <Route path="lifetimes" element={<FlagLifetimesTab />} />
  <Route path="environments" element={<EnvironmentDefaultsTab />} />
  <Route path="members" element={<MembersTab />} />
</Route>
```

**Step 3: Verify TypeScript compiles**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/settings-tabs/web && npx tsc --noEmit`
Expected: No errors.

**Step 4: Run lint**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/settings-tabs/web && npm run lint`
Expected: No errors.

**Step 5: Commit**

```bash
git add web/src/pages/ProjectSettingsPage.tsx web/src/App.tsx
git commit -m "feat(web): tabbed settings navigation with sub-routes (#62)"
```

---

### Task 6: Visual verification

**Step 1: Build the frontend**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/settings-tabs/web && npm run build`
Expected: Build succeeds with no errors.

**Step 2: Start the dev server and verify**

Run: `cd /Users/jonascurth/Documents/git/togglerino/.worktrees/settings-tabs/web && npm run dev`

Verify in browser:
- `/projects/<key>/settings` redirects to `/settings/general`
- General tab shows name, description, and danger zone
- Flag Lifetimes tab shows lifetime config per flag type
- Environments tab shows switches per environment
- Members tab shows "coming soon" placeholder
- Tab underline indicator follows the active route
- Browser back/forward navigates between tabs
- Direct URL access (e.g. `/settings/lifetimes`) loads the correct tab
- Sidebar "Settings" link still highlights correctly
- Mobile responsive layout works

**Step 3: Stop dev server when done**
