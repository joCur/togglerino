import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { User } from '../api/types.ts'

interface AuthStatus {
  setup_required: boolean
}

export function useAuth() {
  const queryClient = useQueryClient()

  const statusQuery = useQuery({
    queryKey: ['auth', 'status'],
    queryFn: () => api.get<AuthStatus>('/auth/status'),
  })

  const meQuery = useQuery({
    queryKey: ['auth', 'me'],
    queryFn: async () => {
      try {
        return await api.get<User>('/auth/me')
      } catch {
        return null
      }
    },
    enabled: statusQuery.data?.setup_required === false,
  })

  const loginMutation = useMutation({
    mutationFn: (data: { email: string; password: string }) =>
      api.post<User>('/auth/login', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth'] })
    },
  })

  const setupMutation = useMutation({
    mutationFn: (data: { email: string; password: string }) =>
      api.post<User>('/auth/setup', data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['auth'] })
    },
  })

  const logoutMutation = useMutation({
    mutationFn: () => api.post('/auth/logout'),
    onSuccess: () => {
      queryClient.setQueryData(['auth', 'me'], null)
      queryClient.removeQueries()
    },
  })

  return {
    status: statusQuery.data,
    user: meQuery.data,
    isLoading: statusQuery.isLoading || (statusQuery.data?.setup_required === false && meQuery.isLoading),
    isAuthenticated: !!meQuery.data,
    setupRequired: statusQuery.data?.setup_required ?? false,
    login: loginMutation.mutateAsync,
    setup: setupMutation.mutateAsync,
    logout: logoutMutation.mutateAsync,
    loginError: loginMutation.error,
    setupError: setupMutation.error,
  }
}
