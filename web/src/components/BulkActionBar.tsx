import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { X, Zap } from 'lucide-react'
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
    <div className="fixed bottom-0 left-0 right-0 z-50 border-t border-[#d4956a]/20 bg-card/95 backdrop-blur-md animate-[slideUp_200ms_ease]">
      <div className="max-w-5xl mx-auto px-4 py-3 flex items-center gap-3 flex-wrap">
        {/* Count badge */}
        <Badge className="bg-[#d4956a]/15 text-[#d4956a] border-[#d4956a]/25 hover:bg-[#d4956a]/15 font-mono text-[12px] px-2.5 py-0.5 gap-1.5">
          {selectedCount}
          <span className="font-sans font-normal">selected</span>
        </Badge>

        {/* Divider */}
        <div className="w-px h-5 bg-border/60" />

        {/* Action select */}
        <select
          className="px-3 py-1.5 text-[13px] border border-border/60 rounded-md bg-background text-foreground outline-none cursor-pointer min-w-[140px] focus:border-[#d4956a]/50 focus:ring-1 focus:ring-[#d4956a]/20 transition-colors"
          value={action}
          onChange={(e) => setAction(e.target.value as BulkAction | '')}
        >
          <option value="">Choose action...</option>
          <option value="enable">Enable</option>
          <option value="disable">Disable</option>
          <option value="archive">Archive</option>
          <option value="add_tags">Add Tags</option>
          <option value="remove_tags">Remove Tags</option>
          <option value="set_owner">Set Owner</option>
        </select>

        {/* Conditional parameter inputs */}
        {needsEnv && (
          <select
            className="px-3 py-1.5 text-[13px] border border-border/60 rounded-md bg-background text-foreground outline-none cursor-pointer min-w-[140px] focus:border-[#d4956a]/50 focus:ring-1 focus:ring-[#d4956a]/20 transition-colors"
            value={envKey}
            onChange={(e) => setEnvKey(e.target.value)}
          >
            <option value="">Environment...</option>
            {environments.map((env) => (
              <option key={env.id} value={env.key}>{env.name}</option>
            ))}
          </select>
        )}

        {needsTags && (
          <Input
            className="w-48 h-8 text-[13px]"
            placeholder="tag1, tag2..."
            value={tagInput}
            onChange={(e) => setTagInput(e.target.value)}
          />
        )}

        {needsOwner && (
          <select
            className="px-3 py-1.5 text-[13px] border border-border/60 rounded-md bg-background text-foreground outline-none cursor-pointer min-w-[140px] focus:border-[#d4956a]/50 focus:ring-1 focus:ring-[#d4956a]/20 transition-colors"
            value={ownerId}
            onChange={(e) => setOwnerId(e.target.value)}
          >
            <option value="">Unassign owner</option>
            {users.map((u) => (
              <option key={u.id} value={u.id}>{u.display_name ?? u.email}</option>
            ))}
          </select>
        )}

        {/* Actions */}
        <div className="flex items-center gap-2 ml-auto">
          <Button
            variant="ghost"
            size="sm"
            onClick={onClear}
            className="text-[13px] text-muted-foreground gap-1"
          >
            <X className="size-3.5" />
            Clear
          </Button>
          <Button
            size="sm"
            disabled={!canExecute}
            onClick={handleExecute}
            className="text-[13px] bg-[#d4956a] hover:bg-[#c4854f] text-white gap-1.5"
          >
            <Zap className="size-3.5" />
            Execute
          </Button>
        </div>
      </div>
    </div>
  )
}
