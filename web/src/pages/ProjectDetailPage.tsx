import { useState, useMemo } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Flag, Environment, FlagEnvironmentConfig } from '../api/types.ts'
import CreateFlagModal from '../components/CreateFlagModal.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

export default function ProjectDetailPage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [search, setSearch] = useState('')
  const [tagFilter, setTagFilter] = useState('')
  const [modalOpen, setModalOpen] = useState(false)

  const { data: flags, isLoading: flagsLoading, error: flagsError } = useQuery({
    queryKey: ['projects', key, 'flags'],
    queryFn: () => api.get<Flag[]>(`/projects/${key}/flags`),
    enabled: !!key,
  })

  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const { data: allConfigs } = useQuery({
    queryKey: ['projects', key, 'all-configs'],
    queryFn: async () => {
      if (!flags || flags.length === 0) return {}
      const configMap: Record<string, FlagEnvironmentConfig[]> = {}
      await Promise.all(
        flags.map(async (flag) => {
          try {
            const resp = await api.get<{ flag: Flag; environment_configs: FlagEnvironmentConfig[] }>(
              `/projects/${key}/flags/${flag.key}`
            )
            configMap[flag.key] = resp.environment_configs
          } catch {
            configMap[flag.key] = []
          }
        })
      )
      return configMap
    },
    enabled: !!flags && flags.length > 0,
  })

  const allTags = useMemo(() => {
    if (!flags) return []
    const tagSet = new Set<string>()
    flags.forEach((f) => f.tags?.forEach((tag) => tagSet.add(tag)))
    return Array.from(tagSet).sort()
  }, [flags])

  const filtered = useMemo(() => {
    if (!flags) return []
    return flags.filter((f) => {
      const matchesSearch =
        !search ||
        f.key.toLowerCase().includes(search.toLowerCase()) ||
        f.name.toLowerCase().includes(search.toLowerCase())
      const matchesTag = !tagFilter || (f.tags && f.tags.includes(tagFilter))
      return matchesSearch && matchesTag
    })
  }, [flags, search, tagFilter])

  if (flagsLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading flags...
      </div>
    )
  }

  if (flagsError) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load flags: {flagsError instanceof Error ? flagsError.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  const getEnvStatus = (flagKey: string, envId: string): boolean => {
    if (!allConfigs || !allConfigs[flagKey]) return false
    const cfg = allConfigs[flagKey].find((c) => c.environment_id === envId)
    return cfg?.enabled ?? false
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      {/* Breadcrumb */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground font-mono text-xs">{key}</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">{key}</h1>
        <Button onClick={() => setModalOpen(true)}>Create Flag</Button>
      </div>

      {/* Filters */}
      <div className="flex gap-2.5 mb-5">
        <Input
          className="flex-1 max-w-[300px]"
          placeholder="Search flags..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        {allTags.length > 0 && (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[130px]"
            value={tagFilter}
            onChange={(e) => setTagFilter(e.target.value)}
          >
            <option value="">All Tags</option>
            {allTags.map((tag) => (
              <option key={tag} value={tag}>{tag}</option>
            ))}
          </select>
        )}
      </div>

      {filtered.length === 0 ? (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">
            {flags && flags.length > 0 ? 'No flags match your filters' : 'No flags yet'}
          </div>
          <div className="text-[13px] text-muted-foreground/60">
            {flags && flags.length > 0
              ? 'Try adjusting your search or tag filter.'
              : 'Create your first feature flag to get started.'}
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Type</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Tags</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Environments</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((flag) => (
                <TableRow
                  key={flag.id}
                  className="cursor-pointer transition-colors hover:bg-[#d4956a]/8"
                  onClick={() => navigate(`/projects/${key}/flags/${flag.key}`)}
                >
                  <TableCell>
                    <span className={`font-mono text-xs text-[#d4956a] tracking-wide ${flag.archived ? 'opacity-50' : ''}`}>{flag.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">
                    <span className={flag.archived ? 'opacity-50' : ''}>
                      {flag.name}
                    </span>
                    {flag.archived && (
                      <Badge variant="secondary" className="ml-2 text-[10px]">Archived</Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary" className="font-mono text-[11px]">{flag.flag_type}</Badge>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {flag.tags?.map((tag) => (
                        <Badge key={tag} variant="outline" className="text-[11px]">{tag}</Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-2">
                      {environments?.map((env) => {
                        const enabled = getEnvStatus(flag.key, env.id)
                        return (
                          <span key={env.id} className="inline-flex items-center gap-1 whitespace-nowrap">
                            <span
                              className={`inline-block w-[7px] h-[7px] rounded-full transition-all duration-300 ${
                                enabled
                                  ? 'bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.4)]'
                                  : 'bg-muted-foreground/60'
                              }`}
                            />
                            <span className="text-[11px] text-muted-foreground/60">{env.name}</span>
                          </span>
                        )
                      })}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateFlagModal
        open={modalOpen}
        projectKey={key!}
        onClose={() => setModalOpen(false)}
      />
    </div>
  )
}
