import { useEffect, useState, type ReactNode } from 'react'
import { useFlag } from '@togglerino/react'
import { ThemeContext, type Theme } from '@/hooks/useTheme'

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

  const [systemTheme, setSystemTheme] = useState<'dark' | 'light'>(getSystemTheme)

  const setTheme = (newTheme: Theme) => {
    setThemeState(newTheme)
    localStorage.setItem(STORAGE_KEY, newTheme)
  }

  // Listen for OS theme changes
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setSystemTheme(e.matches ? 'dark' : 'light')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  // Derive resolvedTheme from state (no useEffect + setState needed)
  const resolvedTheme: 'dark' | 'light' = !isThemeToggleEnabled
    ? 'dark'
    : theme === 'system'
      ? systemTheme
      : theme

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
