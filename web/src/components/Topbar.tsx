import { type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Menu } from 'lucide-react'

interface TopbarProps {
  children?: ReactNode
  onMenuClick?: () => void
}

export default function Topbar({ children, onMenuClick }: TopbarProps) {
  const { user, logout } = useAuth()

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // ignore
    }
  }

  return (
    <header className="flex items-center justify-between px-4 md:px-6 h-[52px] bg-card border-b shrink-0">
      <div className="flex items-center gap-3 md:gap-4">
        {onMenuClick && (
          <Button
            variant="ghost"
            size="sm"
            className="md:hidden h-8 w-8 p-0"
            onClick={onMenuClick}
          >
            <Menu className="w-5 h-5" />
          </Button>
        )}
        <Link to="/projects" className="flex items-center gap-2.5 no-underline">
          <svg width="20" height="12" viewBox="0 0 20 12" fill="none">
            <rect width="20" height="12" rx="6" fill="#d4956a" opacity="0.25" />
            <circle cx="14" cy="6" r="4" fill="#d4956a" />
          </svg>
          <span className="font-mono text-sm font-semibold text-[#d4956a] tracking-wide">
            togglerino
          </span>
        </Link>
        {children}
      </div>

      <div className="flex items-center gap-3.5">
        <div className="w-7 h-7 rounded-full bg-[#d4956a]/8 border border-[#d4956a]/20 flex items-center justify-center text-[11px] font-semibold text-[#d4956a] font-mono">
          {user?.email?.charAt(0).toUpperCase()}
        </div>
        <span className="hidden md:inline text-xs text-muted-foreground">{user?.email}</span>
        <Button variant="outline" size="sm" className="hidden md:flex" onClick={handleLogout}>
          Log out
        </Button>
      </div>
    </header>
  )
}
