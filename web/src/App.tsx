import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TogglerioProvider } from '@togglerino/react'
import { ThemeProvider } from './components/ThemeProvider.tsx'
import { useAuth } from './hooks/useAuth.ts'
import { ApiError } from './api/client.ts'
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
import LifecycleBoardPage from './pages/LifecycleBoardPage.tsx'
import AcceptInvitePage from './pages/AcceptInvitePage.tsx'
import SettingsPage from './pages/SettingsPage.tsx'
import ResetPasswordPage from './pages/ResetPasswordPage.tsx'
import AccountPage from './pages/AccountPage.tsx'
import PreferencesPage from './pages/PreferencesPage.tsx'
import LinkAccountPage from './pages/LinkAccountPage.tsx'
import SegmentsPage from './pages/SegmentsPage.tsx'
import GeneralSettingsTab from './pages/settings/GeneralSettingsTab.tsx'
import FlagLifetimesTab from './pages/settings/FlagLifetimesTab.tsx'
import EnvironmentDefaultsTab from './pages/settings/EnvironmentDefaultsTab.tsx'
import MembersTab from './pages/settings/MembersTab.tsx'
import ProjectTemplatesPage from './pages/ProjectTemplatesPage.tsx'
import TemplatesPage from './pages/TemplatesPage.tsx'
import PlaygroundPage from './pages/PlaygroundPage.tsx'
import KillSwitchDashboardPage from './pages/KillSwitchDashboardPage.tsx'
import MyOverridesPage from './pages/MyOverridesPage.tsx'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
          return false
        }
        return failureCount < 3
      },
    },
  },
})

const togglerinoConfig = {
  serverUrl: 'https://flags.curth.dev',
  sdkKey: 'sdk_37e55bbb1ae453f80d0d97b253a551a8',
}

function AuthRouter() {
  const { isLoading, isAuthenticated, setupRequired } = useAuth()

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-screen text-muted-foreground text-sm tracking-wide">
        <div className="flex items-center gap-3">
          <svg width="20" height="12" viewBox="0 0 20 12" fill="none">
            <rect width="20" height="12" rx="6" fill="#d4956a" opacity="0.25" />
            <circle cx="14" cy="6" r="4" fill="#d4956a" className="animate-pulse" />
          </svg>
          <span className="animate-pulse">Loading...</span>
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
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/preferences" element={<PreferencesPage />} />
        <Route path="/overrides" element={<MyOverridesPage />} />
      </Route>
      <Route path="/projects/:key" element={<ProjectLayout />}>
        <Route index element={<ProjectDetailPage />} />
        <Route path="lifecycle" element={<LifecycleBoardPage />} />
        <Route path="kill-switches" element={<KillSwitchDashboardPage />} />
        <Route path="flags/:flag" element={<FlagDetailPage />} />
        <Route path="environments" element={<EnvironmentsPage />} />
        <Route path="environments/:env/sdk-keys" element={<SDKKeysPage />} />
        <Route path="audit-log" element={<AuditLogPage />} />
        <Route path="templates" element={<ProjectTemplatesPage />} />
        <Route path="settings" element={<ProjectSettingsPage />}>
          <Route index element={<Navigate to="general" replace />} />
          <Route path="general" element={<GeneralSettingsTab />} />
          <Route path="lifetimes" element={<FlagLifetimesTab />} />
          <Route path="environments" element={<EnvironmentDefaultsTab />} />
          <Route path="members" element={<MembersTab />} />
        </Route>
        <Route path="segments" element={<SegmentsPage />} />
        <Route path="playground" element={<PlaygroundPage />} />
      </Route>
      <Route path="/" element={<Navigate to="/projects" replace />} />
      <Route path="*" element={<Navigate to="/projects" replace />} />
    </Routes>
  )
}

function App() {
  return (
    <TogglerioProvider config={togglerinoConfig}>
      <ThemeProvider>
        <QueryClientProvider client={queryClient}>
          <BrowserRouter>
            <Routes>
              <Route path="/invite/:token" element={<AcceptInvitePage />} />
              <Route path="/reset-password/:token" element={<ResetPasswordPage />} />
              <Route path="/link-account" element={<LinkAccountPage />} />
              <Route path="*" element={<AuthRouter />} />
            </Routes>
          </BrowserRouter>
        </QueryClientProvider>
      </ThemeProvider>
    </TogglerioProvider>
  )
}

export default App
