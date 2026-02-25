import { useState, useEffect, type ReactNode } from 'react'
import { Togglerino, type TogglerinoConfig } from '@togglerino/sdk'
import { TogglerioContext } from './context'

interface TogglerioProviderProps {
  config: TogglerinoConfig
  children: ReactNode
}

export function TogglerioProvider({ config, children }: TogglerioProviderProps) {
  const [client] = useState(() => new Togglerino(config))
  const [ready, setReady] = useState(false)

  useEffect(() => {
    client.initialize().then(() => setReady(true))
    return () => client.close()
  }, [client])

  if (!ready) {
    return null
  }

  return (
    <TogglerioContext.Provider value={client}>
      {children}
    </TogglerioContext.Provider>
  )
}
