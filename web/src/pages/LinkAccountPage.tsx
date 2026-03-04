import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

export default function LinkAccountPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      await api.post('/auth/oidc/link', { password })
      queryClient.invalidateQueries({ queryKey: ['auth'] })
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to link account')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex items-center justify-center min-h-screen px-6 md:px-0 bg-background bg-[radial-gradient(ellipse_60%_50%_at_50%_40%,rgba(212,149,106,0.04)_0%,transparent_70%)]">
      <div className="w-full max-w-[400px] p-6 md:p-10 rounded-2xl md:bg-card md:border md:shadow-lg animate-[fadeInUp_400ms_ease]">
        <div className="flex items-center justify-center gap-2.5 mb-2">
          <svg width="24" height="14" viewBox="0 0 24 14" fill="none">
            <rect width="24" height="14" rx="7" fill="#d4956a" opacity="0.25" />
            <circle cx="17" cy="7" r="5" fill="#d4956a" />
          </svg>
          <span className="font-mono text-lg font-semibold text-[#d4956a] tracking-wide">togglerino</span>
        </div>
        <div className="text-[13px] text-muted-foreground text-center mb-2">
          Link your SSO identity
        </div>
        <div className="text-[12px] text-muted-foreground/60 text-center mb-9">
          An account with your email already exists. Enter your password to link your SSO identity.
        </div>

        <form onSubmit={handleSubmit}>
          {error && (
            <Alert variant="destructive" className="mb-5">
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="space-y-1.5">
            <Label>Password</Label>
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Your existing password"
              required
              autoFocus
            />
          </div>

          <Button className="w-full mt-6" disabled={submitting}>
            {submitting ? 'Linking...' : 'Link Account & Sign In'}
          </Button>
        </form>
      </div>
    </div>
  )
}
