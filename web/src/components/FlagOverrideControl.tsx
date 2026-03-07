import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { FlagOverrideEntry, ValueType } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from '@/components/ui/dialog'
import { Badge } from '@/components/ui/badge'

interface FlagOverrideControlProps {
  projectKey: string
  flagKey: string
  envKey: string
  valueType: ValueType
  override?: FlagOverrideEntry
}

export function FlagOverrideControl({ projectKey, flagKey, envKey, valueType, override }: FlagOverrideControlProps) {
  const queryClient = useQueryClient()
  const [showSetDialog, setShowSetDialog] = useState(false)
  const [showIdentityDialog, setShowIdentityDialog] = useState(false)
  const [overrideValue, setOverrideValue] = useState('')
  const [duration, setDuration] = useState('24h')
  const [appUserID, setAppUserID] = useState('')

  const identityQuery = useQuery({
    queryKey: ['app-identity', projectKey],
    queryFn: () => api.appIdentity.get(projectKey),
    retry: false,
  })

  const setIdentityMutation = useMutation({
    mutationFn: (id: string) => api.appIdentity.set(projectKey, id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['app-identity', projectKey] })
      setShowIdentityDialog(false)
      setOverrideValue('')
      setDuration('24h')
      setShowSetDialog(true)
    },
  })

  const setOverrideMutation = useMutation({
    mutationFn: () => {
      let parsedValue: unknown = overrideValue
      if (valueType === 'boolean') parsedValue = overrideValue === 'true'
      else if (valueType === 'number') parsedValue = Number(overrideValue)
      else if (valueType === 'json') {
        try {
          parsedValue = JSON.parse(overrideValue)
        } catch {
          throw new Error('Invalid JSON value')
        }
      }
      const effectiveDuration = duration === 'none' ? null : duration
      return api.overrides.set(projectKey, flagKey, envKey, parsedValue, effectiveDuration)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flag-overrides', projectKey, flagKey] })
      setShowSetDialog(false)
    },
  })

  const deleteOverrideMutation = useMutation({
    mutationFn: () => api.overrides.delete(projectKey, flagKey, envKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['flag-overrides', projectKey, flagKey] })
    },
  })

  const handleOverrideClick = () => {
    if (override) {
      deleteOverrideMutation.mutate()
    } else if (identityQuery.isLoading) {
      return
    } else if (identityQuery.error) {
      setShowIdentityDialog(true)
    } else {
      setOverrideValue('')
      setDuration('24h')
      setValueError(null)
      setShowSetDialog(true)
    }
  }

  return (
    <>
      <div className="flex items-center gap-2">
        {override ? (
          <>
            <Badge variant="outline" className="text-amber-500 border-amber-500/30">
              Override: {JSON.stringify(override.value)}
            </Badge>
            {override.expires_at && (
              <span className="text-xs text-muted-foreground">
                expires {new Date(override.expires_at).toLocaleString()}
              </span>
            )}
            <Button variant="ghost" size="sm" onClick={handleOverrideClick}>
              Remove
            </Button>
          </>
        ) : (
          <Button variant="outline" size="sm" onClick={handleOverrideClick} disabled={identityQuery.isLoading}>
            Override for me
          </Button>
        )}
      </div>

      {/* Identity setup dialog */}
      <Dialog open={showIdentityDialog} onOpenChange={setShowIdentityDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Set your app identity</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            Enter the user ID your application uses to identify you in SDK evaluation context.
          </p>
          <Input
            placeholder="Your app user ID"
            value={appUserID}
            onChange={(e) => setAppUserID(e.target.value)}
          />
          <DialogFooter>
            <Button onClick={() => setIdentityMutation.mutate(appUserID)} disabled={!appUserID}>
              Save & Continue
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Set override dialog */}
      <Dialog open={showSetDialog} onOpenChange={setShowSetDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Set personal override</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <label className="text-sm font-medium">Value</label>
              {valueType === 'boolean' ? (
                <Select value={overrideValue || 'true'} onValueChange={setOverrideValue}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="true">true</SelectItem>
                    <SelectItem value="false">false</SelectItem>
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  value={overrideValue}
                  onChange={(e) => setOverrideValue(e.target.value)}
                  placeholder={valueType === 'number' ? '0' : valueType === 'json' ? '{}' : 'value'}
                />
              )}
            </div>
            <div>
              <label className="text-sm font-medium">Duration</label>
              <Select value={duration} onValueChange={setDuration}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1h">1 hour</SelectItem>
                  <SelectItem value="8h">8 hours</SelectItem>
                  <SelectItem value="24h">24 hours</SelectItem>
                  <SelectItem value="7d">7 days</SelectItem>
                  <SelectItem value="none">No expiry</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {setOverrideMutation.error && (
            <p className="text-sm text-destructive">
              {setOverrideMutation.error instanceof Error ? setOverrideMutation.error.message : 'Failed to set override'}
            </p>
          )}
          <DialogFooter>
            <Button onClick={() => setOverrideMutation.mutate()}>
              Set Override
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
