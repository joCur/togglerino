import { useParams } from 'react-router-dom'

export default function KillSwitchDashboardPage() {
  const { key } = useParams<{ key: string }>()
  return (
    <div>
      <h1 className="text-xl font-semibold tracking-tight mb-6">Kill Switches</h1>
      <p className="text-muted-foreground text-sm">Project: {key}</p>
    </div>
  )
}
