import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { useFlag } from '@togglerino/react'

type Theme = 'dark' | 'light' | 'system'

interface ThemeContextValue {
  theme: Theme
  setTheme: (theme: Theme) => void
  resolvedTheme: 'dark' | 'light'
  isThemeToggleEnabled: boolean
}

const ThemeContext = createContext<ThemeContextValue | undefined>(undefined)

const STORAGE_KEY = 'togglerino-theme'

function getSystemTheme(): 'dark' | 'light' {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function isValidTheme(value: string): value is Theme {
  return value === 'dark' || value === 'light' || value === 'system'
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const isThemeToggleEnabled = useFlag('enable-theme-toggle', false)
  const defaultTheme = useFlag('default-theme', 'dark')

  const [theme, setThemeState] = useState<Theme>(() => {
    if (!isThemeToggleEnabled) return 'dark'
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored && isValidTheme(stored)) return stored
    return isValidTheme(defaultTheme) ? defaultTheme as Theme : 'dark'
  })

  const [resolvedTheme, setResolvedTheme] = useState<'dark' | 'light'>(() => {
    if (!isThemeToggleEnabled) return 'dark'
    return theme === 'system' ? getSystemTheme() : theme
  })

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
    localStorage.setItem(STORAGE_KEY, newTheme)
  }

  // When feature flag is off, force dark
  useEffect(() => {
    if (!isThemeToggleEnabled) {
      setResolvedTheme('dark')
      return
    }
    if (theme === 'system') {
      setResolvedTheme(getSystemTheme())
    } else {
      setResolvedTheme(theme)
    }
  }, [isThemeToggleEnabled, theme])

  // Listen for OS theme changes when in "system" mode
  useEffect(() => {
    if (!isThemeToggleEnabled || theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setResolvedTheme(e.matches ? 'dark' : 'light')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [isThemeToggleEnabled, theme])

  // Apply .dark class to <html>
  useEffect(() => {
    const root = document.documentElement
    if (resolvedTheme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
  }, [resolvedTheme])

  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme, isThemeToggleEnabled }}>
      {children}
    </ThemeContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme must be used within ThemeProvider')
  return ctx
}
