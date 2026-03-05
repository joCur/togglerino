import { useAuth } from '@/hooks/useAuth'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export type ProjectRole = 'admin' | 'editor' | 'viewer'

export interface ProjectMember {
  project_id: string
  user_id: string
  role: ProjectRole
  email: string
  display_name?: string
  created_at: string
  updated_at: string
}

export interface UserProjectAssignment {
  project_id: string
  project_key: string
  project_name: string
  role: ProjectRole
}

export function useIsOrgAdmin(): boolean {
  const { user } = useAuth()
  return user?.role === 'admin'
}

// Hook to get members of a project
export function useProjectMembers(projectKey: string | undefined) {
  return useQuery({
    queryKey: ['project-members', projectKey],
    queryFn: () => api.get<ProjectMember[]>(`/projects/${projectKey}/members`),
    enabled: !!projectKey,
  })
}

// Hook to get base project role
export function useBaseProjectRole() {
  return useQuery({
    queryKey: ['base-project-role'],
    queryFn: () => api.get<{ base_project_role: string }>('/settings/base-project-role'),
  })
}
