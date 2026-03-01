import { Outlet, NavLink, useParams } from 'react-router-dom'
import Topbar from './Topbar.tsx'
import ProjectSwitcher from './ProjectSwitcher.tsx'
import { cn } from '@/lib/utils'

export default function ProjectLayout() {
  const { key } = useParams<{ key: string }>()

  return (
    <div className="flex flex-col min-h-screen">
      <Topbar>
        <ProjectSwitcher />
      </Topbar>

      <div className="flex flex-1">
        {/* Sidebar */}
        <nav className="w-[200px] bg-card border-r py-5 shrink-0 flex flex-col">
          <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
            Project
          </div>
          <NavLink
            to={`/projects/${key}`}
            end
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
                isActive
                  ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
                  : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
              )
            }
          >
            Flags
          </NavLink>
          <NavLink
            to={`/projects/${key}/lifecycle`}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
                isActive
                  ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
                  : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
              )
            }
          >
            Lifecycle
          </NavLink>
          <NavLink
            to={`/projects/${key}/environments`}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
                isActive
                  ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
                  : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
              )
            }
          >
            Environments
          </NavLink>
          <NavLink
            to={`/projects/${key}/audit-log`}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
                isActive
                  ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
                  : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
              )
            }
          >
            Audit Log
          </NavLink>
          <NavLink
            to={`/projects/${key}/settings`}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
                isActive
                  ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
                  : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
              )
            }
          >
            Settings
          </NavLink>
        </nav>

        {/* Main content */}
        <main className="flex-1 p-9 overflow-y-auto animate-[fadeIn_300ms_ease]">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
