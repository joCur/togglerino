import { Outlet, NavLink } from 'react-router-dom'
import Topbar from './Topbar.tsx'
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

export default function OrgLayout() {
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
      <Topbar />

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
            style={({ isActive }) => navLinkStyle(isActive)}
          >
            Projects
          </NavLink>
          <NavLink
            to="/settings/team"
            style={({ isActive }) => navLinkStyle(isActive)}
          >
            Team
          </NavLink>
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
