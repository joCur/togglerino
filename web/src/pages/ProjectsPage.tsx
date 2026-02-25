import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { t } from '../theme.ts'
import CreateProjectModal from '../components/CreateProjectModal.tsx'

export default function ProjectsPage() {
  const navigate = useNavigate()
  const [modalOpen, setModalOpen] = useState(false)

  const { data: projects, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.get<Project[]>('/projects'),
  })

  if (isLoading) {
    return (
      <div style={{ textAlign: 'center', padding: 64, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
        Loading projects...
      </div>
    )
  }

  if (error) {
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
        Failed to load projects: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  return (
    <div style={{ animation: 'fadeIn 300ms ease' }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginBottom: 32,
        }}
      >
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, letterSpacing: '-0.3px' }}>
          Projects
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
            letterSpacing: '0.2px',
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
          Create Project
        </button>
      </div>

      {(!projects || projects.length === 0) ? (
        <div style={{ textAlign: 'center', padding: 64, color: t.textSecondary }}>
          <div style={{ fontSize: 16, fontWeight: 500, color: t.textPrimary, marginBottom: 8 }}>
            No projects yet
          </div>
          <div style={{ fontSize: 13, color: t.textMuted }}>
            Create your first project to start managing feature flags.
          </div>
        </div>
      ) : (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(300px, 1fr))',
            gap: 16,
          }}
        >
          {projects.map((project, index) => (
            <div
              key={project.id}
              style={{
                padding: 24,
                borderRadius: t.radiusLg,
                backgroundColor: t.bgSurface,
                border: `1px solid ${t.border}`,
                cursor: 'pointer',
                transition: 'all 300ms ease',
                animation: `fadeIn 300ms ease ${index * 50}ms both`,
              }}
              onClick={() => navigate(`/projects/${project.key}`)}
              onMouseEnter={(e) => {
                e.currentTarget.style.borderColor = t.accentBorder
                e.currentTarget.style.boxShadow = `0 0 24px ${t.accentGlow}, 0 4px 16px rgba(0,0,0,0.2)`
                e.currentTarget.style.transform = 'translateY(-2px)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.borderColor = t.border
                e.currentTarget.style.boxShadow = 'none'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              <div style={{ fontSize: 15, fontWeight: 600, color: t.textPrimary, marginBottom: 4 }}>
                {project.name}
              </div>
              <div
                style={{
                  fontSize: 12,
                  color: t.accent,
                  fontFamily: t.fontMono,
                  marginBottom: 10,
                  letterSpacing: '0.3px',
                }}
              >
                {project.key}
              </div>
              {project.description && (
                <div style={{ fontSize: 13, color: t.textSecondary, lineHeight: 1.5, marginBottom: 12 }}>
                  {project.description}
                </div>
              )}
              <div style={{ fontSize: 11, color: t.textMuted }}>
                Created {new Date(project.created_at).toLocaleDateString()}
              </div>
            </div>
          ))}
        </div>
      )}

      <CreateProjectModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </div>
  )
}
