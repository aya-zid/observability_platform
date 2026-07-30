import { useState } from 'react'
import { Link } from 'react-router-dom'
import { formatNumber, getStatusBadgeClass } from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'
import { methodBadgeColor } from '@/lib/methodBadgeColor'
import type { NginxDashboardSummary } from '@/api/nginx'

export function NginxSlowRequestTable({
  rows,
  total,
  minRT,
  thresholdValue,
  onThresholdChange,
  onThresholdBlur,
  viewMoreHref,
  loading,
}: {
  rows: NginxDashboardSummary['slow_requests']
  total: number
  minRT: number
  thresholdValue: string
  onThresholdChange: (value: string) => void
  onThresholdBlur: () => void
  viewMoreHref: string
  loading?: boolean
}) {
  const [expandedRow, setExpandedRow] = useState<number | null>(null)

  return (
    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="mb-1 text-sm font-semibold text-foreground">Slow Requests (&gt;{minRT}s) — Recent 100</h3>
          <p className="text-sm text-muted-foreground">
            Showing {formatNumber(rows.length, 0)} of {formatNumber(total, 0)} found. Click a request to view more details.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <label className="flex items-center gap-2 rounded-md border border-border bg-background px-2 py-1.5 text-sm font-medium text-muted-foreground">
            RT
            <input
              type="number"
              min="0.1"
              step="0.1"
              value={thresholdValue}
              onChange={(event) => onThresholdChange(event.target.value)}
              onBlur={onThresholdBlur}
              className="h-7 w-16 rounded border border-border bg-card px-2 text-sm text-foreground outline-none focus:border-primary"
            />
            <span>s</span>
          </label>
          {total > rows.length ? (
            <Link
              to={viewMoreHref}
              className="rounded-md border border-border bg-background px-3 py-1.5 text-sm font-semibold text-foreground hover:bg-muted"
            >
              View More
            </Link>
          ) : null}
        </div>
      </div>
      {loading ? (
        <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
      ) : rows.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">
          No requests over {minRT}s found in the selected range.
        </p>
      ) : (
        <div className="max-h-[320px] overflow-auto">
          <div className="space-y-2">
            {rows.map((req, i) => {
              const isOpen = expandedRow === i
              return (
                <div key={`${req.path}-${req.ip}-${i}`} className="rounded-lg border border-border/70 hover:border-border/100 transition-colors">
                  <button
                    type="button"
                    onClick={() => setExpandedRow(isOpen ? null : i)}
                    className="flex w-full items-center gap-3 px-3 py-3 text-left hover:bg-muted/40 transition-colors"
                  >
                    <span className="min-w-0 flex-1 truncate font-mono text-sm" title={req.path}>
                      {req.path}
                    </span>
                    <span className="hidden w-28 flex-shrink-0 truncate font-mono text-sm text-muted-foreground md:inline" title={req.ip}>
                      {req.ip || '—'}
                    </span>
                    <span
                      className="rounded px-1.5 py-0.5 text-sm font-semibold text-white flex-shrink-0"
                      style={{ background: methodBadgeColor(req.method) }}
                    >
                      {req.method}
                    </span>
                    <span className={cn('rounded px-1.5 py-0.5 text-sm font-semibold flex-shrink-0', getStatusBadgeClass(req.status))}>
                      {req.status}
                    </span>
                    <span
                      className={cn(
                        'w-20 flex-shrink-0 text-right text-sm font-semibold tabular-nums',
                        req.rt >= 5 ? 'text-red-500' : req.rt >= 2 ? 'text-amber-500' : 'text-foreground',
                      )}
                    >
                      {(req.rt * 1000).toFixed(0)}ms
                    </span>
                  </button>
                  {isOpen ? (
                    <div className="border-t border-border bg-muted/30 px-3 py-3 text-sm">
                      <div className="grid grid-cols-1 gap-2 md:grid-cols-2">
                        <div>
                          <span className="text-muted-foreground">Timestamp:</span>{' '}
                          <span className="font-mono">{req.timestamp || '—'}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Path:</span>{' '}
                          <span className="font-mono break-all">{req.path}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Client IP:</span>{' '}
                          <span className="font-mono">{req.ip}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Method:</span>{' '}
                          <span>{req.method}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Status:</span>{' '}
                          <span>{req.status}</span>
                        </div>
                        <div>
                          <span className="text-muted-foreground">Response Time:</span>{' '}
                          <span className="font-mono">{req.rt.toFixed(3)}s</span>
                        </div>
                        {req.referer && (
                          <div className="md:col-span-2">
                            <span className="text-muted-foreground">Referer:</span>{' '}
                            <span className="font-mono break-all">{req.referer}</span>
                          </div>
                        )}
                        {req.user_agent && (
                          <div className="md:col-span-2">
                            <span className="text-muted-foreground">User Agent:</span>{' '}
                            <span className="font-mono break-all text-sm">{req.user_agent}</span>
                          </div>
                        )}
                      </div>
                    </div>
                  ) : null}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
