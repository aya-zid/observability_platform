// src/pages/services/ApacheLogsPage.tsx
//
// CHANGES in this version:
// - log_kind="access" is now always included in the Loki stream selector so that
//   apache error logs (which have a completely different format) never appear in
//   the access log viewer and do not corrupt "Loaded" counts.
// - New "Error Logs" tab per domain: queries /api/apache/:id/error-logs and
//   displays raw unstructured error log lines in a plain text viewer.
// - Cursor-based pagination and auto-refresh unchanged.

import { useState, useCallback, useEffect, useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link, useSearchParams } from 'react-router-dom'
import apiClient from '@/api/client'
import { getDiscoveryService } from '@/api/discovery'
import { getApacheDomains, getApacheErrorLogs } from '@/api/apache'
import {
  parseLogLines,
  parseNginxLogLine as parseLogLine,
  getNowSeconds,
  getStartSeconds,
  getStep,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'
import { RefreshCw } from 'lucide-react'
import { ApacheLogViewerRow } from './components/ApacheLogViewerRow'
import type { LogEntryParsed } from './components/ApacheLogViewerRow'
import { ApacheLogFilters } from './components/ApacheLogFilters'
import type { FilterState } from './components/ApacheLogFilters'
import { ApacheLogStatsCards } from './components/ApacheLogStatsCards'

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
    <div className="flex gap-0.5 rounded-md border border-border bg-muted p-0.5">
      {TIME_RANGES.map((range) => (
        <button
          key={range.label}
          onClick={() => onChange(range)}
          className={cn(
            'px-2 py-0.5 rounded text-xs font-medium transition-colors',
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

function buildDomainHref(basePath: string, domain: string | null) {
  if (!domain) {
    return basePath
  }
  return `${basePath}?domain=${encodeURIComponent(domain)}`
}

function escapeLokiLabelValue(value: string) {
  return value.replaceAll('\\', '\\\\').replaceAll('"', '\\"')
}

export function ApacheLogsPage() {
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

  const serviceName = serviceQuery.data?.service_name ?? 'apache'
  const serviceReady = Boolean(id && serviceQuery.data?.service_name)
  const domainsQuery = useQuery({
    queryKey: ['apache-domains', id],
    queryFn: () => getApacheDomains(id),
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

  // log_kind is structured metadata — must be a pipeline filter (| log_kind="access"),
  // NOT inside the stream selector {}, which only works for indexed labels.
  const lokiFilter = selectedDomain
    ? `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access" | vhost="${escapeLokiLabelValue(selectedDomain)}"`
    : `{service_name="${escapeLokiLabelValue(serviceName)}"} | log_kind="access"`

  // ── Error logs query (only active when logType === 'error') ───────────────
  // logType is part of the queryKey so switching to the error tab always
  // triggers a fresh fetch rather than hitting a stale cache entry.
  const errorLogsQuery = useQuery({
    queryKey: [`apache-error-logs-${id}`, selectedDomain, timeRange.seconds, logType],
    queryFn: async () => {
      const nowNs = secondsToNanos(getNowSeconds())
      const startNs = secondsToNanos(getStartSeconds(timeRange.seconds))
      return getApacheErrorLogs(id, {
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
  const { isLoading: initialLoading, refetch, isRefetching } = useQuery({
    queryKey: [`apache-logs-initial-${id}`, selectedDomain, timeRange.seconds],
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
        parsed: rawLines.map(parseLogLine),
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
        parsed: rawLines.map(parseLogLine),
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
  if (!serviceQuery.data) return <div className="p-6 text-sm text-red-600">Apache service not found for id {id}.</div>

  return (
    <div className="space-y-3 p-4 md:p-6">
      <div className="flex items-center justify-between flex-wrap gap-2">
        <div className="flex items-center gap-2">
          <h1 className="text-lg font-bold">Apache Logs</h1>
          <span className="text-xs text-muted-foreground">· {serviceName}</span>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <TimeRangeSelector selected={timeRange} onChange={(r) => {
            setTimeRange(r)
            setPages([])
            setOlderCursor(null)
            setWindowStartNs(null)
            setHasMore(true)
          }} />
          <Link
            to={buildDomainHref(`/services/apache-dashboard/${id}`, selectedDomain || null)}
            className="rounded-md bg-primary px-2.5 py-1 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
          >
            Dashboard
          </Link>
          <button
            type="button"
            onClick={() => refetch()}
            className="inline-flex items-center gap-1 rounded-md bg-muted px-2.5 py-1 text-xs font-semibold text-foreground hover:bg-muted/80"
          >
            <RefreshCw className={`h-3 w-3 ${isRefetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {domains.length > 0 ? (
        <div className="flex flex-wrap gap-1">
          {domains.map((domain) => {
            const active = domain === selectedDomain
            return (
              <button
                key={domain}
                type="button"
                onClick={() => setSearchParams({ domain })}
                className={cn(
                  'rounded-md border px-2 py-0.5 text-xs font-medium transition-colors',
                  active
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-border bg-muted/50 text-muted-foreground hover:text-foreground',
                )}
              >
                <span className="font-mono">{domain}</span>
              </button>
            )
          })}
        </div>
      ) : domainsQuery.isLoading ? (
        <p className="text-xs text-muted-foreground">Loading domains...</p>
      ) : (
        <p className="text-xs text-muted-foreground">No enabled Apache domains found yet.</p>
      )}

      <div className="flex items-center gap-1.5">
        <span className="text-xs text-muted-foreground">Log type:</span>
        <div className="flex gap-0.5 rounded-md border border-border bg-muted p-0.5">
          {(['access', 'error'] as const).map((type) => (
            <button
              key={type}
              type="button"
              onClick={() => setLogType(type)}
              className={cn(
                'px-2.5 py-0.5 rounded text-xs font-medium transition-colors capitalize',
                logType === type
                  ? type === 'error'
                    ? 'bg-red-600 text-white'
                    : 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {type === 'access' ? 'Access Logs' : 'Error Logs'}
            </button>
          ))}
        </div>
      </div>

      {logType === 'error' && (
        <div className="space-y-2">
          <p className="text-xs text-muted-foreground">
            Raw error log lines — newest first · auto-refresh 30s · up to 200 lines.
          </p>
          {errorLogsQuery.isFetching ? (
            <div className="flex items-center justify-center py-6 text-xs text-muted-foreground">
              Loading error logs...
            </div>
          ) : errorLogsQuery.isError ? (
            <div className="rounded-lg border border-red-300 bg-red-50 p-3 text-xs text-red-700">
              Failed to load error logs. The error log collection may not be enabled for this domain yet.
            </div>
          ) : !errorLogsQuery.data?.lines?.length ? (
            <div className="flex items-center justify-center py-6 text-xs text-muted-foreground">
              No error logs found for this time range.
            </div>
          ) : (
            <div className="rounded-lg border border-border bg-card overflow-hidden">
              <div className="border-b border-border px-3 py-1.5 flex items-center justify-between">
                <span className="text-xs font-semibold">
                  Error Log Lines ({errorLogsQuery.data.lines.length})
                </span>
              </div>
              <div className="max-h-[400px] overflow-y-auto">
                {errorLogsQuery.data.lines.map((line, idx) => (
                  <div
                    key={idx}
                    className={cn(
                      'border-b border-border/50 px-3 py-1 font-mono text-xs leading-relaxed',
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
          <ApacheLogStatsCards
            totalLoaded={totalLoaded}
            filteredEntriesLength={filteredEntries.length}
            pagesLength={pages.length}
            initialLoading={initialLoading}
          />

          <ApacheLogFilters
            filters={filters}
            uniqueMethods={uniqueMethods}
            uniqueStatuses={uniqueStatuses}
            onFilterChange={setFilters}
          />

          <div className="rounded-lg border border-border bg-card">
            <div className="border-b border-b-border px-3 py-1.5 flex items-center justify-between">
              <span className="text-xs font-semibold">
                Log Entries ({filteredEntries.length} of {totalLoaded} loaded)
              </span>
              {pages.length > 1 && (
                <span className="text-xs text-muted-foreground">{pages.length} pages × {PAGE_SIZE} entries</span>
              )}
            </div>

            <div className="space-y-0 max-h-[500px] overflow-y-auto">
              {initialLoading ? (
                <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
                  Loading logs...
                </div>
              ) : filteredEntries.length === 0 ? (
                <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
                  No logs match the selected filters
                </div>
              ) : (
                filteredEntries.map(({ parsed, log, globalIdx }) => (
                  <ApacheLogViewerRow
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
              <div className="border-t border-t-border px-3 py-1.5 flex items-center justify-between">
                <span className="text-xs text-muted-foreground">
                  {hasMore
                    ? `Showing ${totalLoaded} most recent entries.`
                    : `All ${totalLoaded} entries loaded.`}
                </span>
                {hasMore && olderCursor !== null && (
                  <button
                    type="button"
                    onClick={loadOlder}
                    disabled={isLoadingOlder}
                    className="rounded-md bg-muted px-2.5 py-1 text-xs font-semibold hover:bg-muted/80 disabled:opacity-50"
                  >
                    {isLoadingOlder ? 'Loading...' : `Load ${PAGE_SIZE} older`}
                  </button>
                )}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  )
}
