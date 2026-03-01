import { useState } from 'react'
import { Outlet, NavLink, useParams } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { useIsMobile } from '../hooks/useIsMobile.ts'
import Topbar from './Topbar.tsx'
import ProjectSwitcher from './ProjectSwitcher.tsx'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { navLinkClass } from './navLinkClass'

export default function ProjectLayout() {
  const { key } = useParams<{ key: string }>()
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const { user, logout } = useAuth()
  const closeDrawer = () => setDrawerOpen(false)

  const navLinks = (onNavigate?: () => void) => (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Project
      </div>
      <NavLink to={`/projects/${key}`} end className={navLinkClass} onClick={onNavigate}>Flags</NavLink>
      <NavLink to={`/projects/${key}/lifecycle`} className={navLinkClass} onClick={onNavigate}>Lifecycle</NavLink>
      <NavLink to={`/projects/${key}/environments`} className={navLinkClass} onClick={onNavigate}>Environments</NavLink>
      <NavLink to={`/projects/${key}/audit-log`} className={navLinkClass} onClick={onNavigate}>Audit Log</NavLink>
      <NavLink to={`/projects/${key}/settings`} className={navLinkClass} onClick={onNavigate}>Settings</NavLink>
    </>
  )

  return (
    <div className="flex flex-col min-h-screen">
      <Topbar onMenuClick={() => setDrawerOpen(true)}>
        {!isMobile && <ProjectSwitcher />}
      </Topbar>

      <div className="flex flex-1">
        {!isMobile && (
          <nav className="w-[200px] bg-card border-r py-5 shrink-0 flex flex-col">
            {navLinks()}
          </nav>
        )}

        {isMobile && (
          <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
            <SheetContent side="left" className="w-[260px] p-0 flex flex-col" aria-label="Navigation menu">
              <div className="p-4 border-b">
                <ProjectSwitcher />
              </div>
              <nav className="py-5 flex-1 flex flex-col">
                {navLinks(closeDrawer)}
              </nav>
              <div className="border-t p-4 flex flex-col gap-2">
                <div className="text-xs text-muted-foreground truncate">{user?.email}</div>
                <Button variant="outline" size="sm" className="w-full" onClick={() => { logout(); setDrawerOpen(false) }}>
                  Log out
                </Button>
              </div>
            </SheetContent>
          </Sheet>
        )}

        <main className="flex-1 p-4 md:p-9 overflow-y-auto animate-[fadeIn_300ms_ease]">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
