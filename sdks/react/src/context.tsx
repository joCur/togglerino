import { createContext, useContext } from 'react'
import type { Togglerino } from '@togglerino/sdk'

export const TogglerioContext = createContext<Togglerino | null>(null)

export function useTogglerino(): Togglerino {
  const client = useContext(TogglerioContext)
  if (!client) {
    throw new Error('useTogglerino must be used within a <TogglerioProvider>')
  }
  return client
}
