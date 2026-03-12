import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import type { User, OIDCIdentity, PersonalAccessToken } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

function formatRelativeTime(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSecs = Math.floor(diffMs / 1000)
  const diffMins = Math.floor(diffSecs / 60)
  const diffHours = Math.floor(diffMins / 60)
  const diffDays = Math.floor(diffHours / 24)
  const diffMonths = Math.floor(diffDays / 30)
  const diffYears = Math.floor(diffDays / 365)

  if (diffSecs < 60) return 'just now'
  if (diffMins < 60) return `${diffMins} minute${diffMins !== 1 ? 's' : ''} ago`
  if (diffHours < 24) return `${diffHours} hour${diffHours !== 1 ? 's' : ''} ago`
  if (diffDays < 30) return `${diffDays} day${diffDays !== 1 ? 's' : ''} ago`
  if (diffMonths < 12) return `${diffMonths} month${diffMonths !== 1 ? 's' : ''} ago`
  return `${diffYears} year${diffYears !== 1 ? 's' : ''} ago`
}

function formatExpiryDate(dateStr: string | null): string {
  if (!dateStr) return 'Never'
  return new Date(dateStr).toLocaleDateString()
}

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

  // Token state
  const [createTokenOpen, setCreateTokenOpen] = useState(false)
  const [tokenName, setTokenName] = useState('')
  const [tokenExpiry, setTokenExpiry] = useState('')
  const [tokenCreateError, setTokenCreateError] = useState('')
  const [newToken, setNewToken] = useState<string | null>(null)
  const [copiedToken, setCopiedToken] = useState(false)

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

  const tokensQuery = useQuery({
    queryKey: ['auth', 'tokens'],
    queryFn: () => api.tokens.list(),
  })

  const createToken = useMutation({
    mutationFn: (data: { name: string; expires_at?: string }) => api.tokens.create(data),
    onSuccess: (data) => {
      setNewToken(data.token)
      setCreateTokenOpen(false)
      setTokenName('')
      setTokenExpiry('')
      setTokenCreateError('')
      queryClient.invalidateQueries({ queryKey: ['auth', 'tokens'] })
    },
    onError: (err: Error) => {
      setTokenCreateError(err.message)
    },
  })

  const deleteToken = useMutation({
    mutationFn: (id: string) => api.tokens.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth', 'tokens'] })
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

  const handleCreateToken = (e: React.FormEvent) => {
    e.preventDefault()
    setTokenCreateError('')
    if (!tokenName.trim()) {
      setTokenCreateError('Token name is required')
      return
    }
    const data: { name: string; expires_at?: string } = { name: tokenName.trim() }
    if (tokenExpiry) data.expires_at = new Date(tokenExpiry).toISOString()
    createToken.mutate(data)
  }

  const handleCopyToken = () => {
    if (newToken) {
      navigator.clipboard.writeText(newToken)
      setCopiedToken(true)
      setTimeout(() => setCopiedToken(false), 2000)
    }
  }

  const tokens: PersonalAccessToken[] = tokensQuery.data ?? []

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Account</h1>
      <p className="text-sm text-muted-foreground mb-8">Manage your profile and security settings.</p>

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

        {/* API Tokens */}
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="text-sm font-semibold text-foreground">API Tokens</div>
              <Button size="sm" variant="outline" onClick={() => setCreateTokenOpen(true)}>
                Create token
              </Button>
            </div>
            {tokensQuery.isLoading ? (
              <div className="text-[13px] text-muted-foreground/60">Loading...</div>
            ) : tokens.length === 0 ? (
              <div className="text-[13px] text-muted-foreground/60">
                No API tokens yet. Create one to use with the API or SDKs.
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">Name</TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">Prefix</TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">Last used</TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">Expires</TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">Created</TableHead>
                    <TableHead />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tokens.map((token) => (
                    <TableRow key={token.id}>
                      <TableCell className="text-[13px] font-medium">{token.name}</TableCell>
                      <TableCell>
                        <code className="font-mono text-[12px] text-muted-foreground bg-muted/40 px-1.5 py-0.5 rounded">
                          {token.token_prefix}...
                        </code>
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {formatRelativeTime(token.last_used_at)}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {formatExpiryDate(token.expires_at)}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {new Date(token.created_at).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="ghost"
                          size="sm"
                          className="text-destructive hover:text-destructive h-7 px-2 text-[12px]"
                          disabled={deleteToken.isPending}
                          onClick={() => deleteToken.mutate(token.id)}
                        >
                          Revoke
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
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

      {/* Create Token Dialog */}
      <Dialog open={createTokenOpen} onOpenChange={(open) => {
        setCreateTokenOpen(open)
        if (!open) {
          setTokenName('')
          setTokenExpiry('')
          setTokenCreateError('')
        }
      }}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Create API Token</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreateToken} className="flex flex-col gap-4 pt-2">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
              <Input
                value={tokenName}
                onChange={(e) => setTokenName(e.target.value)}
                placeholder="e.g. CI deploy token"
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">
                Expiry date <span className="text-muted-foreground normal-case">(optional)</span>
              </Label>
              <Input
                type="date"
                value={tokenExpiry}
                onChange={(e) => setTokenExpiry(e.target.value)}
                min={new Date().toISOString().split('T')[0]}
              />
            </div>
            {tokenCreateError && (
              <div className="text-[13px] text-destructive">{tokenCreateError}</div>
            )}
            <DialogFooter>
              <Button type="button" variant="outline" size="sm" onClick={() => setCreateTokenOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" size="sm" disabled={createToken.isPending}>
                {createToken.isPending ? 'Creating...' : 'Create token'}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* New Token Display Dialog */}
      <Dialog open={!!newToken} onOpenChange={(open) => {
        if (!open) {
          setNewToken(null)
          setCopiedToken(false)
        }
      }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Token created</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4 pt-2">
            <div className="rounded-md border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-[13px] text-amber-400">
              Make sure to copy your token now. You won't be able to see it again.
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Your API token</Label>
              <div className="flex gap-2">
                <code className="flex-1 font-mono text-[12px] bg-muted/40 border border-border rounded px-3 py-2 break-all select-all">
                  {newToken}
                </code>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="shrink-0 self-start"
                  onClick={handleCopyToken}
                >
                  {copiedToken ? 'Copied!' : 'Copy'}
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter className="pt-2">
            <Button size="sm" onClick={() => {
              setNewToken(null)
              setCopiedToken(false)
            }}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
