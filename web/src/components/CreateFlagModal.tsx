import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag } from '../api/types.ts'

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

const styles = {
  overlay: {
    position: 'fixed',
    inset: 0,
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: 'rgba(0, 0, 0, 0.6)',
    zIndex: 1000,
  } as const,
  modal: {
    width: '100%',
    maxWidth: 500,
    padding: 32,
    borderRadius: 12,
    backgroundColor: '#16213e',
    boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
    maxHeight: '90vh',
    overflowY: 'auto' as const,
  } as const,
  heading: {
    fontSize: 20,
    fontWeight: 700,
    color: '#ffffff',
    marginBottom: 24,
  } as const,
  label: {
    display: 'block',
    fontSize: 13,
    fontWeight: 500,
    color: '#8892b0',
    marginBottom: 6,
  } as const,
  input: {
    width: '100%',
    padding: '10px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    marginBottom: 16,
  } as const,
  textarea: {
    width: '100%',
    padding: '10px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    marginBottom: 16,
    minHeight: 80,
    resize: 'vertical' as const,
    fontFamily: 'inherit',
  } as const,
  select: {
    width: '100%',
    padding: '10px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    marginBottom: 16,
    cursor: 'pointer',
  } as const,
  toggle: {
    display: 'flex',
    alignItems: 'center',
    gap: 10,
    marginBottom: 16,
  } as const,
  toggleTrack: (on: boolean) => ({
    width: 44,
    height: 24,
    borderRadius: 12,
    backgroundColor: on ? '#e94560' : '#2a2a4a',
    position: 'relative' as const,
    cursor: 'pointer',
    transition: 'background-color 0.2s',
    flexShrink: 0,
  }),
  toggleKnob: (on: boolean) => ({
    width: 18,
    height: 18,
    borderRadius: '50%',
    backgroundColor: '#ffffff',
    position: 'absolute' as const,
    top: 3,
    left: on ? 23 : 3,
    transition: 'left 0.2s',
  }),
  toggleLabel: {
    fontSize: 14,
    color: '#e0e0e0',
  } as const,
  actions: {
    display: 'flex',
    justifyContent: 'flex-end',
    gap: 12,
    marginTop: 8,
  } as const,
  cancelBtn: {
    padding: '10px 18px',
    fontSize: 14,
    fontWeight: 500,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: 'transparent',
    color: '#8892b0',
    cursor: 'pointer',
  } as const,
  createBtn: {
    padding: '10px 18px',
    fontSize: 14,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
  } as const,
  disabledBtn: {
    opacity: 0.6,
    cursor: 'not-allowed',
  } as const,
  error: {
    padding: '10px 12px',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    border: '1px solid rgba(233, 69, 96, 0.3)',
    color: '#e94560',
    fontSize: 13,
    marginBottom: 16,
  } as const,
}

function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

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
      key: string
      name: string
      description: string
      flag_type: string
      default_value: unknown
      tags: string[]
    }) => api.post<Flag>(`/projects/${projectKey}/flags`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags'] })
      resetAndClose()
    },
  })

  const resetAndClose = () => {
    setName('')
    setKey('')
    setKeyManual(false)
    setDescription('')
    setFlagType('boolean')
    setDefaultValue('false')
    setBoolValue(false)
    setTags('')
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

  const handleTypeChange = (type: string) => {
    setFlagType(type)
    if (type === 'boolean') {
      setDefaultValue('false')
      setBoolValue(false)
    } else if (type === 'number') {
      setDefaultValue('0')
    } else if (type === 'json') {
      setDefaultValue('{}')
    } else {
      setDefaultValue('')
    }
  }

  const getDefaultValueParsed = (): unknown => {
    if (flagType === 'boolean') return boolValue
    if (flagType === 'number') {
      const n = Number(defaultValue)
      return isNaN(n) ? 0 : n
    }
    if (flagType === 'json') {
      try {
        return JSON.parse(defaultValue)
      } catch {
        return defaultValue
      }
    }
    return defaultValue
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const parsedTags = tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
    mutation.mutate({
      key,
      name,
      description,
      flag_type: flagType,
      default_value: getDefaultValueParsed(),
      tags: parsedTags,
    })
  }

  if (!open) return null

  const errorMsg = mutation.error instanceof Error ? mutation.error.message : ''

  return (
    <div style={styles.overlay} onClick={resetAndClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <h2 style={styles.heading}>Create Flag</h2>
        <form onSubmit={handleSubmit}>
          {errorMsg && <div style={styles.error}>{errorMsg}</div>}

          <label style={styles.label}>Name</label>
          <input
            style={styles.input}
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            placeholder="Dark Mode"
            required
            autoFocus
          />

          <label style={styles.label}>Key</label>
          <input
            style={styles.input}
            value={key}
            onChange={(e) => handleKeyChange(e.target.value)}
            placeholder="dark-mode"
            required
          />

          <label style={styles.label}>Description</label>
          <textarea
            style={styles.textarea}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description"
          />

          <label style={styles.label}>Type</label>
          <select
            style={styles.select}
            value={flagType}
            onChange={(e) => handleTypeChange(e.target.value)}
          >
            {FLAG_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </select>

          <label style={styles.label}>Default Value</label>
          {flagType === 'boolean' ? (
            <div style={styles.toggle}>
              <div
                style={styles.toggleTrack(boolValue)}
                onClick={() => setBoolValue(!boolValue)}
              >
                <div style={styles.toggleKnob(boolValue)} />
              </div>
              <span style={styles.toggleLabel}>{boolValue ? 'true' : 'false'}</span>
            </div>
          ) : flagType === 'number' ? (
            <input
              style={styles.input}
              type="number"
              value={defaultValue}
              onChange={(e) => setDefaultValue(e.target.value)}
            />
          ) : flagType === 'json' ? (
            <textarea
              style={{ ...styles.textarea, fontFamily: 'monospace', fontSize: 13 }}
              value={defaultValue}
              onChange={(e) => setDefaultValue(e.target.value)}
              placeholder='{"key": "value"}'
            />
          ) : (
            <input
              style={styles.input}
              value={defaultValue}
              onChange={(e) => setDefaultValue(e.target.value)}
              placeholder="Default string value"
            />
          )}

          <label style={styles.label}>Tags (comma-separated)</label>
          <input
            style={styles.input}
            value={tags}
            onChange={(e) => setTags(e.target.value)}
            placeholder="ui, experiment, beta"
          />

          <div style={styles.actions}>
            <button type="button" style={styles.cancelBtn} onClick={resetAndClose}>
              Cancel
            </button>
            <button
              type="submit"
              style={{
                ...styles.createBtn,
                ...(mutation.isPending ? styles.disabledBtn : {}),
              }}
              disabled={mutation.isPending}
            >
              {mutation.isPending ? 'Creating...' : 'Create Flag'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
