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

  // User's explicit preference (from localStorage or setTheme calls)
  const [userTheme, setUserTheme] = useState<Theme | null>(() => {
    const stored = localStorage.getItem(STORAGE_KEY)
    return stored && isValidTheme(stored) ? stored : null
  })

  const [systemTheme, setSystemTheme] = useState<'dark' | 'light'>(getSystemTheme)

  // Derive theme from flags + user preference (no useEffect needed)
  const theme: Theme = !isThemeToggleEnabled
    ? 'dark'
    : userTheme ?? (isValidTheme(defaultTheme) ? defaultTheme as Theme : 'dark')

  const setTheme = (newTheme: Theme) => {
    setUserTheme(newTheme)
    localStorage.setItem(STORAGE_KEY, newTheme)
  }

  // Listen for OS theme changes
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setSystemTheme(e.matches ? 'dark' : 'light')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  // Derive resolvedTheme
  const resolvedTheme: 'dark' | 'light' = !isThemeToggleEnabled
    ? 'dark'
    : theme === 'system'
      ? systemTheme
      : theme

  // Apply .dark class to <html> and update meta theme-color
  useEffect(() => {
    const root = document.documentElement
    if (resolvedTheme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
    const meta = document.querySelector('meta[name="theme-color"]')
    if (meta) {
      meta.setAttribute('content', resolvedTheme === 'dark' ? '#09090b' : '#fafafa')
    }
  }, [resolvedTheme])

  return (
    <ThemeContext.Provider value={{ theme, setTheme, resolvedTheme, isThemeToggleEnabled }}>
      {children}
    </ThemeContext.Provider>
  )
}
