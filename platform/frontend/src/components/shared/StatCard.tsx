import { cn } from '@/lib/utils'

export function StatCard({
  title,
  value,
  unit,
  status = 'green',
  loading,
  icon,
}: {
  title: string
  value: string | number
  unit?: string
  status?: 'green' | 'yellow' | 'red'
  loading?: boolean
  icon?: string
}) {
  const color = {
    green: 'text-green-600 dark:text-green-400',
    yellow: 'text-amber-600 dark:text-amber-400',
    red: 'text-red-600 dark:text-red-400',
  }[status]

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-sm text-muted-foreground">{title}</p>
          <div className="mt-2 flex items-baseline gap-2">
            <p className={cn('text-3xl font-bold', loading ? 'text-muted-foreground' : color)}>
              {loading ? '—' : value}
            </p>
            {unit && !loading ? <p className="text-sm text-muted-foreground">{unit}</p> : null}
          </div>
        </div>
        {icon ? <div className="text-4xl opacity-40">{icon}</div> : null}
      </div>
    </div>
  )
}
