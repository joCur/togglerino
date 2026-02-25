import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import CreateProjectModal from '../components/CreateProjectModal.tsx'

const styles = {
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 32,
  } as const,
  title: {
    fontSize: 24,
    fontWeight: 700,
    color: '#ffffff',
  } as const,
  createBtn: {
    padding: '10px 20px',
    fontSize: 14,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
  } as const,
  grid: {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(320, 1fr))',
    gap: 16,
  } as const,
  card: {
    padding: 24,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px solid #2a2a4a',
    cursor: 'pointer',
    transition: 'border-color 0.2s',
  } as const,
  cardName: {
    fontSize: 17,
    fontWeight: 600,
    color: '#ffffff',
    marginBottom: 4,
  } as const,
  cardKey: {
    fontSize: 13,
    color: '#e94560',
    fontFamily: 'monospace',
    marginBottom: 10,
  } as const,
  cardDesc: {
    fontSize: 13,
    color: '#8892b0',
    lineHeight: 1.5,
    marginBottom: 12,
  } as const,
  cardDate: {
    fontSize: 12,
    color: '#5a6580',
  } as const,
  empty: {
    textAlign: 'center' as const,
    padding: 64,
    color: '#8892b0',
  } as const,
  emptyTitle: {
    fontSize: 18,
    fontWeight: 600,
    color: '#e0e0e0',
    marginBottom: 8,
  } as const,
  emptyText: {
    fontSize: 14,
    color: '#8892b0',
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
}

export default function ProjectsPage() {
  const navigate = useNavigate()
  const [modalOpen, setModalOpen] = useState(false)

  const { data: projects, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.get<Project[]>('/projects'),
  })

  if (isLoading) {
    return <div style={styles.loading}>Loading projects...</div>
  }

  if (error) {
    return (
      <div style={styles.errorBox}>
        Failed to load projects: {error instanceof Error ? error.message : 'Unknown error'}
      </div>
    )
  }

  return (
    <div>
      <div style={styles.header}>
        <h1 style={styles.title}>Projects</h1>
        <button style={styles.createBtn} onClick={() => setModalOpen(true)}>
          Create Project
        </button>
      </div>

      {(!projects || projects.length === 0) ? (
        <div style={styles.empty}>
          <div style={styles.emptyTitle}>No projects yet</div>
          <div style={styles.emptyText}>Create your first project to start managing feature flags.</div>
        </div>
      ) : (
        <div style={styles.grid}>
          {projects.map((project) => (
            <div
              key={project.id}
              style={styles.card}
              onClick={() => navigate(`/projects/${project.key}`)}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLDivElement).style.borderColor = '#e94560'
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLDivElement).style.borderColor = '#2a2a4a'
              }}
            >
              <div style={styles.cardName}>{project.name}</div>
              <div style={styles.cardKey}>{project.key}</div>
              {project.description && (
                <div style={styles.cardDesc}>{project.description}</div>
              )}
              <div style={styles.cardDate}>
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
