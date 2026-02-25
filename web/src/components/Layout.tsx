import { Outlet, NavLink } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'

const styles = {
  wrapper: {
    display: 'flex',
    flexDirection: 'column',
    minHeight: '100vh',
    backgroundColor: '#1a1a2e',
    color: '#e0e0e0',
  } as const,
  topBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: '0 24px',
    height: 56,
    backgroundColor: '#16213e',
    borderBottom: '1px solid #2a2a4a',
    flexShrink: 0,
  } as const,
  brand: {
    fontSize: 18,
    fontWeight: 700,
    color: '#e94560',
    letterSpacing: '-0.5px',
  } as const,
  userSection: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
  } as const,
  userEmail: {
    fontSize: 13,
    color: '#8892b0',
  } as const,
  logoutButton: {
    padding: '6px 14px',
    fontSize: 13,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
  } as const,
  body: {
    display: 'flex',
    flex: 1,
  } as const,
  sidebar: {
    width: 220,
    backgroundColor: '#16213e',
    borderRight: '1px solid #2a2a4a',
    padding: '16px 0',
    flexShrink: 0,
  } as const,
  navLink: {
    display: 'block',
    padding: '10px 24px',
    fontSize: 14,
    color: '#8892b0',
    textDecoration: 'none',
    borderLeft: '3px solid transparent',
  } as const,
  navLinkActive: {
    color: '#ffffff',
    backgroundColor: 'rgba(233, 69, 96, 0.1)',
    borderLeftColor: '#e94560',
  } as const,
  main: {
    flex: 1,
    padding: 32,
    overflowY: 'auto',
  } as const,
}

export default function Layout() {
  const { user, logout } = useAuth()

  const handleLogout = async () => {
    try {
      await logout()
    } catch {
      // ignore
    }
  }

  return (
    <div style={styles.wrapper}>
      <header style={styles.topBar}>
        <div style={styles.brand}>togglerino</div>
        <div style={styles.userSection}>
          <span style={styles.userEmail}>{user?.email}</span>
          <button style={styles.logoutButton} onClick={handleLogout}>
            Log out
          </button>
        </div>
      </header>
      <div style={styles.body}>
        <nav style={styles.sidebar}>
          <NavLink
            to="/projects"
            style={({ isActive }) => ({
              ...styles.navLink,
              ...(isActive ? styles.navLinkActive : {}),
            })}
          >
            Projects
          </NavLink>
        </nav>
        <main style={styles.main}>
          <Outlet />
        </main>
      </div>
    </div>
  )
}
