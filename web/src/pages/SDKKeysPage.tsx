import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { SDKKey } from '../api/types.ts'
import { t } from '../theme.ts'

function maskKey(key: string): string {
  if (key.length <= 12) return key
  return key.slice(0, 8) + '...' + key.slice(-4)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

export default function SDKKeysPage() {
  const { key, env } = useParams<{ key: string; env: string }>()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [keyName, setKeyName] = useState('')
  const [copiedId, setCopiedId] = useState<string | null>(null)

  const { data: sdkKeys, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'environments', env, 'sdk-keys'],
    queryFn: () => api.get<SDKKey[]>(`/projects/${key}/environments/${env}/sdk-keys`),
    enabled: !!key && !!env,
  })

  const createMutation = useMutation({
    mutationFn: (data: { name: string }) =>
      api.post<SDKKey>(`/projects/${key}/environments/${env}/sdk-keys`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments', env, 'sdk-keys'] })
      setShowForm(false)
      setKeyName('')
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) =>
      api.delete(`/projects/${key}/environments/${env}/sdk-keys/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments', env, 'sdk-keys'] })
    },
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!keyName.trim()) return
    createMutation.mutate({ name: keyName.trim() })
  }

  const handleRevoke = (sdkKey: SDKKey) => {
    if (window.confirm(`Are you sure you want to revoke the SDK key "${sdkKey.name}"? This action cannot be undone.`)) {
      revokeMutation.mutate(sdkKey.id)
    }
  }

  const handleCopy = async (sdkKey: SDKKey) => {
    try {
      await navigator.clipboard.writeText(sdkKey.key)
      setCopiedId(sdkKey.id)
      setTimeout(() => setCopiedId(null), 2000)
    } catch {
      // Clipboard API may not be available
    }
  }

  if (isLoading) {
    return <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>Loading SDK keys...</div>
  }

  if (error) {
    return <div style={{ padding: '14px 18px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13 }}>Failed to load SDK keys: {error instanceof Error ? error.message : 'Unknown error'}</div>
  }

  return (
    <div style={{ animation: 'fadeIn 300ms ease' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 24, fontSize: 13, color: t.textMuted }}>
        <Link to="/projects" style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}>Projects</Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <Link to={`/projects/${key}`} style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}>{key}</Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <Link to={`/projects/${key}/environments`} style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}>Environments</Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <span style={{ color: t.textPrimary, fontFamily: t.fontMono, fontSize: 12 }}>{env}</span>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <span style={{ color: t.textPrimary }}>SDK Keys</span>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, letterSpacing: '-0.3px' }}>SDK Keys</h1>
        {!showForm && (
          <button
            style={{ padding: '9px 18px', fontSize: 13, fontWeight: 600, border: 'none', borderRadius: t.radiusMd, background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`, color: '#ffffff', cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease', boxShadow: '0 2px 10px rgba(212,149,106,0.15)' }}
            onClick={() => setShowForm(true)}
            onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'; e.currentTarget.style.transform = 'translateY(-1px)' }}
            onMouseLeave={(e) => { e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'; e.currentTarget.style.transform = 'translateY(0)' }}
          >
            Generate New Key
          </button>
        )}
      </div>

      {showForm && (
        <form style={{ display: 'flex', gap: 12, marginBottom: 24, padding: 20, borderRadius: t.radiusLg, backgroundColor: t.bgSurface, border: `1px solid ${t.border}`, alignItems: 'flex-end', animation: 'fadeIn 200ms ease' }} onSubmit={handleCreate}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
            <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>Name</label>
            <input
              style={{ padding: '8px 12px', fontSize: 13, border: `1px solid ${t.border}`, borderRadius: t.radiusMd, backgroundColor: t.bgInput, color: t.textPrimary, outline: 'none', fontFamily: t.fontSans, transition: 'border-color 200ms ease' }}
              placeholder="e.g. Backend Service Key"
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              autoFocus
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
            />
          </div>
          <button type="submit" style={{ padding: '8px 16px', fontSize: 13, fontWeight: 600, border: 'none', borderRadius: t.radiusMd, background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`, color: '#ffffff', cursor: 'pointer', fontFamily: t.fontSans, whiteSpace: 'nowrap', opacity: createMutation.isPending ? 0.7 : 1 }} disabled={createMutation.isPending}>
            {createMutation.isPending ? 'Generating...' : 'Generate'}
          </button>
          <button type="button" style={{ padding: '8px 16px', fontSize: 13, fontWeight: 500, border: `1px solid ${t.border}`, borderRadius: t.radiusMd, backgroundColor: 'transparent', color: t.textSecondary, cursor: 'pointer', fontFamily: t.fontSans, whiteSpace: 'nowrap' }}
            onClick={() => { setShowForm(false); setKeyName(''); createMutation.reset() }}>
            Cancel
          </button>
        </form>
      )}

      {createMutation.error && (
        <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginBottom: 16 }}>
          {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to generate key'}
        </div>
      )}

      {(!sdkKeys || sdkKeys.length === 0) ? (
        <div style={{ textAlign: 'center', padding: 48, color: t.textSecondary }}>
          <div style={{ fontSize: 15, fontWeight: 500, color: t.textPrimary, marginBottom: 6 }}>No SDK keys yet</div>
          <div style={{ fontSize: 13, color: t.textMuted }}>Generate an SDK key to connect your application to this environment.</div>
        </div>
      ) : (
        <div style={{ borderRadius: t.radiusLg, border: `1px solid ${t.border}`, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Key', 'Name', 'Status', 'Created', 'Actions'].map((h) => (
                  <th key={h} style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', borderBottom: `1px solid ${t.border}`, backgroundColor: t.bgSurface, fontFamily: t.fontMono }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {sdkKeys.map((sdkKey) => (
                <tr key={sdkKey.id} style={{ borderBottom: `1px solid ${t.border}`, transition: 'background-color 200ms ease' }}
                  onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.accentSubtle }}
                  onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}>
                  <td style={{ padding: '12px 16px' }}>
                    <span style={{ fontFamily: t.fontMono, fontSize: 12, color: t.accent }}>{maskKey(sdkKey.key)}</span>
                  </td>
                  <td style={{ padding: '12px 16px', fontSize: 13, color: t.textPrimary }}>{sdkKey.name}</td>
                  <td style={{ padding: '12px 16px' }}>
                    {sdkKey.revoked ? (
                      <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 500, backgroundColor: t.dangerSubtle, color: t.danger }}>Revoked</span>
                    ) : (
                      <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 500, backgroundColor: t.successSubtle, color: t.success }}>Active</span>
                    )}
                  </td>
                  <td style={{ padding: '12px 16px', fontSize: 13, color: t.textSecondary }}>{formatDate(sdkKey.created_at)}</td>
                  <td style={{ padding: '12px 16px' }}>
                    {!sdkKey.revoked && (
                      <div style={{ display: 'flex', gap: 8 }}>
                        <button
                          style={{ padding: '4px 10px', fontSize: 12, fontWeight: 500, border: `1px solid ${t.border}`, borderRadius: t.radiusSm, backgroundColor: 'transparent', color: copiedId === sdkKey.id ? t.success : t.textSecondary, cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease' }}
                          onClick={() => handleCopy(sdkKey)}
                          onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.borderHover; e.currentTarget.style.color = t.textPrimary }}
                          onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = copiedId === sdkKey.id ? t.success : t.textSecondary }}
                        >
                          {copiedId === sdkKey.id ? 'Copied!' : 'Copy'}
                        </button>
                        <button
                          style={{ padding: '4px 10px', fontSize: 12, fontWeight: 500, border: `1px solid ${t.dangerBorder}`, borderRadius: t.radiusSm, backgroundColor: 'transparent', color: t.danger, cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease' }}
                          onClick={() => handleRevoke(sdkKey)}
                          disabled={revokeMutation.isPending}
                          onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.dangerSubtle }}
                          onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
                        >
                          Revoke
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
