// src/pages/integrations/NginxLogsPage.tsx

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import apiClient from '@/api/client'
import { getDiscoveryService } from '@/api/discovery'
import {
  parseLogLines,
  parseNginxLogLine,
  getStatusBadgeClass,
  formatBytes,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from '@/lib/queryHelpers'

const LOG_LIMIT = 300

const TIME_RANGES = [
  { label: '15m', seconds: 900 },
  { label: '1h', seconds: 3600 },
  { label: '6h', seconds: 21600 },
  { label: '24h', seconds: 86400 },
]

interface LogEntryParsed {
  ip: string
  timestamp: string
  method: string
  path: string
  status: number
  bytes: number
  referer: string
  user_agent: string
}

interface LogViewerRowProps {
  log: string
  parsed: LogEntryParsed | null
  expanded: boolean
  onToggle: () => void
}

function TimeRangeSelector({
  selected,
  onChange,
}: {
  selected: (typeof TIME_RANGES)[0]
  onChange: (range: (typeof TIME_RANGES)[0]) => void
}) {
  return (
    <div className="flex gap-1 rounded-lg border border-border bg-muted p-1">
      {TIME_RANGES.map((range) => (
        <button
          key={range.label}
          onClick={() => onChange(range)}
          className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
            selected.seconds === range.seconds
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {range.label}
        </button>
      ))}
    </div>
  )
}

function LogViewerRow({ log, parsed, expanded, onToggle }: LogViewerRowProps) {
  if (!parsed) {
    return (
      <div className="rounded p-3 font-mono text-xs text-muted-foreground hover:bg-muted">
        {log.substring(0, 200)}...
      </div>
    )
  }

  return (
    <div className="border-l-2 border-l-border bg-card">
      <button
        type="button"
        onClick={onToggle}
        className="w-full px-3 py-2 text-left hover:bg-muted transition-colors"
      >
        <div className="flex items-center gap-3">
          <span className={`px-2 py-1 rounded text-xs font-medium ${getStatusBadgeClass(parsed.status)}`}>
            {parsed.status}
          </span>
          <span className="font-mono text-xs text-muted-foreground">{parsed.method}</span>
          <span className="flex-1 truncate font-mono text-xs text-foreground">{parsed.path}</span>
          <span className="text-xs text-muted-foreground">{formatBytes(parsed.bytes)}</span>
          <span className="text-xs text-muted-foreground whitespace-nowrap">{parsed.timestamp}</span>
        </div>
      </button>

      {expanded && (
        <div className="border-t border-t-border bg-muted/50 px-3 py-3 text-xs font-mono">
          <div className="space-y-1 text-foreground">
            <div className="flex gap-2">
              <span className="text-muted-foreground">IP:</span>
              <span className="flex-1">{parsed.ip}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">Method:</span>
              <span className="flex-1">{parsed.method}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">Path:</span>
              <span className="flex-1 break-all">{parsed.path}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">Status:</span>
              <span className="flex-1">{parsed.status}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">Bytes:</span>
              <span className="flex-1">{formatBytes(parsed.bytes)}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">Referer:</span>
              <span className="flex-1 break-all">{parsed.referer || '—'}</span>
            </div>
            <div className="flex gap-2">
              <span className="text-muted-foreground">User Agent:</span>
              <span className="flex-1 break-all text-muted-foreground">{parsed.user_agent}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

interface FilterState {
  method: string | null
  statusCode: string | null
  searchPath: string | null
}

export function NginxLogsPage() {
  const { id = '' } = useParams<{ id?: string }>()
  const [expanded, setExpanded] = useState<number | null>(null)
  const [timeRange, setTimeRange] = useState(TIME_RANGES[1]) // 1h default
  const [filters, setFilters] = useState<FilterState>({
    method: null,
    statusCode: null,
    searchPath: null,
  })

  const serviceQuery = useQuery({
    queryKey: ['discovery-service', id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const serviceName = serviceQuery.data?.service_name ?? 'nginx'

  const lokiFilter = `{service_name="${serviceName}"}`

  const recentLogsQuery = useQuery({
    queryKey: [`nginx-logs-${id}`, filters, timeRange.seconds],
    queryFn: async () => {
      // Compute time fresh inside query function
      const queryNow = getNowSeconds()
      const queryStart = getStartSeconds(timeRange.seconds)
      
      const res = await apiClient.get('/api/query/logs', {
        params: {
          query: lokiFilter,
          limit: LOG_LIMIT,
          start: secondsToNanos(queryStart),
          end: secondsToNanos(queryNow),
          direction: 'backward',
        },
      })
      return res.data
    },
    staleTime: 0,
    refetchOnWindowFocus: true,
    refetchInterval: 10000,
  })

  const logs = parseLogLines(recentLogsQuery.data)

  // Parse and filter logs
  const parsedLogs = logs.map(parseNginxLogLine)

  // Reverse to show newest logs first
  const filteredLogs = parsedLogs
    .reverse()
    .map((parsed, idx) => ({
      parsed,
      log: logs[logs.length - 1 - idx],  // Adjust index for reversed array
      idx,
    }))
    .filter(({ parsed }) => {
      if (!parsed) return false
      if (filters.method && parsed.method !== filters.method) return false
      if (filters.statusCode && parsed.status.toString() !== filters.statusCode) return false
      if (filters.searchPath && !parsed.path.includes(filters.searchPath)) return false
      return true
    })

  // Extract unique values for filter dropdowns
  const uniqueMethods = Array.from(new Set(parsedLogs.filter((p): p is LogEntryParsed => p !== null).map((p) => p.method))).sort()
  const uniqueStatuses = Array.from(new Set(parsedLogs.filter((p): p is LogEntryParsed => p !== null).map((p) => p.status))).sort((a, b) => a - b)

  const stats = {
    total: logs.length,
    parsed: parsedLogs.filter(Boolean).length,
    filtered: filteredLogs.length,
  }

  // Early returns after all hooks are called
  if (!id) {
    return <div className="p-6 text-sm text-red-600">Missing service id in path.</div>
  }

  if (serviceQuery.isLoading) {
    return <div className="p-6 text-sm text-muted-foreground">Loading service details...</div>
  }

  if (!serviceQuery.data) {
    return <div className="p-6 text-sm text-red-600">Nginx service not found for id {id}.</div>
  }

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Nginx Logs</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Service: <span className="font-mono font-semibold">{serviceName}</span>
          </p>
        </div>
        <div className="flex items-center gap-4">
          <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
          <Link
            to={`/integrations/nginx-dashboard/${id}`}
            className="inline-block rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-500"
          >
            ← Back to Dashboard
          </Link>
          <button
            type="button"
            onClick={() => recentLogsQuery.refetch()}
            className="inline-block rounded-md bg-muted px-4 py-2 text-sm font-semibold text-foreground hover:bg-muted/80"
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="flex gap-4">
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Total Logs</p>
          <p className="mt-1 text-2xl font-bold text-primary">{stats.total}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Parsed</p>
          <p className="mt-1 text-2xl font-bold text-green-500">{stats.parsed}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Matching Filters</p>
          <p className="mt-1 text-2xl font-bold text-blue-500">{stats.filtered}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4 flex-1">
          {recentLogsQuery.isLoading && <p className="text-sm text-muted-foreground">Updating logs...</p>}
          {!recentLogsQuery.isLoading && (
            <p className="text-xs text-muted-foreground">Last updated {new Date().toLocaleTimeString()}</p>
          )}
        </div>
      </div>

      {/* Filters */}
      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="mb-4 font-semibold">Filters</h3>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-2">HTTP Method</label>
            <select
              value={filters.method || ''}
              onChange={(e) => setFilters({ ...filters, method: e.target.value || null })}
              className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
            >
              <option value="">All</option>
              {uniqueMethods.map((method) => (
                <option key={method} value={method}>
                  {method}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-2">Status Code</label>
            <select
              value={filters.statusCode || ''}
              onChange={(e) => setFilters({ ...filters, statusCode: e.target.value || null })}
              className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
            >
              <option value="">All</option>
              {uniqueStatuses.map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </select>
          </div>

          <div className="md:col-span-2">
            <label className="block text-xs font-semibold text-muted-foreground mb-2">Search Path</label>
            <input
              type="text"
              value={filters.searchPath || ''}
              onChange={(e) => setFilters({ ...filters, searchPath: e.target.value || null })}
              placeholder="e.g., /api, /home, /admin"
              className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
            />
          </div>
        </div>
        <button
          type="button"
          onClick={() => setFilters({ method: null, statusCode: null, searchPath: null })}
          className="mt-4 text-sm text-blue-600 hover:text-blue-500"
        >
          Clear Filters
        </button>
      </div>

      {/* Logs  */}
      <div className="rounded-lg border border-border bg-card">
        <div className="border-b border-b-border p-4">
          <h3 className="font-semibold">
            Log Entries ({stats.filtered} of {stats.parsed} parsed)
          </h3>
          <p className="mt-1 text-xs text-muted-foreground">Click a row to expand and view full details</p>
        </div>

        <div className="space-y-0 max-h-[600px] overflow-y-auto">
          {recentLogsQuery.isLoading ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              Loading logs...
            </div>
          ) : filteredLogs.length === 0 ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              No logs match the selected filters
            </div>
          ) : (
            filteredLogs.map(({ parsed, log, idx }) => (
              <LogViewerRow
                key={idx}
                log={log}
                parsed={parsed}
                expanded={expanded === idx}
                onToggle={() => setExpanded(expanded === idx ? null : idx)}
              />
            ))
          )}
        </div>
      </div>

      {/* Info */}
      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">
        <p className="mb-1 font-semibold">💡 Filter Tips</p>
        <ul className="list-inside space-y-1 text-xs">
          <li>• Select HTTP method to filter by GET, POST, PUT, DELETE, etc.</li>
          <li>• Filter by status code to find errors (4xx, 5xx) or successes (2xx)</li>
          <li>• Use path search to find requests to specific endpoints</li>
          <li>• Logs update automatically every 10 seconds</li>
        </ul>
      </div>
    </div>
  )
}
