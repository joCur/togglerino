import { useState } from 'react'
import { Navigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useAuth } from '@/hooks/useAuth'
import { api } from '@/api/client.ts'
import type { FlagTemplate } from '@/api/types.ts'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { TemplateFormDialog, DeleteTemplateDialog } from '@/components/TemplateFormDialog'

const INVALIDATE_KEY = ['templates', 'global']

export default function TemplatesPage() {
  const { user } = useAuth()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<FlagTemplate | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<FlagTemplate | null>(null)

  const { data: templates, isLoading } = useQuery({
    queryKey: INVALIDATE_KEY,
    queryFn: () => api.templates.listGlobal(),
  })

  if (user?.role !== 'admin') {
    return <Navigate to="/projects" replace />
  }

  const openCreate = () => {
    setEditing(null)
    setDialogOpen(true)
  }

  const openEdit = (t: FlagTemplate) => {
    setEditing(t)
    setDialogOpen(true)
  }

  const closeDialog = () => {
    setDialogOpen(false)
    setEditing(null)
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
            Global Templates
          </h1>
          <p className="text-[13px] text-muted-foreground/60">
            Manage flag templates available to all projects.
          </p>
        </div>
        <Button size="sm" onClick={openCreate}>
          Create Template
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {isLoading ? (
            <div className="text-center py-10 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading templates...
            </div>
          ) : !templates || templates.length === 0 ? (
            <div className="text-center py-10 text-muted-foreground/60 text-[13px]">
              No global templates yet. Create one to get started.
            </div>
          ) : (
            <div className="rounded-md overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Name
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Key
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Flag Type
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Value Type
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      System
                    </TableHead>
                    <TableHead className="font-mono text-[10px] uppercase tracking-wider text-muted-foreground">
                      Actions
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {templates.map((t) => (
                    <TableRow key={t.id} className="transition-colors hover:bg-[#d4956a]/8">
                      <TableCell className="text-[13px] text-foreground font-medium">
                        {t.name}
                      </TableCell>
                      <TableCell>
                        <span className="font-mono text-xs text-[#d4956a]">{t.key}</span>
                      </TableCell>
                      <TableCell>
                        <Badge variant="outline" className="font-mono text-[11px]">
                          {t.flag_type}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="font-mono text-[11px]">
                          {t.value_type}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {t.is_system && (
                          <Badge className="bg-amber-500/10 text-amber-400 border-amber-500/20 font-mono text-[11px]">
                            system
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-xs h-7"
                            onClick={() => openEdit(t)}
                          >
                            Edit
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                            onClick={() => setDeleteTarget(t)}
                            disabled={t.is_system}
                            title={t.is_system ? 'System templates cannot be deleted' : undefined}
                          >
                            Delete
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>

      <TemplateFormDialog
        key={editing?.id ?? 'create'}
        open={dialogOpen}
        editing={editing}
        onClose={closeDialog}
        onSaved={closeDialog}
        createFn={(payload) => api.templates.createGlobal(payload)}
        updateFn={(key, payload) => api.templates.updateGlobal(key, payload)}
        invalidateKey={INVALIDATE_KEY}
      />

      <DeleteTemplateDialog
        open={deleteTarget !== null}
        template={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        deleteFn={(key) => api.templates.deleteGlobal(key)}
        invalidateKey={INVALIDATE_KEY}
      />
    </div>
  )
}
