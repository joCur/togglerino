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

// Resolve the user's effective project role
export function useProjectRole(projectKey: string | undefined): ProjectRole | null {
  const { user } = useAuth()
  const { data: members } = useProjectMembers(projectKey)
  const { data: baseRole } = useBaseProjectRole()

  if (!user || !projectKey) return null
  if (user.role === 'admin') return 'admin'

  const membership = members?.find((m) => m.user_id === user.id)
  if (membership) return membership.role

  const base = baseRole?.base_project_role
  if (base && base !== 'none') return base as ProjectRole

  return null
}

// Check if the user has write access (admin or editor) for a project
export function useCanWrite(projectKey: string | undefined): boolean {
  const role = useProjectRole(projectKey)
  return role === 'admin' || role === 'editor'
}

// Check if the user has project admin access
export function useIsProjectAdmin(projectKey: string | undefined): boolean {
  const role = useProjectRole(projectKey)
  return role === 'admin'
}
