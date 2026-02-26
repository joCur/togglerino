import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'

function GeneralSettings({ project, projectKey }: { project: Project; projectKey: string }) {
  const queryClient = useQueryClient()
  const [name, setName] = useState(project.name)
  const [description, setDescription] = useState(project.description)
  const [saveSuccess, setSaveSuccess] = useState(false)

  const hasChanges = name !== project.name || description !== project.description

  const updateMutation = useMutation({
    mutationFn: (data: { name: string; description: string }) =>
      api.put<Project>(`/projects/${projectKey}`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey] })
      setSaveSuccess(true)
      setTimeout(() => setSaveSuccess(false), 2000)
    },
  })

  const handleSave = () => {
    if (!hasChanges) return
    updateMutation.mutate({ name: name.trim(), description: description.trim() })
  }

  return (
    <Card className="mb-6">
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-4">
          General
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label className="font-mono text-[10px] uppercase tracking-wider">Description</Label>
            <Textarea
              className="min-h-[80px]"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>

          <div className="flex items-center gap-3">
            <Button
              onClick={handleSave}
              disabled={!hasChanges || updateMutation.isPending}
            >
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>

            {saveSuccess && (
              <span className="text-[13px] text-emerald-400 animate-[fadeIn_200ms_ease]">Saved</span>
            )}

            {updateMutation.error && (
              <span className="text-[13px] text-destructive">
                {updateMutation.error instanceof Error ? updateMutation.error.message : 'Failed to save'}
              </span>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export default function ProjectSettingsPage() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const { data: project, isLoading, error } = useQuery({
    queryKey: ['projects', key],
    queryFn: () => api.get<Project>(`/projects/${key}`),
    enabled: !!key,
  })

  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deleteConfirmInput, setDeleteConfirmInput] = useState('')

  const deleteMutation = useMutation({
    mutationFn: () => api.delete(`/projects/${key}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects'] })
      navigate('/projects')
    },
  })

  const handleDelete = () => {
    if (deleteConfirmInput !== key) return
    deleteMutation.mutate()
  }

  if (isLoading) {
    return (
      <div className="text-center py-16 text-muted-foreground/60 text-[13px] animate-pulse">
        Loading project settings...
      </div>
    )
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertDescription>
          Failed to load project: {error instanceof Error ? error.message : 'Unknown error'}
        </AlertDescription>
      </Alert>
    )
  }

  return (
    <div className="animate-[fadeIn_300ms_ease] max-w-[640px]">
      <div className="mb-8">
        <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
          Project Settings
        </h1>
        <div className="text-[13px] text-muted-foreground/60">
          Manage settings for <span className="font-mono text-muted-foreground">{key}</span>
        </div>
      </div>

      {/* General Section */}
      {project && <GeneralSettings key={`${project.name}|${project.description}`} project={project} projectKey={key!} />}

      {/* Members Section (placeholder) */}
      <Card className="mb-6">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-foreground mb-3">
            Members
          </div>
          <div className="text-[13px] text-muted-foreground/60">
            Project-level member management coming soon.
          </div>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border-destructive/25">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-destructive mb-3">
            Danger Zone
          </div>
          <div className="text-[13px] text-muted-foreground leading-relaxed mb-4">
            Deleting this project is permanent and cannot be undone. All flags, environments, and SDK keys associated with this project will be removed.
          </div>

          {!showDeleteConfirm ? (
            <Button
              variant="outline"
              className="border-destructive/50 text-destructive hover:bg-destructive/10"
              onClick={() => setShowDeleteConfirm(true)}
            >
              Delete Project
            </Button>
          ) : (
            <div className="flex flex-col gap-3 animate-[fadeIn_200ms_ease]">
              <div className="text-[13px] text-muted-foreground">
                Type <span className="font-mono text-destructive font-semibold">{key}</span> to confirm deletion:
              </div>
              <Input
                value={deleteConfirmInput}
                onChange={(e) => setDeleteConfirmInput(e.target.value)}
                placeholder={key}
                autoFocus
              />
              <div className="flex gap-3">
                <Button
                  variant="destructive"
                  disabled={deleteConfirmInput !== key || deleteMutation.isPending}
                  onClick={handleDelete}
                >
                  {deleteMutation.isPending ? 'Deleting...' : 'Delete'}
                </Button>
                <Button
                  variant="outline"
                  onClick={() => { setShowDeleteConfirm(false); setDeleteConfirmInput('') }}
                >
                  Cancel
                </Button>
              </div>
              {deleteMutation.error && (
                <Alert variant="destructive">
                  <AlertDescription>
                    {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to delete project'}
                  </AlertDescription>
                </Alert>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
