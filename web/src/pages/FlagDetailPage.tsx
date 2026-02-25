import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type {
  Flag,
  Environment,
  FlagEnvironmentConfig,
  Variant,
  TargetingRule,
} from '../api/types.ts'
import VariantEditor from '../components/VariantEditor.tsx'
import RuleBuilder from '../components/RuleBuilder.tsx'

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
  metaCard: {
    padding: 24,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px solid #2a2a4a',
    marginBottom: 24,
  } as const,
  flagName: {
    fontSize: 22,
    fontWeight: 700,
    color: '#ffffff',
    marginBottom: 4,
  } as const,
  flagKey: {
    fontSize: 14,
    fontFamily: 'monospace',
    color: '#e94560',
    marginBottom: 12,
  } as const,
  metaRow: {
    display: 'flex',
    gap: 24,
    flexWrap: 'wrap' as const,
    marginBottom: 8,
  } as const,
  metaLabel: {
    fontSize: 12,
    fontWeight: 600,
    color: '#8892b0',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    marginBottom: 2,
  } as const,
  metaValue: {
    fontSize: 14,
    color: '#e0e0e0',
  } as const,
  typeTag: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 12,
    fontWeight: 600,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    color: '#e94560',
  } as const,
  tag: {
    display: 'inline-block',
    padding: '2px 8px',
    borderRadius: 4,
    fontSize: 12,
    backgroundColor: 'rgba(15, 52, 96, 0.8)',
    color: '#8892b0',
    marginRight: 4,
  } as const,
  description: {
    fontSize: 14,
    color: '#8892b0',
    lineHeight: 1.5,
    marginTop: 8,
  } as const,
  tabs: {
    display: 'flex',
    gap: 0,
    marginBottom: 24,
    borderBottom: '1px solid #2a2a4a',
  } as const,
  tab: (active: boolean) => ({
    padding: '10px 20px',
    fontSize: 14,
    fontWeight: active ? 600 : 400,
    color: active ? '#e94560' : '#8892b0',
    backgroundColor: 'transparent',
    border: 'none',
    borderBottom: active ? '2px solid #e94560' : '2px solid transparent',
    cursor: 'pointer',
    marginBottom: -1,
  }),
  envConfig: {
    padding: 24,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px solid #2a2a4a',
  } as const,
  toggleRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 16,
    marginBottom: 24,
    padding: 16,
    borderRadius: 8,
    backgroundColor: 'rgba(15, 52, 96, 0.3)',
  } as const,
  toggleTrack: (on: boolean) => ({
    width: 52,
    height: 28,
    borderRadius: 14,
    backgroundColor: on ? '#4caf50' : '#555',
    position: 'relative' as const,
    cursor: 'pointer',
    transition: 'background-color 0.2s',
    flexShrink: 0,
  }),
  toggleKnob: (on: boolean) => ({
    width: 22,
    height: 22,
    borderRadius: '50%',
    backgroundColor: '#ffffff',
    position: 'absolute' as const,
    top: 3,
    left: on ? 27 : 3,
    transition: 'left 0.2s',
  }),
  toggleLabel: {
    fontSize: 16,
    fontWeight: 600,
    color: '#e0e0e0',
  } as const,
  section: {
    marginBottom: 24,
  } as const,
  sectionTitle: {
    fontSize: 15,
    fontWeight: 600,
    color: '#e0e0e0',
    marginBottom: 12,
  } as const,
  select: {
    padding: '9px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    cursor: 'pointer',
    minWidth: 160,
  } as const,
  saveBtn: {
    padding: '12px 28px',
    fontSize: 15,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
    marginTop: 8,
  } as const,
  disabledBtn: {
    opacity: 0.6,
    cursor: 'not-allowed',
  } as const,
  successMsg: {
    padding: '10px 16px',
    borderRadius: 6,
    backgroundColor: 'rgba(76, 175, 80, 0.15)',
    border: '1px solid rgba(76, 175, 80, 0.3)',
    color: '#4caf50',
    fontSize: 13,
    marginTop: 12,
  } as const,
  errorMsg: {
    padding: '10px 16px',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    border: '1px solid rgba(233, 69, 96, 0.3)',
    color: '#e94560',
    fontSize: 13,
    marginTop: 12,
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
  noConfig: {
    padding: 32,
    textAlign: 'center' as const,
    color: '#8892b0',
    fontSize: 14,
  } as const,
}

interface FlagDetailResponse {
  flag: Flag
  environment_configs: FlagEnvironmentConfig[]
}

export default function FlagDetailPage() {
  const { key, flag: flagKey } = useParams<{ key: string; flag: string }>()
  const queryClient = useQueryClient()

  const [selectedEnvKey, setSelectedEnvKey] = useState<string>('')
  const [enabled, setEnabled] = useState(false)
  const [defaultVariant, setDefaultVariant] = useState('')
  const [variants, setVariants] = useState<Variant[]>([])
  const [rules, setRules] = useState<TargetingRule[]>([])
  const [saved, setSaved] = useState(false)

  const { data, isLoading, error } = useQuery({
    queryKey: ['projects', key, 'flags', flagKey],
    queryFn: () => api.get<FlagDetailResponse>(`/projects/${key}/flags/${flagKey}`),
    enabled: !!key && !!flagKey,
  })

  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  // Set initial selected env tab
  useEffect(() => {
    if (environments && environments.length > 0 && !selectedEnvKey) {
      setSelectedEnvKey(environments[0].key)
    }
  }, [environments, selectedEnvKey])

  // Load config for selected env
  useEffect(() => {
    if (!data || !environments || !selectedEnvKey) return
    const env = environments.find((e) => e.key === selectedEnvKey)
    if (!env) return
    const cfg = data.environment_configs.find((c) => c.environment_id === env.id)
    if (cfg) {
      setEnabled(cfg.enabled)
      setDefaultVariant(cfg.default_variant)
      setVariants(cfg.variants ?? [])
      setRules(cfg.targeting_rules ?? [])
    } else {
      setEnabled(false)
      setDefaultVariant('')
      setVariants([])
      setRules([])
    }
    setSaved(false)
  }, [data, environments, selectedEnvKey])

  const updateConfig = useMutation({
    mutationFn: (config: {
      enabled: boolean
      default_variant: string
      variants: Variant[]
      targeting_rules: TargetingRule[]
    }) => api.put(`/projects/${key}/flags/${flagKey}/environments/${selectedEnvKey}`, config),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', key, 'flags', flagKey] })
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    },
  })

  const handleSave = () => {
    updateConfig.mutate({
      enabled,
      default_variant: defaultVariant,
      variants,
      targeting_rules: rules,
    })
  }

  if (isLoading) {
    return <div style={styles.loading}>Loading flag details...</div>
  }

  if (error || !data) {
    return (
      <div style={styles.errorBox}>
        Failed to load flag: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  const flag = data.flag

  return (
    <div>
      {/* Breadcrumb */}
      <div style={styles.breadcrumb}>
        <Link to="/projects" style={styles.breadcrumbLink}>
          Projects
        </Link>
        <span>/</span>
        <Link to={`/projects/${key}`} style={styles.breadcrumbLink}>
          {key}
        </Link>
        <span>/</span>
        <span style={{ color: '#e0e0e0' }}>{flagKey}</span>
      </div>

      {/* Flag Metadata */}
      <div style={styles.metaCard}>
        <div style={styles.flagName}>{flag.name}</div>
        <div style={styles.flagKey}>{flag.key}</div>
        <div style={styles.metaRow}>
          <div>
            <div style={styles.metaLabel}>Type</div>
            <span style={styles.typeTag}>{flag.flag_type}</span>
          </div>
          {flag.tags && flag.tags.length > 0 && (
            <div>
              <div style={styles.metaLabel}>Tags</div>
              <div>
                {flag.tags.map((t) => (
                  <span key={t} style={styles.tag}>
                    {t}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
        {flag.description && <div style={styles.description}>{flag.description}</div>}
      </div>

      {/* Environment Tabs */}
      {environments && environments.length > 0 && (
        <>
          <div style={styles.tabs}>
            {environments.map((env) => (
              <button
                key={env.key}
                style={styles.tab(selectedEnvKey === env.key)}
                onClick={() => setSelectedEnvKey(env.key)}
              >
                {env.name}
              </button>
            ))}
          </div>

          {/* Per-environment Config */}
          <div style={styles.envConfig}>
            {/* Enable / Disable toggle */}
            <div style={styles.toggleRow}>
              <div
                style={styles.toggleTrack(enabled)}
                onClick={() => setEnabled(!enabled)}
              >
                <div style={styles.toggleKnob(enabled)} />
              </div>
              <span style={styles.toggleLabel}>
                {enabled ? 'Enabled' : 'Disabled'} in {selectedEnvKey}
              </span>
            </div>

            {/* Default Variant */}
            <div style={styles.section}>
              <div style={styles.sectionTitle}>Default Variant</div>
              {variants.length > 0 ? (
                <select
                  style={styles.select}
                  value={defaultVariant}
                  onChange={(e) => setDefaultVariant(e.target.value)}
                >
                  <option value="">-- Select --</option>
                  {variants.map((v) => (
                    <option key={v.key} value={v.key}>
                      {v.key}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  style={{
                    ...styles.select,
                    cursor: 'text',
                    minWidth: 200,
                  }}
                  placeholder="Variant key"
                  value={defaultVariant}
                  onChange={(e) => setDefaultVariant(e.target.value)}
                />
              )}
            </div>

            {/* Variants */}
            <div style={styles.section}>
              <VariantEditor
                variants={variants}
                flagType={flag.flag_type}
                onChange={setVariants}
              />
            </div>

            {/* Targeting Rules */}
            <div style={styles.section}>
              <RuleBuilder
                rules={rules}
                variants={variants}
                onChange={setRules}
              />
            </div>

            {/* Save */}
            <button
              style={{
                ...styles.saveBtn,
                ...(updateConfig.isPending ? styles.disabledBtn : {}),
              }}
              onClick={handleSave}
              disabled={updateConfig.isPending}
            >
              {updateConfig.isPending ? 'Saving...' : 'Save Configuration'}
            </button>

            {saved && (
              <div style={styles.successMsg}>Configuration saved successfully.</div>
            )}
            {updateConfig.error && (
              <div style={styles.errorMsg}>
                Failed to save: {updateConfig.error instanceof Error ? updateConfig.error.message : 'Unknown error'}
              </div>
            )}
          </div>
        </>
      )}

      {(!environments || environments.length === 0) && (
        <div style={styles.noConfig}>
          No environments found for this project.
        </div>
      )}
    </div>
  )
}
