import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useAuth } from './hooks/useAuth.ts'
import SetupPage from './pages/SetupPage.tsx'
import LoginPage from './pages/LoginPage.tsx'
import Layout from './components/Layout.tsx'

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
          backgroundColor: '#1a1a2e',
          color: '#8892b0',
          fontSize: 16,
        }}
      >
        Loading...
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
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/projects" replace />} />
        <Route
          path="/projects"
          element={<div>Projects page (coming next)</div>}
        />
        <Route path="*" element={<Navigate to="/projects" replace />} />
      </Route>
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
