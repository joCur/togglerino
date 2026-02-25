import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { SDKKey } from '../api/types.ts'

function maskKey(key: string): string {
  if (key.length <= 12) return key
  return key.slice(0, 8) + '...' + key.slice(-4)
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

const styles = {
  breadcrumb: {
    display: 'flex',
    alignItems: 'center',
    gap: 8,
    marginBottom: 24,
    fontSize: 13,
    color: '#8892b0',
  } as const,
  breadcrumbLink: {
    color: '#8892b0',
    textDecoration: 'none',
  } as const,
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 24,
  } as const,
  title: {
    fontSize: 24,
    fontWeight: 700,
    color: '#ffffff',
  } as const,
  createBtn: {
    padding: '10px 20px',
    fontSize: 14,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
  } as const,
  form: {
    display: 'flex',
    gap: 12,
    marginBottom: 24,
    padding: 20,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px solid #2a2a4a',
    alignItems: 'flex-end',
  } as const,
  formField: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: 6,
    flex: 1,
  } as const,
  formLabel: {
    fontSize: 12,
    fontWeight: 600,
    color: '#8892b0',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
  } as const,
  input: {
    padding: '9px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
  } as const,
  submitBtn: {
    padding: '9px 18px',
    fontSize: 14,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
    whiteSpace: 'nowrap' as const,
  } as const,
  cancelBtn: {
    padding: '9px 18px',
    fontSize: 14,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
    whiteSpace: 'nowrap' as const,
  } as const,
  table: {
    width: '100%',
    borderCollapse: 'collapse' as const,
  } as const,
  th: {
    textAlign: 'left' as const,
    padding: '10px 16px',
    fontSize: 12,
    fontWeight: 600,
    color: '#8892b0',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    borderBottom: '1px solid #2a2a4a',
  } as const,
  tr: {
    borderBottom: '1px solid rgba(42, 42, 74, 0.5)',
  } as const,
  td: {
    padding: '12px 16px',
    fontSize: 14,
    color: '#e0e0e0',
  } as const,
  keyText: {
    fontFamily: 'monospace',
    fontSize: 13,
    color: '#e94560',
  } as const,
  statusActive: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 11,
    fontWeight: 600,
    backgroundColor: 'rgba(76, 175, 80, 0.15)',
    color: '#4caf50',
  } as const,
  statusRevoked: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 11,
    fontWeight: 600,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    color: '#e94560',
  } as const,
  actionBtn: {
    padding: '5px 12px',
    fontSize: 12,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 4,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
    marginRight: 8,
  } as const,
  revokeBtn: {
    padding: '5px 12px',
    fontSize: 12,
    fontWeight: 500,
    border: '1px solid rgba(233, 69, 96, 0.3)',
    borderRadius: 4,
    backgroundColor: 'transparent',
    color: '#e94560',
    cursor: 'pointer',
  } as const,
  copiedMsg: {
    fontSize: 11,
    color: '#4caf50',
    marginLeft: 8,
  } as const,
  empty: {
    textAlign: 'center' as const,
    padding: 48,
    color: '#8892b0',
  } as const,
  emptyTitle: {
    fontSize: 16,
    fontWeight: 600,
    color: '#e0e0e0',
    marginBottom: 8,
  } as const,
  loading: {
    textAlign: 'center' as const,
    padding: 64,
    color: '#8892b0',
    fontSize: 14,
  } as const,
  errorBox: {
    padding: '16px 20px',
    borderRadius: 8,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    border: '1px solid rgba(233, 69, 96, 0.3)',
    color: '#e94560',
    fontSize: 14,
  } as const,
  formError: {
    padding: '10px 16px',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    border: '1px solid rgba(233, 69, 96, 0.3)',
    color: '#e94560',
    fontSize: 13,
    marginBottom: 16,
  } as const,
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
    return <div style={styles.loading}>Loading SDK keys...</div>
  }

  if (error) {
    return (
      <div style={styles.errorBox}>
        Failed to load SDK keys: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  return (
    <div>
      <div style={styles.breadcrumb}>
        <Link to="/projects" style={styles.breadcrumbLink}>Projects</Link>
        <span>/</span>
        <Link to={`/projects/${key}`} style={styles.breadcrumbLink}>{key}</Link>
        <span>/</span>
        <Link to={`/projects/${key}/environments`} style={styles.breadcrumbLink}>Environments</Link>
        <span>/</span>
        <span style={{ color: '#e0e0e0' }}>{env}</span>
        <span>/</span>
        <span style={{ color: '#e0e0e0' }}>SDK Keys</span>
      </div>

      <div style={styles.header}>
        <h1 style={styles.title}>SDK Keys</h1>
        {!showForm && (
          <button style={styles.createBtn} onClick={() => setShowForm(true)}>
            Generate New Key
          </button>
        )}
      </div>

      {showForm && (
        <form style={styles.form} onSubmit={handleCreate}>
          <div style={styles.formField}>
            <label style={styles.formLabel}>Name</label>
            <input
              style={styles.input}
              placeholder="e.g. Backend Service Key"
              value={keyName}
              onChange={(e) => setKeyName(e.target.value)}
              autoFocus
            />
          </div>
          <button
            type="submit"
            style={{
              ...styles.submitBtn,
              opacity: createMutation.isPending ? 0.7 : 1,
            }}
            disabled={createMutation.isPending}
          >
            {createMutation.isPending ? 'Generating...' : 'Generate'}
          </button>
          <button
            type="button"
            style={styles.cancelBtn}
            onClick={() => {
              setShowForm(false)
              setKeyName('')
              createMutation.reset()
            }}
          >
            Cancel
          </button>
        </form>
      )}

      {createMutation.error && (
        <div style={styles.formError}>
          {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to generate key'}
        </div>
      )}

      {(!sdkKeys || sdkKeys.length === 0) ? (
        <div style={styles.empty}>
          <div style={styles.emptyTitle}>No SDK keys yet</div>
          <div style={{ fontSize: 14, color: '#8892b0' }}>
            Generate an SDK key to connect your application to this environment.
          </div>
        </div>
      ) : (
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>Key</th>
              <th style={styles.th}>Name</th>
              <th style={styles.th}>Status</th>
              <th style={styles.th}>Created</th>
              <th style={styles.th}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {sdkKeys.map((sdkKey) => (
              <tr key={sdkKey.id} style={styles.tr}>
                <td style={styles.td}>
                  <span style={styles.keyText}>{maskKey(sdkKey.key)}</span>
                </td>
                <td style={styles.td}>{sdkKey.name}</td>
                <td style={styles.td}>
                  {sdkKey.revoked ? (
                    <span style={styles.statusRevoked}>Revoked</span>
                  ) : (
                    <span style={styles.statusActive}>Active</span>
                  )}
                </td>
                <td style={styles.td}>{formatDate(sdkKey.created_at)}</td>
                <td style={styles.td}>
                  {!sdkKey.revoked && (
                    <>
                      <button
                        style={styles.actionBtn}
                        onClick={() => handleCopy(sdkKey)}
                      >
                        {copiedId === sdkKey.id ? 'Copied!' : 'Copy'}
                      </button>
                      <button
                        style={styles.revokeBtn}
                        onClick={() => handleRevoke(sdkKey)}
                        disabled={revokeMutation.isPending}
                      >
                        Revoke
                      </button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
