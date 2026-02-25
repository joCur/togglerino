import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { AuditEntry } from '../api/types.ts'
import { t } from '../theme.ts'

const PAGE_SIZE = 50

function formatRelativeTime(dateStr: string): string {
  const now = Date.now()
  const date = new Date(dateStr).getTime()
  const diff = now - date
  const seconds = Math.floor(diff / 1000)
  if (seconds < 60) return 'just now'
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 30) return `${days}d ago`
  return new Date(dateStr).toLocaleDateString()
}

function formatAction(action: string): string {
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
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
    return <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>Loading audit log...</div>
  }

  if (error && allEntries.length === 0) {
    return <div style={{ padding: '14px 18px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13 }}>Failed to load audit log: {error instanceof Error ? error.message : 'Unknown error'}</div>
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
        <span style={{ color: t.textPrimary }}>Audit Log</span>
      </div>

      <div style={{ marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, letterSpacing: '-0.3px' }}>Audit Log</h1>
      </div>

      {allEntries.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 48, color: t.textSecondary }}>
          <div style={{ fontSize: 15, fontWeight: 500, color: t.textPrimary, marginBottom: 6 }}>No audit log entries</div>
          <div style={{ fontSize: 13, color: t.textMuted }}>Activity in this project will be recorded here.</div>
        </div>
      ) : (
        <>
          <div style={{ borderRadius: t.radiusLg, border: `1px solid ${t.border}`, overflow: 'hidden' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  {['Time', 'User', 'Action', 'Entity Type', 'Entity', 'Details'].map((h) => (
                    <th key={h} style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', borderBottom: `1px solid ${t.border}`, backgroundColor: t.bgSurface, fontFamily: t.fontMono }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {allEntries.map((entry) => (
                  <tr key={entry.id} style={{ borderBottom: `1px solid ${t.border}`, transition: 'background-color 200ms ease' }}
                    onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.accentSubtle }}
                    onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}>
                    <td style={{ padding: '12px 16px' }}>
                      <span style={{ fontSize: 12, color: t.textSecondary, whiteSpace: 'nowrap', fontFamily: t.fontMono }} title={new Date(entry.created_at).toISOString()}>
                        {formatRelativeTime(entry.created_at)}
                      </span>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span style={{ fontSize: 12, color: t.textSecondary, fontFamily: t.fontMono }}>
                        {entry.user_id ? entry.user_id.slice(0, 8) + '...' : '--'}
                      </span>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span style={{ display: 'inline-block', padding: '2px 8px', borderRadius: 4, fontSize: 11, fontWeight: 500, backgroundColor: t.accentSubtle, color: t.accent, fontFamily: t.fontMono }}>
                        {formatAction(entry.action)}
                      </span>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span style={{ fontSize: 11, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.3px', fontFamily: t.fontMono }}>{entry.entity_type}</span>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span style={{ fontFamily: t.fontMono, fontSize: 12, color: t.accent }}>{entry.entity_id.slice(0, 12)}</span>
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span
                        style={{ fontSize: 12, color: t.textMuted, maxWidth: 200, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', display: 'block' }}
                        title={entry.new_value ? JSON.stringify(entry.new_value) : undefined}
                      >
                        {entry.new_value ? JSON.stringify(entry.new_value).slice(0, 50) : '--'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div style={{ textAlign: 'center', fontSize: 12, color: t.textMuted, marginTop: 16 }}>
            Showing {allEntries.length} entries
          </div>

          {hasMore && (
            <button
              style={{
                display: 'block', margin: '16px auto 0', padding: '9px 24px', fontSize: 13, fontWeight: 500,
                border: `1px solid ${t.border}`, borderRadius: t.radiusMd, backgroundColor: 'transparent',
                color: t.textSecondary, cursor: 'pointer', fontFamily: t.fontSans, transition: 'all 200ms ease',
                opacity: isLoading ? 0.7 : 1,
              }}
              onClick={handleLoadMore}
              disabled={isLoading}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.borderHover; e.currentTarget.style.color = t.textPrimary }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = t.textSecondary }}
            >
              {isLoading ? 'Loading...' : 'Load More'}
            </button>
          )}
        </>
      )}
    </div>
  )
}
