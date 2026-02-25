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
import { t } from '../theme.ts'
import VariantEditor from '../components/VariantEditor.tsx'
import RuleBuilder from '../components/RuleBuilder.tsx'

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

  useEffect(() => {
    if (environments && environments.length > 0 && !selectedEnvKey) {
      setSelectedEnvKey(environments[0].key)
    }
  }, [environments, selectedEnvKey])

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
    return (
      <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
        Loading flag details...
      </div>
    )
  }

  if (error || !data) {
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
        Failed to load flag: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  const flag = data.flag

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
        <Link to={`/projects/${key}`} style={{ color: t.textSecondary, textDecoration: 'none', transition: 'color 200ms ease' }}
          onMouseEnter={(e) => { e.currentTarget.style.color = t.textPrimary }}
          onMouseLeave={(e) => { e.currentTarget.style.color = t.textSecondary }}
        >
          {key}
        </Link>
        <span style={{ opacity: 0.4 }}>&rsaquo;</span>
        <span style={{ color: t.textPrimary, fontFamily: t.fontMono, fontSize: 12 }}>{flagKey}</span>
      </div>

      {/* Flag Metadata Card */}
      <div
        style={{
          padding: 24,
          borderRadius: t.radiusLg,
          backgroundColor: t.bgSurface,
          border: `1px solid ${t.border}`,
          marginBottom: 24,
        }}
      >
        <div style={{ fontSize: 20, fontWeight: 600, color: t.textPrimary, marginBottom: 4, letterSpacing: '-0.3px' }}>
          {flag.name}
        </div>
        <div style={{ fontSize: 13, fontFamily: t.fontMono, color: t.accent, marginBottom: 14, letterSpacing: '0.2px' }}>
          {flag.key}
        </div>
        <div style={{ display: 'flex', gap: 24, flexWrap: 'wrap', marginBottom: 8 }}>
          <div>
            <div style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', marginBottom: 4, fontFamily: t.fontMono }}>
              Type
            </div>
            <span
              style={{
                display: 'inline-block',
                padding: '2px 10px',
                borderRadius: 4,
                fontSize: 12,
                fontWeight: 500,
                backgroundColor: t.accentSubtle,
                color: t.accent,
                fontFamily: t.fontMono,
              }}
            >
              {flag.flag_type}
            </span>
          </div>
          {flag.tags && flag.tags.length > 0 && (
            <div>
              <div style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', marginBottom: 4, fontFamily: t.fontMono }}>
                Tags
              </div>
              <div style={{ display: 'flex', gap: 4 }}>
                {flag.tags.map((tag) => (
                  <span
                    key={tag}
                    style={{
                      display: 'inline-block',
                      padding: '2px 8px',
                      borderRadius: 4,
                      fontSize: 11,
                      backgroundColor: t.bgElevated,
                      color: t.textSecondary,
                      border: `1px solid ${t.border}`,
                    }}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          )}
        </div>
        {flag.description && (
          <div style={{ fontSize: 13, color: t.textSecondary, lineHeight: 1.6, marginTop: 8 }}>
            {flag.description}
          </div>
        )}
      </div>

      {/* Environment Tabs */}
      {environments && environments.length > 0 && (
        <>
          <div style={{ display: 'flex', gap: 0, marginBottom: 24, borderBottom: `1px solid ${t.border}` }}>
            {environments.map((env) => {
              const isActive = selectedEnvKey === env.key
              return (
                <button
                  key={env.key}
                  style={{
                    padding: '10px 20px',
                    fontSize: 13,
                    fontWeight: isActive ? 500 : 400,
                    color: isActive ? t.accent : t.textSecondary,
                    backgroundColor: 'transparent',
                    border: 'none',
                    borderBottom: `2px solid ${isActive ? t.accent : 'transparent'}`,
                    cursor: 'pointer',
                    marginBottom: -1,
                    fontFamily: t.fontSans,
                    transition: 'all 200ms ease',
                  }}
                  onClick={() => setSelectedEnvKey(env.key)}
                  onMouseEnter={(e) => {
                    if (!isActive) e.currentTarget.style.color = t.textPrimary
                  }}
                  onMouseLeave={(e) => {
                    if (!isActive) e.currentTarget.style.color = t.textSecondary
                  }}
                >
                  {env.name}
                </button>
              )
            })}
          </div>

          {/* Per-environment Config */}
          <div
            style={{
              padding: 24,
              borderRadius: t.radiusLg,
              backgroundColor: t.bgSurface,
              border: `1px solid ${t.border}`,
            }}
          >
            {/* Toggle */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 16,
                marginBottom: 24,
                padding: 16,
                borderRadius: t.radiusMd,
                backgroundColor: t.bgElevated,
                border: `1px solid ${t.border}`,
              }}
            >
              <div
                style={{
                  width: 48,
                  height: 26,
                  borderRadius: 13,
                  backgroundColor: enabled ? t.success : t.textMuted,
                  position: 'relative',
                  cursor: 'pointer',
                  transition: 'background-color 300ms cubic-bezier(0.4, 0, 0.2, 1)',
                  boxShadow: enabled ? `0 0 12px ${t.successBorder}` : 'none',
                  flexShrink: 0,
                }}
                onClick={() => setEnabled(!enabled)}
              >
                <div
                  style={{
                    width: 20,
                    height: 20,
                    borderRadius: '50%',
                    backgroundColor: '#ffffff',
                    position: 'absolute',
                    top: 3,
                    left: enabled ? 25 : 3,
                    transition: 'left 300ms cubic-bezier(0.4, 0, 0.2, 1)',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
                  }}
                />
              </div>
              <span style={{ fontSize: 14, fontWeight: 500, color: t.textPrimary }}>
                {enabled ? 'Enabled' : 'Disabled'} in {selectedEnvKey}
              </span>
            </div>

            {/* Default Variant */}
            <div style={{ marginBottom: 24 }}>
              <div style={{ fontSize: 13, fontWeight: 500, color: t.textPrimary, marginBottom: 10 }}>
                Default Variant
              </div>
              {variants.length > 0 ? (
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
                    minWidth: 160,
                    fontFamily: t.fontSans,
                  }}
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
                    padding: '8px 12px',
                    fontSize: 13,
                    border: `1px solid ${t.border}`,
                    borderRadius: t.radiusMd,
                    backgroundColor: t.bgInput,
                    color: t.textPrimary,
                    outline: 'none',
                    minWidth: 200,
                    fontFamily: t.fontSans,
                    transition: 'border-color 200ms ease, box-shadow 200ms ease',
                  }}
                  placeholder="Variant key"
                  value={defaultVariant}
                  onChange={(e) => setDefaultVariant(e.target.value)}
                  onFocus={(e) => {
                    e.currentTarget.style.borderColor = t.accentBorder
                    e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}`
                  }}
                  onBlur={(e) => {
                    e.currentTarget.style.borderColor = t.border
                    e.currentTarget.style.boxShadow = 'none'
                  }}
                />
              )}
            </div>

            {/* Variants */}
            <div style={{ marginBottom: 24 }}>
              <VariantEditor
                variants={variants}
                flagType={flag.flag_type}
                onChange={setVariants}
              />
            </div>

            {/* Targeting Rules */}
            <div style={{ marginBottom: 24 }}>
              <RuleBuilder
                rules={rules}
                variants={variants}
                onChange={setRules}
              />
            </div>

            {/* Save */}
            <button
              style={{
                padding: '10px 24px',
                fontSize: 13,
                fontWeight: 600,
                border: 'none',
                borderRadius: t.radiusMd,
                background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`,
                color: '#ffffff',
                cursor: updateConfig.isPending ? 'not-allowed' : 'pointer',
                opacity: updateConfig.isPending ? 0.6 : 1,
                fontFamily: t.fontSans,
                transition: 'all 200ms ease',
                boxShadow: '0 2px 10px rgba(212,149,106,0.15)',
              }}
              onClick={handleSave}
              disabled={updateConfig.isPending}
              onMouseEnter={(e) => {
                if (!updateConfig.isPending) {
                  e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'
                  e.currentTarget.style.transform = 'translateY(-1px)'
                }
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              {updateConfig.isPending ? 'Saving...' : 'Save Configuration'}
            </button>

            {saved && (
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: t.radiusMd,
                  backgroundColor: t.successSubtle,
                  border: `1px solid ${t.successBorder}`,
                  color: t.success,
                  fontSize: 13,
                  marginTop: 12,
                  animation: 'fadeIn 200ms ease',
                }}
              >
                Configuration saved successfully.
              </div>
            )}
            {updateConfig.error && (
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: t.radiusMd,
                  backgroundColor: t.dangerSubtle,
                  border: `1px solid ${t.dangerBorder}`,
                  color: t.danger,
                  fontSize: 13,
                  marginTop: 12,
                }}
              >
                Failed to save: {updateConfig.error instanceof Error ? updateConfig.error.message : 'Unknown error'}
              </div>
            )}
          </div>
        </>
      )}

      {(!environments || environments.length === 0) && (
        <div style={{ padding: 32, textAlign: 'center', color: t.textMuted, fontSize: 13 }}>
          No environments found for this project.
        </div>
      )}
    </div>
  )
}
