import { useState } from 'react'
import type { FormEvent } from 'react'
import { useAuth } from '../hooks/useAuth.ts'

const styles = {
  container: {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    minHeight: '100vh',
    backgroundColor: '#1a1a2e',
    color: '#e0e0e0',
  } as const,
  card: {
    width: '100%',
    maxWidth: 400,
    padding: 40,
    borderRadius: 12,
    backgroundColor: '#16213e',
    boxShadow: '0 4px 24px rgba(0, 0, 0, 0.3)',
  } as const,
  heading: {
    fontSize: 28,
    fontWeight: 700,
    marginBottom: 32,
    color: '#ffffff',
    textAlign: 'center',
  } as const,
  label: {
    display: 'block',
    fontSize: 13,
    fontWeight: 500,
    color: '#8892b0',
    marginBottom: 6,
  } as const,
  input: {
    width: '100%',
    padding: '10px 12px',
    fontSize: 14,
    border: '1px solid #2a2a4a',
    borderRadius: 6,
    backgroundColor: '#0f3460',
    color: '#e0e0e0',
    outline: 'none',
    marginBottom: 16,
  } as const,
  button: {
    width: '100%',
    padding: '12px 16px',
    fontSize: 15,
    fontWeight: 600,
    border: 'none',
    borderRadius: 6,
    backgroundColor: '#e94560',
    color: '#ffffff',
    cursor: 'pointer',
    marginTop: 8,
  } as const,
  buttonDisabled: {
    opacity: 0.6,
    cursor: 'not-allowed',
  } as const,
  error: {
    padding: '10px 12px',
    borderRadius: 6,
    backgroundColor: 'rgba(233, 69, 96, 0.15)',
    border: '1px solid rgba(233, 69, 96, 0.3)',
    color: '#e94560',
    fontSize: 13,
    marginBottom: 16,
  } as const,
}

export default function LoginPage() {
  const { login, loginError } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    try {
      await login({ email, password })
    } catch {
      // error is captured by loginError
    } finally {
      setSubmitting(false)
    }
  }

  const displayError = loginError instanceof Error ? loginError.message : ''

  return (
    <div style={styles.container}>
      <div style={styles.card}>
        <h1 style={styles.heading}>Sign in to togglerino</h1>
        <form onSubmit={handleSubmit}>
          {displayError && <div style={styles.error}>{displayError}</div>}
          <label style={styles.label}>Email</label>
          <input
            style={styles.input}
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            required
            autoFocus
          />
          <label style={styles.label}>Password</label>
          <input
            style={styles.input}
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Your password"
            required
          />
          <button
            type="submit"
            style={{
              ...styles.button,
              ...(submitting ? styles.buttonDisabled : {}),
            }}
            disabled={submitting}
          >
            {submitting ? 'Signing In...' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  )
}
