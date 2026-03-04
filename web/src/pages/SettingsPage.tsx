import { Navigate } from 'react-router-dom'
import { useAuth } from '@/hooks/useAuth'
import OIDCSettingsTab from './settings/OIDCSettingsTab'
import TemplatesSettingsTab from './settings/TemplatesSettingsTab'

export default function SettingsPage() {
  const { user } = useAuth()

  if (user?.role !== 'admin') {
    return <Navigate to="/projects" replace />
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Settings</h1>
      <p className="text-sm text-muted-foreground mb-8">Organization settings.</p>

      <OIDCSettingsTab />

      <div className="my-10 border-t border-border" />

      <TemplatesSettingsTab />
    </div>
  )
}
