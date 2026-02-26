import { useState } from 'react'
import type { FormEvent } from 'react'
import { useAuth } from '../hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function SetupPage() {
  const { setup, setupError } = useAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [validationError, setValidationError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setValidationError('')

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
      await setup({ email, password })
    } catch {
      // error is captured by setupError
    } finally {
      setSubmitting(false)
    }
  }

  const displayError = validationError || (setupError instanceof Error ? setupError.message : '')

  return (
    <div className="flex items-center justify-center min-h-screen bg-background bg-[radial-gradient(ellipse_60%_50%_at_50%_40%,rgba(212,149,106,0.04)_0%,transparent_70%)]">
      <div className="w-full max-w-[400px] p-10 rounded-2xl bg-card border shadow-[0_8px_40px_rgba(0,0,0,0.4)] animate-[fadeInUp_400ms_ease]">
        {/* Brand */}
        <div className="flex items-center justify-center gap-2.5 mb-2">
          <svg width="24" height="14" viewBox="0 0 24 14" fill="none">
            <rect width="24" height="14" rx="7" fill="#d4956a" opacity="0.25" />
            <circle cx="17" cy="7" r="5" fill="#d4956a" />
          </svg>
          <span className="font-mono text-lg font-semibold text-[#d4956a] tracking-wide">togglerino</span>
        </div>
        <div className="text-[13px] text-muted-foreground text-center mb-9">
          Create your admin account to get started
        </div>

        <form onSubmit={handleSubmit}>
          {displayError && (
            <Alert variant="destructive" className="mb-5">
              <AlertDescription>{displayError}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-4">
            <div className="space-y-1.5">
              <Label>Email</Label>
              <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin@example.com" required autoFocus />
            </div>

            <div className="space-y-1.5">
              <Label>Password</Label>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="At least 8 characters" required minLength={8} />
            </div>

            <div className="space-y-1.5">
              <Label>Confirm Password</Label>
              <Input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} placeholder="Confirm your password" required />
            </div>
          </div>

          <Button className="w-full mt-6" disabled={submitting}>
            {submitting ? 'Creating Account...' : 'Create Account'}
          </Button>
        </form>
      </div>
    </div>
  )
}
