import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { FlagTemplate, ValueType } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TemplateFormDialog } from '@/components/TemplateFormDialog'

function formatDefaultValue(value: unknown, valueType: ValueType): string {
  if (valueType === 'boolean') return String(value)
  if (valueType === 'json') return JSON.stringify(value)
  return String(value ?? '')
}

export default function ProjectTemplatesPage() {
  const { key: projectKey } = useParams<{ key: string }>()
  const queryClient = useQueryClient()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTemplate, setEditTemplate] = useState<FlagTemplate | null>(null)

  const invalidateKey = ['projects', projectKey!, 'templates']

  const { data, isLoading, error } = useQuery({
    queryKey: invalidateKey,
    queryFn: () => api.templates.listForProject(projectKey!),
    enabled: !!projectKey,
  })

  const deleteMutation = useMutation({
    mutationFn: (templateKey: string) => api.templates.deleteForProject(projectKey!, templateKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: invalidateKey })
      setEditTemplate(null)
    },
  })

  const templates = data?.project ?? []

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading templates...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load templates: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div>
      {/* Breadcrumbs */}
      <div className="flex items-center gap-2 mb-6 text-[13px] text-muted-foreground/60">
        <Link to="/projects" className="text-muted-foreground hover:text-foreground transition-colors">
          Projects
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <Link to={`/projects/${projectKey}`} className="text-muted-foreground hover:text-foreground transition-colors">
          {projectKey}
        </Link>
        <span className="opacity-40">&rsaquo;</span>
        <span className="text-foreground">Templates</span>
      </div>

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
            Project Templates
          </h1>
          <div className="text-[13px] text-muted-foreground/60">
            Templates specific to this project for quickly creating flags with pre-filled settings.
          </div>
        </div>
        <Button size="sm" onClick={() => setCreateOpen(true)}>
          Create Template
        </Button>
      </div>

      {templates.length === 0 ? (
        <div className="text-center py-12 border rounded-lg">
          <div className="text-[15px] font-medium text-foreground mb-1.5">No project templates yet</div>
          <div className="text-[13px] text-muted-foreground/60">
            Create templates to help your team create flags consistently.
          </div>
        </div>
      ) : (
        <div className="rounded-lg border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Key</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Name</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Type</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Value</TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">Tags</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {templates.map((template) => (
                <TableRow
                  key={template.id}
                  className="transition-colors hover:bg-[#d4956a]/8 cursor-pointer"
                  onClick={() => setEditTemplate(template)}
                >
                  <TableCell>
                    <span className="font-mono text-xs text-[#d4956a] tracking-wide">{template.key}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-foreground">{template.name}</TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    <span className="font-mono text-xs">{template.flag_type}</span>
                    {' / '}
                    <span className="font-mono text-xs">{template.value_type}</span>
                  </TableCell>
                  <TableCell className="text-[13px] text-muted-foreground">
                    <span className="font-mono text-xs">
                      {formatDefaultValue(template.default_value, template.value_type)}
                    </span>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {(template.tags ?? []).map((tag) => (
                        <Badge key={tag} variant="secondary" className="text-[10px] font-mono px-1.5 py-0">
                          {tag}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <TemplateFormDialog
        open={createOpen}
        editing={null}
        onClose={() => setCreateOpen(false)}
        onSaved={() => setCreateOpen(false)}
        createFn={(payload) => api.templates.createForProject(projectKey!, payload)}
        updateFn={(key, payload) => api.templates.updateForProject(projectKey!, key, payload)}
        invalidateKey={invalidateKey}
      />

      {editTemplate && (
        <TemplateFormDialog
          key={editTemplate.id}
          open={!!editTemplate}
          editing={editTemplate}
          onClose={() => setEditTemplate(null)}
          onSaved={() => setEditTemplate(null)}
          onDelete={(t) => deleteMutation.mutate(t.key)}
          createFn={(payload) => api.templates.createForProject(projectKey!, payload)}
          updateFn={(key, payload) => api.templates.updateForProject(projectKey!, key, payload)}
          invalidateKey={invalidateKey}
        />
      )}
    </div>
  )
}
