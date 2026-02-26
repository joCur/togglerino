import { useState } from 'react'
import type { FormEvent } from 'react'
import { useAuth } from '../hooks/useAuth.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

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
    <div className="flex items-center justify-center min-h-screen bg-background bg-[radial-gradient(ellipse_60%_50%_at_50%_40%,rgba(212,149,106,0.04)_0%,transparent_70%)]">
      <div className="w-full max-w-[400px] p-10 rounded-2xl bg-card border shadow-lg animate-[fadeInUp_400ms_ease]">
        {/* Brand */}
        <div className="flex items-center justify-center gap-2.5 mb-2">
          <svg width="24" height="14" viewBox="0 0 24 14" fill="none">
            <rect width="24" height="14" rx="7" fill="#d4956a" opacity="0.25" />
            <circle cx="17" cy="7" r="5" fill="#d4956a" />
          </svg>
          <span className="font-mono text-lg font-semibold text-[#d4956a] tracking-wide">togglerino</span>
        </div>
        <div className="text-[13px] text-muted-foreground text-center mb-9">
          Sign in to your account
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
              <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" required autoFocus />
            </div>

            <div className="space-y-1.5">
              <Label>Password</Label>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Your password" required />
            </div>
          </div>

          <Button className="w-full mt-6" disabled={submitting}>
            {submitting ? 'Signing In...' : 'Sign In'}
          </Button>
        </form>
      </div>
    </div>
  )
}
