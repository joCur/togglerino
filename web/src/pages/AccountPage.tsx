import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import type { User, OIDCIdentity } from '../api/types.ts'
import Topbar from '@/components/Topbar.tsx'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft } from 'lucide-react'
import { useTheme, type Theme } from '@/hooks/useTheme'
import { cn } from '@/lib/utils'

const themes: { value: Theme; label: string; description: string; icon: React.ReactNode }[] = [
  {
    value: 'light',
    label: 'Light',
    description: 'A clean, bright interface',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2" />
        <path d="M12 20v2" />
        <path d="m4.93 4.93 1.41 1.41" />
        <path d="m17.66 17.66 1.41 1.41" />
        <path d="M2 12h2" />
        <path d="M20 12h2" />
        <path d="m6.34 17.66-1.41 1.41" />
        <path d="m19.07 4.93-1.41 1.41" />
      </svg>
    ),
  },
  {
    value: 'dark',
    label: 'Dark',
    description: 'Easy on the eyes',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
      </svg>
    ),
  },
  {
    value: 'system',
    label: 'System',
    description: 'Follows your OS setting',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect width="20" height="14" x="2" y="3" rx="2" />
        <line x1="8" x2="16" y1="21" y2="21" />
        <line x1="12" x2="12" y1="17" y2="21" />
      </svg>
    ),
  },
]

export default function AccountPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const { theme, setTheme, isThemeToggleEnabled } = useTheme()

  const [displayName, setDisplayName] = useState(user?.display_name || '')
  const [email, setEmail] = useState(user?.email || '')
  const [profileError, setProfileError] = useState('')
  const [profileSuccess, setProfileSuccess] = useState('')

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [passwordSuccess, setPasswordSuccess] = useState('')

  const updateProfile = useMutation({
    mutationFn: (data: { email?: string; display_name?: string }) =>
      api.put<User>('/auth/me', data),
    onSuccess: (data) => {
      queryClient.setQueryData(['auth', 'me'], data)
      setProfileSuccess('Profile updated')
      setProfileError('')
    },
    onError: (err: Error) => {
      setProfileError(err.message)
      setProfileSuccess('')
    },
  })

  const changePassword = useMutation({
    mutationFn: (data: { current_password: string; new_password: string }) =>
      api.post('/auth/change-password', data),
    onSuccess: () => {
      setPasswordSuccess('Password changed')
      setPasswordError('')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    },
    onError: (err: Error) => {
      setPasswordError(err.message)
      setPasswordSuccess('')
    },
  })

  const identitiesQuery = useQuery({
    queryKey: ['oidc', 'identities'],
    queryFn: () => api.get<OIDCIdentity[]>('/auth/oidc/identities'),
  })

  const handleProfileSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setProfileError('')
    setProfileSuccess('')
    const updates: { email?: string; display_name?: string } = {}
    if (email !== user?.email) updates.email = email
    if (displayName !== (user?.display_name || '')) updates.display_name = displayName
    if (Object.keys(updates).length === 0) return
    updateProfile.mutate(updates)
  }

  const handlePasswordSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setPasswordError('')
    setPasswordSuccess('')
    if (newPassword !== confirmPassword) {
      setPasswordError('Passwords do not match')
      return
    }
    changePassword.mutate({
      current_password: currentPassword,
      new_password: newPassword,
    })
  }

  return (
    <div className="min-h-screen bg-background">
      <Topbar />
      <div className="mx-auto max-w-2xl px-4 md:px-6 py-8 md:py-10 animate-[fadeIn_300ms_ease]">
        <div className="mb-8">
          <Link
            to="/projects"
            className="inline-flex items-center gap-1.5 text-[12px] text-muted-foreground/60 hover:text-foreground transition-colors mb-4 no-underline"
          >
            <ArrowLeft className="h-3 w-3" />
            Back
          </Link>
          <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
            Account
          </h1>
          <div className="text-[13px] text-muted-foreground/60">
            Manage your profile and security settings.
          </div>
        </div>

        {/* Profile */}
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Profile
            </div>
            <form onSubmit={handleProfileSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Display name</Label>
                <Input
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Your display name"
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Email</Label>
                <Input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
              {profileError && (
                <div className="text-[13px] text-destructive">{profileError}</div>
              )}
              {profileSuccess && (
                <div className="text-[13px] text-emerald-500">{profileSuccess}</div>
              )}
              <Button type="submit" size="sm" className="self-start" disabled={updateProfile.isPending}>
                {updateProfile.isPending ? 'Saving...' : 'Save changes'}
              </Button>
            </form>
          </CardContent>
        </Card>

        {/* Change Password */}
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Change Password
            </div>
            <form onSubmit={handlePasswordSubmit} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Current password</Label>
                <Input
                  type="password"
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">New password</Label>
                <Input
                  type="password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                />
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Confirm new password</Label>
                <Input
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                />
              </div>
              {passwordError && (
                <div className="text-[13px] text-destructive">{passwordError}</div>
              )}
              {passwordSuccess && (
                <div className="text-[13px] text-emerald-500">{passwordSuccess}</div>
              )}
              <Button type="submit" size="sm" className="self-start" disabled={changePassword.isPending}>
                {changePassword.isPending ? 'Changing...' : 'Change password'}
              </Button>
            </form>
          </CardContent>
        </Card>

        {/* SSO Identity */}
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              SSO Identity
            </div>
            {identitiesQuery.data && identitiesQuery.data.length > 0 ? (
              <div className="flex flex-col gap-2">
                {identitiesQuery.data.map((ident) => (
                  <div key={ident.id} className="flex items-center gap-3">
                    <Badge variant="secondary" className="font-mono text-[11px]">SSO</Badge>
                    <span className="text-[13px] text-muted-foreground">{ident.email || ident.subject}</span>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-[13px] text-muted-foreground/60">
                No SSO identity linked to this account.
              </div>
            )}
          </CardContent>
        </Card>

        {/* Appearance */}
        {isThemeToggleEnabled && (
          <Card className="mb-5">
            <CardContent className="p-6">
              <div className="text-sm font-semibold text-foreground mb-1">
                Appearance
              </div>
              <p className="text-[13px] text-muted-foreground/60 mb-4">Choose how the dashboard looks to you.</p>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {themes.map((t) => (
                  <button
                    key={t.value}
                    onClick={() => setTheme(t.value)}
                    className={cn(
                      'flex flex-col items-center gap-2.5 rounded-lg border p-5 text-center transition-all duration-200 cursor-pointer',
                      theme === t.value
                        ? 'border-[#d4956a] bg-[#d4956a]/8 ring-1 ring-[#d4956a]/30'
                        : 'border-border bg-card hover:bg-accent/50'
                    )}
                  >
                    <div className={cn(
                      'text-muted-foreground transition-colors',
                      theme === t.value && 'text-[#d4956a]'
                    )}>
                      {t.icon}
                    </div>
                    <div>
                      <div className={cn(
                        'text-sm font-medium',
                        theme === t.value && 'text-[#d4956a]'
                      )}>
                        {t.label}
                      </div>
                      <div className="text-xs text-muted-foreground mt-0.5">
                        {t.description}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            </CardContent>
          </Card>
        )}

        {/* Account Info */}
        <Card>
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Account Info
            </div>
            <div className="flex flex-col gap-3.5">
              <div className="flex items-center gap-3">
                <span className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider min-w-[70px]">Role</span>
                <Badge
                  variant={user?.role === 'admin' ? 'secondary' : 'outline'}
                  className="font-mono text-[11px]"
                >
                  {user?.role || 'member'}
                </Badge>
              </div>
              <div className="flex items-center gap-3">
                <span className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider min-w-[70px]">Joined</span>
                <span className="text-[13px] text-muted-foreground">
                  {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '--'}
                </span>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
