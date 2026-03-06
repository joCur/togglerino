import { useState, useEffect, useCallback } from 'react'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Environment, Flag, PlaygroundRequest, PlaygroundResponse, EvaluationTrace } from '../api/types.ts'
import PlaygroundTrace from '../components/PlaygroundTrace.tsx'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { cn } from '@/lib/utils'
import { ChevronDown, ChevronRight, Plus, X } from 'lucide-react'

function reasonColor(reason: string) {
  switch (reason) {
    case 'rule_match':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
    case 'default':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/20'
    case 'disabled':
      return 'bg-muted text-muted-foreground'
    case 'archived':
      return 'bg-red-500/10 text-red-400 border-red-500/20'
    default:
      return ''
  }
}

function formatValue(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  return String(value)
}

export default function PlaygroundPage() {
  const { key } = useParams<{ key: string }>()
  const [searchParams, setSearchParams] = useSearchParams()

  // Form state
  const [envKey, setEnvKey] = useState(searchParams.get('env') ?? '')
  const [flagKey, setFlagKey] = useState(searchParams.get('flag') ?? '')
  const [userId, setUserId] = useState(searchParams.get('uid') ?? '')
  const [attributes, setAttributes] = useState<{ key: string; value: string }[]>(() => {
    const attrs: { key: string; value: string }[] = []
    searchParams.forEach((v, k) => {
      if (k.startsWith('attr.')) {
        attrs.push({ key: k.slice(5), value: v })
      }
    })
    return attrs
  })

  // Expanded traces
  const [expandedTraces, setExpandedTraces] = useState<Set<string>>(new Set())

  // Data queries
  const { data: environments } = useQuery({
    queryKey: ['projects', key, 'environments'],
    queryFn: () => api.get<Environment[]>(`/projects/${key}/environments`),
    enabled: !!key,
  })

  const { data: flags } = useQuery({
    queryKey: ['projects', key, 'flags'],
    queryFn: () => api.get<Flag[]>(`/projects/${key}/flags`),
    enabled: !!key,
  })

  // Set default environment when environments load
  useEffect(() => {
    if (environments && environments.length > 0 && !envKey) {
      setEnvKey(environments[0].key)
    }
  }, [environments, envKey])

  // Evaluation mutation
  const evaluateMutation = useMutation({
    mutationFn: (body: PlaygroundRequest) => api.playground.evaluate(key!, body),
    onSuccess: (data: PlaygroundResponse) => {
      // If single flag, auto-expand its trace
      if (flagKey && data.results.length === 1) {
        setExpandedTraces(new Set([data.results[0].flag_key]))
      }
    },
  })

  const handleEvaluate = useCallback(() => {
    if (!envKey) return

    const attrMap: Record<string, string> = {}
    for (const attr of attributes) {
      if (attr.key.trim()) {
        attrMap[attr.key.trim()] = attr.value
      }
    }

    const body: PlaygroundRequest = {
      environment_key: envKey,
      flag_key: flagKey || undefined,
      context: (userId || Object.keys(attrMap).length > 0)
        ? { user_id: userId, attributes: attrMap }
        : undefined,
    }

    // Update URL params
    const params = new URLSearchParams()
    params.set('env', envKey)
    if (flagKey) params.set('flag', flagKey)
    if (userId) params.set('uid', userId)
    for (const attr of attributes) {
      if (attr.key.trim()) {
        params.set(`attr.${attr.key.trim()}`, attr.value)
      }
    }
    setSearchParams(params, { replace: true })

    evaluateMutation.mutate(body)
  }, [envKey, flagKey, userId, attributes, setSearchParams, evaluateMutation.mutate])

  // Auto-evaluate on page load if params exist
  useEffect(() => {
    if (!environments || environments.length === 0) return
    if (!searchParams.get('env')) return
    evaluateMutation.mutate({
      environment_key: envKey,
      flag_key: flagKey || undefined,
      context: (userId || attributes.some(a => a.key.trim()))
        ? {
            user_id: userId,
            attributes: Object.fromEntries(
              attributes.filter(a => a.key.trim()).map(a => [a.key.trim(), a.value])
            ),
          }
        : undefined,
    })
    // Only on initial load when environments arrive
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [environments])

  const addAttribute = () => {
    setAttributes([...attributes, { key: '', value: '' }])
  }

  const removeAttribute = (index: number) => {
    setAttributes(attributes.filter((_, i) => i !== index))
  }

  const updateAttribute = (index: number, field: 'key' | 'value', val: string) => {
    const updated = [...attributes]
    updated[index] = { ...updated[index], [field]: val }
    setAttributes(updated)
  }

  const toggleTrace = (flagKey: string) => {
    setExpandedTraces((prev) => {
      const next = new Set(prev)
      if (next.has(flagKey)) {
        next.delete(flagKey)
      } else {
        next.add(flagKey)
      }
      return next
    })
  }

  const results = evaluateMutation.data?.results

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      {/* Breadcrumbs */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${key}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {key}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Playground</span>
      </div>

      <div className="mb-6">
        <h1 className="text-[22px] font-semibold text-foreground tracking-tight">Playground</h1>
        <p className="text-[13px] text-muted-foreground/60 mt-0.5">
          Test flag evaluation against a custom context.
        </p>
      </div>

      {/* Form */}
      <div className="rounded-lg border bg-card p-5 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Environment */}
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Environment</Label>
            <select
              className="w-full px-2.5 py-1.5 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer"
              value={envKey}
              onChange={(e) => setEnvKey(e.target.value)}
            >
              {!environments && <option value="">Loading...</option>}
              {environments?.map((env) => (
                <option key={env.key} value={env.key}>{env.name}</option>
              ))}
            </select>
          </div>

          {/* Flag key */}
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Flag Key <span className="text-muted-foreground/40">(optional)</span></Label>
            <div className="relative">
              <Input
                className="text-[13px] font-mono"
                placeholder="Leave empty to evaluate all flags"
                value={flagKey}
                onChange={(e) => setFlagKey(e.target.value)}
                list="flag-keys"
              />
              <datalist id="flag-keys">
                {flags?.map((f) => (
                  <option key={f.key} value={f.key} />
                ))}
              </datalist>
            </div>
          </div>

          {/* User ID */}
          <div className="flex flex-col gap-1.5 md:col-span-2">
            <Label className="font-mono text-[10px] uppercase tracking-wider">User ID <span className="text-muted-foreground/40">(optional)</span></Label>
            <Input
              className="text-[13px] font-mono"
              placeholder="e.g. user-123"
              value={userId}
              onChange={(e) => setUserId(e.target.value)}
            />
          </div>
        </div>

        {/* Context attributes */}
        <div className="mt-4">
          <Label className="font-mono text-[10px] uppercase tracking-wider">Context Attributes</Label>
          <div className="flex flex-col gap-1.5 mt-1.5">
            {attributes.map((attr, idx) => (
              <div key={idx} className="flex items-center gap-1.5">
                <Input
                  className="flex-1 text-[13px] font-mono"
                  placeholder="Key"
                  value={attr.key}
                  onChange={(e) => updateAttribute(idx, 'key', e.target.value)}
                />
                <Input
                  className="flex-1 text-[13px] font-mono"
                  placeholder="Value"
                  value={attr.value}
                  onChange={(e) => updateAttribute(idx, 'value', e.target.value)}
                />
                <Button
                  variant="ghost"
                  size="sm"
                  className="shrink-0 text-destructive h-8 w-8 p-0"
                  onClick={() => removeAttribute(idx)}
                >
                  <X className="w-3.5 h-3.5" />
                </Button>
              </div>
            ))}
            <Button
              variant="outline"
              size="sm"
              className="text-[11px] h-7 self-start"
              onClick={addAttribute}
            >
              <Plus className="w-3 h-3 mr-1" />
              Add attribute
            </Button>
          </div>
        </div>

        <div className="mt-4 flex items-center gap-2">
          <Button
            onClick={handleEvaluate}
            disabled={!envKey || evaluateMutation.isPending}
          >
            {evaluateMutation.isPending ? 'Evaluating...' : 'Evaluate'}
          </Button>
        </div>
      </div>

      {/* Error */}
      {evaluateMutation.error && (
        <Alert variant="destructive" className="mb-6">
          <AlertDescription>
            {evaluateMutation.error instanceof Error ? evaluateMutation.error.message : 'Evaluation failed'}
          </AlertDescription>
        </Alert>
      )}

      {/* Results */}
      {results && results.length > 0 && (
        <div>
          <div className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider mb-3">
            Results ({results.length} flag{results.length !== 1 ? 's' : ''})
          </div>
          <div className="flex flex-col gap-2">
            {results.map((result: EvaluationTrace) => {
              const isExpanded = expandedTraces.has(result.flag_key)
              return (
                <div key={result.flag_key} className="rounded-lg border bg-card overflow-hidden">
                  <button
                    type="button"
                    className="flex items-center gap-3 w-full px-4 py-3 text-left hover:bg-foreground/[0.02] transition-colors cursor-pointer"
                    onClick={() => toggleTrace(result.flag_key)}
                  >
                    {isExpanded
                      ? <ChevronDown className="w-4 h-4 text-muted-foreground shrink-0" />
                      : <ChevronRight className="w-4 h-4 text-muted-foreground shrink-0" />
                    }
                    <span className="font-mono text-[13px] text-[#d4956a] tracking-wide">{result.flag_key}</span>
                    <span className="text-[13px] text-foreground font-mono ml-auto mr-2 truncate max-w-[200px]">
                      {formatValue(result.value)}
                    </span>
                    <Badge variant="outline" className="text-[10px] font-mono shrink-0">{result.variant}</Badge>
                    <Badge
                      variant="secondary"
                      className={cn('text-[10px] shrink-0', reasonColor(result.reason))}
                    >
                      {result.reason.replace(/_/g, ' ')}
                    </Badge>
                  </button>
                  {isExpanded && (
                    <div className="px-4 pb-4 border-t border-border/50">
                      <div className="mt-3">
                        <PlaygroundTrace trace={result} />
                      </div>
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      {results && results.length === 0 && (
        <div className="text-center py-12">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No results</div>
          <div className="text-[13px] text-muted-foreground/60">
            No flags matched the query. Check the flag key and environment.
          </div>
        </div>
      )}
    </div>
  )
}
