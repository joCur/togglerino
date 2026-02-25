import { useAuth } from '../hooks/useAuth.ts'
import { t } from '../theme.ts'

export default function TeamPage() {
  const { user } = useAuth()

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
            <span
              style={{
                fontSize: 10,
                fontWeight: 500,
                color: t.textMuted,
                textTransform: 'uppercase',
                letterSpacing: '0.8px',
                minWidth: 70,
                fontFamily: t.fontMono,
              }}
            >
              Email
            </span>
            <span style={{ fontSize: 13, color: t.textPrimary }}>{user?.email}</span>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
            <span
              style={{
                fontSize: 10,
                fontWeight: 500,
                color: t.textMuted,
                textTransform: 'uppercase',
                letterSpacing: '0.8px',
                minWidth: 70,
                fontFamily: t.fontMono,
              }}
            >
              Role
            </span>
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
            <span
              style={{
                fontSize: 10,
                fontWeight: 500,
                color: t.textMuted,
                textTransform: 'uppercase',
                letterSpacing: '0.8px',
                minWidth: 70,
                fontFamily: t.fontMono,
              }}
            >
              Joined
            </span>
            <span style={{ fontSize: 13, color: t.textSecondary }}>
              {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '--'}
            </span>
          </div>
        </div>
      </div>

      <div
        style={{
          padding: 28,
          borderRadius: t.radiusLg,
          backgroundColor: t.bgSurface,
          border: `1px dashed ${t.border}`,
          textAlign: 'center',
        }}
      >
        <div style={{ fontSize: 13, color: t.textMuted, lineHeight: 1.7 }}>
          User management features are coming soon.
          <br />
          You will be able to invite team members, manage roles, and control access from this page.
        </div>
      </div>
    </div>
  )
}
