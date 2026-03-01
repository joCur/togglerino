import { cn } from '@/lib/utils'

export const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'flex items-center gap-2.5 px-5 py-2 text-[13px] border-l-2 transition-all duration-200',
    isActive
      ? 'font-medium text-foreground border-[#d4956a] bg-[#d4956a]/8'
      : 'font-normal text-muted-foreground border-transparent hover:text-foreground hover:bg-foreground/[0.03]'
  )
