import type { NginxErrorPathStat } from '@/api/nginx'
import { cn } from '@/lib/utils'
import { formatNumber } from '@/lib/queryHelpers'

interface NginxErrorPathsPanelProps {
  title: string
  subtitle: string
  data: NginxErrorPathStat[]
  emptyMessage: string
  isLoading: boolean
}

export function NginxErrorPathsPanel({
  title,
  subtitle,
  data,
  emptyMessage,
  isLoading,
}: NginxErrorPathsPanelProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <h3 className="mb-1 text-sm font-semibold text-foreground">{title}</h3>
      <p className="mb-4 text-xs text-muted-foreground">{subtitle}</p>
      <div className="max-h-64 space-y-2 overflow-y-auto">
        {isLoading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
        ) : data.length > 0 ? (
          data.map((item, index) => (
            <div key={`${item.path}-${index}`} className="flex items-center gap-2 rounded px-2 py-2 text-xs hover:bg-muted/50 transition-colors">
              <span
                className={cn(
                  'flex-shrink-0 rounded px-2 py-0.5 font-medium',
                  item.err_pct > 10
                    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
                    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
                )}
              >
                {item.err_pct > 10 ? 'ERROR' : 'WARN'}
              </span>
              <span className="flex-1 truncate font-mono">{item.path}</span>
              <span className="flex-shrink-0 text-muted-foreground">
                {formatNumber(Math.round(item.errors))}/{formatNumber(Math.round(item.total))}
              </span>
              <span className="w-14 flex-shrink-0 text-right font-semibold">{item.err_pct.toFixed(1)}%</span>
            </div>
          ))
        ) : (
          <p className="py-4 text-center text-sm text-muted-foreground">{emptyMessage}</p>
        )}
      </div>
    </div>
  )
}
