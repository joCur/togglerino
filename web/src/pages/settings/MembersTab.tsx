import { useState, useEffect } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '@/hooks/useAuth'
import { useProjectMembers, type ProjectRole } from '@/hooks/usePermissions'
import { api } from '@/api/client'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
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
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'

interface UserSearchResult {
  id: string
  email: string
  display_name: string | null
}

function useUserSearch(query: string) {
  return useQuery({
    queryKey: ['user-search', query],
    queryFn: () =>
      api.get<UserSearchResult[]>(`/users/search?q=${encodeURIComponent(query)}`),
    enabled: query.length >= 1,
    staleTime: 30_000,
  })
}

const roleOptions: ProjectRole[] = ['admin', 'editor', 'viewer']

function roleBadgeVariant(role: string): 'secondary' | 'outline' | 'default' {
  if (role === 'admin') return 'secondary'
  return 'outline'
}

export default function MembersTab() {
  const { key } = useParams<{ key: string }>()
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const { data: members, isLoading } = useProjectMembers(key)

  const [addOpen, setAddOpen] = useState(false)
  const [selectedUser, setSelectedUser] = useState<UserSearchResult | null>(null)
  const [userSearchQuery, setUserSearchQuery] = useState('')
  const [userPopoverOpen, setUserPopoverOpen] = useState(false)
  const [addRole, setAddRole] = useState<ProjectRole>('editor')
  const [error, setError] = useState('')

  const { data: searchResults } = useUserSearch(userSearchQuery)

  // Reset form state when dialog closes
  useEffect(() => {
    if (!addOpen) {
      setSelectedUser(null)
      setUserSearchQuery('')
      setAddRole('editor')
      setError('')
    }
  }, [addOpen])

  const addMutation = useMutation({
    mutationFn: (data: { user_id: string; role: ProjectRole }) =>
      api.post(`/projects/${key}/members`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-members', key] })
      setAddOpen(false)
    },
    onError: (err: Error) => {
      setError(err.message)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: ProjectRole }) =>
      api.put(`/projects/${key}/members/${userId}`, { role }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-members', key] })
    },
  })

  const removeMutation = useMutation({
    mutationFn: (userId: string) =>
      api.delete(`/projects/${key}/members/${userId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['project-members', key] })
    },
  })

  const handleAdd = (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedUser) return
    setError('')
    addMutation.mutate({ user_id: selectedUser.id, role: addRole })
  }

  const handleRemove = (userId: string, email: string) => {
    if (window.confirm(`Remove ${email} from this project?`)) {
      removeMutation.mutate(userId)
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <Card>
        <CardContent className="p-6">
          <div className="flex items-center justify-between mb-4">
            <div>
              <div className="text-sm font-semibold text-foreground mb-1">
                Project Members
              </div>
              <div className="text-[13px] text-muted-foreground/60">
                Manage who has access to this project and their roles.
              </div>
            </div>
            <Dialog open={addOpen} onOpenChange={setAddOpen}>
              <DialogTrigger asChild>
                <Button size="sm">Add Member</Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  <DialogTitle>Add Project Member</DialogTitle>
                  <DialogDescription>
                    Add a team member to this project by their email address.
                  </DialogDescription>
                </DialogHeader>
                <form onSubmit={handleAdd} className="flex flex-col gap-4">
                  <div className="flex flex-col gap-1.5">
                    <Label className="font-mono text-[10px] uppercase tracking-wider">
                      User
                    </Label>
                    <Popover open={userPopoverOpen} onOpenChange={setUserPopoverOpen}>
                      <PopoverTrigger asChild>
                        <Button
                          variant="outline"
                          role="combobox"
                          aria-expanded={userPopoverOpen}
                          className="justify-between font-normal"
                          type="button"
                        >
                          {selectedUser
                            ? selectedUser.email
                            : 'Search for a user...'}
                          <span className="ml-2 opacity-50 text-xs">
                            {selectedUser ? '' : ''}
                          </span>
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent className="w-[--radix-popover-trigger-width] p-0" align="start">
                        <Command shouldFilter={false}>
                          <CommandInput
                            placeholder="Search by email or name..."
                            value={userSearchQuery}
                            onValueChange={setUserSearchQuery}
                          />
                          <CommandList>
                            <CommandEmpty>
                              {userSearchQuery.length < 1
                                ? 'Type to search...'
                                : 'No users found.'}
                            </CommandEmpty>
                            <CommandGroup>
                              {(searchResults ?? []).map((u) => (
                                <CommandItem
                                  key={u.id}
                                  value={u.id}
                                  onSelect={() => {
                                    setSelectedUser(u)
                                    setUserPopoverOpen(false)
                                  }}
                                >
                                  <div className="flex flex-col">
                                    <span className="text-sm">{u.email}</span>
                                    {u.display_name && (
                                      <span className="text-xs text-muted-foreground">
                                        {u.display_name}
                                      </span>
                                    )}
                                  </div>
                                </CommandItem>
                              ))}
                            </CommandGroup>
                          </CommandList>
                        </Command>
                      </PopoverContent>
                    </Popover>
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <Label className="font-mono text-[10px] uppercase tracking-wider">
                      Role
                    </Label>
                    <Select
                      value={addRole}
                      onValueChange={(v) => setAddRole(v as ProjectRole)}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {roleOptions.map((r) => (
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
                      disabled={!selectedUser || addMutation.isPending}
                    >
                      {addMutation.isPending ? 'Adding...' : 'Add Member'}
                    </Button>
                  </DialogFooter>
                </form>
              </DialogContent>
            </Dialog>
          </div>

          {isLoading ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px] animate-pulse">
              Loading members...
            </div>
          ) : !members || members.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground/60 text-[13px]">
              No project members. Access is controlled by the base project role setting.
            </div>
          ) : (
            <div className="rounded-md border overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Email
                    </TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">
                      Display Name
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
                  {members.map((member) => (
                    <TableRow
                      key={member.user_id}
                      className="transition-colors hover:bg-[#d4956a]/8"
                    >
                      <TableCell className="text-[13px] text-foreground">
                        {member.email}
                        {member.user_id === user?.id && (
                          <span className="ml-2 text-[11px] text-muted-foreground italic">
                            (you)
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {member.display_name || '-'}
                      </TableCell>
                      <TableCell>
                        <Select
                          value={member.role}
                          onValueChange={(v) =>
                            updateMutation.mutate({
                              userId: member.user_id,
                              role: v as ProjectRole,
                            })
                          }
                          disabled={member.user_id === user?.id}
                        >
                          <SelectTrigger className="w-[110px] h-8 text-xs">
                            <SelectValue>
                              <Badge
                                variant={roleBadgeVariant(member.role)}
                                className="font-mono text-[11px]"
                              >
                                {member.role}
                              </Badge>
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent>
                            {roleOptions.map((r) => (
                              <SelectItem key={r} value={r}>
                                {r}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell>
                        {member.user_id !== user?.id && (
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-xs h-7 border-destructive/50 text-destructive hover:bg-destructive/10"
                            onClick={() =>
                              handleRemove(member.user_id, member.email)
                            }
                            disabled={removeMutation.isPending}
                          >
                            Remove
                          </Button>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {(updateMutation.error || removeMutation.error) && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>
                {(updateMutation.error || removeMutation.error) instanceof Error
                  ? (updateMutation.error || removeMutation.error)?.message
                  : 'An error occurred'}
              </AlertDescription>
            </Alert>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
