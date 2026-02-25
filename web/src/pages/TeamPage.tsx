import { useAuth } from '../hooks/useAuth.ts'

const styles = {
  header: {
    marginBottom: 32,
  } as const,
  title: {
    fontSize: 24,
    fontWeight: 700,
    color: '#ffffff',
    marginBottom: 8,
  } as const,
  subtitle: {
    fontSize: 14,
    color: '#8892b0',
  } as const,
  card: {
    padding: 24,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px solid #2a2a4a',
    marginBottom: 24,
  } as const,
  cardTitle: {
    fontSize: 16,
    fontWeight: 600,
    color: '#ffffff',
    marginBottom: 16,
  } as const,
  infoRow: {
    display: 'flex',
    alignItems: 'center',
    gap: 12,
    marginBottom: 12,
  } as const,
  infoLabel: {
    fontSize: 12,
    fontWeight: 600,
    color: '#8892b0',
    textTransform: 'uppercase' as const,
    letterSpacing: '0.5px',
    minWidth: 80,
  } as const,
  infoValue: {
    fontSize: 14,
    color: '#e0e0e0',
  } as const,
  roleBadge: (role: string) => ({
    display: 'inline-block',
    padding: '3px 10px',
    borderRadius: 4,
    fontSize: 12,
    fontWeight: 600,
    backgroundColor: role === 'admin' ? 'rgba(233, 69, 96, 0.15)' : 'rgba(15, 52, 96, 0.8)',
    color: role === 'admin' ? '#e94560' : '#8892b0',
  }),
  placeholder: {
    padding: 32,
    borderRadius: 10,
    backgroundColor: '#16213e',
    border: '1px dashed #2a2a4a',
    textAlign: 'center' as const,
  } as const,
  placeholderText: {
    fontSize: 14,
    color: '#8892b0',
    lineHeight: 1.6,
  } as const,
}

export default function TeamPage() {
  const { user } = useAuth()

  return (
    <div>
      <div style={styles.header}>
        <h1 style={styles.title}>Team Management</h1>
        <div style={styles.subtitle}>Manage your team members and their roles.</div>
      </div>

      <div style={styles.card}>
        <div style={styles.cardTitle}>Your Account</div>
        <div style={styles.infoRow}>
          <span style={styles.infoLabel}>Email</span>
          <span style={styles.infoValue}>{user?.email}</span>
        </div>
        <div style={styles.infoRow}>
          <span style={styles.infoLabel}>Role</span>
          <span style={styles.roleBadge(user?.role || 'member')}>
            {user?.role || 'member'}
          </span>
        </div>
        <div style={styles.infoRow}>
          <span style={styles.infoLabel}>Joined</span>
          <span style={styles.infoValue}>
            {user?.created_at ? new Date(user.created_at).toLocaleDateString() : '--'}
          </span>
        </div>
      </div>

      <div style={styles.placeholder}>
        <div style={styles.placeholderText}>
          User management features are coming soon.
          <br />
          You will be able to invite team members, manage roles, and control access from this page.
        </div>
      </div>
    </div>
  )
}
