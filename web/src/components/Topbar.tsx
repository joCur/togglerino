import { type ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { t } from '../theme.ts'

interface TopbarProps {
  children?: ReactNode
}

export default function Topbar({ children }: TopbarProps) {
  const { user, logout } = useAuth()

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // ignore
    }
  }

  return (
    <header
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 24px',
        height: 52,
        backgroundColor: t.bgSurface,
        borderBottom: `1px solid ${t.border}`,
        flexShrink: 0,
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', gap: 16 }}>
        <Link
          to="/projects"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            textDecoration: 'none',
          }}
        >
          <svg width="20" height="12" viewBox="0 0 20 12" fill="none">
            <rect width="20" height="12" rx="6" fill={t.accent} opacity="0.25" />
            <circle cx="14" cy="6" r="4" fill={t.accent} />
          </svg>
          <span
            style={{
              fontFamily: t.fontMono,
              fontSize: 14,
              fontWeight: 600,
              color: t.accent,
              letterSpacing: '0.5px',
            }}
          >
            togglerino
          </span>
        </Link>
        {children}
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 14 }}>
        <div
          style={{
            width: 28,
            height: 28,
            borderRadius: '50%',
            backgroundColor: t.accentSubtle,
            border: `1px solid ${t.accentBorder}`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: 11,
            fontWeight: 600,
            color: t.accent,
            fontFamily: t.fontMono,
          }}
        >
          {user?.email?.charAt(0).toUpperCase()}
        </div>
        <span style={{ fontSize: 12, color: t.textSecondary }}>{user?.email}</span>
        <button
          onClick={handleLogout}
          style={{
            padding: '5px 12px',
            fontSize: 12,
            fontWeight: 500,
            fontFamily: t.fontSans,
            border: `1px solid ${t.border}`,
            borderRadius: t.radiusSm,
            backgroundColor: 'transparent',
            color: t.textSecondary,
            cursor: 'pointer',
            transition: 'all 200ms ease',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = t.borderHover
            e.currentTarget.style.color = t.textPrimary
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = t.border
            e.currentTarget.style.color = t.textSecondary
          }}
        >
          Log out
        </button>
      </div>
    </header>
  )
}
