import { useState, useMemo } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig } from '../api/types.ts'
import { t } from '../theme.ts'
import CreateFlagModal from '../components/CreateFlagModal.tsx'

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

  const allTags = useMemo(() => {
    if (!flags) return []
    const tagSet = new Set<string>()
    flags.forEach((f) => f.tags?.forEach((tag) => tagSet.add(tag)))
    return Array.from(tagSet).sort()
  }, [flags])

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
    return (
      <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
        Loading flags...
      </div>
    )
  }

  if (flagsError) {
    return (
      <div
        style={{
          padding: '14px 18px',
          borderRadius: t.radiusMd,
          backgroundColor: t.dangerSubtle,
          border: `1px solid ${t.dangerBorder}`,
          color: t.danger,
          fontSize: 13,
        }}
      >
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
    <div style={{ animation: 'fadeIn 300ms ease' }}>
      {/* Breadcrumb */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 24, fontSize: 13, color: t.textMuted }}>
        <Link to="/projects" style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}
        >
          Projects
        </Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <span style={{ color: t.textPrimary, fontFamily: t.fontMono, fontSize: 12 }}>{key}</span>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, letterSpacing: '-0.3px' }}>
          {key}
        </h1>
        <button
          style={{
            padding: '9px 18px',
            fontSize: 13,
            fontWeight: 600,
            border: 'none',
            borderRadius: t.radiusMd,
            background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`,
            color: '#ffffff',
            cursor: 'pointer',
            fontFamily: t.fontSans,
            transition: 'all 200ms ease',
            boxShadow: '0 2px 10px rgba(212,149,106,0.15)',
          }}
          onClick={() => setModalOpen(true)}
          onMouseEnter={(e) => {
            e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'
            e.currentTarget.style.transform = 'translateY(-1px)'
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'
            e.currentTarget.style.transform = 'translateY(0)'
          }}
        >
          Create Flag
        </button>
      </div>

      {/* Filters */}
      <div style={{ display: 'flex', gap: 10, marginBottom: 20 }}>
        <input
          style={{
            padding: '8px 14px',
            fontSize: 13,
            border: `1px solid ${t.border}`,
            borderRadius: t.radiusMd,
            backgroundColor: t.bgInput,
            color: t.textPrimary,
            outline: 'none',
            flex: 1,
            maxWidth: 300,
            fontFamily: t.fontSans,
            transition: 'border-color 200ms ease, box-shadow 200ms ease',
          }}
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onFocus={(e) => {
            e.currentTarget.style.borderColor = t.accentBorder
            e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}`
          }}
          onBlur={(e) => {
            e.currentTarget.style.borderColor = t.border
            e.currentTarget.style.boxShadow = 'none'
          }}
        />
        {allTags.length > 0 && (
          <select
            style={{
              padding: '8px 12px',
              fontSize: 13,
              border: `1px solid ${t.border}`,
              borderRadius: t.radiusMd,
              backgroundColor: t.bgInput,
              color: t.textPrimary,
              outline: 'none',
              cursor: 'pointer',
              minWidth: 130,
              fontFamily: t.fontSans,
            }}
            value={tagFilter}
            onChange={(e) => setTagFilter(e.target.value)}
          >
            <option value="">All Tags</option>
            {allTags.map((tag) => (
              <option key={tag} value={tag}>
                {tag}
              </option>
            ))}
          </select>
        )}
      </div>

      {filtered.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 48, color: t.textSecondary }}>
          <div style={{ fontSize: 15, fontWeight: 500, color: t.textPrimary, marginBottom: 6 }}>
            {flags && flags.length > 0 ? 'No flags match your filters' : 'No flags yet'}
          </div>
          <div style={{ fontSize: 13, color: t.textMuted }}>
            {flags && flags.length > 0
              ? 'Try adjusting your search or tag filter.'
              : 'Create your first feature flag to get started.'}
          </div>
        </div>
      ) : (
        <div
          style={{
            borderRadius: t.radiusLg,
            border: `1px solid ${t.border}`,
            overflow: 'hidden',
          }}
        >
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                {['Key', 'Name', 'Type', 'Tags', 'Environments'].map((h) => (
                  <th
                    key={h}
                    style={{
                      textAlign: 'left',
                      padding: '10px 16px',
                      fontSize: 11,
                      fontWeight: 500,
                      color: t.textMuted,
                      textTransform: 'uppercase',
                      letterSpacing: '0.8px',
                      borderBottom: `1px solid ${t.border}`,
                      backgroundColor: t.bgSurface,
                      fontFamily: t.fontMono,
                    }}
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((flag) => (
                <tr
                  key={flag.id}
                  style={{
                    cursor: 'pointer',
                    borderBottom: `1px solid ${t.border}`,
                    transition: 'background-color 200ms ease',
                  }}
                  onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.backgroundColor = t.accentSubtle
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.backgroundColor = 'transparent'
                  }}
                >
                  <td style={{ padding: '12px 16px', fontSize: 13 }}>
                    <span style={{ fontFamily: t.fontMono, fontSize: 12, color: t.accent, letterSpacing: '0.2px' }}>
                      {flag.key}
                    </span>
                  </td>
                  <td style={{ padding: '12px 16px', fontSize: 13, color: t.textPrimary }}>
                    {flag.name}
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <span
                      style={{
                        display: 'inline-block',
                        padding: '2px 8px',
                        borderRadius: 4,
                        fontSize: 11,
                        fontWeight: 500,
                        backgroundColor: t.accentSubtle,
                        color: t.accent,
                        fontFamily: t.fontMono,
                        letterSpacing: '0.2px',
                      }}
                    >
                      {flag.flag_type}
                    </span>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    {flag.tags?.map((tag) => (
                      <span
                        key={tag}
                        style={{
                          display: 'inline-block',
                          padding: '2px 8px',
                          borderRadius: 4,
                          fontSize: 11,
                          backgroundColor: t.bgElevated,
                          color: t.textSecondary,
                          marginRight: 4,
                          border: `1px solid ${t.border}`,
                        }}
                      >
                        {tag}
                      </span>
                    ))}
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    {environments?.map((env) => (
                      <span key={env.id} style={{ whiteSpace: 'nowrap', marginRight: 8, display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                        <span
                          style={{
                            display: 'inline-block',
                            width: 7,
                            height: 7,
                            borderRadius: '50%',
                            backgroundColor: getEnvStatus(flag.key, env.id) ? t.success : t.textMuted,
                            boxShadow: getEnvStatus(flag.key, env.id) ? `0 0 6px ${t.successBorder}` : 'none',
                            transition: 'all 300ms ease',
                          }}
                        />
                        <span style={{ fontSize: 11, color: t.textMuted }}>{env.name}</span>
                      </span>
                    ))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CreateFlagModal
        open={modalOpen}
        projectKey={key!}
        onClose={() => setModalOpen(false)}
      />
    </div>
  )
}
