import { useState } from 'react'
import type { FormEvent } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '../api/client.ts'
import { t } from '../theme.ts'

const inputStyle = {
  width: '100%',
  padding: '10px 14px',
  fontSize: 14,
  border: `1px solid ${t.border}`,
  borderRadius: t.radiusMd,
  backgroundColor: t.bgInput,
  color: t.textPrimary,
  outline: 'none',
  marginBottom: 18,
  fontFamily: t.fontSans,
  transition: 'border-color 200ms ease, box-shadow 200ms ease',
} as const

const handleFocus = (e: React.FocusEvent<HTMLInputElement>) => {
  e.currentTarget.style.borderColor = t.accentBorder
  e.currentTarget.style.boxShadow = `0 0 0 3px ${t.accentSubtle}`
}

const handleBlur = (e: React.FocusEvent<HTMLInputElement>) => {
  e.currentTarget.style.borderColor = t.border
  e.currentTarget.style.boxShadow = 'none'
}

export default function ResetPasswordPage() {
  const { token } = useParams<{ token: string }>()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [validationError, setValidationError] = useState('')
  const [apiError, setApiError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setValidationError('')
    setApiError('')

    if (password !== confirmPassword) {
      setValidationError('Passwords do not match')
      return
    }
    if (password.length < 8) {
      setValidationError('Password must be at least 8 characters')
      return
    }

    setSubmitting(true)
    try {
      await api.post('/auth/reset-password', { token, password })
      setSuccess(true)
    } catch (err) {
      setApiError(err instanceof Error ? err.message : 'Failed to reset password')
    } finally {
      setSubmitting(false)
    }
  }

  const displayError = validationError || apiError

  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        backgroundColor: t.bgBase,
        color: t.textPrimary,
        fontFamily: t.fontSans,
        background: `radial-gradient(ellipse 60% 50% at 50% 40%, rgba(212,149,106,0.04) 0%, ${t.bgBase} 70%)`,
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: 400,
          padding: 40,
          borderRadius: t.radiusXl,
          backgroundColor: t.bgSurface,
          border: `1px solid ${t.border}`,
          boxShadow: '0 8px 40px rgba(0,0,0,0.4), 0 0 80px rgba(212,149,106,0.03)',
          animation: 'fadeInUp 400ms ease',
        }}
      >
        {/* Brand */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 10,
            marginBottom: 8,
          }}
        >
          <svg width="24" height="14" viewBox="0 0 24 14" fill="none">
            <rect width="24" height="14" rx="7" fill={t.accent} opacity="0.25" />
            <circle cx="17" cy="7" r="5" fill={t.accent} />
          </svg>
          <span
            style={{
              fontFamily: t.fontMono,
              fontSize: 18,
              fontWeight: 600,
              color: t.accent,
              letterSpacing: '0.5px',
            }}
          >
            togglerino
          </span>
        </div>
        <div
          style={{
            fontSize: 13,
            color: t.textMuted,
            textAlign: 'center',
            marginBottom: 36,
          }}
        >
          Reset Your Password
        </div>

        {success ? (
          <div style={{ textAlign: 'center', animation: 'fadeIn 300ms ease' }}>
            <div
              style={{
                padding: '14px 18px',
                borderRadius: t.radiusMd,
                backgroundColor: t.successSubtle,
                border: `1px solid ${t.successBorder}`,
                color: t.success,
                fontSize: 14,
                fontWeight: 500,
                marginBottom: 24,
              }}
            >
              Your password has been reset successfully!
            </div>
            <Link
              to="/"
              style={{
                display: 'inline-block',
                padding: '11px 24px',
                fontSize: 14,
                fontWeight: 600,
                border: 'none',
                borderRadius: t.radiusMd,
                background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`,
                color: '#ffffff',
                textDecoration: 'none',
                fontFamily: t.fontSans,
                transition: 'all 200ms ease',
                boxShadow: '0 2px 12px rgba(212,149,106,0.2)',
                letterSpacing: '0.3px',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.boxShadow = '0 4px 20px rgba(212,149,106,0.35)'
                e.currentTarget.style.transform = 'translateY(-1px)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = '0 2px 12px rgba(212,149,106,0.2)'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              Go to Login
            </Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit}>
            {displayError && (
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: t.radiusMd,
                  backgroundColor: t.dangerSubtle,
                  border: `1px solid ${t.dangerBorder}`,
                  color: t.danger,
                  fontSize: 13,
                  marginBottom: 20,
                }}
              >
                {displayError}
              </div>
            )}

            <label
              style={{
                display: 'block',
                fontSize: 12,
                fontWeight: 500,
                color: t.textSecondary,
                marginBottom: 6,
                letterSpacing: '0.3px',
              }}
            >
              New Password
            </label>
            <input
              style={inputStyle}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
              required
              minLength={8}
              autoFocus
              onFocus={handleFocus}
              onBlur={handleBlur}
            />

            <label
              style={{
                display: 'block',
                fontSize: 12,
                fontWeight: 500,
                color: t.textSecondary,
                marginBottom: 6,
                letterSpacing: '0.3px',
              }}
            >
              Confirm Password
            </label>
            <input
              style={inputStyle}
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Confirm your password"
              required
              onFocus={handleFocus}
              onBlur={handleBlur}
            />

            <button
              type="submit"
              disabled={submitting}
              style={{
                width: '100%',
                padding: '11px 16px',
                fontSize: 14,
                fontWeight: 600,
                border: 'none',
                borderRadius: t.radiusMd,
                background: `linear-gradient(135deg, ${t.accent}, #c07e4e)`,
                color: '#ffffff',
                cursor: submitting ? 'not-allowed' : 'pointer',
                opacity: submitting ? 0.6 : 1,
                fontFamily: t.fontSans,
                transition: 'all 200ms ease',
                boxShadow: '0 2px 12px rgba(212,149,106,0.2)',
                letterSpacing: '0.3px',
              }}
              onMouseEnter={(e) => {
                if (!submitting) {
                  e.currentTarget.style.boxShadow = '0 4px 20px rgba(212,149,106,0.35)'
                  e.currentTarget.style.transform = 'translateY(-1px)'
                }
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = '0 2px 12px rgba(212,149,106,0.2)'
                e.currentTarget.style.transform = 'translateY(0)'
              }}
            >
              {submitting ? 'Resetting Password...' : 'Reset Password'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
