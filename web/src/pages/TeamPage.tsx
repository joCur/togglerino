import { useState } from 'react'
import { useQuery, useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import type { UserProjectAssignment, ProjectRole } from '@/hooks/usePermissions'
import { useRoles } from '@/hooks/useRoles'
import type { Project, PaginatedResponse } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

interface SafeUser {
  id: string
  email: string
  role: string
  created_at: string
}

interface Invite {
  id: string
  email: string
  role: string
  expires_at: string
  created_at: string
}

interface InviteResponse {
  id: string
  token: string
  expires_at: string
}

const PAGE_SIZE = 50

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

function UserProjectAssignments({ userId }: { userId: string }) {
  const queryClient = useQueryClient()
  const { data: roles } = useRoles()
  const projectRoleOptions = (roles ?? []).map(r => r.name)
  const [addOpen, setAddOpen] = useState(false)
  const [selectedProject, setSelectedProject] = useState('')
  const [selectedRole, setSelectedRole] = useState<ProjectRole>('editor')
  const [error, setError] = useState('')

  const { data: assignments, isLoading } = useQuery({
    queryKey: ['user-projects', userId],
    queryFn: () =>
      api.get<UserProjectAssignment[]>(`/management/users/${userId}/projects`),
  })

  const { data: projectsResponse } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.get<PaginatedResponse<Project>>('/projects'),
  })
  const projects = projectsResponse?.data

  const saveMutation = useMutation({
    mutationFn: (data: UserProjectAssignment[]) =>
      api.put(`/management/users/${userId}/projects`, {
        assignments: data.map((a) => ({ project_id: a.project_id, role: a.role })),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['user-projects', userId] })
    },
  })

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedProject) return
    setError('')

    const project = projects?.find((p) => p.key === selectedProject)
    if (!project) return

    // Check duplicate
    if (assignments?.some((a) => a.project_key === selectedProject)) {
      setError('User already has an assignment for this project')
      return
    }

    const updated = [
      ...(assignments ?? []),
      {
        project_id: project.id,
        project_key: project.key,
        project_name: project.name,
        role: selectedRole,
      },
    ]
    saveMutation.mutate(updated, {
      onSuccess: () => {
        setAddOpen(false)
        setSelectedProject('')
        setSelectedRole('editor')
        setError('')
      },
      onError: (err: Error) => {
        setError(err.message)
      },
    })
  }

  const handleRemove = (projectKey: string) => {
    const updated = (assignments ?? []).filter(
      (a) => a.project_key !== projectKey,
    )
    saveMutation.mutate(updated)
  }

  const handleRoleChange = (projectKey: string, role: ProjectRole) => {
    const updated = (assignments ?? []).map((a) =>
      a.project_key === projectKey ? { ...a, role } : a,
    )
    saveMutation.mutate(updated)
  }

  // Filter out projects that already have assignments
  const availableProjects = projects?.filter(
    (p) => !assignments?.some((a) => a.project_key === p.key),
  )

  if (isLoading) {
    return (
      <div className="text-[13px] text-muted-foreground/60 animate-pulse py-2 pl-4">
        Loading assignments...
      </div>
    )
  }

  return (
    <div className="pl-4 border-l-2 border-border/50 ml-2 mt-2">
      <div className="flex items-center justify-between mb-2">
        <div className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
          Project Assignments
        </div>
        <Dialog open={addOpen} onOpenChange={setAddOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" size="sm" className="text-xs h-7">
              Add Project
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add Project Assignment</DialogTitle>
              <DialogDescription>
                Assign this user to a project with a specific role.
              </DialogDescription>
            </DialogHeader>
            <form onSubmit={handleAdd} className="flex flex-col gap-4">
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">
                  Project
                </Label>
                <Select
                  value={selectedProject}
                  onValueChange={setSelectedProject}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select a project" />
                  </SelectTrigger>
                  <SelectContent>
                    {availableProjects?.map((p) => (
                      <SelectItem key={p.key} value={p.key}>
                        {p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="flex flex-col gap-1.5">
                <Label className="font-mono text-[10px] uppercase tracking-wider">
                  Role
                </Label>
                <Select
                  value={selectedRole}
                  onValueChange={(v) => setSelectedRole(v as ProjectRole)}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {projectRoleOptions.map((r) => (
                      <SelectItem key={r} value={r}>
                        {r}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {error && (
                <div className="text-[13px] text-destructive">{error}</div>
              )}
              <DialogFooter>
                <Button
                  type="submit"
                  size="sm"
                  disabled={saveMutation.isPending || !selectedProject}
                >
                  {saveMutation.isPending ? 'Saving...' : 'Add Assignment'}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {!assignments || assignments.length === 0 ? (
        <div className="text-[13px] text-muted-foreground/60 py-2">
          No project-specific assignments. Access determined by base project
          role.
        </div>
      ) : (
        <div className="rounded-md border overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                  Project
                </TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                  Role
                </TableHead>
                <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                  Actions
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {assignments.map((a) => (
                <TableRow
                  key={a.project_key}
                  className="transition-colors hover:bg-[#d4956a]/8"
                >
                  <TableCell className="text-[13px] text-foreground">
                    {a.project_name}
                    <span className="ml-1.5 text-[11px] text-muted-foreground font-mono">
                      {a.project_key}
                    </span>
                  </TableCell>
                  <TableCell>
                    <Select
                      value={a.role}
                      onValueChange={(v) =>
                        handleRoleChange(a.project_key, v as ProjectRole)
                      }
                    >
                      <SelectTrigger className="w-[110px] h-8 text-xs">
                        <SelectValue>
                          <Badge
                            variant={
                              a.role === 'admin' ? 'secondary' : 'outline'
                            }
                            className="font-mono text-[11px]"
                          >
                            {a.role}
                          </Badge>
                        </SelectValue>
                      </SelectTrigger>
                      <SelectContent>
                        {projectRoleOptions.map((r) => (
                          <SelectItem key={r} value={r}>
                            {r}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="outline"
                      size="sm"
                      className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                      onClick={() => handleRemove(a.project_key)}
                      disabled={saveMutation.isPending}
                    >
                      Remove
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {saveMutation.error && (
        <Alert variant="destructive" className="mt-2">
          <AlertDescription>
            {saveMutation.error instanceof Error
              ? saveMutation.error.message
              : 'Failed to update assignments'}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}

export default function TeamPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState<'admin' | 'member'>('member')
  const [inviteLink, setInviteLink] = useState<string | null>(null)
  const [copiedLink, setCopiedLink] = useState(false)
  const [expandedUser, setExpandedUser] = useState<string | null>(null)

  const {
    data: membersData,
    isLoading: membersLoading,
    hasNextPage: membersHasNextPage,
    fetchNextPage: fetchNextMembers,
    isFetchingNextPage: membersFetchingNext,
  } = useInfiniteQuery({
    queryKey: ['users'],
    queryFn: ({ pageParam = 0 }) =>
      api.users.list({ limit: PAGE_SIZE, offset: pageParam }),
    initialPageParam: 0,
    getNextPageParam: (lastPage) =>
      lastPage.offset + lastPage.limit < lastPage.total
        ? lastPage.offset + lastPage.limit
        : undefined,
  })

  const members = membersData?.pages.flatMap((page) => page.data)

  const { data: invites, isLoading: invitesLoading } = useQuery({
    queryKey: ['invites'],
    queryFn: () => api.get<Invite[]>('/management/users/invites'),
  })

  const inviteMutation = useMutation({
    mutationFn: (data: { email: string; role: string }) =>
      api.post<InviteResponse>('/management/users/invite', data),
    onSuccess: (data) => {
      const link = `${window.location.origin}/invite/${data.token}`
      setInviteLink(link)
      setInviteEmail('')
      setInviteRole('member')
      queryClient.invalidateQueries({ queryKey: ['invites'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => api.delete<void>(`/management/users/${id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const handleInvite = (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteEmail.trim()) return
    setInviteLink(null)
    setCopiedLink(false)
    inviteMutation.mutate({ email: inviteEmail.trim(), role: inviteRole })
  }

  const handleCopyLink = async () => {
    if (!inviteLink) return
    try {
      await navigator.clipboard.writeText(inviteLink)
      setCopiedLink(true)
      setTimeout(() => setCopiedLink(false), 2000)
    } catch {
      // Clipboard API may not be available
    }
  }

  const handleDelete = (member: SafeUser) => {
    if (window.confirm(`Are you sure you want to remove ${member.email} from the team? This action cannot be undone.`)) {
      deleteMutation.mutate(member.id)
    }
  }

  const isAdmin = user?.role === 'admin'

  const toggleExpanded = (userId: string) => {
    setExpandedUser((prev) => (prev === userId ? null : userId))
  }

  return (
    <div className="animate-[fadeIn_300ms_ease]">
      <div className="mb-8">
        <h1 className="text-[22px] font-semibold text-foreground mb-1.5 tracking-tight">
          Team Management
        </h1>
        <div className="text-[13px] text-muted-foreground/60">
          Manage your team members and their roles.
        </div>
      </div>

      {/* Invite Team Member */}
      {isAdmin && (
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Invite Team Member
            </div>

            <form onSubmit={handleInvite} className="flex flex-col md:flex-row gap-3 md:items-end">
              <div className="flex flex-col gap-1.5 flex-1">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Email</Label>
                <Input
                  type="email"
                  placeholder="colleague@example.com"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5 w-full md:w-auto">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Role</Label>
                <select
                  className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer"
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value as 'admin' | 'member')}
                >
                  <option value="member">Member</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <Button type="submit" disabled={inviteMutation.isPending}>
                {inviteMutation.isPending ? 'Sending...' : 'Send Invite'}
              </Button>
            </form>

            {inviteMutation.error && (
              <Alert variant="destructive" className="mt-4">
                <AlertDescription>
                  {inviteMutation.error instanceof Error ? inviteMutation.error.message : 'Failed to send invite'}
                </AlertDescription>
              </Alert>
            )}

            {inviteLink && (
              <div className="mt-4 p-4 rounded-md bg-emerald-500/10 border border-emerald-500/20 animate-[fadeIn_200ms_ease]">
                <div className="text-[13px] font-medium text-emerald-400 mb-2.5">
                  Invite sent! Share this link with the team member:
                </div>
                <div className="flex flex-col md:flex-row gap-2 md:items-center">
                  <Input
                    readOnly
                    value={inviteLink}
                    className="flex-1 font-mono text-xs"
                    onClick={(e) => e.currentTarget.select()}
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleCopyLink}
                  >
                    {copiedLink ? 'Copied!' : 'Copy'}
                  </Button>
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Team Members */}
      <Card className="mb-5">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-foreground mb-4">
            Team Members
          </div>

          {membersLoading ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading members...
            </div>
          ) : !members || members.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
              No team members found.
            </div>
          ) : (
            <div className="flex flex-col gap-0">
              {members.map((member) => (
                <div key={member.id} className="border-b last:border-b-0">
                  <div className="flex items-center gap-4 py-3 px-2 transition-colors hover:bg-[#d4956a]/8">
                    <div className="flex-1 text-[13px] text-foreground">
                      {member.email}
                      {member.id === user?.id && (
                        <span className="ml-2 text-[11px] text-muted-foreground italic">(you)</span>
                      )}
                    </div>
                    <Badge
                      variant={member.role === 'admin' ? 'secondary' : 'outline'}
                      className="font-mono text-[11px]"
                    >
                      {member.role}
                    </Badge>
                    <div className="text-[13px] text-muted-foreground w-24">
                      {formatDate(member.created_at)}
                    </div>
                    <div className="flex items-center gap-2">
                      {isAdmin && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-xs h-7"
                          onClick={() => toggleExpanded(member.id)}
                        >
                          {expandedUser === member.id ? 'Hide Projects' : 'Projects'}
                        </Button>
                      )}
                      {isAdmin && member.id !== user?.id && (
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                          onClick={() => handleDelete(member)}
                          disabled={deleteMutation.isPending}
                        >
                          Remove
                        </Button>
                      )}
                    </div>
                  </div>
                  {isAdmin && expandedUser === member.id && (
                    <div className="pb-4 px-2 animate-[fadeIn_200ms_ease]">
                      <UserProjectAssignments userId={member.id} />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}

          {deleteMutation.error && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to remove member'}
              </AlertDescription>
            </Alert>
          )}

          {membersHasNextPage && (
            <div className="text-center mt-4">
              <Button variant="outline" onClick={() => fetchNextMembers()} disabled={membersFetchingNext}>
                {membersFetchingNext ? 'Loading...' : 'Load More'}
              </Button>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Pending Invites */}
      {isAdmin && (
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Pending Invites
            </div>

            {invitesLoading ? (
              <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
                Loading invites...
              </div>
            ) : !invites || invites.length === 0 ? (
              <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
                No pending invites.
              </div>
            ) : (
              <div className="rounded-md border overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="font-mono text-[11px] uppercase tracking-wider">Email</TableHead>
                      <TableHead className="font-mono text-[11px] uppercase tracking-wider">Role</TableHead>
                      <TableHead className="font-mono text-[11px] uppercase tracking-wider">Expires</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {invites.map((invite) => (
                      <TableRow key={invite.id} className="transition-colors hover:bg-[#d4956a]/8">
                        <TableCell className="text-[13px] text-foreground">
                          {invite.email}
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant={invite.role === 'admin' ? 'secondary' : 'outline'}
                            className="font-mono text-[11px]"
                          >
                            {invite.role}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-[13px] text-muted-foreground">
                          {formatDate(invite.expires_at)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
