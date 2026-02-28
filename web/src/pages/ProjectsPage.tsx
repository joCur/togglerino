import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import CreateProjectModal from '../components/CreateProjectModal.tsx'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function ProjectsPage() {
  const navigate = useNavigate()
  const [modalOpen, setModalOpen] = useState(false)

  const { data: projects, isLoading, error } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.get<Project[]>('/projects'),
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading projects...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load projects: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between mb-6 md:mb-8">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Projects</h1>
        <Button onClick={() => setModalOpen(true)}>Create Project</Button>
      </div>

      {(!projects || projects.length === 0) ? (
        <div className="text-center py-16">
          <div className="text-base font-medium text-foreground mb-2">No projects yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Create your first project to start managing feature flags.
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-[repeat(auto-fill,minmax(300px,1fr))] gap-3 md:gap-4">
          {projects.map((project, index) => (
            <Card
              key={project.id}
              className="cursor-pointer transition-all duration-300 hover:border-[#d4956a]/25 hover:shadow-[0_0_24px_rgba(212,149,106,0.15)] hover:-translate-y-0.5 animate-[fadeIn_300ms_ease_both]"
              style={{ animationDelay: `${index * 50}ms` }}
              onClick={() => navigate(`/projects/${project.key}`)}
            >
              <CardContent className="p-6">
                <div className="text-[15px] font-semibold text-foreground mb-1">{project.name}</div>
                <div className="text-xs font-mono text-[#d4956a] mb-2.5 tracking-wide">{project.key}</div>
                {project.description && (
                  <div className="text-[13px] text-muted-foreground leading-relaxed mb-3">
                    {project.description}
                  </div>
                )}
                <div className="text-[11px] text-muted-foreground/60">
                  Created {new Date(project.created_at).toLocaleDateString()}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      <CreateProjectModal open={modalOpen} onClose={() => setModalOpen(false)} />
    </div>
  )
}
