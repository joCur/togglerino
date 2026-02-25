import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuth } from './hooks/useAuth.ts'
import { t } from './theme.ts'
import SetupPage from './pages/SetupPage.tsx'
import LoginPage from './pages/LoginPage.tsx'
import ProjectsPage from './pages/ProjectsPage.tsx'
import ProjectDetailPage from './pages/ProjectDetailPage.tsx'
import FlagDetailPage from './pages/FlagDetailPage.tsx'
import EnvironmentsPage from './pages/EnvironmentsPage.tsx'
import SDKKeysPage from './pages/SDKKeysPage.tsx'
import AuditLogPage from './pages/AuditLogPage.tsx'
import TeamPage from './pages/TeamPage.tsx'
import OrgLayout from './components/OrgLayout.tsx'
import ProjectLayout from './components/ProjectLayout.tsx'
import ProjectSettingsPage from './pages/ProjectSettingsPage.tsx'

const queryClient = new QueryClient()

function AuthRouter() {
  const { isLoading, isAuthenticated, setupRequired } = useAuth()

  if (isLoading) {
    return (
      <div
        style={{
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          height: '100vh',
          backgroundColor: t.bgBase,
          color: t.textMuted,
          fontFamily: t.fontSans,
          fontSize: 14,
          letterSpacing: '0.5px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <svg width="20" height="12" viewBox="0 0 20 12" fill="none">
            <rect width="20" height="12" rx="6" fill={t.accent} opacity="0.25" />
            <circle cx="14" cy="6" r="4" fill={t.accent} style={{ animation: 'shimmer 1.5s ease infinite' }} />
          </svg>
          <span style={{ animation: 'shimmer 1.5s ease infinite' }}>Loading...</span>
        </div>
      </div>
    )
  }

  if (setupRequired) {
    return (
      <Routes>
        <Route path="*" element={<SetupPage />} />
      </Routes>
    )
  }

  if (!isAuthenticated) {
    return (
      <Routes>
        <Route path="*" element={<LoginPage />} />
      </Routes>
    )
  }

  return (
    <Routes>
      <Route element={<OrgLayout />}>
        <Route path="/projects" element={<ProjectsPage />} />
        <Route path="/settings/team" element={<TeamPage />} />
      </Route>
      <Route path="/projects/:key" element={<ProjectLayout />}>
        <Route index element={<ProjectDetailPage />} />
        <Route path="flags/:flag" element={<FlagDetailPage />} />
        <Route path="environments" element={<EnvironmentsPage />} />
        <Route path="environments/:env/sdk-keys" element={<SDKKeysPage />} />
        <Route path="audit-log" element={<AuditLogPage />} />
        <Route path="settings" element={<ProjectSettingsPage />} />
      </Route>
      <Route path="/" element={<Navigate to="/projects" replace />} />
      <Route path="*" element={<Navigate to="/projects" replace />} />
    </Routes>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthRouter />
      </BrowserRouter>
    </QueryClientProvider>
  )
}

export default App
