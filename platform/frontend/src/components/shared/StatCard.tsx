import { cn } from '@/lib/utils'

export function StatCard({
  title,
  value,
  unit,
  loading,
  icon,
}: {
  title: string
  value: string | number
  unit?: string
  status?: string
  loading?: boolean
  icon?: string
}) {
  return (
    <div className="rounded-lg border border-border/60 bg-card p-3 shadow-sm">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <p className="text-xs font-semibold text-muted-foreground uppercase tracking-widest">
            {title}
          </p>
          <div className="mt-1 flex items-baseline gap-1">
            <p className={cn('text-sm font-bold tracking-tight text-foreground', loading && 'text-muted-foreground')}>
              {loading ? '—' : value}
            </p>
            {unit && !loading ? <span className="text-xs text-muted-foreground/70">{unit}</span> : null}
          </div>
        </div>
        {icon ? <div className="text-base opacity-25 mt-0.5 shrink-0">{icon}</div> : null}
      </div>
    </div>
  )
}
