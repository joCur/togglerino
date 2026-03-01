import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api } from '../api/client.ts'
import type { ContextAttribute } from '../api/types.ts'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'

interface Props {
  value: string
  onChange: (value: string) => void
}

export default function AttributeCombobox({ value, onChange }: Props) {
  const { key: projectKey } = useParams<{ key: string }>()
  const [open, setOpen] = useState(false)
  const [inputValue, setInputValue] = useState('')

  const { data: attributes } = useQuery({
    queryKey: ['projects', projectKey, 'context-attributes'],
    queryFn: () => api.get<ContextAttribute[]>(`/projects/${projectKey}/context-attributes`),
    enabled: !!projectKey,
    staleTime: 30_000,
  })

  const suggestions = attributes?.map((a) => a.name) ?? []

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          role="combobox"
          aria-expanded={open}
          className="flex-1 flex items-center px-3 py-1.5 text-xs border rounded-md bg-input text-foreground text-left outline-none cursor-pointer hover:border-foreground/30 transition-colors min-w-0 h-9"
        >
          <span className={value ? 'text-foreground' : 'text-muted-foreground/60'}>
            {value || 'e.g. user_id, email, plan'}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-[240px] p-0" align="start">
        <Command>
          <CommandInput
            placeholder="Search or type attribute..."
            value={inputValue}
            onValueChange={setInputValue}
          />
          <CommandList>
            <CommandEmpty>
              {inputValue ? (
                <button
                  type="button"
                  className="w-full px-2 py-1.5 text-xs text-left hover:bg-accent rounded cursor-pointer"
                  onClick={() => {
                    onChange(inputValue)
                    setOpen(false)
                    setInputValue('')
                  }}
                >
                  Use "<span className="font-medium">{inputValue}</span>"
                </button>
              ) : (
                <span className="text-xs text-muted-foreground">No known attributes yet.</span>
              )}
            </CommandEmpty>
            {suggestions.length > 0 && (
              <CommandGroup heading="Known attributes">
                {suggestions.map((name) => (
                  <CommandItem
                    key={name}
                    value={name}
                    onSelect={(val) => {
                      onChange(val)
                      setOpen(false)
                      setInputValue('')
                    }}
                  >
                    {name}
                  </CommandItem>
                ))}
              </CommandGroup>
            )}
            {inputValue && !suggestions.includes(inputValue) && suggestions.length > 0 && (
              <CommandGroup heading="Custom">
                {/* Prefixed value avoids collision with known attribute names in cmdk filtering;
                    onSelect reads inputValue from closure instead of the cmdk value */}
                <CommandItem
                  value={`custom-${inputValue}`}
                  onSelect={() => {
                    onChange(inputValue)
                    setOpen(false)
                    setInputValue('')
                  }}
                >
                  Use "{inputValue}"
                </CommandItem>
              </CommandGroup>
            )}
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
