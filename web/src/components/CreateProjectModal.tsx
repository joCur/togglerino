import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'

interface Props {
  open: boolean
  onClose: () => void
}

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
    maxWidth: 460,
    padding: 32,
    borderRadius: 12,
    backgroundColor: '#16213e',
    boxShadow: '0 8px 32px rgba(0, 0, 0, 0.4)',
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
    <div style={styles.overlay} onClick={resetAndClose}>
      <div style={styles.modal} onClick={(e) => e.stopPropagation()}>
        <h2 style={styles.heading}>Create Project</h2>
        <form onSubmit={handleSubmit}>
          {errorMsg && <div style={styles.error}>{errorMsg}</div>}
          <label style={styles.label}>Name</label>
          <input
            style={styles.input}
            value={name}
            onChange={(e) => handleNameChange(e.target.value)}
            placeholder="My Project"
            required
            autoFocus
          />
          <label style={styles.label}>Key</label>
          <input
            style={styles.input}
            value={key}
            onChange={(e) => handleKeyChange(e.target.value)}
            placeholder="my-project"
            required
          />
          <label style={styles.label}>Description</label>
          <textarea
            style={styles.textarea}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Optional description"
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
              {mutation.isPending ? 'Creating...' : 'Create Project'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
