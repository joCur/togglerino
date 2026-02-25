import { Outlet, NavLink, useLocation, Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { t } from '../theme.ts'

const navLinkStyle = (isActive: boolean) => ({
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '8px 20px',
  fontSize: 13,
  fontWeight: isActive ? 500 : 400,
  color: isActive ? t.textPrimary : t.textSecondary,
  textDecoration: 'none' as const,
  borderLeft: `2px solid ${isActive ? t.accent : 'transparent'}`,
  backgroundColor: isActive ? t.accentSubtle : 'transparent',
  transition: 'all 200ms ease',
  fontFamily: t.fontSans,
})

export default function Layout() {
  const { user, logout } = useAuth()
  const location = useLocation()

  const projectMatch = location.pathname.match(/^\/projects\/([^/]+)/)
  const projectKey = projectMatch ? projectMatch[1] : null

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // ignore
    }
  }

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        minHeight: '100vh',
        backgroundColor: t.bgBase,
        color: t.textPrimary,
        fontFamily: t.fontSans,
      }}
    >
      {/* Top bar */}
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

      <div style={{ display: 'flex', flex: 1 }}>
        {/* Sidebar */}
        <nav
          style={{
            width: 200,
            backgroundColor: t.bgSurface,
            borderRight: `1px solid ${t.border}`,
            padding: '20px 0',
            flexShrink: 0,
            display: 'flex',
            flexDirection: 'column',
            gap: 0,
          }}
        >
          <div
            style={{
              padding: '0 20px 10px',
              fontSize: 10,
              fontWeight: 500,
              color: t.textMuted,
              textTransform: 'uppercase',
              letterSpacing: '1.2px',
              fontFamily: t.fontMono,
            }}
          >
            Navigation
          </div>
          <NavLink
            to="/projects"
            end
            style={({ isActive }) => navLinkStyle(isActive && !projectKey)}
          >
            Projects
          </NavLink>
          <NavLink
            to="/settings/team"
            style={({ isActive }) => navLinkStyle(isActive)}
          >
            Team
          </NavLink>

          {/* Project-scoped nav */}
          {projectKey && (
            <>
              <div
                style={{
                  margin: '16px 20px 0',
                  paddingTop: 16,
                  borderTop: `1px solid ${t.border}`,
                }}
              />
              <div
                style={{
                  padding: '0 20px 6px',
                  fontSize: 10,
                  fontWeight: 500,
                  color: t.textMuted,
                  textTransform: 'uppercase',
                  letterSpacing: '1.2px',
                  fontFamily: t.fontMono,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                }}
              >
                <span style={{ opacity: 0.6 }}>&rsaquo;</span>
                <span
                  style={{
                    maxWidth: 120,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {projectKey}
                </span>
              </div>
              <NavLink
                to={`/projects/${projectKey}`}
                end
                style={({ isActive }) => navLinkStyle(isActive)}
              >
                Flags
              </NavLink>
              <NavLink
                to={`/projects/${projectKey}/environments`}
                style={({ isActive }) => navLinkStyle(isActive)}
              >
                Environments
              </NavLink>
              <NavLink
                to={`/projects/${projectKey}/audit-log`}
                style={({ isActive }) => navLinkStyle(isActive)}
              >
                Audit Log
              </NavLink>
            </>
          )}
        </nav>

        {/* Main content */}
        <main
          style={{
            flex: 1,
            padding: 36,
            overflowY: 'auto',
            animation: 'fadeIn 300ms ease',
          }}
        >
          <Outlet />
        </main>
      </div>
    </div>
  )
}
