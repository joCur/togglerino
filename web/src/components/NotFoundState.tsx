import { Link } from 'react-router-dom'

interface NotFoundStateProps {
  title: string
  description: string
  backTo: string
  backLabel: string
}

export default function NotFoundState({ title, description, backTo, backLabel }: NotFoundStateProps) {
  return (
    <div className="text-center py-16 animate-[fadeIn_300ms_ease]">
      <div className="text-[15px] font-medium text-foreground mb-1.5">{title}</div>
      <div className="text-[13px] text-muted-foreground/60 mb-6">{description}</div>
      <Link
        to={backTo}
        className="text-[13px] text-[#d4956a] hover:text-[#e0a87a] transition-colors"
      >
        &larr; Back to {backLabel}
      </Link>
    </div>
  )
}
