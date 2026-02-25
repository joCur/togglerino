import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag } from '../api/types.ts'
import { t } from '../theme.ts'

interface Props {
  open: boolean
  projectKey: string
  onClose: () => void
}

const FLAG_TYPES = [
  { value: 'boolean', label: 'Boolean' },
  { value: 'string', label: 'String' },
  { value: 'number', label: 'Number' },
  { value: 'json', label: 'JSON' },
]

function slugify(text: string): string {
  return text.toLowerCase().trim().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
}

const inputBase = {
  width: '100%',
  padding: '10px 14px',
  fontSize: 13,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusMd,
  backgroundColor: t.bgInput,
  color: t.textPrimary,
  outline: 'none',
  marginBottom: 18,
  fontFamily: t.fontSans,
  transition: 'border-color 200ms ease, box-shadow 200ms ease',
} as const

const handleFocus = (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
  e.currentTarget.style.borderColor = t.accentBorder
  e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}`
}

const handleBlur = (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => {
  e.currentTarget.style.borderColor = t.border
  e.currentTarget.style.boxShadow = 'none'
}

const labelStyle = {
  display: 'block',
  fontSize: 12,
  fontWeight: 500,
  color: t.textSecondary,
  marginBottom: 6,
  letterSpacing: '0.3px',
} as const

export default function CreateFlagModal({ open, projectKey, onClose }: Props) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const [keyManual, setKeyManual] = useState(false)
  const [description, setDescription] = useState('')
  const [flagType, setFlagType] = useState('boolean')
  const [defaultValue, setDefaultValue] = useState<string>('false')
  const [boolValue, setBoolValue] = useState(false)
  const [tags, setTags] = useState('')

  const mutation = useMutation({
    mutationFn: (data: {
      key: string; name: string; description: string
      flag_type: string; default_value: unknown; tags: string[]
    }) => api.post<Flag>(`/projects/${projectKey}/flags`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags'] })
      resetAndClose()
    },
  })

  const resetAndClose = () => {
    setName(''); setKey(''); setKeyManual(false); setDescription('')
    setFlagType('boolean'); setDefaultValue('false'); setBoolValue(false); setTags('')
    mutation.reset(); onClose()
  }

  const handleNameChange = (val: string) => {
    setName(val)
    if (!keyManual) setKey(slugify(val))
  }

  const handleKeyChange = (val: string) => { setKeyManual(true); setKey(val) }

  const handleTypeChange = (type: string) => {
    setFlagType(type)
    if (type === 'boolean') { setDefaultValue('false'); setBoolValue(false) }
    else if (type === 'number') setDefaultValue('0')
    else if (type === 'json') setDefaultValue('{}')
    else setDefaultValue('')
  }

  const getDefaultValueParsed = (): unknown => {
    if (flagType === 'boolean') return boolValue
    if (flagType === 'number') { const n = Number(defaultValue); return isNaN(n) ? 0 : n }
    if (flagType === 'json') { try { return JSON.parse(defaultValue) } catch { return defaultValue } }
    return defaultValue
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const parsedTags = tags.split(',').map((tag) => tag.trim()).filter(Boolean)
    mutation.mutate({ key, name, description, flag_type: flagType, default_value: getDefaultValueParsed(), tags: parsedTags })
  }

  if (!open) return null

  const errorMsg = mutation.error instanceof Error ? mutation.error.message : ''

  return (
    <div
      style={{ position: 'fixed', inset: 0, display: 'flex', justifyContent: 'center', alignItems: 'center', backgroundColor: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)', zIndex: 1000, animation: 'overlayIn 200ms ease' }}
      onClick={resetAndClose}
    >
      <div
        style={{
          width: '100%', maxWidth: 500, padding: 32, borderRadius: t.radiusXl,
          backgroundColor: t.bgSurface, border: `1px solid ${t.borderStrong}`,
          boxShadow: '0 16px 48px rgba(0,0,0,0.5)', maxHeight: '90vh', overflowY: 'auto',
          animation: 'modalIn 250ms ease',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ fontSize: 18, fontWeight: 600, color: t.textPrimary, marginBottom: 24, letterSpacing: '-0.2px' }}>
          Create Flag
        </h2>
        <form onSubmit={handleSubmit}>
          {errorMsg && (
            <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginBottom: 18 }}>{errorMsg}</div>
          )}

          <label style={labelStyle}>Name</label>
          <input style={inputBase} value={name} onChange={(e) => handleNameChange(e.target.value)} placeholder="Dark Mode" required autoFocus onFocus={handleFocus} onBlur={handleBlur} />

          <label style={labelStyle}>Key</label>
          <input style={{ ...inputBase, fontFamily: t.fontMono, fontSize: 12 }} value={key} onChange={(e) => handleKeyChange(e.target.value)} placeholder="dark-mode" required onFocus={handleFocus} onBlur={handleBlur} />

          <label style={labelStyle}>Description</label>
          <textarea style={{ ...inputBase, minHeight: 72, resize: 'vertical' }} value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Optional description" onFocus={handleFocus as unknown as React.FocusEventHandler<HTMLTextAreaElement>} onBlur={handleBlur as unknown as React.FocusEventHandler<HTMLTextAreaElement>} />

          <label style={labelStyle}>Type</label>
          <select
            style={{ ...inputBase, cursor: 'pointer' }}
            value={flagType}
            onChange={(e) => handleTypeChange(e.target.value)}
            onFocus={handleFocus as unknown as React.FocusEventHandler<HTMLSelectElement>}
            onBlur={handleBlur as unknown as React.FocusEventHandler<HTMLSelectElement>}
          >
            {FLAG_TYPES.map((ft) => <option key={ft.value} value={ft.value}>{ft.label}</option>)}
          </select>

          <label style={labelStyle}>Default Value</label>
          {flagType === 'boolean' ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 18 }}>
              <div
                style={{
                  width: 44, height: 24, borderRadius: 12,
                  backgroundColor: boolValue ? t.accent : t.textMuted,
                  position: 'relative', cursor: 'pointer',
                  transition: 'background-color 300ms cubic-bezier(0.4,0,0.2,1)',
                  flexShrink: 0,
                }}
                onClick={() => setBoolValue(!boolValue)}
              >
                <div style={{
                  width: 18, height: 18, borderRadius: '50%', backgroundColor: '#ffffff',
                  position: 'absolute', top: 3, left: boolValue ? 23 : 3,
                  transition: 'left 300ms cubic-bezier(0.4,0,0.2,1)',
                  boxShadow: '0 1px 2px rgba(0,0,0,0.3)',
                }} />
              </div>
              <span style={{ fontSize: 13, color: t.textPrimary, fontFamily: t.fontMono }}>{boolValue ? 'true' : 'false'}</span>
            </div>
          ) : flagType === 'number' ? (
            <input style={inputBase} type="number" value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} onFocus={handleFocus} onBlur={handleBlur} />
          ) : flagType === 'json' ? (
            <textarea style={{ ...inputBase, fontFamily: t.fontMono, fontSize: 12, minHeight: 72 }} value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} placeholder='{"key": "value"}' onFocus={handleFocus as unknown as React.FocusEventHandler<HTMLTextAreaElement>} onBlur={handleBlur as unknown as React.FocusEventHandler<HTMLTextAreaElement>} />
          ) : (
            <input style={inputBase} value={defaultValue} onChange={(e) => setDefaultValue(e.target.value)} placeholder="Default string value" onFocus={handleFocus} onBlur={handleBlur} />
          )}

          <label style={labelStyle}>Tags (comma-separated)</label>
          <input style={inputBase} value={tags} onChange={(e) => setTags(e.target.value)} placeholder="ui, experiment, beta" onFocus={handleFocus} onBlur={handleBlur} />

          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 10, marginTop: 4 }}>
            <button
              type="button"
              style={{
                padding: '9px 16px', fontSize: 13, fontWeight: 500,
                border: `1px solid ${t.border}`, borderRadius: t.radiusMd,
                backgroundColor: 'transparent', color: t.textSecondary, cursor: 'pointer',
                fontFamily: t.fontSans, transition: 'all 200ms ease',
              }}
              onClick={resetAndClose}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = t.borderHover; e.currentTarget.style.color = t.textPrimary }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.color = t.textSecondary }}
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              style={{
                padding: '9px 16px', fontSize: 13, fontWeight: 600, border: 'none', borderRadius: t.radiusMd,
                background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`, color: '#ffffff',
                cursor: mutation.isPending ? 'not-allowed' : 'pointer',
                opacity: mutation.isPending ? 0.6 : 1, fontFamily: t.fontSans,
                transition: 'all 200ms ease', boxShadow: '0 2px 10px rgba(212,149,106,0.15)',
              }}
              onMouseEnter={(e) => { if (!mutation.isPending) e.currentTarget.style.boxShadow = '0 4px 16px rgba(212,149,106,0.3)' }}
              onMouseLeave={(e) => { e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)' }}
            >
              {mutation.isPending ? 'Creating...' : 'Create Flag'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
