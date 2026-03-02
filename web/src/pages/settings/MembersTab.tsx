import { Card, CardContent } from '@/components/ui/card'

export default function MembersTab() {
  return (
    <Card>
      <CardContent className="p-6">
        <div className="text-sm font-semibold text-foreground mb-3">
          Members
        </div>
        <div className="text-[13px] text-muted-foreground/60">
          Project-level member management coming soon.
        </div>
      </CardContent>
    </Card>
  )
}
