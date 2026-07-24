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
    <div className="rounded-xl border border-border/60 bg-card p-5 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="text-[11px] font-semibold text-muted-foreground uppercase tracking-widest">
            {title}
          </p>
          <div className="mt-2 flex items-baseline gap-1.5">
            <p className={cn('text-xl font-bold tracking-tight text-foreground', loading && 'text-muted-foreground')}>
              {loading ? '—' : value}
            </p>
            {unit && !loading ? <span className="text-xs text-muted-foreground/70">{unit}</span> : null}
          </div>
        </div>
        {icon ? <div className="text-xl opacity-25 mt-1 shrink-0">{icon}</div> : null}
      </div>
    </div>
  )
}
