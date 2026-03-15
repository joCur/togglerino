import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Variant, TargetingRule } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Clock } from 'lucide-react'

interface Props {
  open: boolean
  onClose: () => void
  projectKey: string
  flagKey: string
  envKey: string
  valueType?: string
  currentConfig: {
    enabled: boolean
    default_variant: string
    variants: Variant[]
    targeting_rules: TargetingRule[]
  }
}

export default function ScheduleChangeDialog({
  open,
  onClose,
  projectKey,
  flagKey,
  envKey,
  currentConfig,
  valueType,
}: Props) {
  const queryClient = useQueryClient()
  const [scheduledAt, setScheduledAt] = useState('')

  const createSchedule = useMutation({
    mutationFn: (data: { scheduled_at: string; config_snapshot: typeof currentConfig }) =>
      api.post(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/schedules`, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey, 'environments', envKey, 'schedules'] })
      handleClose()
    },
  })

  const handleClose = () => {
    setScheduledAt('')
    createSchedule.reset()
    onClose()
  }

  const handleSubmit = () => {
    if (!scheduledAt) return
    const utcTimestamp = new Date(scheduledAt).toISOString()
    createSchedule.mutate({
      scheduled_at: utcTimestamp,
      config_snapshot: currentConfig,
    })
  }

  const isInFuture = scheduledAt ? new Date(scheduledAt) > new Date() : false

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) handleClose() }}>
      <DialogContent className="sm:max-w-[480px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Clock className="w-4 h-4 text-[#d4956a]" />
            Schedule a change
          </DialogTitle>
          <DialogDescription>
            The current configuration will be applied at the scheduled time.
            This includes the enabled state, variants, and targeting rules as they are right now in the editor.
          </DialogDescription>
        </DialogHeader>

        <div className="py-4">
          <div className="text-[13px] font-medium text-foreground mb-1.5">Schedule for</div>
          <div className="text-xs text-muted-foreground mb-2.5">
            Choose when this configuration should take effect.
          </div>
          <Input
            type="datetime-local"
            value={scheduledAt}
            onChange={(e) => setScheduledAt(e.target.value)}
            min={new Date().toISOString().slice(0, 16)}
            className="w-full"
          />
          {scheduledAt && !isInFuture && (
            <p className="text-xs text-red-400 mt-1.5">Must be a future date and time.</p>
          )}
        </div>

        {/* Config preview */}
        <div className="rounded-md bg-secondary/30 border border-dashed p-3 text-[11px] font-mono space-y-1">
          <div className="text-muted-foreground">
            Enabled: <span className={currentConfig.enabled ? 'text-emerald-400' : 'text-red-400'}>
              {currentConfig.enabled ? 'true' : 'false'}
            </span>
          </div>
          {valueType !== 'boolean' && (
          <div className="text-muted-foreground">
            Default variant: <span className="text-[#d4956a]">{currentConfig.default_variant || '—'}</span>
          </div>
          )}
          <div className="text-muted-foreground">
            {valueType !== 'boolean' && <>Variants: {currentConfig.variants.length} · </>}Rules: {currentConfig.targeting_rules.length}
          </div>
        </div>

        {createSchedule.error && (
          <Alert variant="destructive" className="mt-2">
            <AlertDescription>
              {createSchedule.error instanceof Error ? createSchedule.error.message : 'Failed to create schedule'}
            </AlertDescription>
          </Alert>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            disabled={!scheduledAt || !isInFuture || createSchedule.isPending}
          >
            {createSchedule.isPending ? 'Scheduling...' : 'Schedule Change'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
