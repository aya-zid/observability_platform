/**
 * Data table components for dashboard tables
 */

export interface TableRow {
  label: string
  value?: string | number
  percent?: number
  badge?: string
  status?: 'success' | 'warning' | 'error'
  details?: string[]
}

export interface DataTableProps {
  title: string
  rows: TableRow[]
  loading?: boolean
  columns?: string[]
  maxHeight?: string
}

/**
 * Generic paginated data table
 */
export function DataTable({
  title,
  rows,
  loading,
  maxHeight = 'max-h-64',
}: DataTableProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">{title}</h3>
      <div className={`space-y-1 ${maxHeight} overflow-y-auto`}>
        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
        ) : rows.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
        ) : (
          rows.map((row, idx) => (
            <div
              key={idx}
              className={`flex items-center justify-between rounded px-2 py-2 hover:bg-muted/50 text-xs
                ${row.status === 'error' ? 'border-l-2 border-red-500' : ''}
                ${row.status === 'warning' ? 'border-l-2 border-yellow-500' : ''}
                ${row.status === 'success' ? 'border-l-2 border-green-500' : ''}
              `}
            >
              <div className="flex items-center gap-2 flex-1 min-w-0">
                {row.badge && (
                  <span
                    className={`px-2 py-0.5 rounded text-xs font-medium whitespace-nowrap
                      ${row.badge === 'ERROR' ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' : ''}
                      ${row.badge === 'OK' ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400' : ''}
                      ${row.badge === 'SLOW' ? 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400' : ''}
                      ${row.badge ? `bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400` : ''}
                    `}
                  >
                    {row.badge}
                  </span>
                )}
                <span className="font-mono truncate">{row.label}</span>
                {row.details && (
                  <span className="text-muted-foreground whitespace-nowrap ml-auto mr-2">
                    {row.details.join(' • ')}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-2 ml-2 flex-shrink-0">
                {row.percent !== undefined && (
                  <span className="text-muted-foreground text-xs w-12 text-right">{row.percent.toFixed(1)}%</span>
                )}
                {row.value !== undefined && (
                  <span className="font-semibold text-primary text-xs w-20 text-right">{row.value}</span>
                )}
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}

/**
 * Table for slow requests (requires rt= field in logs)
 */
export interface SlowRequest {
  path: string
  method: string
  status: number
  response_time: number // ms
  client_ip: string
  timestamp?: string
}

export function SlowRequestsTable({
  requests,
  loading,
}: {
  requests: SlowRequest[]
  loading?: boolean
}) {
  const statusBgColor = (status: number) => {
    if (status >= 200 && status < 300) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
    if (status >= 300 && status < 400) return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
    if (status >= 400 && status < 500) return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Slow Requests (&gt;1s) — Last 50</h3>
      <div className="overflow-x-auto">
        <div className="min-w-full space-y-1 max-h-96 overflow-y-auto">
          {loading ? (
            <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
          ) : requests.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">No slow requests detected (great!)</p>
          ) : (
            <div className="space-y-1">
              {/* Header */}
              <div className="flex items-center gap-2 px-2 py-1.5 bg-muted text-xs font-semibold rounded mb-1">
                <div className="flex-1 min-w-48">Path</div>
                <div className="w-12 text-center">Method</div>
                <div className="w-12 text-center">Status</div>
                <div className="w-20 text-right">Time (ms)</div>
                <div className="w-20 text-right">Client IP</div>
              </div>
              {/* Rows */}
              {requests.map((req, idx) => (
                <div
                  key={idx}
                  className="flex items-center gap-2 px-2 py-1.5 hover:bg-muted/50 rounded text-xs border-l-2 border-yellow-500"
                >
                  <div className="flex-1 min-w-48 truncate font-mono">{req.path}</div>
                  <div className="w-12 text-center font-medium">{req.method}</div>
                  <div className={`w-12 text-center px-1 py-0.5 rounded text-xs font-semibold ${statusBgColor(req.status)}`}>
                    {req.status}
                  </div>
                  <div className="w-20 text-right font-semibold">
                    {req.response_time.toFixed(1)}
                  </div>
                  <div className="w-20 text-right font-mono text-muted-foreground">{req.client_ip}</div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

/**
 * Security alerting table for brute force IPs
 */
export interface BruteForceAlert {
  ip: string
  error_count: number
  first_seen: string
  last_seen: string
  severity: 'low' | 'medium' | 'high'
}

export function BruteForceAlertsTable({
  alerts,
  loading,
}: {
  alerts: BruteForceAlert[]
  loading?: boolean
}) {
  const severityBgColor = (severity: string) => {
    switch (severity) {
      case 'low':
        return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
      case 'medium':
        return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
      case 'high':
        return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
      default:
        return ''
    }
  }

  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-2 font-semibold text-sm flex items-center gap-2">
        <span className="text-lg">🔒</span> Security: IPs with 4xx Spike
      </h3>
      <p className="text-xs text-muted-foreground mb-4">
        IPs with &gt;20 4xx errors in last 5 minutes (potential brute force)
      </p>
      <div className="max-h-80 overflow-y-auto">
        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
        ) : alerts.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">✅ No threats detected</p>
        ) : (
          <div className="space-y-1">
            {alerts.map((alert, idx) => (
              <div
                key={idx}
                className="flex items-center justify-between px-2 py-2 rounded hover:bg-muted/50 border-l-4 border-red-500"
              >
                <div className="flex items-center gap-2 flex-1">
                  <span className={`px-2 py-1 rounded text-xs font-semibold ${severityBgColor(alert.severity)}`}>
                    {alert.severity.toUpperCase()}
                  </span>
                  <span className="font-mono text-sm">{alert.ip}</span>
                </div>
                <div className="flex items-center gap-4 text-xs">
                  <div>
                    <p className="text-muted-foreground">4xx Count</p>
                    <p className="font-bold text-red-600">{alert.error_count}</p>
                  </div>
                  <div>
                    <p className="text-muted-foreground text-right">First Seen</p>
                    <p className="font-mono text-xs text-right">{alert.first_seen}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

/**
 * Traffic classification table (bot vs human)
 */
export interface TrafficClassification {
  agent_type: 'bot' | 'human'
  count: number
  percent: number
}

export function BotHumanBreakdownTable({
  data,
  loading,
}: {
  data: TrafficClassification[]
  loading?: boolean
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold text-sm">Traffic Classification</h3>
      <div className="space-y-2">
        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
        ) : data.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
        ) : (
          data.map((row, idx) => (
            <div key={idx} className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className={`px-3 py-1 rounded-full text-sm font-semibold ${
                  row.agent_type === 'bot'
                    ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
                    : 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                }`}>
                  {row.agent_type === 'bot' ? '🤖 Bots' : '👤 Humans'}
                </span>
              </div>
              <div className="flex items-center gap-4">
                <div className="w-32 bg-muted rounded-full h-2">
                  <div
                    className={`h-2 rounded-full ${
                      row.agent_type === 'bot'
                        ? 'bg-purple-500'
                        : 'bg-green-500'
                    }`}
                    style={{ width: `${row.percent}%` }}
                  />
                </div>
                <span className="text-sm font-semibold w-16 text-right">{row.percent.toFixed(1)}%</span>
                <span className="text-xs text-muted-foreground w-20 text-right">{row.count} req</span>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  )
}
