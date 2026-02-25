import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { t } from '../theme.ts'

const inputStyle = {
  width: '100%',
  padding: '9px 14px',
  fontSize: 13,
  fontFamily: t.fontSans,
  color: t.textPrimary,
  backgroundColor: t.bgInput,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusSm,
  outline: 'none',
  boxSizing: 'border-box' as const,
}

const cardStyle = {
  padding: 24,
  borderRadius: t.radiusLg,
  backgroundColor: t.bgSurface,
  border: `1px solid ${t.border}`,
  marginBottom: 24,
}

export default function ProjectSettingsPage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: project, isLoading, error } = useQuery({
    queryKey: ['projects', key],
    queryFn: () => api.get<Project>(`/projects/${key}`),
    enabled: !!key,
  })

  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saveSuccess, setSaveSuccess] = useState(false)

  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('')

  useEffect(() => {
    if (project) {
      setName(project.name)
      setDescription(project.description)
    }
  }, [project])

  const hasChanges = project ? (name !== project.name || description !== project.description) : false

  const updateMutation = useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.put<Project>(`/projects/${key}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      queryClient.invalidateQueries({ queryKey: ['projects', key] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${key}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate('/projects')
    },
  })

  const handleSave = () => {
    if (!hasChanges) return
    updateMutation.mutate({ name: name.trim(), description: description.trim() })
  }

  const handleDelete = () => {
    if (deleteConfirmInput !== key) return
    deleteMutation.mutate()
  }

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
        Loading project settings...
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ padding: '14px 18px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13 }}>
        Failed to load project: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  return (
    <div style={{ animation: 'fadeIn 300ms ease', maxWidth: 640 }}>
      <div style={{ marginBottom: 32 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, marginBottom: 6, letterSpacing: '-0.3px' }}>
          Project Settings
        </h1>
        <div style={{ fontSize: 13, color: t.textMuted }}>
          Manage settings for <span style={{ fontFamily: t.fontMono, color: t.textSecondary }}>{key}</span>
        </div>
      </div>

      {/* General Section */}
      <div style={cardStyle}>
        <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 18 }}>
          General
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>
              Name
            </label>
            <input
              style={inputStyle}
              value={name}
              onChange={(e) => setName(e.target.value)}
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder; e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}` }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.boxShadow = 'none' }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
            <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>
              Description
            </label>
            <textarea
              style={{ ...inputStyle, minHeight: 80, resize: 'vertical' }}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              onFocus={(e) => { e.currentTarget.style.borderColor = t.accentBorder; e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}` }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.boxShadow = 'none' }}
            />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <button
              style={{
                padding: '9px 18px',
                fontSize: 13,
                fontWeight: 600,
                border: 'none',
                borderRadius: t.radiusMd,
                background: hasChanges ? `linear-gradient(135deg, ${t.accent}, #c07e4e)` : t.bgElevated,
                color: hasChanges ? '#ffffff' : t.textMuted,
                cursor: hasChanges ? 'pointer' : 'default',
                fontFamily: t.fontSans,
                transition: 'all 200ms ease',
                boxShadow: hasChanges ? '0 2px 10px rgba(212,149,106,0.15)' : 'none',
                opacity: updateMutation.isPending ? 0.7 : 1,
              }}
              disabled={!hasChanges || updateMutation.isPending}
              onClick={handleSave}
              onMouseEnter={(e) => { if (hasChanges) { e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'; e.currentTarget.style.transform = 'translateY(-1px)' } }}
              onMouseLeave={(e) => { if (hasChanges) { e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'; e.currentTarget.style.transform = 'translateY(0)' } }}
            >
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </button>

            {saveSuccess && (
              <span style={{ fontSize: 13, color: t.success, animation: 'fadeIn 200ms ease' }}>Saved</span>
            )}

            {updateMutation.error && (
              <span style={{ fontSize: 13, color: t.danger }}>
                {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to save'}
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Members Section (placeholder) */}
      <div style={cardStyle}>
        <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 12 }}>
          Members
        </div>
        <div style={{ fontSize: 13, color: t.textMuted }}>
          Project-level member management coming soon.
        </div>
      </div>

      {/* Danger Zone */}
      <div style={{ ...cardStyle, border: `1px solid ${t.dangerBorder}` }}>
        <div style={{ fontSize: 14, fontWeight: 600, color: t.danger, marginBottom: 12 }}>
          Danger Zone
        </div>
        <div style={{ fontSize: 13, color: t.textSecondary, marginBottom: 16, lineHeight: 1.6 }}>
          Deleting this project is permanent and cannot be undone. All flags, environments, and SDK keys associated with this project will be removed.
        </div>

        {!showDeleteConfirm ? (
          <button
            style={{
              padding: '9px 18px',
              fontSize: 13,
              fontWeight: 600,
              border: `1px solid ${t.dangerBorder}`,
              borderRadius: t.radiusMd,
              backgroundColor: 'transparent',
              color: t.danger,
              cursor: 'pointer',
              fontFamily: t.fontSans,
              transition: 'all 200ms ease',
            }}
            onClick={() => setShowDeleteConfirm(true)}
            onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.dangerSubtle }}
            onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
          >
            Delete Project
          </button>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 12, animation: 'fadeIn 200ms ease' }}>
            <div style={{ fontSize: 13, color: t.textSecondary }}>
              Type <span style={{ fontFamily: t.fontMono, color: t.danger, fontWeight: 600 }}>{key}</span> to confirm deletion:
            </div>
            <input
              style={inputStyle}
              value={deleteConfirmInput}
              onChange={(e) => setDeleteConfirmInput(e.target.value)}
              placeholder={key}
              autoFocus
              onFocus={(e) => { e.currentTarget.style.borderColor = t.dangerBorder; e.currentTarget.style.boxShadow = `0 0 0 3px ${t.dangerSubtle}` }}
              onBlur={(e) => { e.currentTarget.style.borderColor = t.border; e.currentTarget.style.boxShadow = 'none' }}
            />
            <div style={{ display: 'flex', gap: 12 }}>
              <button
                style={{
                  padding: '9px 18px',
                  fontSize: 13,
                  fontWeight: 600,
                  border: `1px solid ${t.dangerBorder}`,
                  borderRadius: t.radiusMd,
                  backgroundColor: deleteConfirmInput === key ? t.danger : 'transparent',
                  color: deleteConfirmInput === key ? '#ffffff' : t.textMuted,
                  cursor: deleteConfirmInput === key ? 'pointer' : 'default',
                  fontFamily: t.fontSans,
                  transition: 'all 200ms ease',
                  opacity: deleteMutation.isPending ? 0.7 : 1,
                }}
                disabled={deleteConfirmInput !== key || deleteMutation.isPending}
                onClick={handleDelete}
              >
                {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
              </button>
              <button
                style={{
                  padding: '9px 18px',
                  fontSize: 13,
                  fontWeight: 500,
                  border: `1px solid ${t.border}`,
                  borderRadius: t.radiusMd,
                  backgroundColor: 'transparent',
                  color: t.textSecondary,
                  cursor: 'pointer',
                  fontFamily: t.fontSans,
                  transition: 'all 200ms ease',
                }}
                onClick={() => { setShowDeleteConfirm(false); setDeleteConfirmInput('') }}
              >
                Cancel
              </button>
            </div>
            {deleteMutation.error && (
              <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13 }}>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete project'}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
