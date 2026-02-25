import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { AuditEntry } from '../api/types.ts'

const PAGE_SIZE = 50

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = now - date

  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`

  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} day${days === 1 ? '' : 's'} ago`

  return new Date(dateStr).toLocaleDateString()
}

function formatAction(action: string): string {
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
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
    marginBottom: 24,
  } as const,
  title: {
    fontSize: 24,
    fontWeight: 700,
    color: '#ffffff',
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
    verticalAlign: 'top' as const,
  } as const,
  time: {
    fontSize: 13,
    color: '#8892b0',
    whiteSpace: 'nowrap' as const,
  } as const,
  action: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 12,
    fontWeight: 600,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    color: '#e94560',
  } as const,
  entityType: {
    fontSize: 12,
    color: '#8892b0',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.3px',
  } as const,
  entityId: {
    fontFamily: 'monospace',
    fontSize: 13,
    color: '#e94560',
  } as const,
  userId: {
    fontSize: 13,
    color: '#e0e0e0',
    fontFamily: 'monospace',
  } as const,
  details: {
    fontSize: 12,
    color: '#8892b0',
    maxWidth: 200,
    overflow: 'hidden' as const,
    textOverflow: 'ellipsis' as const,
    whiteSpace: 'nowrap' as const,
  } as const,
  loadMoreBtn: {
    display: 'block',
    margin: '24px auto 0',
    padding: '10px 28px',
    fontSize: 14,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
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
  countInfo: {
    textAlign: 'center' as const,
    fontSize: 13,
    color: '#8892b0',
    marginTop: 16,
  } as const,
}

export default function AuditLogPage() {
  const { key } = useParams<{ key: string }>()
  const [offset, setOffset] = useState(0)
  const [allEntries, setAllEntries] = useState<AuditEntry[]>([])
  const [hasMore, setHasMore] = useState(true)

  const { isLoading, error } = useQuery({
    queryKey: ['projects', key, 'audit-log', offset],
    queryFn: async () => {
      const entries = await api.get<AuditEntry[]>(
        `/projects/${key}/audit-log?limit=${PAGE_SIZE}&offset=${offset}`
      )
      if (offset === 0) {
        setAllEntries(entries)
      } else {
        setAllEntries((prev) => [...prev, ...entries])
      }
      setHasMore(entries.length === PAGE_SIZE)
      return entries
    },
    enabled: !!key,
  })

  const handleLoadMore = () => {
    setOffset((prev) => prev + PAGE_SIZE)
  }

  if (isLoading && allEntries.length === 0) {
    return <div style={styles.loading}>Loading audit log...</div>
  }

  if (error && allEntries.length === 0) {
    return (
      <div style={styles.errorBox}>
        Failed to load audit log: {error instanceof Error ? error.message : 'Unknown error'}
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
        <span style={{ color: '#e0e0e0' }}>Audit Log</span>
      </div>

      <div style={styles.header}>
        <h1 style={styles.title}>Audit Log</h1>
      </div>

      {allEntries.length === 0 ? (
        <div style={styles.empty}>
          <div style={styles.emptyTitle}>No audit log entries</div>
          <div style={{ fontSize: 14, color: '#8892b0' }}>
            Activity in this project will be recorded here.
          </div>
        </div>
      ) : (
        <>
          <table style={styles.table}>
            <thead>
              <tr>
                <th style={styles.th}>Time</th>
                <th style={styles.th}>User</th>
                <th style={styles.th}>Action</th>
                <th style={styles.th}>Entity Type</th>
                <th style={styles.th}>Entity</th>
                <th style={styles.th}>Details</th>
              </tr>
            </thead>
            <tbody>
              {allEntries.map((entry) => (
                <tr key={entry.id} style={styles.tr}>
                  <td style={styles.td}>
                    <span style={styles.time} title={new Date(entry.created_at).toISOString()}>
                      {formatRelativeTime(entry.created_at)}
                    </span>
                  </td>
                  <td style={styles.td}>
                    <span style={styles.userId}>
                      {entry.user_id ? entry.user_id.slice(0, 8) + '...' : '--'}
                    </span>
                  </td>
                  <td style={styles.td}>
                    <span style={styles.action}>{formatAction(entry.action)}</span>
                  </td>
                  <td style={styles.td}>
                    <span style={styles.entityType}>{entry.entity_type}</span>
                  </td>
                  <td style={styles.td}>
                    <span style={styles.entityId}>{entry.entity_id.slice(0, 12)}</span>
                  </td>
                  <td style={styles.td}>
                    <span style={styles.details} title={
                      entry.new_value ? JSON.stringify(entry.new_value) : undefined
                    }>
                      {entry.new_value
                        ? JSON.stringify(entry.new_value).slice(0, 50)
                        : '--'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div style={styles.countInfo}>
            Showing {allEntries.length} entries
          </div>

          {hasMore && (
            <button
              style={{
                ...styles.loadMoreBtn,
                opacity: isLoading ? 0.7 : 1,
              }}
              onClick={handleLoadMore}
              disabled={isLoading}
            >
              {isLoading ? 'Loading...' : 'Load More'}
            </button>
          )}
        </>
      )}
    </div>
  )
}
