import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment } from '../api/types.ts'

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
    minWidth: 180,
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
  envKey: {
    fontFamily: 'monospace',
    fontSize: 13,
    color: '#e94560',
  } as const,
  linkStyle: {
    color: '#e94560',
    textDecoration: 'none',
    fontSize: 13,
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
    return <div style={styles.loading}>Loading environments...</div>
  }

  if (error) {
    return (
      <div style={styles.errorBox}>
        Failed to load environments: {error instanceof Error ? error.message : 'Unknown error'}
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
        <span style={{ color: '#e0e0e0' }}>Environments</span>
      </div>

      <div style={styles.header}>
        <h1 style={styles.title}>Environments</h1>
        {!showForm && (
          <button style={styles.createBtn} onClick={() => setShowForm(true)}>
            Create Environment
          </button>
        )}
      </div>

      {showForm && (
        <form style={styles.form} onSubmit={handleCreate}>
          <div style={styles.formField}>
            <label style={styles.formLabel}>Key</label>
            <input
              style={styles.input}
              placeholder="e.g. staging"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
              autoFocus
            />
          </div>
          <div style={styles.formField}>
            <label style={styles.formLabel}>Name</label>
            <input
              style={styles.input}
              placeholder="e.g. Staging"
              value={envName}
              onChange={(e) => setEnvName(e.target.value)}
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
            {createMutation.isPending ? 'Creating...' : 'Create'}
          </button>
          <button
            type="button"
            style={styles.cancelBtn}
            onClick={() => {
              setShowForm(false)
              setEnvKey('')
              setEnvName('')
              createMutation.reset()
            }}
          >
            Cancel
          </button>
        </form>
      )}

      {createMutation.error && (
        <div style={styles.formError}>
          {createMutation.error instanceof Error ? createMutation.error.message : 'Failed to create environment'}
        </div>
      )}

      {(!environments || environments.length === 0) ? (
        <div style={styles.empty}>
          <div style={styles.emptyTitle}>No environments yet</div>
          <div style={{ fontSize: 14, color: '#8892b0' }}>
            Create your first environment to start configuring feature flags per environment.
          </div>
        </div>
      ) : (
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>Key</th>
              <th style={styles.th}>Name</th>
              <th style={styles.th}>Created</th>
              <th style={styles.th}>SDK Keys</th>
            </tr>
          </thead>
          <tbody>
            {environments.map((env) => (
              <tr key={env.id} style={styles.tr}>
                <td style={styles.td}>
                  <span style={styles.envKey}>{env.key}</span>
                </td>
                <td style={styles.td}>{env.name}</td>
                <td style={styles.td}>
                  {new Date(env.created_at).toLocaleDateString()}
                </td>
                <td style={styles.td}>
                  <Link
                    to={`/projects/${key}/environments/${env.key}/sdk-keys`}
                    style={styles.linkStyle}
                  >
                    Manage SDK Keys
                  </Link>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
