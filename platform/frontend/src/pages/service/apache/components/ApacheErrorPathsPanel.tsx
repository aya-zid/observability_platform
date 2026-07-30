import type { ApacheErrorPathStat } from '@/api/apache'
import { formatNumber } from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

interface Props {
  aggTopErrorPaths: ApacheErrorPathStat[]
  aggTopNormalizedErrorPaths: ApacheErrorPathStat[]
  aggTotalsLabel: string
  isLoading: boolean
}

export function ApacheErrorPathsPanel({
  aggTopErrorPaths,
  aggTopNormalizedErrorPaths,
  aggTotalsLabel,
  isLoading,
}: Props) {
  return (
    <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
        <h3 className="mb-1 text-sm font-semibold text-foreground">Top Paths by Error Rate</h3>
        <p className="mb-4 text-sm text-muted-foreground">{aggTotalsLabel}</p>
        <div className="max-h-64 space-y-2 overflow-y-auto">
          {isLoading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
          ) : aggTopErrorPaths.length > 0 ? (
            aggTopErrorPaths.map((item, index) => (
              <div key={`${item.path}-${index}`} className="flex items-center gap-2 rounded px-2 py-2 text-sm hover:bg-muted/50 transition-colors">
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
                  {formatNumber(Math.round(item.errors), 0)}/{formatNumber(Math.round(item.total), 0)}
                </span>
                <span className="w-14 flex-shrink-0 text-right font-semibold">{item.err_pct.toFixed(1)}%</span>
              </div>
            ))
          ) : (
            <p className="py-4 text-center text-sm text-muted-foreground">No error-heavy paths found in the selected range.</p>
          )}
        </div>
      </div>

      <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
        <h3 className="mb-1 text-sm font-semibold text-foreground">Top Paths by Error Rate (Normalized)</h3>
        <p className="mb-4 text-sm text-muted-foreground">{aggTotalsLabel}</p>
        <div className="max-h-64 space-y-2 overflow-y-auto">
          {isLoading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
          ) : aggTopNormalizedErrorPaths.length > 0 ? (
            aggTopNormalizedErrorPaths.map((item) => (
              <div key={item.path} className="flex items-center gap-2 rounded px-2 py-2 text-sm hover:bg-muted/50 transition-colors">
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
                  {formatNumber(Math.round(item.errors), 0)}/{formatNumber(Math.round(item.total), 0)}
                </span>
                <span className="w-14 flex-shrink-0 text-right font-semibold">{item.err_pct.toFixed(1)}%</span>
              </div>
            ))
          ) : (
            <p className="py-4 text-center text-sm text-muted-foreground">No error-heavy normalized paths found in the selected range.</p>
          )}
        </div>
      </div>
    </div>
  )
}
