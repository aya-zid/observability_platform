import { cn } from '@/lib/utils'
import { formatBytes, getStatusBadgeClass } from '@/lib/queryHelpers'

export interface LogEntryParsed {
  ip: string
  timestamp: string
  method: string
  path: string
  status: number
  bytes: number
  referer: string
  user_agent: string
  rt: number
}

interface NginxLogViewerRowProps {
  log: string
  parsed: LogEntryParsed | null
  expanded: boolean
  onToggle: () => void
}

export function NginxLogViewerRow({
  log,
  parsed,
  expanded,
  onToggle,
}: NginxLogViewerRowProps) {
  if (!parsed) {
    return (
      <div className="rounded p-3 font-mono text-sm text-muted-foreground hover:bg-muted">
        {log.substring(0, 200)}...
      </div>
    )
  }

  return (
    <div className="border-l-[2px] border-l-border bg-card">
      <button
        type="button"
        onClick={onToggle}
        className="w-full px-2.5 py-1 text-left hover:bg-muted transition-colors"
      >
        <div className="flex items-center gap-2">
          <span className={cn('px-1.5 py-0.5 rounded text-xs font-medium', getStatusBadgeClass(parsed.status))}>
            {parsed.status}
          </span>
          <span className="font-mono text-xs text-muted-foreground">{parsed.method}</span>
          <span className="flex-1 truncate font-mono text-xs text-foreground">{parsed.path}</span>
          <span className="text-xs text-muted-foreground">{formatBytes(parsed.bytes)}</span>
          {parsed.rt > 0 && (
            <span
              className={cn(
                'text-xs font-mono',
                parsed.rt >= 2 ? 'text-red-500' : parsed.rt >= 1 ? 'text-amber-500' : 'text-muted-foreground',
              )}
            >
              {(parsed.rt * 1000).toFixed(0)}ms
            </span>
          )}
          <span className="text-xs text-muted-foreground whitespace-nowrap">{parsed.timestamp}</span>
        </div>
      </button>

      {expanded && (
        <div className="border-t border-t-border bg-muted/50 px-2.5 py-2 text-xs font-mono">
          <div className="space-y-0.5 text-foreground">
            {[
              ['IP', parsed.ip],
              ['Method', parsed.method],
              ['Path', parsed.path],
              ['Status', String(parsed.status)],
              ['Bytes', formatBytes(parsed.bytes)],
              ['Response Time', parsed.rt > 0 ? `${(parsed.rt * 1000).toFixed(0)}ms` : '—'],
              ['Referer', parsed.referer || '—'],
              ['User Agent', parsed.user_agent],
            ].map(([label, value]) => (
              <div key={label} className="flex gap-1.5">
                <span className="text-muted-foreground w-20 flex-shrink-0">{label}:</span>
                <span className="flex-1 break-all">{value}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
