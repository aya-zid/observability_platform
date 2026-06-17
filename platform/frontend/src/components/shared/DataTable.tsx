import { cn } from '@/lib/utils'

export function DataTable({
  title,
  subtitle,
  rows,
  loading,
}: {
  title: string
  subtitle?: string
  rows: Array<{ label: string; value: string; percent?: number; badgeClass?: string }>
  loading?: boolean
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold text-foreground">{title}</h3>
      {subtitle ? <p className="mb-3 text-xs text-muted-foreground">{subtitle}</p> : <div className="mb-3" />}
      <div className="max-h-64 space-y-1 overflow-y-auto">
        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
        ) : rows.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
        ) : (
          rows.map((row, index) => (
            <div key={`${row.label}-${index}`} className="flex items-center justify-between rounded px-2 py-1.5 hover:bg-muted/50 transition-colors">
              <div className="mr-2 flex min-w-0 flex-1 items-center gap-2">
                {row.badgeClass ? (
                  <span className={cn('rounded px-2 py-0.5 text-xs font-semibold', row.badgeClass)}>{row.label}</span>
                ) : (
                  <span className="truncate font-mono text-sm">{row.label}</span>
                )}
              </div>
              <div className="flex flex-shrink-0 items-center gap-2">
                {row.percent !== undefined ? (
                  <span className="text-xs text-muted-foreground">{row.percent.toFixed(1)}%</span>
                ) : null}
                <span className="font-semibold text-primary">{row.value}</span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
