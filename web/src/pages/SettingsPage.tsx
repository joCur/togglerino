import { useTheme } from '@/hooks/useTheme'
import { cn } from '@/lib/utils'

const themes = [
  {
    value: 'light' as const,
    label: 'Light',
    description: 'A clean, bright interface',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="4" />
        <path d="M12 2v2" />
        <path d="M12 20v2" />
        <path d="m4.93 4.93 1.41 1.41" />
        <path d="m17.66 17.66 1.41 1.41" />
        <path d="M2 12h2" />
        <path d="M20 12h2" />
        <path d="m6.34 17.66-1.41 1.41" />
        <path d="m19.07 4.93-1.41 1.41" />
      </svg>
    ),
  },
  {
    value: 'dark' as const,
    label: 'Dark',
    description: 'Easy on the eyes',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
      </svg>
    ),
  },
  {
    value: 'system' as const,
    label: 'System',
    description: 'Follows your OS setting',
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect width="20" height="14" x="2" y="3" rx="2" />
        <line x1="8" x2="16" y1="21" y2="21" />
        <line x1="12" x2="12" y1="17" y2="21" />
      </svg>
    ),
  },
]

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="max-w-2xl">
      <h1 className="text-lg font-semibold mb-1">Settings</h1>
      <p className="text-sm text-muted-foreground mb-8">Manage your preferences.</p>

      <div>
        <h2 className="text-sm font-medium mb-1">Appearance</h2>
        <p className="text-xs text-muted-foreground mb-4">Choose how the dashboard looks to you.</p>

        <div className="grid grid-cols-3 gap-3">
          {themes.map((t) => (
            <button
              key={t.value}
              onClick={() => setTheme(t.value)}
              className={cn(
                'flex flex-col items-center gap-2.5 rounded-lg border p-5 text-center transition-all duration-200 cursor-pointer',
                theme === t.value
                  ? 'border-[#d4956a] bg-[#d4956a]/8 ring-1 ring-[#d4956a]/30'
                  : 'border-border bg-card hover:bg-accent/50'
              )}
            >
              <div className={cn(
                'text-muted-foreground transition-colors',
                theme === t.value && 'text-[#d4956a]'
              )}>
                {t.icon}
              </div>
              <div>
                <div className={cn(
                  'text-sm font-medium',
                  theme === t.value && 'text-[#d4956a]'
                )}>
                  {t.label}
                </div>
                <div className="text-xs text-muted-foreground mt-0.5">
                  {t.description}
                </div>
              </div>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
