import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'

export interface PermissionInfo {
  key: string
  label: string
}

export interface RoleDefinition {
  id: string
  name: string
  description: string
  permissions: string[]
  is_built_in: boolean
  created_at: string
  updated_at: string
}

export function useProjectPermissions() {
  return useQuery({
    queryKey: ['permissions'],
    queryFn: () => api.get<PermissionInfo[]>('/permissions'),
    staleTime: Infinity,
  })
}

export function useRoles() {
  return useQuery({
    queryKey: ['roles'],
    queryFn: () => api.get<RoleDefinition[]>('/roles'),
    staleTime: 5 * 60 * 1000,
  })
}

export function useRole(name: string) {
  return useQuery({
    queryKey: ['roles', name],
    queryFn: () => api.get<RoleDefinition>(`/roles/${name}`),
    enabled: !!name,
  })
}

export function useCreateRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (data: { name: string; description: string; permissions: string[] }) =>
      api.post<RoleDefinition>('/roles', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}

export function useUpdateRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ name, ...data }: { name: string; description: string; permissions: string[] }) =>
      api.put<RoleDefinition>(`/roles/${name}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}

export function useDeleteRole() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (name: string) => api.delete(`/roles/${name}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['roles'] })
    },
  })
}
