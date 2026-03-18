import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { ScheduledFlagChange } from '../api/types.ts'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Clock, X } from 'lucide-react'

interface Props {
  projectKey: string
  flagKey: string
  envKey: string
  flagId: string
  environmentId: string
}

function formatScheduleTime(dateStr: string): string {
  const date = new Date(dateStr)
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(date)
}

function formatFullTime(dateStr: string): string {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'full',
    timeStyle: 'short',
  }).format(new Date(dateStr))
}

export default function PendingSchedules({
  projectKey,
  flagKey,
  envKey,
  flagId,
  environmentId,
}: Props) {
  const queryClient = useQueryClient()
  const [cancelTarget, setCancelTarget] = useState<string | null>(null)

  const { data: schedules } = useQuery({
    queryKey: ['projects', projectKey, 'flags', flagKey, 'environments', envKey, 'schedules'],
    queryFn: () => api.get<ScheduledFlagChange[]>(
      `/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/schedules`
    ),
    enabled: !!flagId && !!environmentId,
  })

  const cancelMutation = useMutation({
    mutationFn: (scheduleId: string) =>
      api.delete(`/projects/${projectKey}/flags/${flagKey}/environments/${envKey}/schedules/${scheduleId}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['projects', projectKey, 'flags', flagKey, 'environments', envKey, 'schedules'] })
      setCancelTarget(null)
    },
  })

  const pending = schedules?.filter((s) => s.status === 'pending') ?? []

  if (pending.length === 0) return null

  return (
    <>
      <div className="mb-4">
        <div className="flex items-center gap-1.5 mb-2">
          <Clock className="w-3.5 h-3.5 text-[#d4956a]" />
          <span className="text-[11px] font-mono font-medium text-[#d4956a] uppercase tracking-wider">
            Scheduled Changes
          </span>
        </div>
        <div className="space-y-1.5">
          {pending.map((schedule) => (
            <div
              key={schedule.id}
              className="flex items-center justify-between gap-3 px-3 py-2 rounded-md bg-[#d4956a]/5 border border-[#d4956a]/20 text-[12px]"
            >
              <div className="flex items-center gap-2 min-w-0">
                <span
                  className="font-mono text-[#d4956a]"
                  title={formatFullTime(schedule.scheduled_at)}
                >
                  {formatScheduleTime(schedule.scheduled_at)}
                </span>
                <span className="text-muted-foreground/50">·</span>
                <span className="text-muted-foreground truncate">
                  {schedule.config_snapshot.enabled ? 'Enable' : 'Disable'}
                  {schedule.config_snapshot.fallthrough_variant && (
                    <>, variant: <span className="text-foreground">{schedule.config_snapshot.fallthrough_variant}</span></>
                  )}
                </span>
              </div>
              <Button
                variant="ghost"
                size="sm"
                className="h-6 w-6 p-0 text-muted-foreground hover:text-red-400 shrink-0"
                onClick={() => setCancelTarget(schedule.id)}
                title="Cancel schedule"
              >
                <X className="w-3.5 h-3.5" />
              </Button>
            </div>
          ))}
        </div>
      </div>

      {/* Cancel confirmation */}
      <Dialog open={cancelTarget !== null} onOpenChange={(o) => { if (!o) setCancelTarget(null) }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Cancel scheduled change?</DialogTitle>
            <DialogDescription>
              This scheduled configuration change will not be applied. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setCancelTarget(null)}>
              Keep schedule
            </Button>
            <Button
              variant="destructive"
              onClick={() => { if (cancelTarget) cancelMutation.mutate(cancelTarget) }}
              disabled={cancelMutation.isPending}
            >
              {cancelMutation.isPending ? 'Cancelling...' : 'Cancel Schedule'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
