import { useState, useCallback, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import apiClient from '@/api/client'
import { getDiscoveryService } from '@/api/discovery'
import { getNginxDomains, getNginxErrorLogs } from '@/api/nginx'
import {
  parseLogLines,
  parseNginxLogLine,
  getNowSeconds,
  getStartSeconds,
  getStep,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'
import { RefreshCw } from 'lucide-react'
import { NginxLogViewerRow, type LogEntryParsed } from './components/NginxLogViewerRow'
import { NginxLogFilters, type FilterState } from './components/NginxLogFilters'
import { NginxLogStatsCards } from './components/NginxLogStatsCards'

const PAGE_SIZE = 200

const TIME_RANGES = [
  { label: '15m', seconds: 900 },
  { label: '1h', seconds: 3600 },
  { label: '6h', seconds: 21600 },
  { label: '24h', seconds: 86400 },
]

interface LogPage {
  rawLines: string[]
  parsed: (LogEntryParsed | null)[]
  oldestNs: bigint | null
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
          className={cn(
            'px-3 py-1.5 rounded text-sm font-medium transition-colors',
            selected.seconds === range.seconds
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground',
          )}
        >
          {range.label}
        </button>
      ))}
    </div>
  )
}

function extractOldestNsFromLokiResponse(raw: unknown): bigint | null {
  try {
    const data = (raw as Record<string, unknown>)?.data as Record<string, unknown> | undefined
    const result = data?.result as Array<unknown> | undefined
    if (!Array.isArray(result) || result.length === 0) return null

    let oldest: bigint | null = null
    for (const stream of result) {
      const entries = ((stream as Record<string, unknown>).values as Array<[string, string]>) ?? []
      for (const [ts] of entries) {
        try {
          const tsNs = BigInt(ts)
          if (oldest === null || tsNs < oldest) {
            oldest = tsNs
          }
        } catch {
          // ignore non-numeric timestamps
        }
      }
    }
    return oldest
  } catch {
    return null
  }
}

function buildDomainHref(basePath: string, domain: string | null) {
  if (!domain) {
    return basePath
  }
  return `${basePath}?domain=${encodeURIComponent(domain)}`
}

function escapeLokiLabelValue(value: string) {
  return value.replaceAll('\\', '\\\\').replaceAll('"', '\\"')
}

export function NginxLogsPage() {
  const { id = '' } = useParams<{ id?: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [expanded, setExpanded] = useState<number | null>(null)
  const [timeRange, setTimeRange] = useState(TIME_RANGES[1])
  const [logType, setLogType] = useState<'access' | 'error'>('access')
  const [filters, setFilters] = useState<FilterState>({
    method: null,
    statusCode: null,
    searchPath: null,
    searchIP: null,
  })

  const [pages, setPages] = useState<LogPage[]>([])
  const [olderCursor, setOlderCursor] = useState<bigint | null>(null)
  const [isLoadingOlder, setIsLoadingOlder] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [windowStartNs, setWindowStartNs] = useState<string | null>(null)

  const serviceQuery = useQuery({
    queryKey: ['discovery-service', id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const serviceName = serviceQuery.data?.service_name ?? 'nginx'
  const serviceReady = Boolean(id && serviceQuery.data?.service_name)
  const domainsQuery = useQuery({
    queryKey: ['nginx-domains', id],
    queryFn: () => getNginxDomains(id),
    enabled: serviceReady,
    staleTime: 30_000,
  })

  const domains = useMemo(
    () => (domainsQuery.data ?? [])
      .filter((item) => item.enabled)
      .map((item) => item.domain),
    [domainsQuery.data],
  )

  const requestedDomain = (searchParams.get('domain') ?? '').trim()
  const selectedDomain = useMemo(() => {
    if (domains.length === 0) {
      return ''
    }
    if (requestedDomain && domains.includes(requestedDomain)) {
      return requestedDomain
    }
    return domains[0]
  }, [domains, requestedDomain])

  useEffect(() => {
    if (domains.length === 0) {
      if (requestedDomain) {
        setSearchParams({}, { replace: true })
      }
      return
    }

    if (selectedDomain && selectedDomain !== requestedDomain) {
      setSearchParams({ domain: selectedDomain }, { replace: true })
    }
  }, [domains.length, requestedDomain, selectedDomain, setSearchParams])

  useEffect(() => {
    setExpanded(null)
    setPages([])
    setOlderCursor(null)
    setWindowStartNs(null)
    setHasMore(true)
    setLogType('access')
  }, [selectedDomain, timeRange.seconds])

  const lokiFilter = selectedDomain
    ? `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access" | vhost="${escapeLokiLabelValue(selectedDomain)}"`
    : `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access"`

  const errorLogsQuery = useQuery({
    queryKey: [`nginx-error-logs-${id}`, selectedDomain, timeRange.seconds, logType],
    queryFn: async () => {
      const nowNs = secondsToNanos(getNowSeconds())
      const startNs = secondsToNanos(getStartSeconds(timeRange.seconds))
      return getNginxErrorLogs(id, {
        start: startNs,
        end: nowNs,
        domain: selectedDomain || undefined,
        limit: 200,
      })
    },
    enabled: serviceReady && logType === 'error',
    refetchInterval: 30_000,
    staleTime: 0,
  })

  const { isLoading: initialLoading, refetch, isRefetching } = useQuery({
    queryKey: [`nginx-logs-initial-${id}`, selectedDomain, timeRange.seconds],
    queryFn: async () => {
      const nowNs = secondsToNanos(getNowSeconds())
      const startNs = secondsToNanos(getStartSeconds(timeRange.seconds))

      setWindowStartNs(startNs)

      const res = await apiClient.get('/api/query/logs', {
        params: {
          query: lokiFilter,
          limit: PAGE_SIZE,
          start: startNs,
          end: nowNs,
          direction: 'backward',
          step: getStep(timeRange.seconds),
        },
      })

      const rawLines = parseLogLines(res.data)
      const oldest = extractOldestNsFromLokiResponse(res.data)

      const newPage: LogPage = {
        rawLines,
        parsed: rawLines.map(parseNginxLogLine),
        oldestNs: oldest,
      }

      setPages([newPage])
      setOlderCursor(oldest !== null ? oldest - 1n : null)
      setHasMore(rawLines.length === PAGE_SIZE)

      return res.data
    },
    staleTime: 0,
    refetchInterval: 15000,
    refetchOnWindowFocus: true,
    enabled: !!id && !!serviceQuery.data && (domains.length > 0 || domainsQuery.isSuccess) && logType === 'access',
  })

  const loadOlder = useCallback(async () => {
    if (!olderCursor || isLoadingOlder || !windowStartNs) return
    setIsLoadingOlder(true)
    try {
      const res = await apiClient.get('/api/query/logs', {
        params: {
          query: lokiFilter,
          limit: PAGE_SIZE,
          start: windowStartNs,
          end: String(olderCursor),
          direction: 'backward',
          step: getStep(timeRange.seconds),
        },
      })
      const rawLines = parseLogLines(res.data)
      const oldest = extractOldestNsFromLokiResponse(res.data)

      const newPage: LogPage = {
        rawLines,
        parsed: rawLines.map(parseNginxLogLine),
        oldestNs: oldest,
      }
      setPages((prev) => [...prev, newPage])
      setOlderCursor(oldest !== null ? oldest - 1n : null)
      setHasMore(rawLines.length === PAGE_SIZE)
    } finally {
      setIsLoadingOlder(false)
    }
  }, [olderCursor, isLoadingOlder, lokiFilter, timeRange.seconds, windowStartNs])

  const allEntries = pages.flatMap((page, pageIdx) =>
    page.rawLines.map((log, lineIdx) => ({
      log,
      parsed: page.parsed[lineIdx],
      globalIdx: pageIdx * PAGE_SIZE + lineIdx,
    })),
  )

  const filteredEntries = allEntries.filter(({ parsed }) => {
    if (!parsed) return false
    if (filters.method && parsed.method !== filters.method) return false
    if (filters.statusCode && parsed.status.toString() !== filters.statusCode) return false
    if (filters.searchPath && !parsed.path.includes(filters.searchPath)) return false
    if (filters.searchIP && !parsed.ip.includes(filters.searchIP)) return false
    return true
  })

  const totalLoaded = allEntries.filter(({ parsed }) => parsed !== null).length

  const allParsed = allEntries.map((e) => e.parsed).filter((p): p is LogEntryParsed => p !== null)
  const uniqueMethods  = Array.from(new Set(allParsed.map((p) => p.method))).sort()
  const uniqueStatuses = Array.from(new Set(allParsed.map((p) => p.status))).sort((a, b) => a - b)

  if (!id) return <div className="p-6 text-sm text-red-600">Missing service id in path.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading service details...</div>
  if (!serviceQuery.data) return <div className="p-6 text-sm text-red-600">Nginx service not found for id {id}.</div>

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-3xl font-bold">Nginx Logs</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Service: <span className="font-mono font-semibold">{serviceName}</span>
          </p>
        </div>
        <div className="flex items-center gap-4 flex-wrap">
          <TimeRangeSelector selected={timeRange} onChange={(r) => {
            setTimeRange(r)
            setPages([])
            setOlderCursor(null)
            setWindowStartNs(null)
            setHasMore(true)
          }} />
          <Link
            to={buildDomainHref(`/services/nginx-dashboard/${id}`, selectedDomain || null)}
            className="inline-block rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-500"
          >
            ← Back to Dashboard
          </Link>
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-1.5 rounded-md bg-muted px-4 py-2 text-sm font-semibold text-foreground hover:bg-muted/80"
          >
            <RefreshCw className={`h-4 w-4 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex flex-col gap-3">
          <div>
            <p className="text-sm font-semibold">Domains</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Switch tabs to inspect logs for one Nginx domain at a time.
            </p>
          </div>
          {domainsQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading domains...</p>
          ) : domains.length === 0 ? (
            <p className="text-sm text-muted-foreground">No enabled Nginx domains found yet.</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {domains.map((domain) => {
                const active = domain === selectedDomain
                return (
                  <button
                    key={domain}
                    type="button"
                    onClick={() => setSearchParams({ domain })}
                    className={cn(
                      'rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                      active
                        ? 'border-blue-600 bg-blue-600 text-white'
                        : 'border-border bg-muted/50 text-foreground hover:bg-muted',
                    )}
                  >
                    <span className="font-mono">{domain}</span>
                  </button>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {selectedDomain ? (
        <div className="rounded-lg border border-border bg-muted/30 px-4 py-3 text-sm text-muted-foreground">
          Active domain: <span className="font-mono font-semibold text-foreground">{selectedDomain}</span>
        </div>
      ) : null}

      <div className="flex items-center gap-2">
        <span className="text-sm font-medium text-muted-foreground">Log type:</span>
        <div className="flex gap-1 rounded-lg border border-border bg-muted p-1">
          {(['access', 'error'] as const).map((type) => (
            <button
              key={type}
              type="button"
              onClick={() => setLogType(type)}
              className={cn(
                'px-4 py-1.5 rounded text-sm font-medium transition-colors capitalize',
                logType === type
                  ? type === 'error'
                    ? 'bg-red-600 text-white'
                    : 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {type === 'access' ? '📋 Access Logs' : '🔴 Error Logs'}
            </button>
          ))}
        </div>
      </div>

      {logType === 'error' && (
        <div className="space-y-4">
          <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
            <p className="font-semibold">🔴 Nginx Error Logs</p>
            <p className="mt-0.5 text-xs">
              Raw unstructured error log lines — upstream failures, connection errors, config issues.
              Newest first · auto-refresh 30s · up to 200 lines shown.
            </p>
          </div>

          {errorLogsQuery.isFetching ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              Loading error logs...
            </div>
          ) : errorLogsQuery.isError ? (
            <div className="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-700">
              Failed to load error logs. The error log collection may not be enabled for this domain yet.
            </div>
          ) : !errorLogsQuery.data?.lines?.length ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              No error logs found for this time range — that's a good sign! 🎉
            </div>
          ) : (
            <div className="rounded-lg border border-border bg-card overflow-hidden">
              <div className="border-b border-border p-4 flex items-center justify-between">
                <h3 className="font-semibold">
                  Error Log Lines ({errorLogsQuery.data.lines.length} most recent)
                </h3>
                <p className="text-xs text-muted-foreground">
                  Last updated {new Date().toLocaleTimeString()}
                </p>
              </div>
              <div className="max-h-[600px] overflow-y-auto">
                {errorLogsQuery.data.lines.map((line, idx) => (
                  <div
                    key={idx}
                    className={cn(
                      'border-b border-border/50 px-4 py-2 font-mono text-xs leading-relaxed',
                      line.includes('[error]') || line.includes('[crit]') || line.includes('[alert]') || line.includes('[emerg]')
                        ? 'text-red-700 dark:text-red-400 bg-red-50/30 dark:bg-red-900/10'
                        : line.includes('[warn]')
                          ? 'text-yellow-700 dark:text-yellow-400 bg-yellow-50/30 dark:bg-yellow-900/10'
                          : 'text-foreground hover:bg-muted/30',
                    )}
                  >
                    {line}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {logType === 'access' && (
        <>
          <NginxLogStatsCards
            totalLoaded={totalLoaded}
            filteredCount={filteredEntries.length}
            pagesCount={pages.length}
            initialLoading={initialLoading}
          />

          <NginxLogFilters
            filters={filters}
            setFilters={setFilters}
            uniqueMethods={uniqueMethods}
            uniqueStatuses={uniqueStatuses}
          />

          <div className="rounded-lg border border-border bg-card">
            <div className="border-b border-b-border p-4 flex items-center justify-between">
              <div>
                <h3 className="font-semibold">
                  Log Entries ({filteredEntries.length} of {totalLoaded} loaded) — newest first
                </h3>
                <p className="mt-1 text-xs text-muted-foreground">Click a row to expand and view full details</p>
              </div>
              {pages.length > 1 && (
                <p className="text-xs text-muted-foreground">{pages.length} pages × {PAGE_SIZE} entries</p>
              )}
            </div>

            <div className="space-y-0 max-h-[600px] overflow-y-auto">
              {initialLoading ? (
                <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
                  Loading logs...
                </div>
              ) : filteredEntries.length === 0 ? (
                <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
                  No logs match the selected filters
                </div>
              ) : (
                filteredEntries.map(({ parsed, log, globalIdx }) => (
                  <NginxLogViewerRow
                    key={globalIdx}
                    log={log}
                    parsed={parsed}
                    expanded={expanded === globalIdx}
                    onToggle={() => setExpanded(expanded === globalIdx ? null : globalIdx)}
                  />
                ))
              )}
            </div>

            {!initialLoading && totalLoaded > 0 && (
              <div className="border-t border-t-border p-4 flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                  {hasMore
                    ? `Showing ${totalLoaded} most recent entries. Click to load older entries.`
                    : `All ${totalLoaded} entries loaded for this time range.`}
                </p>
                {hasMore && olderCursor !== null && (
                  <button
                    type="button"
                    onClick={loadOlder}
                    disabled={isLoadingOlder}
                    className="rounded-md bg-muted px-4 py-2 text-sm font-semibold hover:bg-muted/80 disabled:opacity-50"
                  >
                    {isLoadingOlder ? 'Loading...' : `Load ${PAGE_SIZE} older entries`}
                  </button>
                )}
              </div>
            )}
          </div>

          <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">
            <p className="mb-1 font-semibold">💡 Filter Tips</p>
            <ul className="list-inside space-y-1 text-xs">
              <li>• Logs are shown newest-first. Use "Load older entries" to page backward in time.</li>
              <li>• Select HTTP method to filter by GET, POST, PUT, DELETE, etc.</li>
              <li>• Filter by status code to find errors (4xx, 5xx) or successes (2xx)</li>
              <li>• Use path search to find requests to specific endpoints</li>
              <li>• Use IP search to filter by client IP address or subnet prefix</li>
              <li>• Logs auto-refresh every 15 seconds (newest entries only)</li>
            </ul>
          </div>
        </>
      )}
    </div>
  )
}
