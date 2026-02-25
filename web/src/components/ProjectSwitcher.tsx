import { useState, useRef, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { Project } from '../api/types.ts'
import { t } from '../theme.ts'

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
    <div ref={containerRef} style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
      {/* Separator */}
      <span
        style={{
          color: t.textMuted,
          fontSize: 18,
          fontWeight: 300,
          marginRight: 12,
          userSelect: 'none',
        }}
      >
        /
      </span>

      {/* Trigger button */}
      <button
        onClick={() => setOpen(!open)}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: 8,
          padding: '5px 10px',
          fontSize: 13,
          fontWeight: 500,
          fontFamily: t.fontSans,
          color: t.textPrimary,
          backgroundColor: open ? t.bgElevated : 'transparent',
          border: `1px solid ${open ? t.borderStrong : 'transparent'}`,
          borderRadius: t.radiusSm,
          cursor: 'pointer',
          transition: 'all 200ms ease',
        }}
        onMouseEnter={(e) => {
          if (!open) {
            e.currentTarget.style.backgroundColor = t.bgHover
          }
        }}
        onMouseLeave={(e) => {
          if (!open) {
            e.currentTarget.style.backgroundColor = 'transparent'
          }
        }}
      >
        <span>{currentProject?.name ?? key}</span>
        <svg
          width="10"
          height="6"
          viewBox="0 0 10 6"
          fill="none"
          style={{
            transform: open ? 'rotate(180deg)' : 'rotate(0deg)',
            transition: 'transform 200ms ease',
          }}
        >
          <path
            d="M1 1L5 5L9 1"
            stroke={t.textSecondary}
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </button>

      {/* Dropdown */}
      {open && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            marginTop: 6,
            width: 260,
            backgroundColor: t.bgSurface,
            border: `1px solid ${t.borderStrong}`,
            borderRadius: t.radiusLg,
            boxShadow: '0 8px 30px rgba(0,0,0,0.4)',
            zIndex: 100,
            overflow: 'hidden',
          }}
        >
          {/* Search input */}
          <div style={{ padding: 8 }}>
            <input
              ref={searchInputRef}
              type="text"
              placeholder="Search projects..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              style={{
                width: '100%',
                padding: '7px 10px',
                fontSize: 12,
                fontFamily: t.fontSans,
                color: t.textPrimary,
                backgroundColor: t.bgInput,
                border: `1px solid ${t.border}`,
                borderRadius: t.radiusSm,
                outline: 'none',
                boxSizing: 'border-box',
                transition: 'all 200ms ease',
              }}
              onFocus={(e) => {
                e.currentTarget.style.borderColor = t.accentBorder
                e.currentTarget.style.boxShadow = `0 0 0 2px ${t.accentSubtle}`
              }}
              onBlur={(e) => {
                e.currentTarget.style.borderColor = t.border
                e.currentTarget.style.boxShadow = 'none'
              }}
            />
          </div>

          {/* Project list */}
          <div
            style={{
              maxHeight: 240,
              overflowY: 'auto',
              padding: '0 4px 4px',
            }}
          >
            {filteredProjects.map((project) => {
              const isCurrent = project.key === key
              return (
                <button
                  key={project.id}
                  onClick={() => handleSelect(project)}
                  style={{
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 2,
                    width: '100%',
                    padding: '8px 10px',
                    fontSize: 13,
                    fontFamily: t.fontSans,
                    color: isCurrent ? t.accent : t.textPrimary,
                    backgroundColor: isCurrent ? t.accentSubtle : 'transparent',
                    border: 'none',
                    borderRadius: t.radiusSm,
                    cursor: 'pointer',
                    textAlign: 'left',
                    transition: 'background-color 150ms ease',
                  }}
                  onMouseEnter={(e) => {
                    if (!isCurrent) {
                      e.currentTarget.style.backgroundColor = t.bgHover
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (!isCurrent) {
                      e.currentTarget.style.backgroundColor = 'transparent'
                    }
                  }}
                >
                  <span style={{ fontWeight: 600 }}>{project.name}</span>
                  <span
                    style={{
                      fontSize: 11,
                      fontFamily: t.fontMono,
                      color: t.textMuted,
                    }}
                  >
                    {project.key}
                  </span>
                </button>
              )
            })}
            {filteredProjects.length === 0 && (
              <div
                style={{
                  padding: '12px 10px',
                  fontSize: 12,
                  color: t.textMuted,
                  textAlign: 'center',
                }}
              >
                No projects found
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
