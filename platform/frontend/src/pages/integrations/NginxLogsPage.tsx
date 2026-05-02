// src/pages/integrations/NginxLogsPage.tsx
//
// CHANGES in this version:
// - log_kind="access" is now always included in the Loki stream selector so that
//   nginx error logs (which have a completely different format) never appear in
//   the access log viewer and do not corrupt "Loaded" counts.
// - New "Error Logs" tab per domain: queries /api/nginx/:id/error-logs and
//   displays raw unstructured error log lines in a plain text viewer.
// - Cursor-based pagination and auto-refresh unchanged.

import { useState, useCallback, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import apiClient from '@/api/client'
import { getDiscoveryService } from '@/api/discovery'
import { getNginxDomains, getNginxErrorLogs } from '@/api/nginx'
import {
  parseLogLines,
  parseNginxLogLine,
  getStatusBadgeClass,
  formatBytes,
  getNowSeconds,
  getStartSeconds,
  getStep,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

const PAGE_SIZE = 200

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
  rt: number
}

interface LogPage {
  rawLines: string[]
  parsed: (LogEntryParsed | null)[]
  // Oldest timestamp in this page (nanoseconds as bigint) — used as next cursor
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

function LogViewerRow({
  log,
  parsed,
  expanded,
  onToggle,
}: {
  log: string
  parsed: LogEntryParsed | null
  expanded: boolean
  onToggle: () => void
}) {
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
          <span className={cn('px-2 py-1 rounded text-xs font-medium', getStatusBadgeClass(parsed.status))}>
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
        <div className="border-t border-t-border bg-muted/50 px-3 py-3 text-xs font-mono">
          <div className="space-y-1 text-foreground">
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
              <div key={label} className="flex gap-2">
                <span className="text-muted-foreground w-24 flex-shrink-0">{label}:</span>
                <span className="flex-1 break-all">{value}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// Extract the oldest timestamp (in nanoseconds) from a Loki streams response.
// Loki returns entries sorted newest-first when direction=backward,
// so the oldest entry is the LAST one in the list.
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

interface FilterState {
  method: string | null
  statusCode: string | null
  searchPath: string | null
  searchIP: string | null
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
  const [timeRange, setTimeRange] = useState(TIME_RANGES[1]) // 1h default
  const [logType, setLogType] = useState<'access' | 'error'>('access')
  const [filters, setFilters] = useState<FilterState>({
    method: null,
    statusCode: null,
    searchPath: null,
    searchIP: null,
  })

  // Cursor-based pagination state
  // pages[0] = newest page (top of list), pages[1] = older, etc.
  const [pages, setPages] = useState<LogPage[]>([])
  const [olderCursor, setOlderCursor] = useState<bigint | null>(null)
  const [isLoadingOlder, setIsLoadingOlder] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  // The window start is captured once when the initial fetch runs so that
  // "load older" pages use exactly the same start boundary — not a freshly
  // recomputed one that would silently shrink the window as time passes.
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
      .filter((item) => item.enabled && !item.is_shared_log)
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

  // log_kind is structured metadata — must be a pipeline filter (| log_kind="access"),
  // NOT inside the stream selector {}, which only works for indexed labels.
  const lokiFilter = selectedDomain
    ? `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access" | vhost="${escapeLokiLabelValue(selectedDomain)}"`
    : `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access"`

  // ── Error logs query (only active when logType === 'error') ───────────────
  // logType is part of the queryKey so switching to the error tab always
  // triggers a fresh fetch rather than hitting a stale cache entry.
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

  // ── Initial / live-refresh query ───────────────────────────────────────────
  // Fetches the NEWEST PAGE_SIZE logs. direction=backward means Loki returns
  // newest first — so the first entry in the response is the most recent log.
  const { isLoading: initialLoading, refetch } = useQuery({
    queryKey: [`nginx-logs-initial-${id}`, selectedDomain, timeRange.seconds],
    queryFn: async () => {
      // Compute the window boundaries fresh on every fetch (auto-refresh).
      // These are captured as nanosecond strings using BigInt so the boundary
      // is exact — no float64 precision loss at the 19-digit nanosecond scale.
      const nowNs = secondsToNanos(getNowSeconds())
      const startNs = secondsToNanos(getStartSeconds(timeRange.seconds))

      // Persist startNs so loadOlder uses the same boundary, not a later "now".
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

      // parseLogLines returns them in the order Loki gave them (newest first with backward)
      const rawLines = parseLogLines(res.data)
      const oldest = extractOldestNsFromLokiResponse(res.data)

      const newPage: LogPage = {
        rawLines, // already newest-first
        parsed: rawLines.map(parseNginxLogLine),
        oldestNs: oldest,
      }

      // Reset all pages on refresh or time-range change
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

  // ── Load older (cursor-based) ──────────────────────────────────────────────
  const loadOlder = useCallback(async () => {
    if (!olderCursor || isLoadingOlder || !windowStartNs) return
    setIsLoadingOlder(true)
    try {
      // Use the window start that was captured when the initial fetch ran.
      // Re-computing getStartSeconds() here would produce a later timestamp
      // (because time has passed), silently cutting off earlier entries.
      const res = await apiClient.get('/api/query/logs', {
        params: {
          query: lokiFilter,
          limit: PAGE_SIZE,
          start: windowStartNs,
          end: String(olderCursor), // end = cursor means "up to but not including the last seen"
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
      // Append older page at the END of the pages array (it goes below current entries)
      setPages((prev) => [...prev, newPage])
      setOlderCursor(oldest !== null ? oldest - 1n : null)
      setHasMore(rawLines.length === PAGE_SIZE)
    } finally {
      setIsLoadingOlder(false)
    }
  }, [olderCursor, isLoadingOlder, lokiFilter, timeRange.seconds, windowStartNs])

  // ── Flatten pages for rendering ────────────────────────────────────────────
  // pages[0] = newest (top), pages[1] = older (below), etc.
  // Within each page, rawLines are newest-first (from Loki direction=backward).
  // So the final flat array is correctly sorted newest → oldest from top to bottom.
  const allEntries = pages.flatMap((page, pageIdx) =>
    page.rawLines.map((log, lineIdx) => ({
      log,
      parsed: page.parsed[lineIdx],
      globalIdx: pageIdx * PAGE_SIZE + lineIdx,
    })),
  )

  // Client-side filter.
  // When no filters are active, all parseable entries are shown.
  // Unparseable lines (parsed === null) are always hidden from the log list
  // because we can't filter or display them meaningfully — but they should
  // not inflate the "Loaded" counter either.
  const filteredEntries = allEntries.filter(({ parsed }) => {
    if (!parsed) return false // skip lines that couldn't be parsed regardless
    if (filters.method && parsed.method !== filters.method) return false
    if (filters.statusCode && parsed.status.toString() !== filters.statusCode) return false
    if (filters.searchPath && !parsed.path.includes(filters.searchPath)) return false
    if (filters.searchIP && !parsed.ip.includes(filters.searchIP)) return false
    return true
  })

  // Count only parseable entries so "Loaded" matches what's visible when
  // no filters are active — eliminates the confusing "Loaded 12 / Showing 11" mismatch.
  const totalLoaded = allEntries.filter(({ parsed }) => parsed !== null).length

  // Dropdown options from all loaded pages
  const allParsed = allEntries.map((e) => e.parsed).filter((p): p is LogEntryParsed => p !== null)
  const uniqueMethods  = Array.from(new Set(allParsed.map((p) => p.method))).sort()
  const uniqueStatuses = Array.from(new Set(allParsed.map((p) => p.status))).sort((a, b) => a - b)

  // ── Guard renders ─────────────────────────────────────────────────────────
  if (!id) return <div className="p-6 text-sm text-red-600">Missing service id in path.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading service details...</div>
  if (!serviceQuery.data) return <div className="p-6 text-sm text-red-600">Nginx service not found for id {id}.</div>

  return (
    <div className="space-y-6 p-6">
      {/* Header */}
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
            to={buildDomainHref(`/integrations/nginx-dashboard/${id}`, selectedDomain || null)}
            className="inline-block rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-500"
          >
            ← Back to Dashboard
          </Link>
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-block rounded-md bg-muted px-4 py-2 text-sm font-semibold text-foreground hover:bg-muted/80"
          >
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

      {/* Log type toggle — Access Logs vs Error Logs */}
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

      {/* ── ERROR LOGS VIEW ─────────────────────────────────────────────────── */}
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

      {/* ── ACCESS LOGS VIEW (hidden when error tab active) ─────────────────── */}
      {logType === 'access' && (
        <>
      <div className="flex gap-4 flex-wrap">
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Loaded</p>
          <p className="mt-1 text-2xl font-bold text-primary">{totalLoaded}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Matching Filters</p>
          <p className="mt-1 text-2xl font-bold text-blue-500">{filteredEntries.length}</p>
        </div>
        <div className="rounded-lg border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">Pages loaded</p>
          <p className="mt-1 text-2xl font-bold">{pages.length}</p>
        </div>
        <div className="rounded-lg border border-border bg-card flex-1 p-4">
          {initialLoading ? (
            <p className="text-sm text-muted-foreground">Updating logs...</p>
          ) : (
            <p className="text-xs text-muted-foreground">
              Last updated {new Date().toLocaleTimeString()} · auto-refresh 15s
            </p>
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
              {uniqueMethods.map((m) => (
                <option key={m} value={m}>{m}</option>
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
              {uniqueStatuses.map((s) => (
                <option key={s} value={s}>{s}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-2">Search Path</label>
            <input
              type="text"
              value={filters.searchPath || ''}
              onChange={(e) => setFilters({ ...filters, searchPath: e.target.value || null })}
              placeholder="e.g., /api, /home, /admin"
              className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-muted-foreground mb-2">Search IP</label>
            <input
              type="text"
              value={filters.searchIP || ''}
              onChange={(e) => setFilters({ ...filters, searchIP: e.target.value || null })}
              placeholder="e.g., 192.168.1"
              className="w-full rounded-md border border-border bg-muted px-3 py-2 text-sm font-mono"
            />
          </div>
        </div>
        <button
          type="button"
          onClick={() => setFilters({ method: null, statusCode: null, searchPath: null, searchIP: null })}
          className="mt-4 text-sm text-blue-600 hover:text-blue-500"
        >
          Clear Filters
        </button>
      </div>

      {/* Log list */}
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
              <LogViewerRow
                key={globalIdx}
                log={log}
                parsed={parsed}
                expanded={expanded === globalIdx}
                onToggle={() => setExpanded(expanded === globalIdx ? null : globalIdx)}
              />
            ))
          )}
        </div>

        {/* Pagination footer */}
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

      {/* Tips */}
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
