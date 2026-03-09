import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client.ts'
import type { OIDCProvider } from '@/api/types.ts'
import { useAuth } from '@/hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface OIDCConfigResponse {
  configured: boolean
  provider?: OIDCProvider
}

function OIDCForm({ provider, configured }: { provider?: OIDCProvider; configured: boolean }) {
  const queryClient = useQueryClient()

  const [name, setName] = useState(provider?.name ?? '')
  const [issuerUrl, setIssuerUrl] = useState(provider?.issuer_url ?? '')
  const [clientId, setClientId] = useState(provider?.client_id ?? '')
  const [clientSecret, setClientSecret] = useState('')
  const [scopes, setScopes] = useState(provider?.scopes ?? 'openid email profile')
  const [defaultRole, setDefaultRole] = useState<'admin' | 'member'>(provider?.default_role ?? 'member')
  const [enabled, setEnabled] = useState(provider?.enabled ?? true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)

  const saveMutation = useMutation({
    mutationFn: (data: {
      name: string
      issuer_url: string
      client_id: string
      client_secret: string
      scopes: string
      default_role: string
      enabled: boolean
    }) => api.put<OIDCProvider>('/auth/oidc/config', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oidc', 'config'] })
      queryClient.invalidateQueries({ queryKey: ['auth', 'status'] })
      setSuccess('OIDC configuration saved')
      setError('')
      setClientSecret('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete('/auth/oidc/config'),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oidc', 'config'] })
      queryClient.invalidateQueries({ queryKey: ['auth', 'status'] })
      setShowDeleteDialog(false)
      setSuccess('OIDC configuration removed')
      setError('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  const callbackUrl = `${window.location.origin}/api/v1/auth/oidc/callback`

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    if (!clientSecret && !configured) {
      setError('Client secret is required for initial setup')
      return
    }
    saveMutation.mutate({
      name,
      issuer_url: issuerUrl,
      client_id: clientId,
      client_secret: clientSecret || '',
      scopes,
      default_role: defaultRole,
      enabled,
    })
  }

  return (
    <>
      <Card className="mb-5">
        <CardContent className="p-6">
          <div className="text-xs font-mono text-muted-foreground mb-1">Callback URL</div>
          <div className="flex items-center gap-2">
            <code className="text-xs bg-muted px-2.5 py-1.5 rounded font-mono flex-1 break-all">
              {callbackUrl}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => navigator.clipboard.writeText(callbackUrl)}
            >
              Copy
            </Button>
          </div>
          <p className="text-[11px] text-muted-foreground/60 mt-2">
            Add this URL as the redirect URI in your identity provider configuration.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="p-6">
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Provider Name</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Okta, Azure AD" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Issuer URL</Label>
              <Input value={issuerUrl} onChange={(e) => setIssuerUrl(e.target.value)} placeholder="https://accounts.google.com" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Client ID</Label>
              <Input value={clientId} onChange={(e) => setClientId(e.target.value)} placeholder="your-client-id" required />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Client Secret</Label>
              <Input
                type="password"
                value={clientSecret}
                onChange={(e) => setClientSecret(e.target.value)}
                placeholder={configured ? '(unchanged)' : 'your-client-secret'}
              />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Scopes</Label>
              <Input value={scopes} onChange={(e) => setScopes(e.target.value)} placeholder="openid email profile" />
            </div>

            <div className="flex flex-col gap-1.5">
              <Label className="font-mono text-[10px] uppercase tracking-wider">Default Role for New Users</Label>
              <Select value={defaultRole} onValueChange={(v) => setDefaultRole(v as 'admin' | 'member')}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="member">Member</SelectItem>
                  <SelectItem value="admin">Admin</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center gap-3">
              <Switch checked={enabled} onCheckedChange={setEnabled} />
              <Label className="text-sm">Enabled</Label>
            </div>

            {error && <div className="text-[13px] text-destructive">{error}</div>}
            {success && <div className="text-[13px] text-emerald-500">{success}</div>}

            <div className="flex gap-2">
              <Button type="submit" size="sm" disabled={saveMutation.isPending}>
                {saveMutation.isPending ? 'Saving...' : 'Save Configuration'}
              </Button>
              {configured && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setShowDeleteDialog(true)}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending ? 'Removing...' : 'Remove OIDC'}
                </Button>
              )}
            </div>
          </form>
        </CardContent>
      </Card>

      <Dialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Remove OIDC Configuration</DialogTitle>
            <DialogDescription>
              This will remove the OIDC configuration. Users who signed in exclusively via SSO will lose access until OIDC is reconfigured.
            </DialogDescription>
          </DialogHeader>
          {deleteMutation.isError && (
            <div className="text-[13px] text-destructive">{deleteMutation.error.message}</div>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteDialog(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => deleteMutation.mutate()}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? 'Removing...' : 'Remove OIDC'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

export default function OIDCSettingsTab() {
  const { user } = useAuth()

  const configQuery = useQuery({
    queryKey: ['oidc', 'config'],
    queryFn: () => api.get<OIDCConfigResponse>('/auth/oidc/config'),
    enabled: user?.role === 'admin',
  })

  if (user?.role !== 'admin') {
    return <div className="text-sm text-muted-foreground">Admin access required.</div>
  }

  const configured = configQuery.data?.configured ?? false

  return (
    <div className="max-w-2xl">
      <h2 className="text-sm font-medium mb-1">OpenID Connect (OIDC)</h2>
      <p className="text-xs text-muted-foreground mb-6">
        Configure SSO with your identity provider (Okta, Azure AD, Google Workspace, etc.)
      </p>
      <OIDCForm
        key={configQuery.dataUpdatedAt}
        provider={configQuery.data?.provider}
        configured={configured}
      />
    </div>
  )
}
