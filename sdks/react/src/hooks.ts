import { useState, useEffect, useCallback } from 'react'
import type { EvaluationContext, FlagChangeEvent } from '@togglerino/sdk'
import { useTogglerino } from './context'

export function useFlag(key: string, defaultValue: boolean): boolean
export function useFlag(key: string, defaultValue: string): string
export function useFlag(key: string, defaultValue: number): number
export function useFlag<T = unknown>(key: string, defaultValue: T): T
export function useFlag(key: string, defaultValue: unknown): unknown {
  const client = useTogglerino()

  const getCurrentValue = useCallback(() => {
    if (typeof defaultValue === 'boolean') return client.getBool(key, defaultValue)
    if (typeof defaultValue === 'number') return client.getNumber(key, defaultValue)
    if (typeof defaultValue === 'string') return client.getString(key, defaultValue)
    return client.getJson(key, defaultValue)
  }, [client, key, defaultValue])

  const [value, setValue] = useState(getCurrentValue)

  useEffect(() => {
    // Re-sync in case the value changed between render and effect
    setValue(getCurrentValue())

    const unsubscribe = client.on('change', (event: FlagChangeEvent) => {
      if (event.flagKey === key) {
        setValue(getCurrentValue())
      }
    })
    return unsubscribe
  }, [client, key, getCurrentValue])

  return value
}

export function useTogglerinoContext() {
  const client = useTogglerino()

  const [context, setContext] = useState<EvaluationContext>(() => client.getContext())

  useEffect(() => {
    const unsubscribe = client.on('context_change', (newContext: EvaluationContext) => {
      setContext(newContext)
    })
    return unsubscribe
  }, [client])

  const updateContext = useCallback(
    (ctx: Partial<EvaluationContext>) => client.updateContext(ctx),
    [client]
  )

  return { context, updateContext }
}
