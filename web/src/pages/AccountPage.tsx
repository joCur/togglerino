import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import type { User } from '../api/types.ts'
import Topbar from '@/components/Topbar.tsx'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { ArrowLeft } from 'lucide-react'

export default function AccountPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()

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
