import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAuth } from '../hooks/useAuth.ts'
import { api } from '../api/client.ts'
import { t } from '../theme.ts'

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

const labelStyle = {
  fontSize: 10,
  fontWeight: 500,
  color: t.textMuted,
  textTransform: 'uppercase' as const,
  letterSpacing: '0.8px',
  minWidth: 70,
  fontFamily: t.fontMono,
}

const inputStyle = {
  padding: '8px 12px',
  fontSize: 13,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusMd,
  backgroundColor: t.bgInput,
  color: t.textPrimary,
  outline: 'none',
  fontFamily: t.fontSans,
  transition: 'border-color 200ms ease',
} as const

const handleInputFocus = (e: React.FocusEvent<HTMLInputElement | HTMLSelectElement>) => {
  e.currentTarget.style.borderColor = t.accentBorder
}

const handleInputBlur = (e: React.FocusEvent<HTMLInputElement | HTMLSelectElement>) => {
  e.currentTarget.style.borderColor = t.border
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
    <div style={{ animation: 'fadeIn 300ms ease' }}>
      <div style={{ marginBottom: 32 }}>
        <h1 style={{ fontSize: 22, fontWeight: 600, color: t.textPrimary, marginBottom: 6, letterSpacing: '-0.3px' }}>
          Team Management
        </h1>
        <div style={{ fontSize: 13, color: t.textMuted }}>
          Manage your team members and their roles.
        </div>
      </div>

      {/* Your Account */}
      <div
        style={{
          padding: 24,
          borderRadius: t.radiusLg,
          backgroundColor: t.bgSurface,
          border: `1px solid ${t.border}`,
          marginBottom: 20,
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 18 }}>
          Your Account
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={labelStyle}>Email</span>
            <span style={{ fontSize: 13, color: t.textPrimary }}>{user?.email}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={labelStyle}>Role</span>
            <span
              style={{
                display: 'inline-block',
                padding: '3px 10px',
                borderRadius: 4,
                fontSize: 11,
                fontWeight: 500,
                backgroundColor: user?.role === 'admin' ? t.accentSubtle : t.bgElevated,
                color: user?.role === 'admin' ? t.accent : t.textSecondary,
                border: `1px solid ${user?.role === 'admin' ? t.accentBorder : t.border}`,
                fontFamily: t.fontMono,
                letterSpacing: '0.2px',
              }}
            >
              {user?.role || 'member'}
            </span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span style={labelStyle}>Joined</span>
            <span style={{ fontSize: 13, color: t.textSecondary }}>
              {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '--'}
            </span>
          </div>
        </div>
      </div>

      {/* Invite Team Member */}
      {isAdmin && (
        <div
          style={{
            padding: 24,
            borderRadius: t.radiusLg,
            backgroundColor: t.bgSurface,
            border: `1px solid ${t.border}`,
            marginBottom: 20,
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 18 }}>
            Invite Team Member
          </div>

          <form onSubmit={handleInvite} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1, minWidth: 200 }}>
              <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>
                Email
              </label>
              <input
                style={inputStyle}
                type="email"
                placeholder="colleague@example.com"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                required
                onFocus={handleInputFocus}
                onBlur={handleInputBlur}
              />
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 6, minWidth: 120 }}>
              <label style={{ fontSize: 10, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', fontFamily: t.fontMono }}>
                Role
              </label>
              <select
                style={{
                  ...inputStyle,
                  cursor: 'pointer',
                  appearance: 'none',
                  backgroundImage: `url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='12' height='12' viewBox='0 0 12 12'%3E%3Cpath fill='%234c4c56' d='M3 5l3 3 3-3'/%3E%3C/svg%3E")`,
                  backgroundRepeat: 'no-repeat',
                  backgroundPosition: 'right 10px center',
                  paddingRight: 28,
                }}
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value as 'admin' | 'member')}
                onFocus={handleInputFocus}
                onBlur={handleInputBlur}
              >
                <option value="member">Member</option>
                <option value="admin">Admin</option>
              </select>
            </div>
            <button
              type="submit"
              disabled={inviteMutation.isPending}
              style={{
                padding: '8px 18px',
                fontSize: 13,
                fontWeight: 600,
                border: 'none',
                borderRadius: t.radiusMd,
                background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`,
                color: '#ffffff',
                cursor: inviteMutation.isPending ? 'not-allowed' : 'pointer',
                fontFamily: t.fontSans,
                transition: 'all 200ms ease',
                boxShadow: '0 2px 10px rgba(212,149,106,0.15)',
                opacity: inviteMutation.isPending ? 0.7 : 1,
                whiteSpace: 'nowrap',
              }}
              onMouseEnter={(e) => {
                if (!inviteMutation.isPending) {
                  e.currentTarget.style.boxShadow = '0 4px 18px rgba(212,149,106,0.3)'
                  e.currentTarget.style.transform = 'translateY(-1px)'
                }
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = '0 2px 10px rgba(212,149,106,0.15)'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              {inviteMutation.isPending ? 'Sending...' : 'Send Invite'}
            </button>
          </form>

          {inviteMutation.error && (
            <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginTop: 16 }}>
              {inviteMutation.error instanceof Error ? inviteMutation.error.message : 'Failed to send invite'}
            </div>
          )}

          {inviteLink && (
            <div
              style={{
                marginTop: 16,
                padding: 16,
                borderRadius: t.radiusMd,
                backgroundColor: t.successSubtle,
                border: `1px solid ${t.successBorder}`,
                animation: 'fadeIn 200ms ease',
              }}
            >
              <div style={{ fontSize: 13, fontWeight: 500, color: t.success, marginBottom: 10 }}>
                Invite sent! Share this link with the team member:
              </div>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <input
                  readOnly
                  value={inviteLink}
                  style={{
                    flex: 1,
                    padding: '8px 12px',
                    fontSize: 12,
                    fontFamily: t.fontMono,
                    border: `1px solid ${t.successBorder}`,
                    borderRadius: t.radiusSm,
                    backgroundColor: t.bgBase,
                    color: t.textPrimary,
                    outline: 'none',
                  }}
                  onClick={(e) => e.currentTarget.select()}
                />
                <button
                  type="button"
                  onClick={handleCopyLink}
                  style={{
                    padding: '8px 14px',
                    fontSize: 12,
                    fontWeight: 500,
                    border: `1px solid ${t.successBorder}`,
                    borderRadius: t.radiusSm,
                    backgroundColor: 'transparent',
                    color: copiedLink ? t.success : t.textSecondary,
                    cursor: 'pointer',
                    fontFamily: t.fontSans,
                    transition: 'all 200ms ease',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {copiedLink ? 'Copied!' : 'Copy'}
                </button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Team Members */}
      <div
        style={{
          padding: 24,
          borderRadius: t.radiusLg,
          backgroundColor: t.bgSurface,
          border: `1px solid ${t.border}`,
          marginBottom: 20,
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 18 }}>
          Team Members
        </div>

        {membersLoading ? (
          <div style={{ textAlign: 'center', padding: 32, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
            Loading members...
          </div>
        ) : !members || members.length === 0 ? (
          <div style={{ textAlign: 'center', padding: 32, color: t.textMuted, fontSize: 13 }}>
            No team members found.
          </div>
        ) : (
          <div style={{ borderRadius: t.radiusMd, border: `1px solid ${t.border}`, overflow: 'hidden' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead>
                <tr>
                  {['Email', 'Role', 'Joined', ...(isAdmin ? ['Actions'] : [])].map((h) => (
                    <th key={h} style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', borderBottom: `1px solid ${t.border}`, backgroundColor: t.bgElevated, fontFamily: t.fontMono }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {members.map((member) => (
                  <tr
                    key={member.id}
                    style={{ borderBottom: `1px solid ${t.border}`, transition: 'background-color 200ms ease' }}
                    onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.accentSubtle }}
                    onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
                  >
                    <td style={{ padding: '12px 16px', fontSize: 13, color: t.textPrimary }}>
                      {member.email}
                      {member.id === user?.id && (
                        <span style={{ marginLeft: 8, fontSize: 11, color: t.textMuted, fontStyle: 'italic' }}>
                          (you)
                        </span>
                      )}
                    </td>
                    <td style={{ padding: '12px 16px' }}>
                      <span
                        style={{
                          display: 'inline-block',
                          padding: '2px 8px',
                          borderRadius: 4,
                          fontSize: 11,
                          fontWeight: 500,
                          backgroundColor: member.role === 'admin' ? t.accentSubtle : t.bgElevated,
                          color: member.role === 'admin' ? t.accent : t.textSecondary,
                          border: `1px solid ${member.role === 'admin' ? t.accentBorder : t.border}`,
                          fontFamily: t.fontMono,
                          letterSpacing: '0.2px',
                        }}
                      >
                        {member.role}
                      </span>
                    </td>
                    <td style={{ padding: '12px 16px', fontSize: 13, color: t.textSecondary }}>
                      {formatDate(member.created_at)}
                    </td>
                    {isAdmin && (
                      <td style={{ padding: '12px 16px' }}>
                        {member.id !== user?.id && (
                          <button
                            style={{
                              padding: '4px 10px',
                              fontSize: 12,
                              fontWeight: 500,
                              border: `1px solid ${t.dangerBorder}`,
                              borderRadius: t.radiusSm,
                              backgroundColor: 'transparent',
                              color: t.danger,
                              cursor: 'pointer',
                              fontFamily: t.fontSans,
                              transition: 'all 200ms ease',
                            }}
                            onClick={() => handleDelete(member)}
                            disabled={deleteMutation.isPending}
                            onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.dangerSubtle }}
                            onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
                          >
                            Remove
                          </button>
                        )}
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {deleteMutation.error && (
          <div style={{ padding: '10px 14px', borderRadius: t.radiusMd, backgroundColor: t.dangerSubtle, border: `1px solid ${t.dangerBorder}`, color: t.danger, fontSize: 13, marginTop: 16 }}>
            {deleteMutation.error instanceof Error ? deleteMutation.error.message : 'Failed to remove member'}
          </div>
        )}
      </div>

      {/* Pending Invites */}
      {isAdmin && (
        <div
          style={{
            padding: 24,
            borderRadius: t.radiusLg,
            backgroundColor: t.bgSurface,
            border: `1px solid ${t.border}`,
            marginBottom: 20,
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 600, color: t.textPrimary, marginBottom: 18 }}>
            Pending Invites
          </div>

          {invitesLoading ? (
            <div style={{ textAlign: 'center', padding: 32, color: t.textMuted, fontSize: 13, animation: 'shimmer 1.5s ease infinite' }}>
              Loading invites...
            </div>
          ) : !invites || invites.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 32, color: t.textMuted, fontSize: 13 }}>
              No pending invites.
            </div>
          ) : (
            <div style={{ borderRadius: t.radiusMd, border: `1px solid ${t.border}`, overflow: 'hidden' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                <thead>
                  <tr>
                    {['Email', 'Role', 'Expires'].map((h) => (
                      <th key={h} style={{ textAlign: 'left', padding: '10px 16px', fontSize: 11, fontWeight: 500, color: t.textMuted, textTransform: 'uppercase', letterSpacing: '0.8px', borderBottom: `1px solid ${t.border}`, backgroundColor: t.bgElevated, fontFamily: t.fontMono }}>
                        {h}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {invites.map((invite) => (
                    <tr
                      key={invite.id}
                      style={{ borderBottom: `1px solid ${t.border}`, transition: 'background-color 200ms ease' }}
                      onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = t.accentSubtle }}
                      onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent' }}
                    >
                      <td style={{ padding: '12px 16px', fontSize: 13, color: t.textPrimary }}>
                        {invite.email}
                      </td>
                      <td style={{ padding: '12px 16px' }}>
                        <span
                          style={{
                            display: 'inline-block',
                            padding: '2px 8px',
                            borderRadius: 4,
                            fontSize: 11,
                            fontWeight: 500,
                            backgroundColor: invite.role === 'admin' ? t.accentSubtle : t.bgElevated,
                            color: invite.role === 'admin' ? t.accent : t.textSecondary,
                            border: `1px solid ${invite.role === 'admin' ? t.accentBorder : t.border}`,
                            fontFamily: t.fontMono,
                            letterSpacing: '0.2px',
                          }}
                        >
                          {invite.role}
                        </span>
                      </td>
                      <td style={{ padding: '12px 16px', fontSize: 13, color: t.textSecondary }}>
                        {formatDate(invite.expires_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
