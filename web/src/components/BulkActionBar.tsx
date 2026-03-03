import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Environment, User, BulkAction } from '../api/types'

interface Props {
  selectedCount: number
  environments: Environment[]
  users: User[]
  onExecute: (action: BulkAction, params: {
    environmentKey?: string
    tags?: string[]
    ownerId?: string | null
  }) => void
  onClear: () => void
}

export default function BulkActionBar({ selectedCount, environments, users, onExecute, onClear }: Props) {
  const [action, setAction] = useState<BulkAction | ''>('')
  const [envKey, setEnvKey] = useState('')
  const [tagInput, setTagInput] = useState('')
  const [ownerId, setOwnerId] = useState<string>('')

  const needsEnv = action === 'enable' || action === 'disable'
  const needsTags = action === 'add_tags' || action === 'remove_tags'
  const needsOwner = action === 'set_owner'

  const canExecute =
    action !== '' &&
    (!needsEnv || envKey !== '') &&
    (!needsTags || tagInput.trim() !== '') &&
    (!needsOwner || true)

  const handleExecute = () => {
    if (!action) return
    onExecute(action, {
      environmentKey: needsEnv ? envKey : undefined,
      tags: needsTags ? tagInput.split(',').map((t) => t.trim()).filter(Boolean) : undefined,
      ownerId: needsOwner ? (ownerId || null) : undefined,
    })
  }

  return (
    <div className="fixed bottom-0 left-0 right-0 z-50 border-t bg-card/95 backdrop-blur-sm px-4 py-3 animate-[slideUp_200ms_ease]">
      <div className="max-w-5xl mx-auto flex items-center gap-3 flex-wrap">
        <span className="text-[13px] text-foreground font-medium whitespace-nowrap">
          {selectedCount} flag{selectedCount !== 1 ? 's' : ''} selected
        </span>

        <select
          className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
          value={action}
          onChange={(e) => setAction(e.target.value as BulkAction | '')}
        >
          <option value="">Select action...</option>
          <option value="enable">Enable</option>
          <option value="disable">Disable</option>
          <option value="archive">Archive</option>
          <option value="add_tags">Add Tags</option>
          <option value="remove_tags">Remove Tags</option>
          <option value="set_owner">Set Owner</option>
        </select>

        {needsEnv && (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
            value={envKey}
            onChange={(e) => setEnvKey(e.target.value)}
          >
            <option value="">Select environment...</option>
            {environments.map((env) => (
              <option key={env.id} value={env.key}>{env.name}</option>
            ))}
          </select>
        )}

        {needsTags && (
          <Input
            className="w-48"
            placeholder="tag1, tag2..."
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
          />
        )}

        {needsOwner && (
          <select
            className="px-3 py-2 text-[13px] border rounded-md bg-input text-foreground outline-none cursor-pointer min-w-[140px]"
            value={ownerId}
            onChange={(e) => setOwnerId(e.target.value)}
          >
            <option value="">Unassign owner</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>{u.display_name ?? u.email}</option>
            ))}
          </select>
        )}

        <div className="flex items-center gap-2 ml-auto">
          <Button variant="ghost" size="sm" onClick={onClear} className="text-[13px]">
            Clear
          </Button>
          <Button
            size="sm"
            disabled={!canExecute}
            onClick={handleExecute}
            className="text-[13px]"
          >
            Execute
          </Button>
        </div>
      </div>
    </div>
  )
}
