import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Link } from 'react-router-dom'

export default function MyOverridesPage() {
  const queryClient = useQueryClient()

  const { data: overrides, isLoading, error } = useQuery({
    queryKey: ['my-overrides'],
    queryFn: () => api.overrides.listMine(),
  })

  const deleteMutation = useMutation({
    mutationFn: (o: { projectKey: string; flagKey: string; envKey: string }) =>
      api.overrides.delete(o.projectKey, o.flagKey, o.envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['my-overrides'] })
    },
  })

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading overrides...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load overrides: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-mono text-[#d4956a] tracking-wide">My Overrides</h1>

      {overrides?.length === 0 ? (
        <p className="text-muted-foreground/60 text-[13px]">No active overrides.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Project</TableHead>
              <TableHead>Flag</TableHead>
              <TableHead>Environment</TableHead>
              <TableHead>Value</TableHead>
              <TableHead>Expires</TableHead>
              <TableHead></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {overrides?.filter((o) => o.project_key && o.flag_key && o.environment_key).map((o) => (
              <TableRow key={o.id}>
                <TableCell>
                  <Link to={`/projects/${o.project_key}`} className="text-[#d4956a] hover:underline">
                    {o.project_key}
                  </Link>
                </TableCell>
                <TableCell>
                  <Link
                    to={`/projects/${o.project_key}/flags/${o.flag_key}`}
                    className="font-mono text-sm text-[#d4956a] hover:underline"
                  >
                    {o.flag_key}
                  </Link>
                </TableCell>
                <TableCell>
                  <Badge variant="outline">{o.environment_key}</Badge>
                </TableCell>
                <TableCell className="font-mono text-sm">{JSON.stringify(o.value)}</TableCell>
                <TableCell className="text-sm text-muted-foreground">
                  {o.expires_at ? new Date(o.expires_at).toLocaleString() : 'Never'}
                </TableCell>
                <TableCell>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      deleteMutation.mutate({
                        projectKey: o.project_key as string,
                        flagKey: o.flag_key as string,
                        envKey: o.environment_key as string,
                      })
                    }
                  >
                    Remove
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  )
}
