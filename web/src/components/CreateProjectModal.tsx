import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { t } from '../theme.ts'

interface Props {
  open: boolean
  onClose: () => void
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

const inputStyle = {
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

const handleFocus = (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => {
  e.currentTarget.style.borderColor = t.accentBorder
  e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}`
}

const handleBlur = (e: React.FocusEvent<HTMLInputElement | HTMLTextAreaElement>) => {
  e.currentTarget.style.borderColor = t.border
  e.currentTarget.style.boxShadow = 'none'
}

export default function CreateProjectModal({ open, onClose }: Props) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [key, setKey] = useState('')
  const [keyManual, setKeyManual] = useState(false)
  const [description, setDescription] = useState('')

  const mutation = useMutation({
    mutationFn: (data: { key: string; name: string; description: string }) =>
      api.post<Project>('/projects', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      resetAndClose()
    },
  })

  const resetAndClose = () => {
    setName('')
    setKey('')
    setKeyManual(false)
    setDescription('')
    mutation.reset()
    onClose()
  }

  const handleNameChange = (val: string) => {
    setName(val)
    if (!keyManual) {
      setKey(slugify(val))
    }
  }

  const handleKeyChange = (val: string) => {
    setKeyManual(true)
    setKey(val)
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    mutation.mutate({ key, name, description })
  }

  if (!open) return null

  const errorMsg = mutation.error instanceof Error ? mutation.error.message : ''

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        backgroundColor: 'rgba(0,0,0,0.6)',
        backdropFilter: 'blur(4px)',
        zIndex: 1000,
        animation: 'overlayIn 200ms ease',
      }}
      onClick={resetAndClose}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 460,
          padding: 32,
          borderRadius: t.radiusXl,
          backgroundColor: t.bgSurface,
          border: `1px solid ${t.borderStrong}`,
          boxShadow: '0 16px 48px rgba(0,0,0,0.5)',
          animation: 'modalIn 250ms ease',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 style={{ fontSize: 18, fontWeight: 600, color: t.textPrimary, marginBottom: 24, letterSpacing: '-0.2px' }}>
          Create Project
        </h2>
        <form onSubmit={handleSubmit}>
          {errorMsg && (
            <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginBottom: 18 }}>
              {errorMsg}
            </div>
          )}

          <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: t.textSecondary, marginBottom: 6, letterSpacing: '0.3px' }}>Name</label>
          <input style={inputStyle} value={name} onChange={(e) => handleNameChange(e.target.value)} placeholder="My Project" required autoFocus onFocus={handleFocus} onBlur={handleBlur} />

          <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: t.textSecondary, marginBottom: 6, letterSpacing: '0.3px' }}>Key</label>
          <input style={{ ...inputStyle, fontFamily: t.fontMono, fontSize: 12 }} value={key} onChange={(e) => handleKeyChange(e.target.value)} placeholder="my-project" required onFocus={handleFocus} onBlur={handleBlur} />

          <label style={{ display: 'block', fontSize: 12, fontWeight: 500, color: t.textSecondary, marginBottom: 6, letterSpacing: '0.3px' }}>Description</label>
          <textarea
            style={{
              ...inputStyle,
              minHeight: 80,
              resize: 'vertical',
            }}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description"
            onFocus={handleFocus as unknown as React.FocusEventHandler<HTMLTextAreaElement>}
            onBlur={handleBlur as unknown as React.FocusEventHandler<HTMLTextAreaElement>}
          />

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
              onMouseEnter={(e) => { if (!mutation.isPending) { e.currentTarget.style.boxShadow = '0 4px 16px rgba(212,149,106,0.3)' } }}
              onMouseLeave={(e) => { e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)' }}
            >
              {mutation.isPending ? 'Creating...' : 'Create Project'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
