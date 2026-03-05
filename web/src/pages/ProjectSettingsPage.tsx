import { useMemo } from 'react'
import { NavLink, Outlet, useParams, Link } from 'react-router-dom'
import { cn } from '@/lib/utils'
import { useIsProjectAdmin } from '@/hooks/usePermissions'

const allSettingsTabs = [
  { to: 'general', label: 'General', adminOnly: true },
  { to: 'lifetimes', label: 'Flag Lifetimes', adminOnly: true },
  { to: 'environments', label: 'Environments', adminOnly: true },
  { to: 'members', label: 'Members', adminOnly: false },
]

export default function ProjectSettingsPage() {
  const { key } = useParams<{ key: string }>()
  const isProjectAdmin = useIsProjectAdmin(key)

  const settingsTabs = useMemo(() =>
    allSettingsTabs.filter((tab) => !tab.adminOnly || isProjectAdmin),
    [isProjectAdmin],
  )

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
