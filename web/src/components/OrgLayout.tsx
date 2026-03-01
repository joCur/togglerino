import { useState } from 'react'
import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { useFlag } from '@togglerino/react'
import { useAuth } from '../hooks/useAuth.ts'
import { useIsMobile } from '../hooks/useIsMobile.ts'
import Topbar from './Topbar.tsx'
import { Sheet, SheetContent } from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { navLinkClass } from './navLinkClass'
import { User as UserIcon, LogOut } from 'lucide-react'

function SidebarNav({ onNavigate }: { onNavigate?: () => void }) {
  const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
  return (
    <>
      <div className="px-5 pb-2.5 text-[10px] font-medium text-muted-foreground/60 uppercase tracking-[1.2px] font-mono">
        Navigation
      </div>
      <NavLink to="/projects" end className={navLinkClass} onClick={onNavigate}>Projects</NavLink>
      <NavLink to="/settings/team" className={navLinkClass} onClick={onNavigate}>Team</NavLink>
      {isThemeToggleEnabled && (
        <NavLink to="/settings" end className={navLinkClass} onClick={onNavigate}>Settings</NavLink>
      )}
    </>
  )
}

export default function OrgLayout() {
  const isMobile = useIsMobile()
  const [drawerOpen, setDrawerOpen] = useState(false)
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const closeDrawer = () => setDrawerOpen(false)

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
            <SheetContent side="left" className="w-[260px] p-0 flex flex-col" aria-label="Navigation menu">
              <nav className="py-5 flex-1 flex flex-col">
                <SidebarNav onNavigate={closeDrawer} />
              </nav>
              <div className="border-t p-4 flex flex-col gap-2">
                <div className="text-xs text-muted-foreground truncate">{user?.display_name || user?.email}</div>
                <Button variant="outline" size="sm" className="w-full justify-start gap-2" onClick={() => { navigate('/account'); setDrawerOpen(false) }}>
                  <UserIcon className="h-4 w-4" />
                  Account
                </Button>
                <Button variant="outline" size="sm" className="w-full justify-start gap-2" onClick={() => { logout(); setDrawerOpen(false) }}>
                  <LogOut className="h-4 w-4" />
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
