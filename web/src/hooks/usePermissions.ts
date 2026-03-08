import { useAuth } from '@/hooks/useAuth'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'

export type ProjectRole = string

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

interface ProjectRoleResponse {
  role: string
  permissions: string[]
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

// Shared query for the user's effective project role and permissions.
function useProjectRoleQuery(projectKey: string | undefined) {
  return useQuery({
    queryKey: ['my-project-role', projectKey],
    queryFn: () => api.get<ProjectRoleResponse>(`/auth/me/project-role/${projectKey}`),
    enabled: !!projectKey,
    staleTime: 5 * 60 * 1000,
  })
}

// Resolve the user's effective project role via server-side endpoint
export function useProjectRole(projectKey: string | undefined): ProjectRole | null {
  const { data } = useProjectRoleQuery(projectKey)

  if (!data) return null
  if (data.role === 'none') return null
  return data.role
}

// Hook to get the current user's permissions for a project
export function useProjectPermissions(projectKey: string | undefined): string[] {
  const { data } = useProjectRoleQuery(projectKey)

  return data?.permissions ?? []
}

// Check if the user has write access for a project
export function useCanWrite(projectKey: string | undefined): boolean {
  const perms = useProjectPermissions(projectKey)
  return perms.includes('flags:write')
}

// Check if the user has project admin access
export function useIsProjectAdmin(projectKey: string | undefined): boolean {
  const perms = useProjectPermissions(projectKey)
  return perms.includes('project:settings')
}

// Generic permission check hook
export function useHasPermission(projectKey: string | undefined, permission: string): boolean {
  const perms = useProjectPermissions(projectKey)
  return perms.includes(permission)
}

// Convenience hooks for specific permissions
export function useEnvironmentsWrite(projectKey: string | undefined): boolean {
  return useHasPermission(projectKey, 'environments:write')
}

export function useSdkKeysManage(projectKey: string | undefined): boolean {
  return useHasPermission(projectKey, 'sdk_keys:manage')
}

export function useSegmentsWrite(projectKey: string | undefined): boolean {
  return useHasPermission(projectKey, 'segments:write')
}

export function useTemplatesManage(projectKey: string | undefined): boolean {
  return useHasPermission(projectKey, 'templates:manage')
}
