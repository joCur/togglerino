import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment } from '../api/types.ts'
import { t } from '../theme.ts'

export default function EnvironmentsPage() {
  const { key } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [envKey, setEnvKey] = useState('')
  const [envName, setEnvName] = useState('')

  const { data: environments, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const createMutation = useMutation({
    mutationFn: (data: { key: string; name: string }) =>
      api.post<Environment>(`/projects/${key}/environments`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'environments'] })
      setShowForm(false)
      setEnvKey('')
      setEnvName('')
    },
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    if (!envKey.trim() || !envName.trim()) return
    createMutation.mutate({ key: envKey.trim(), name: envName.trim() })
  }

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
        Loading environments...
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: '14px 18px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13 }}>
        Failed to load environments: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  return (
    <div style={{ animation: 'fadeIn 300ms ease' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 24, fontSize: 13, color: t.textMuted }}>
        <Link to="/projects" style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}>
          Projects
        </Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <Link to={`/projects/${key}`} style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}>
          {key}
        </Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <span style={{ color: t.textPrimary }}>Environments</span>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, letterSpacing: '-0.3px' }}>Environments</h1>
        {!showForm && (
          <button
            style={{
              padding: '9px 18px', fontSize: 13, fontWeight: 600, border: 'none', borderRadius: t.radiusMd,
              background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`, color: '#ffffff', cursor: 'pointer',
              fontFamily: t.fontSans, transition: 'all 200ms ease', boxShadow: '0 2px 10px rgba(212,149,106,0.15)',
            }}
            onClick={() => setShowForm(true)}
            onMouseEnter={(e) => { e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'; e.currentTarget.style.transform = 'translateY(-1px)' }}
            onMouseLeave={(e) => { e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'; e.currentTarget.style.transform = 'translateY(0)' }}
          >
            Create Environment
          </button>
        )}
      </div>

      {showForm && (
        <form
          style={{
            display: 'flex', gap: 12, marginBottom: 24, padding: 20, borderRadius: t.radiusLg,
            backgroundColor: t.bgSurface, border: `1px solid ${t.border}`, alignItems: 'flex-end',
            animation: 'fadeIn 200ms ease',
          }}
          onSubmit={handleCreate}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>Key</label>
            <input
              style={{ padding: '8px 12px', fontSize: 13, border: `1px solid ${t.border}`, borderRadius: t.radiusMd, backgroundColor: t.bgInput, color: t.textPrimary, outline: 'none', minWidth: 160, fontFamily: t.fontSans, transition: 'border-color 200ms ease' }}
              placeholder="e.g. staging"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
              autoFocus
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
            />
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>Name</label>
            <input
              style={{ padding: '8px 12px', fontSize: 13, border: `1px solid ${t.border}`, borderRadius: t.radiusMd, backgroundColor: t.bgInput, color: t.textPrimary, outline: 'none', minWidth: 160, fontFamily: t.fontSans, transition: 'border-color 200ms ease' }}
              placeholder="e.g. Staging"
              value={envName}
              onChange={(e) => setEnvName(e.target.value)}
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border }}
            />
          </div>
          <button
            type="submit"
            style={{
              padding: '8px 16px', fontSize: 13, fontWeight: 600, border: 'none', borderRadius: t.radiusMd,
              background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`, color: '#ffffff', cursor: 'pointer',
              fontFamily: t.fontSans, whiteSpace: 'nowrap', opacity: createMutation.isPending ? 0.7 : 1,
            }}
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </button>
          <button
            type="button"
            style={{
              padding: '8px 16px', fontSize: 13, fontWeight: 500, border: `1px solid ${t.border}`, borderRadius: t.radiusMd,
              backgroundColor: 'transparent', color: t.textSecondary, cursor: 'pointer', fontFamily: t.fontSans, whiteSpace: 'nowrap',
            }}
            onClick={() => { setShowForm(false); setEnvKey(''); setEnvName(''); createMutation.reset() }}
          >
            Cancel
          </button>
        </form>
      )}

      {createMutation.error && (
        <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginBottom: 16 }}>
          {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create environment'}
        </div>
      )}

      {(!environments || environments.length === 0) ? (
        <div style={{ textAlign: 'center', padding: 48, color: t.textSecondary }}>
          <div style={{ fontSize: 15, fontWeight: 500, color: t.textPrimary, marginBottom: 6 }}>No environments yet</div>
          <div style={{ fontSize: 13, color: t.textMuted }}>Create your first environment to start configuring feature flags per environment.</div>
        </div>
      ) : (
        <div style={{ borderRadius: t.radiusLg, border: `1px solid ${t.border}`, overflow: 'hidden' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Key', 'Name', 'Created', 'SDK Keys'].map((h) => (
                  <th key={h} style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', borderBottom: `1px solid ${t.border}`, backgroundColor: t.bgSurface, fontFamily: t.fontMono }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {environments.map((env) => (
                <tr key={env.id} style={{ borderBottom: `1px solid ${t.border}`, transition: 'background-color 200ms ease' }}
                  onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.accentSubtle }}
                  onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
                >
                  <td style={{ padding: '12px 16px', fontSize: 13 }}>
                    <span style={{ fontFamily: t.fontMono, fontSize: 12, color: t.accent }}>{env.key}</span>
                  </td>
                  <td style={{ padding: '12px 16px', fontSize: 13, color: t.textPrimary }}>{env.name}</td>
                  <td style={{ padding: '12px 16px', fontSize: 13, color: t.textSecondary }}>{new Date(env.created_at).toLocaleDateString()}</td>
                  <td style={{ padding: '12px 16px' }}>
                    <Link
                      to={`/projects/${key}/environments/${env.key}/sdk-keys`}
                      style={{ color: t.accent, textDecoration: 'none', fontSize: 13, transition: 'color 200ms ease' }}
                      onMouseEnter={(e) => { e.currentTarget.style.color = t.accentLight }}
                      onMouseLeave={(e) => { e.currentTarget.style.color = t.accent }}
                    >
                      Manage SDK Keys
                    </Link>
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
