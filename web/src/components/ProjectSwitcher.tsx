import { useState, useRef, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'

export default function ProjectSwitcher() {
  const { key } = useParams<{ key: string }>()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const containerRef = useRef<HTMLDivElement>(null)
  const searchInputRef = useRef<HTMLInputElement>(null)

  const { data: projects = [] } = useQuery({
    queryKey: ['projects'],
    queryFn: () => api.get<Project[]>('/projects'),
  })

  const currentProject = projects.find((p) => p.key === key)

  // Close on outside click
  useEffect(() => {
    if (!open) return

    function handleMouseDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false)
        setSearch('')
      }
    }

    document.addEventListener('mousedown', handleMouseDown)
    return () => document.removeEventListener('mousedown', handleMouseDown)
  }, [open])

  // Auto-focus search input when dropdown opens
  useEffect(() => {
    if (open && searchInputRef.current) {
      searchInputRef.current.focus()
    }
  }, [open])

  const filteredProjects = projects.filter(
    (p) =>
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      p.key.toLowerCase().includes(search.toLowerCase()),
  )

  function handleSelect(project: Project) {
    navigate(`/projects/${project.key}`)
    setOpen(false)
    setSearch('')
  }

  return (
    <div ref={containerRef} className="relative flex items-center">
      {/* Separator */}
      <span className="text-muted-foreground/60 text-lg font-light mr-3 select-none">/</span>

      {/* Trigger button */}
      <button
        onClick={() => setOpen(!open)}
        className={cn(
          'flex items-center gap-2 px-2.5 py-1.5 text-[13px] font-medium text-foreground border rounded-md cursor-pointer transition-all duration-200',
          open
            ? 'bg-[#1a1a1f] border-white/10'
            : 'bg-transparent border-transparent hover:bg-white/[0.04]'
        )}
      >
        <span>{currentProject?.name ?? key}</span>
        <svg
          width="10"
          height="6"
          viewBox="0 0 10 6"
          fill="none"
          className={cn('transition-transform duration-200', open && 'rotate-180')}
        >
          <path
            d="M1 1L5 5L9 1"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="text-muted-foreground"
          />
        </svg>
      </button>

      {/* Dropdown */}
      {open && (
        <div className="absolute top-full left-0 mt-1.5 w-[260px] bg-card border border-white/10 rounded-lg shadow-[0_8px_30px_rgba(0,0,0,0.4)] z-[100] overflow-hidden">
          {/* Search input */}
          <div className="p-2">
            <Input
              ref={searchInputRef}
              type="text"
              placeholder="Search projects..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="text-xs"
            />
          </div>

          {/* Project list */}
          <div className="max-h-60 overflow-y-auto px-1 pb-1">
            {filteredProjects.map((project) => {
              const isCurrent = project.key === key
              return (
                <button
                  key={project.id}
                  onClick={() => handleSelect(project)}
                  className={cn(
                    'flex flex-col gap-0.5 w-full px-2.5 py-2 text-[13px] border-none rounded-md cursor-pointer text-left transition-colors duration-150',
                    isCurrent
                      ? 'text-[#d4956a] bg-[#d4956a]/8'
                      : 'text-foreground bg-transparent hover:bg-white/[0.04]'
                  )}
                >
                  <span className="font-semibold">{project.name}</span>
                  <span className="text-[11px] font-mono text-muted-foreground/60">
                    {project.key}
                  </span>
                </button>
              )
            })}
            {filteredProjects.length === 0 && (
              <div className="py-3 px-2.5 text-xs text-muted-foreground/60 text-center">
                No projects found
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
