import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { useProjectRole, useIsOrgAdmin } from '@/hooks/usePermissions'

/**
 * Hook that returns a function to check whether the current user can write
 * flag configs to a specific environment within a project.
 *
 * Returns `canWriteEnv(envKey)` which accounts for:
 * - Org admins bypass all restrictions
 * - No restrictions for user's role = unrestricted (all envs writable)
 * - Restrictions exist = only listed environments are writable
 */
export function useEnvironmentWriteAccess(projectKey: string | undefined) {
  const isOrgAdmin = useIsOrgAdmin()
  const role = useProjectRole(projectKey)

  const { data } = useQuery({
    queryKey: ['projects', projectKey, 'environment-access'],
    queryFn: () => api.environmentAccess.get(projectKey!),
    enabled: !!projectKey,
    staleTime: 5 * 60 * 1000,
  })

  const canWriteEnv = (envId: string): boolean => {
    // Org admins bypass everything
    if (isOrgAdmin) return true

    // No data yet or no role — assume allowed (optimistic)
    if (!data || !role) return true

    // Find restrictions for the user's role
    const restriction = data.restrictions.find(r => r.role_name === role)

    // No restriction for this role = unrestricted
    if (!restriction) return true

    // Check if env is in the allow-list
    return restriction.environment_ids.includes(envId)
  }

  return { canWriteEnv, environmentAccess: data }
}
