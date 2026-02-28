import { useState } from 'react'
import { Outlet, NavLink } from 'react-router-dom'
import { useFlag } from '@togglerino/react'
import { useAuth } from '../hooks/useAuth.ts'
import { useIsMobile } from '../hooks/useIsMobile.ts'
import Topbar from './Topbar.tsx'
import { cn } from '@/lib/utils'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
    isActive
      ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
      : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
  )

function SidebarNav() {
  const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
  return (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Navigation
      </div>
      <NavLink to="/projects" end className={navLinkClass}>Projects</NavLink>
      <NavLink to="/settings/team" className={navLinkClass}>Team</NavLink>
      {isThemeToggleEnabled && (
        <NavLink to="/settings" end className={navLinkClass}>Settings</NavLink>
      )}
    </>
  )
}

export default function OrgLayout() {
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const { user, logout } = useAuth()

  return (
    <div className="flex flex-col min-h-screen">
      <Topbar onMenuClick={() => setDrawerOpen(true)} />

      <div className="flex flex-1">
        {/* Desktop sidebar */}
        {!isMobile && (
          <nav className="w-[200px] bg-card border-r py-5 shrink-0 flex flex-col">
            <SidebarNav />
          </nav>
        )}

        {/* Mobile drawer */}
        {isMobile && (
          <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
            <SheetContent side="left" className="w-[260px] p-0 flex flex-col">
              <nav className="py-5 flex-1 flex flex-col" onClick={() => setDrawerOpen(false)}>
                <SidebarNav />
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

        {/* Main content */}
        <main className="flex-1 p-4 md:p-9 overflow-y-auto animate-[fadeIn_300ms_ease]">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
