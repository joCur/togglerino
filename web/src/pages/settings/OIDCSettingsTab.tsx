import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client.ts'
import type { OIDCProvider } from '@/api/types.ts'
import { useAuth } from '@/hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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

export default function OIDCSettingsTab() {
  const { user } = useAuth()
  const queryClient = useQueryClient()

  const configQuery = useQuery({
    queryKey: ['oidc', 'config'],
    queryFn: () => api.get<OIDCConfigResponse>('/auth/oidc/config'),
    enabled: user?.role === 'admin',
  })

  const [name, setName] = useState('')
  const [issuerUrl, setIssuerUrl] = useState('')
  const [clientId, setClientId] = useState('')
  const [clientSecret, setClientSecret] = useState('')
  const [scopes, setScopes] = useState('openid email profile')
  const [defaultRole, setDefaultRole] = useState<'admin' | 'member'>('member')
  const [enabled, setEnabled] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  useEffect(() => {
    if (configQuery.data?.provider) {
      const p = configQuery.data.provider
      setName(p.name)
      setIssuerUrl(p.issuer_url)
      setClientId(p.client_id)
      setScopes(p.scopes)
      setDefaultRole(p.default_role)
      setEnabled(p.enabled)
    }
  }, [configQuery.data])

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
      setName('')
      setIssuerUrl('')
      setClientId('')
      setClientSecret('')
      setScopes('openid email profile')
      setDefaultRole('member')
      setEnabled(true)
      setSuccess('OIDC configuration removed')
      setError('')
    },
    onError: (err: Error) => {
      setError(err.message)
      setSuccess('')
    },
  })

  if (user?.role !== 'admin') {
    return <div className="text-sm text-muted-foreground">Admin access required.</div>
  }

  const callbackUrl = `${window.location.origin}/api/v1/auth/oidc/callback`

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    if (!clientSecret && !configQuery.data?.configured) {
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
    <div className="max-w-2xl">
      <h2 className="text-sm font-medium mb-1">OpenID Connect (OIDC)</h2>
      <p className="text-xs text-muted-foreground mb-6">
        Configure SSO with your identity provider (Okta, Azure AD, Google Workspace, etc.)
      </p>

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
                placeholder={configQuery.data?.configured ? '(unchanged)' : 'your-client-secret'}
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
              {configQuery.data?.configured && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => deleteMutation.mutate()}
                  disabled={deleteMutation.isPending}
                >
                  {deleteMutation.isPending ? 'Removing...' : 'Remove OIDC'}
                </Button>
              )}
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
