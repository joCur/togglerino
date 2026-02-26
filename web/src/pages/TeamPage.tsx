import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Card, CardContent } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

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

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString()
}

export default function TeamPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState<'admin' | 'member'>('member')
  const [inviteLink, setInviteLink] = useState<string | null>(null)
  const [copiedLink, setCopiedLink] = useState(false)

  const { data: members, isLoading: membersLoading } = useQuery({
    queryKey: ['users'],
    queryFn: () => api.get<SafeUser[]>('/management/users'),
  })

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

      {/* Your Account */}
      <Card className="mb-5">
        <CardContent className="p-6">
          <div className="text-sm font-semibold text-foreground mb-4">
            Your Account
          </div>
          <div className="flex flex-col gap-3.5">
            <div className="flex items-center gap-3">
              <span className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider min-w-[70px]">Email</span>
              <span className="text-[13px] text-foreground">{user?.email}</span>
            </div>
            <div className="flex items-center gap-3">
              <span className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider min-w-[70px]">Role</span>
              <Badge
                variant={user?.role === 'admin' ? 'secondary' : 'outline'}
                className="font-mono text-[11px]"
              >
                {user?.role || 'member'}
              </Badge>
            </div>
            <div className="flex items-center gap-3">
              <span className="font-mono text-[10px] font-medium text-muted-foreground uppercase tracking-wider min-w-[70px]">Joined</span>
              <span className="text-[13px] text-muted-foreground">
                {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '--'}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Invite Team Member */}
      {isAdmin && (
        <Card className="mb-5">
          <CardContent className="p-6">
            <div className="text-sm font-semibold text-foreground mb-4">
              Invite Team Member
            </div>

            <form onSubmit={handleInvite} className="flex gap-3 items-end flex-wrap">
              <div className="flex flex-col gap-1.5 flex-1 min-w-[200px]">
                <Label className="font-mono text-[10px] uppercase tracking-wider">Email</Label>
                <Input
                  type="email"
                  placeholder="colleague@example.com"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  required
                />
              </div>
              <div className="flex flex-col gap-1.5 min-w-[120px]">
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
                <div className="flex gap-2 items-center">
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
            <div className="rounded-md border overflow-hidden">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Email</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Role</TableHead>
                    <TableHead className="font-mono text-[11px] uppercase tracking-wider">Joined</TableHead>
                    {isAdmin && <TableHead className="font-mono text-[11px] uppercase tracking-wider">Actions</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {members.map((member) => (
                    <TableRow key={member.id} className="transition-colors hover:bg-[#d4956a]/8">
                      <TableCell className="text-[13px] text-foreground">
                        {member.email}
                        {member.id === user?.id && (
                          <span className="ml-2 text-[11px] text-muted-foreground italic">(you)</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <Badge
                          variant={member.role === 'admin' ? 'secondary' : 'outline'}
                          className="font-mono text-[11px]"
                        >
                          {member.role}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-[13px] text-muted-foreground">
                        {formatDate(member.created_at)}
                      </TableCell>
                      {isAdmin && (
                        <TableCell>
                          {member.id !== user?.id && (
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
                        </TableCell>
                      )}
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}

          {deleteMutation.error && (
            <Alert variant="destructive" className="mt-4">
              <AlertDescription>
                {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to remove member'}
              </AlertDescription>
            </Alert>
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
              <div className="rounded-md border overflow-hidden">
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
