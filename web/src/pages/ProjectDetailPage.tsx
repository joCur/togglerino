import { useState, useMemo } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig } from '../api/types.ts'
import CreateFlagModal from '../components/CreateFlagModal.tsx'

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
  controls: {
    display: 'flex',
    gap: 12,
    marginBottom: 20,
  } as const,
  searchInput: {
    padding: '9px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    flex: 1,
    maxWidth: 320,
  } as const,
  tagSelect: {
    padding: '9px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    cursor: 'pointer',
    minWidth: 140,
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
    cursor: 'pointer',
    borderBottom: '1px solid rgba(42, 42, 74, 0.5)',
  } as const,
  td: {
    padding: '12px 16px',
    fontSize: 14,
    color: '#e0e0e0',
  } as const,
  flagKey: {
    fontFamily: 'monospace',
    fontSize: 13,
    color: '#e94560',
  } as const,
  flagType: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 11,
    fontWeight: 600,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    color: '#e94560',
  } as const,
  tag: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 11,
    backgroundColor: 'rgba(15, 52, 96, 0.8)',
    color: '#8892b0',
    marginRight: 4,
  } as const,
  envDot: (enabled: boolean) => ({
    display: 'inline-block',
    width: 10,
    height: 10,
    borderRadius: '50%',
    backgroundColor: enabled ? '#4caf50' : '#555',
    marginRight: 6,
  }),
  envLabel: {
    fontSize: 11,
    color: '#8892b0',
    marginRight: 12,
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
}

export default function ProjectDetailPage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [modalOpen, setModalOpen] = useState(false)

  const { data: flags, isLoading: flagsLoading, error: flagsError } = useQuery({
    queryKey: ['projects', key, 'flags'],
    queryFn: () => api.get<Flag[]>(`/projects/${key}/flags`),
    enabled: !!key,
  })

  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  // Fetch all flag configs for environment status display
  const { data: allConfigs } = useQuery({
    queryKey: ['projects', key, 'all-configs'],
    queryFn: async () => {
      if (!flags || flags.length === 0) return {}
      const configMap: Record<string, FlagEnvironmentConfig[]> = {}
      await Promise.all(
        flags.map(async (flag) => {
          try {
            const resp = await api.get<{ flag: Flag; environment_configs: FlagEnvironmentConfig[] }>(
              `/projects/${key}/flags/${flag.key}`
            )
            configMap[flag.key] = resp.environment_configs
          } catch {
            configMap[flag.key] = []
          }
        })
      )
      return configMap
    },
    enabled: !!flags && flags.length > 0,
  })

  // Collect all unique tags
  const allTags = useMemo(() => {
    if (!flags) return []
    const tagSet = new Set<string>()
    flags.forEach((f) => f.tags?.forEach((t) => tagSet.add(t)))
    return Array.from(tagSet).sort()
  }, [flags])

  // Filter flags
  const filtered = useMemo(() => {
    if (!flags) return []
    return flags.filter((f) => {
      const matchesSearch =
        !search ||
        f.key.toLowerCase().includes(search.toLowerCase()) ||
        f.name.toLowerCase().includes(search.toLowerCase())
      const matchesTag = !tagFilter || (f.tags && f.tags.includes(tagFilter))
      return matchesSearch && matchesTag
    })
  }, [flags, search, tagFilter])

  if (flagsLoading) {
    return <div style={styles.loading}>Loading flags...</div>
  }

  if (flagsError) {
    return (
      <div style={styles.errorBox}>
        Failed to load flags: {flagsError instanceof Error ? flagsError.message : 'Unknown error'}
      </div>
    )
  }

  const getEnvStatus = (flagKey: string, envId: string): boolean => {
    if (!allConfigs || !allConfigs[flagKey]) return false
    const cfg = allConfigs[flagKey].find((c) => c.environment_id === envId)
    return cfg?.enabled ?? false
  }

  return (
    <div>
      <div style={styles.breadcrumb}>
        <Link to="/projects" style={styles.breadcrumbLink}>
          Projects
        </Link>
        <span>/</span>
        <span style={{ color: '#e0e0e0' }}>{key}</span>
      </div>

      <div style={styles.header}>
        <h1 style={styles.title}>{key}</h1>
        <button style={styles.createBtn} onClick={() => setModalOpen(true)}>
          Create Flag
        </button>
      </div>

      <div style={styles.controls}>
        <input
          style={styles.searchInput}
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {allTags.length > 0 && (
          <select
            style={styles.tagSelect}
            value={tagFilter}
            onChange={(e) => setTagFilter(e.target.value)}
          >
            <option value="">All Tags</option>
            {allTags.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
      </div>

      {filtered.length === 0 ? (
        <div style={styles.empty}>
          <div style={styles.emptyTitle}>
            {flags && flags.length > 0 ? 'No flags match your filters' : 'No flags yet'}
          </div>
          <div style={{ fontSize: 14, color: '#8892b0' }}>
            {flags && flags.length > 0
              ? 'Try adjusting your search or tag filter.'
              : 'Create your first feature flag to get started.'}
          </div>
        </div>
      ) : (
        <table style={styles.table}>
          <thead>
            <tr>
              <th style={styles.th}>Key</th>
              <th style={styles.th}>Name</th>
              <th style={styles.th}>Type</th>
              <th style={styles.th}>Tags</th>
              <th style={styles.th}>Environments</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((flag) => (
              <tr
                key={flag.id}
                style={styles.tr}
                onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLTableRowElement).style.backgroundColor =
                    'rgba(233, 69, 96, 0.05)'
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLTableRowElement).style.backgroundColor = 'transparent'
                }}
              >
                <td style={styles.td}>
                  <span style={styles.flagKey}>{flag.key}</span>
                </td>
                <td style={styles.td}>{flag.name}</td>
                <td style={styles.td}>
                  <span style={styles.flagType}>{flag.flag_type}</span>
                </td>
                <td style={styles.td}>
                  {flag.tags?.map((t) => (
                    <span key={t} style={styles.tag}>
                      {t}
                    </span>
                  ))}
                </td>
                <td style={styles.td}>
                  {environments?.map((env) => (
                    <span key={env.id} style={{ whiteSpace: 'nowrap', marginRight: 4 }}>
                      <span style={styles.envDot(getEnvStatus(flag.key, env.id))} />
                      <span style={styles.envLabel}>{env.name}</span>
                    </span>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      <CreateFlagModal
        open={modalOpen}
        projectKey={key!}
        onClose={() => setModalOpen(false)}
      />
    </div>
  )
}
