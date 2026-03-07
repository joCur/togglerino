import { cn } from '@/lib/utils'
import type { FlagEnvironmentConfig } from '../api/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

interface PromoteDialogProps {
  sourceConfig: FlagEnvironmentConfig | null
  targetConfig: FlagEnvironmentConfig | null
  sourceEnvName: string
  targetEnvName: string
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  isLoading: boolean
  error: Error | null
}

function formatValue(val: unknown): string {
  if (val === null || val === undefined) return 'null'
  if (typeof val === 'string') return val
  return JSON.stringify(val, null, 2)
}

function DiffRow({ label, source, target }: { label: string; source: string; target: string }) {
  const changed = source !== target
  return (
    <div className="space-y-1">
      <div className="text-[11px] font-mono text-muted-foreground/50 uppercase tracking-wider">{label}</div>
      <div className="grid grid-cols-2 gap-3">
        <div className={cn(
          'rounded-md border p-2 text-[12px] font-mono whitespace-pre-wrap break-all',
          changed ? 'border-muted-foreground/20' : 'border-border',
        )}>
          {source}
        </div>
        <div className={cn(
          'rounded-md border p-2 text-[12px] font-mono whitespace-pre-wrap break-all',
          changed ? 'border-[#d4956a]/40 bg-[#d4956a]/5' : 'border-border',
        )}>
          {target}
        </div>
      </div>
    </div>
  )
}

export default function PromoteDialog({
  sourceConfig,
  targetConfig,
  sourceEnvName,
  targetEnvName,
  open,
  onOpenChange,
  onConfirm,
  isLoading,
  error,
}: PromoteDialogProps) {
  const sourceDefault = sourceConfig?.default_variant ?? 'none'
  const targetDefault = targetConfig?.default_variant ?? 'none'
  const sourceVariants = formatValue(sourceConfig?.variants ?? [])
  const targetVariants = formatValue(targetConfig?.variants ?? [])
  const sourceRules = formatValue(sourceConfig?.targeting_rules ?? [])
  const targetRules = formatValue(targetConfig?.targeting_rules ?? [])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>Promote Configuration</DialogTitle>
          <DialogDescription>
            Copy flag configuration from <span className="font-medium text-foreground">{sourceEnvName}</span> to <span className="font-medium text-foreground">{targetEnvName}</span>.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 my-2">
          <div className="grid grid-cols-2 gap-3">
            <div className="text-[11px] font-mono text-muted-foreground/50 uppercase tracking-wider">
              Source: {sourceEnvName}
            </div>
            <div className="text-[11px] font-mono text-muted-foreground/50 uppercase tracking-wider">
              Target: {targetEnvName}
            </div>
          </div>

          <DiffRow label="Default Variant" source={sourceDefault} target={targetDefault} />
          <DiffRow label="Variants" source={sourceVariants} target={targetVariants} />
          <DiffRow label="Targeting Rules" source={sourceRules} target={targetRules} />

          <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-[12px] text-muted-foreground">
            Enabled state preserved (unchanged)
          </div>
        </div>

        {error && (
          <div className="rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-[12px] text-destructive">
            {error.message || 'Promotion failed'}
          </div>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={onConfirm} disabled={isLoading}>
            {isLoading ? 'Promoting...' : 'Confirm Promotion'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
