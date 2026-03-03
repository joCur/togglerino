import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { api } from '../api/client'
import type { BulkAction, BulkActionResponse, BulkActionResult } from '../api/types'

interface Props {
  open: boolean
  onClose: () => void
  projectKey: string
  flagKeys: string[]
  action: BulkAction
  environmentKey?: string
  tags?: string[]
  ownerId?: string | null
  onComplete: () => void
}

const actionLabels: Record<BulkAction, string> = {
  enable: 'Enable',
  disable: 'Disable',
  archive: 'Archive',
  add_tags: 'Add tags to',
  remove_tags: 'Remove tags from',
  set_owner: 'Set owner for',
}

export default function BulkConfirmDialog({
  open,
  onClose,
  projectKey,
  flagKeys,
  action,
  environmentKey,
  tags,
  ownerId,
  onComplete,
}: Props) {
  const [results, setResults] = useState<BulkActionResult[] | null>(null)

  const mutation = useMutation({
    mutationFn: () =>
      api.flags.bulk(projectKey, {
        action,
        flag_keys: flagKeys,
        environment_key: environmentKey,
        tags,
        owner_id: ownerId,
      }),
    onSuccess: (data: BulkActionResponse) => {
      setResults(data.results)
    },
  })

  const handleClose = () => {
    if (results) {
      onComplete()
    }
    setResults(null)
    mutation.reset()
    onClose()
  }

  const summary = `${actionLabels[action]} ${flagKeys.length} flag${flagKeys.length !== 1 ? 's' : ''}${environmentKey ? ` in ${environmentKey}` : ''}?`
  const successCount = results?.filter((r) => r.success).length ?? 0
  const failCount = results?.filter((r) => !r.success).length ?? 0

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle className="text-[15px]">
            {results ? 'Bulk Action Results' : 'Confirm Bulk Action'}
          </DialogTitle>
          <DialogDescription className="text-[13px] text-muted-foreground/60">
            {results
              ? `${successCount} succeeded, ${failCount} failed`
              : summary}
          </DialogDescription>
        </DialogHeader>

        {!results ? (
          <>
            {tags && tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5 mb-2">
                {tags.map((t) => (
                  <Badge key={t} variant="secondary" className="text-[11px]">
                    {t}
                  </Badge>
                ))}
              </div>
            )}
            <div className="max-h-48 overflow-y-auto space-y-1">
              {flagKeys.map((key) => (
                <div
                  key={key}
                  className="text-[13px] font-mono text-[#d4956a] px-2 py-1 rounded bg-muted/30"
                >
                  {key}
                </div>
              ))}
            </div>
          </>
        ) : (
          <div className="max-h-48 overflow-y-auto space-y-1">
            {results.map((r) => (
              <div
                key={r.flag_key}
                className="flex items-center justify-between px-2 py-1 rounded bg-muted/30"
              >
                <span className="text-[13px] font-mono text-[#d4956a]">
                  {r.flag_key}
                </span>
                {r.success ? (
                  <Badge
                    variant="secondary"
                    className="text-[10px] bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
                  >
                    OK
                  </Badge>
                ) : (
                  <Badge
                    variant="secondary"
                    className="text-[10px] bg-red-500/10 text-red-400 border-red-500/20"
                  >
                    {r.error}
                  </Badge>
                )}
              </div>
            ))}
          </div>
        )}

        {mutation.isError && (
          <div className="text-[13px] text-red-400 px-2 py-1">
            Request failed: {mutation.error instanceof Error ? mutation.error.message : 'Unknown error'}
          </div>
        )}

        <DialogFooter>
          {!results ? (
            <>
              <Button
                variant="ghost"
                onClick={handleClose}
                disabled={mutation.isPending}
              >
                Cancel
              </Button>
              <Button
                onClick={() => mutation.mutate()}
                disabled={mutation.isPending}
              >
                {mutation.isPending ? 'Processing...' : 'Confirm'}
              </Button>
            </>
          ) : (
            <Button onClick={handleClose}>Done</Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
