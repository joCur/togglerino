import { createContext, useContext } from 'react'

export type Theme = 'dark' | 'light' | 'system'

export interface ThemeContextValue {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolvedTheme: 'dark' | 'light'
  isThemeToggleEnabled: boolean
}

export const ThemeContext = createContext<ThemeContextValue | undefined>(undefined)

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
