// src/pages/integrations/NginxDashboardPage.tsx
//
// UI: IDENTICAL to the original — every chart, every table, every stat card preserved.
// Only change: all data comes from ONE call to GET /api/nginx/:id/dashboard
// instead of 22 separate Loki/VM queries from the browser.
//
// The backend (nginx_dashboard_service.go) runs all queries in parallel,
// caches for 20 s in Redis, returns a single NginxDashboardSummary JSON.
// The frontend maps that JSON onto the same components as before.

import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { getDiscoveryService } from '@/api/discovery'
import { getNginxDashboardSummary, getNginxDomains, getNginxRTDistribution } from '@/api/nginx'
import type { NginxDashboardSummary, NginxDomainItem, NginxRTDistribution, NginxTimePoint } from '@/api/nginx'
import {
  formatBytes,
  formatNumber,
  formatPercent,
  dateTimeInputToSeconds,
  formatDateInputValue,
  formatTimeInputValue,
  getStatusBadgeClass,
  getNowSeconds,
  getStartSeconds,
  getStep,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'
import { RefreshCw } from 'lucide-react'
import { methodBadgeColor } from '@/lib/methodBadgeColor'
import { NginxSlowRequestTable } from '@/components/nginx/NginxSlowRequestTable'
import { IPInvestigationBlock } from '@/components/nginx/IPInvestigationBlock'
import { NginxPerformanceSection, NginxRTDistributionChart } from '@/components/nginx/NginxPerformanceSection'
import { StatCard } from '@/components/shared/StatCard'
import { ChartBox } from '@/components/shared/ChartBox'
import { DataTable } from '@/components/shared/DataTable'
import {
  TimeRangeSelector,
  TIME_RANGES,
  type TimeSelection,
} from '@/components/shared/TimeRangeSelector'

const STATUS_SUMMARY_STYLES = {
  green: {
    card: 'border-green-200 bg-green-50/70 dark:border-green-900/40 dark:bg-green-950/20',
    text: 'text-green-700 dark:text-green-400',
  },
  blue: {
    card: 'border-blue-200 bg-blue-50/70 dark:border-blue-900/40 dark:bg-blue-950/20',
    text: 'text-blue-700 dark:text-blue-400',
  },
  amber: {
    card: 'border-amber-200 bg-amber-50/70 dark:border-amber-900/40 dark:bg-amber-950/20',
    text: 'text-amber-700 dark:text-amber-400',
  },
  red: {
    card: 'border-red-200 bg-red-50/70 dark:border-red-900/40 dark:bg-red-950/20',
    text: 'text-red-700 dark:text-red-400',
  },
} as const

// Status colors — shared with IPInvestigationBlock
const STATUS_COLORS = {
  '2xx': { hex: '#22c55e' },
  '3xx': { hex: '#3b82f6' },
  '4xx': { hex: '#f59e0b' },
  '5xx': { hex: '#ef4444' },
} as const

const DEFAULT_RT_PERCENTILES = [50, 95, 99]
const CUSTOM_RT_COLORS = [
  '#06b6d4',
  '#8b5cf6',
  '#ec4899',
  '#14b8a6',
  '#f97316',
  '#6366f1',
  '#84cc16',
  '#d946ef',
  '#0ea5e9',
  '#a855f7',
]
const DEFAULT_RT_COLORS: Record<string, string> = {
  p50: '#22c55e',
  p95: '#f59e0b',
  p99: '#ef4444',
}

type DashboardViewMode = 'server' | 'domain'

function buildDomainHref(basePath: string, domain: string | null) {
  if (!domain) {
    return basePath
  }
  return `${basePath}?domain=${encodeURIComponent(domain)}`
}

function buildSlowRequestsHref({
  id,
  domain,
  start,
  end,
  minRT,
}: {
  id: string
  domain: string
  start: number
  end: number
  minRT: string
}) {
  const params = new URLSearchParams({
    start: secondsToNanos(start),
    end: secondsToNanos(end),
    min_rt: minRT,
  })
  if (domain) {
    params.set('domain', domain)
  }
  return `/integrations/nginx-dashboard/${id}/slow-requests?${params.toString()}`
}

// ─── Chart styling (polished, precise tooltips) ──────────────────────────────

const chartStyle = {
  grid: 'hsl(var(--border))',
  tick: { fill: '#94a3b8', fontSize: 10 },
  tooltip: {
    background: 'hsl(var(--card))',
    border: '1px solid hsl(var(--border))',
    borderRadius: '6px',
    padding: '8px',
    fontSize: '12px',
    color: 'hsl(var(--foreground))',
  },
} as const

// ─── Helpers ──────────────────────────────────────────────────────────────────

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === 'preset') {
    const end = getNowSeconds()
    const start = getStartSeconds(selection.preset.seconds)
    return {
      start,
      end,
      seconds: selection.preset.seconds,
      step: getStep(selection.preset.seconds),
      isManual: false,
      error: '',
    }
  }

  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start === null || end === null) {
    return { start: 0, end: 0, seconds: 0, step: '30s', isManual: true, error: 'Use DD-MM-YYYY and HH:mm.' }
  }
  if (end <= start) {
    return { start, end, seconds: 0, step: '30s', isManual: true, error: 'End time must be after start time.' }
  }

  const seconds = end - start
  return {
    start,
    end,
    seconds,
    step: getStep(seconds),
    isManual: true,
    error: '',
  }
}

function formatChartTime(unixSeconds: number) {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function readNullableNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

// Convert backend TimePoint[] (time = unix seconds) to recharts rows.
function mapTimeseries(points: NginxTimePoint[]): Record<string, number | string>[] {
  return (points ?? []).map((pt) => {
    const row: Record<string, number | string> = {}
    for (const [k, v] of Object.entries(pt)) {
      row[k] = k === 'time' && typeof v === 'number' ? formatChartTime(v) : (v ?? 0)
    }
    return row
  })
}

function mapRTTimeseries(points: NginxTimePoint[]): Array<{
  time: string
  [key: string]: number | string | null
}> {
  return (points ?? []).map((pt) => {
    const row: { time: string; [key: string]: number | string | null } = {
      time: formatChartTime(typeof pt.time === 'number' ? pt.time : 0),
    }
    for (const [key, value] of Object.entries(pt)) {
      if (key === 'time') continue
      row[key] = readNullableNumber(value)
    }
    return row
  })
}

// Get the latest value of a named key from a timeseries array.
function latestFromSeries(points: NginxTimePoint[], key: string): number | null {
  if (!points || points.length === 0) return null
  const last = points[points.length - 1]
  return readNullableNumber(last[key])
}

function normalizePercentileInput(value: string) {
  const parsed = Number.parseInt(value.trim().replace(/^p/i, ''), 10)
  if (!Number.isFinite(parsed) || parsed <= 0 || parsed >= 100) {
    return null
  }
  return parsed
}


// ─── Main page ────────────────────────────────────────────────────────────────

export function NginxDashboardPage() {
  const { id = '' } = useParams<{ id?: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({
    mode: 'preset',
    preset: TIME_RANGES[1],
  })
  const [slowThreshold, setSlowThreshold] = useState(searchParams.get('min_rt') || '1')
  const [rtPercentileInput, setRtPercentileInput] = useState('')
  const [customRTPercentiles, setCustomRTPercentiles] = useState<number[]>([])

  const serviceQuery = useQuery({
    queryKey: ['discovery-service', id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const serviceName = serviceQuery.data?.service_name ?? 'nginx'
  const hostId = serviceQuery.data?.machine_id ?? ''
  const serviceReady = Boolean(id && serviceQuery.data?.service_name)
  const configuredMetricsEndpoint =
    serviceQuery.data?.user_metrics_endpoint || serviceQuery.data?.suggested_metrics_endpoint || ''

  const domainsQuery = useQuery<NginxDomainItem[]>({
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
  const viewMode: DashboardViewMode = searchParams.get('view') === 'server' ? 'server' : 'domain'
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
      const nextParams = new URLSearchParams(searchParams)
      nextParams.set('domain', selectedDomain)
      setSearchParams(nextParams, { replace: true })
    }
  }, [domains.length, requestedDomain, searchParams, selectedDomain, setSearchParams])

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])
  const slowThresholdNumber = Number.parseFloat(slowThreshold)
  const slowMinRT = Number.isFinite(slowThresholdNumber) && slowThresholdNumber > 0 ? slowThresholdNumber : 1
  const rtPercentilesParam = customRTPercentiles.join(',')

  // ── Single backend call replacing all 22 frontend queries ─────────────────
  const dashQuery = useQuery<NginxDashboardSummary>({
    // Include the actual epoch minute in the key so that each auto-refresh
    // cycle creates a new cache entry aligned to real time, rather than
    // reusing a stale key whose internal timestamps have drifted.
    queryKey: [
      'nginx-dashboard-summary',
      id,
      serviceName,
      selectedDomain,
      timeSelection,
      timeWindow.start,
      timeWindow.end,
      slowMinRT,
      rtPercentilesParam,
    ],
    enabled: serviceReady && (domains.length > 0 || domainsQuery.isSuccess) && !timeWindow.error,
    queryFn: () => {
      return getNginxDashboardSummary(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        step: timeWindow.step,
        domain: selectedDomain || undefined,
        min_rt: String(slowMinRT),
        percentiles: rtPercentilesParam || undefined,
      })
    },
    placeholderData: (previousData) => previousData,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 25_000, // slightly above the 20 s Redis cache TTL on backend
    refetchOnWindowFocus: false,
  })

  const rtDistributionQuery = useQuery<NginxRTDistribution>({
    queryKey: [
      'nginx-dashboard-rt-distribution',
      id,
      selectedDomain,
      timeSelection,
      timeWindow.start,
      timeWindow.end,
    ],
    enabled: serviceReady && (domains.length > 0 || domainsQuery.isSuccess) && !timeWindow.error,
    queryFn: () => {
      return getNginxRTDistribution(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        domain: selectedDomain || undefined,
      })
    },
    placeholderData: (previousData) => previousData,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 25_000,
    refetchOnWindowFocus: false,
  })

  const d = dashQuery.data
  const isLoading = dashQuery.isLoading

  // ── Scalars ───────────────────────────────────────────────────────────────
  const totalRequests = d?.total_requests ?? 0
  const total2xx = d?.total_2xx ?? 0
  const total3xx = d?.total_3xx ?? 0
  const total4xx = d?.total_4xx ?? 0
  const total5xx = d?.total_5xx ?? 0

  const successPct  = totalRequests > 0 ? (total2xx / totalRequests) * 100 : 0
  const redirectPct = totalRequests > 0 ? (total3xx / totalRequests) * 100 : 0
  const rate4xxPct  = totalRequests > 0 ? (total4xx / totalRequests) * 100 : 0
  const rate5xxPct  = totalRequests > 0 ? (total5xx / totalRequests) * 100 : 0

  const aggTotalsLabel = totalRequests > 0
    ? `${formatNumber(Math.round(totalRequests))} total requests in range`
    : 'Full selected range'

  // Connection stat cards from backend scalars
  const activeConnectionsLatest  = d?.conn_active  ?? null
  const waitingConnectionsLatest = d?.conn_waiting ?? null
  const readingConnectionsLatest = d?.conn_reading ?? null
  const writingConnectionsLatest = d?.conn_writing ?? null

  // VM counters (latest scalar values)
  const requestsLatest = d?.vm_requests_total  ?? null
  const acceptedLatest = d?.vm_accepted_total  ?? null
  const handledLatest  = d?.vm_handled_total   ?? null

  // Total connections = sum of all connection states (latest point)
  const connPts = d?.conn_timeseries ?? []
  const totalConnectionsLatest = connPts.length > 0
    ? (['active', 'waiting', 'reading', 'writing'] as const)
        .reduce((s, k) => s + (typeof connPts[connPts.length - 1][k] === 'number' ? (connPts[connPts.length - 1][k] as number) : 0), 0)
    : null

  // P95 latest for the stat card (already in ms from backend)
  const p95LatestMs = latestFromSeries(d?.rt_timeseries ?? [], 'p95')

  // ── Chart data ─────────────────────────────────────────────────────────────
  const statusData   = mapTimeseries(d?.status_timeseries ?? [])
  const rtData       = mapRTTimeseries(d?.rt_timeseries ?? [])
  const reqRateData  = mapTimeseries(d?.req_rate_timeseries ?? [])
  const bandwidthData = mapTimeseries(d?.bandwidth_timeseries ?? [])
  const connData     = mapTimeseries(d?.conn_timeseries ?? [])
  const totalConnData = mapTimeseries(d?.vm_total_conn_timeseries ?? [])
  const requestsData  = mapTimeseries(d?.vm_requests_timeseries ?? [])
  const botRawData   = mapTimeseries(d?.bot_timeseries ?? [])

  // Bot trend: need humanRate derived on client (same as original)
  const botTrendData = botRawData.map((row) => {
    const totalRate = typeof row.totalRate === 'number' ? row.totalRate : 0
    const botRate   = typeof row.botRate   === 'number' ? row.botRate   : 0
    return { ...row, humanRate: Math.max(0, totalRate - botRate) }
  })

  const lastBotRow   = botRawData[botRawData.length - 1]
  const rawBot   = lastBotRow && typeof lastBotRow.botRate   === 'number' ? lastBotRow.botRate   : 0
  const rawTotal = lastBotRow && typeof lastBotRow.totalRate === 'number' ? lastBotRow.totalRate : 0
  const botPct   = rawTotal > 0 ? Math.min(100, (rawBot / rawTotal) * 100) : 0
  const humanPct = 100 - botPct

  const rtPercentiles = [...DEFAULT_RT_PERCENTILES, ...customRTPercentiles]
  const rtPercentileKeys = rtPercentiles.map((p) => `p${p}`)
  const rtChartTitle = `Response Time ${rtPercentileKeys.map((key) => key.toUpperCase()).join(' / ')} (ms)`

  function addRTPercentile() {
    const percentile = normalizePercentileInput(rtPercentileInput)
    if (percentile === null) {
      return
    }
    if (DEFAULT_RT_PERCENTILES.includes(percentile) || customRTPercentiles.includes(percentile)) {
      setRtPercentileInput('')
      return
    }
    if (customRTPercentiles.length >= 10) {
      return
    }
    setCustomRTPercentiles((current) => [...current, percentile].sort((a, b) => a - b))
    setRtPercentileInput('')
  }

  // Slow requests (backend sends top 100 directly)
  const slowRequests = d?.slow_requests ?? []
  const slowRequestsTotal = d?.slow_requests_total ?? 0
  const slowRequestsNormalizedTotal = d?.slow_requests_normalized_total ?? 0
  const slowRequestsHref = buildSlowRequestsHref({
    id,
    domain: selectedDomain,
    start: timeWindow.start,
    end: timeWindow.end,
    minRT: String(slowMinRT),
  })

  // Aggregation tables
  const aggTopPaths            = d?.top_paths            ?? []
  const aggTopStatuses         = d?.top_statuses        ?? []
  const aggTopIPs              = d?.top_ips             ?? []
  const aggMethods             = d?.top_methods         ?? []
  const aggMethodTotal         = aggMethods.reduce((s, m) => s + m.count, 0)
  const aggTopErrorPaths       = d?.top_error_paths     ?? []
  const aggSuspiciousIPs       = d?.suspicious_ips      ?? []
  // Normalized path aggregations
  const aggTopNormalizedPaths      = d?.top_normalized_paths      ?? []
  const aggTopNormalizedErrorPaths = d?.top_normalized_error_paths ?? []
  const slowRequestsNormalized     = d?.slow_requests_normalized   ?? []

  // ── Guard renders ─────────────────────────────────────────────────────────
  if (!id) return <div className="p-6 text-sm text-red-600">Missing service id.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading...</div>
  if (!serviceQuery.data) return <div className="p-6 text-sm text-red-600">Service not found for id {id}.</div>

  // ── Render — identical structure to original ───────────────────────────────
  return (
    <div className="space-y-6 p-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3 pb-2">
        <div>
          <h1 className="text-3xl font-bold">Nginx Dashboard</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Service: <span className="font-mono font-semibold">{serviceName}</span>
            {hostId ? (
              <>
                {' '}• Host: <span className="font-mono font-semibold">{hostId}</span>
              </>
            ) : null}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => dashQuery.refetch()}
            className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
            title="Refresh"
          >
            <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
          </button>
          <TimeRangeSelector selected={timeSelection} onChange={setTimeSelection} />
          <Link
            to={buildDomainHref(`/integrations/nginx-logs/${id}`, selectedDomain || null)}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-500 transition-colors"
          >
            View Logs →
          </Link>
        </div>
      </div>

      {timeWindow.error ? (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm font-medium text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">
          {timeWindow.error}
        </div>
      ) : null}

      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex flex-col gap-3">
          <div>
            <p className="text-sm font-semibold">Domains</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Same dashboard, same metrics and widgets. Switch tabs to isolate one Nginx domain at a time.
            </p>
          </div>
          {domainsQuery.isLoading ? (
            <p className="text-sm text-muted-foreground">Loading domains...</p>
          ) : domains.length === 0 ? (
            <p className="text-sm text-muted-foreground">No enabled Nginx domains found yet.</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => {
                  const nextParams = new URLSearchParams(searchParams)
                  nextParams.set('view', 'server')
                  if (selectedDomain) {
                    nextParams.set('domain', selectedDomain)
                  }
                  setSearchParams(nextParams)
                }}
                className={cn(
                  'rounded-lg border px-3 py-2 text-sm font-medium transition-colors',
                  viewMode === 'server'
                    ? 'border-slate-900 bg-slate-900 text-white dark:border-slate-100 dark:bg-slate-100 dark:text-slate-900'
                    : 'border-border bg-muted/50 text-foreground hover:bg-muted',
                )}
              >
                Server Metrics
              </button>
              {domains.map((domain) => {
                const active = viewMode === 'domain' && domain === selectedDomain
                return (
                  <button
                    key={domain}
                    type="button"
                    onClick={() => {
                      const nextParams = new URLSearchParams(searchParams)
                      nextParams.set('view', 'domain')
                      nextParams.set('domain', domain)
                      setSearchParams(nextParams)
                    }}
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
          {viewMode === 'server' ? (
            <>
              Viewing nginx-wide status metrics. Domain logs remain available under{' '}
              <span className="font-mono font-semibold text-foreground">{selectedDomain}</span>
              .
            </>
          ) : (
            <>
              Active domain: <span className="font-mono font-semibold text-foreground">{selectedDomain}</span>
            </>
          )}
        </div>
      ) : null}

      {/* Metrics source info banner */}
      <div className="rounded-xl border border-border bg-card p-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p className="text-sm font-semibold">Metrics source</p>
            <p className="mt-1 text-xs text-muted-foreground">
              The integration script scrapes the endpoint saved for this service in the database. That saved endpoint is client-specific.
            </p>
          </div>
          <div className="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs">
            <span className="text-muted-foreground">Configured endpoint:</span>{' '}
            <span className="font-mono break-all">{configuredMetricsEndpoint || 'Not configured'}</span>
          </div>
        </div>
      </div>

      {viewMode === 'server' ? (
        <>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <StatCard
              title="Active Connections"
              icon="🔌"
              value={activeConnectionsLatest !== null ? formatNumber(activeConnectionsLatest) : '—'}
              loading={isLoading}
            />
            <StatCard
              title="Total Connections"
              icon="🔗"
              value={totalConnectionsLatest !== null ? formatNumber(totalConnectionsLatest) : '—'}
              loading={isLoading}
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <StatCard
              title="Reading"
              icon="📥"
              value={readingConnectionsLatest !== null ? formatNumber(readingConnectionsLatest) : '—'}
              loading={isLoading}
            />
            <StatCard
              title="Writing"
              icon="📤"
              value={writingConnectionsLatest !== null ? formatNumber(writingConnectionsLatest) : '—'}
              loading={isLoading}
            />
            <StatCard
              title="Waiting"
              icon="⏸️"
              value={waitingConnectionsLatest !== null ? formatNumber(waitingConnectionsLatest) : '—'}
              loading={isLoading}
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <StatCard
              title="Accepted"
              icon="✅"
              value={acceptedLatest !== null ? formatNumber(acceptedLatest) : '—'}
              loading={isLoading}
            />
            <StatCard
              title="Handled"
              icon="🛠️"
              value={handledLatest !== null ? formatNumber(handledLatest) : '—'}
              loading={isLoading}
            />
            <StatCard
              title="Total Requests"
              icon="📊"
              value={requestsLatest !== null ? formatNumber(requestsLatest) : '—'}
              loading={isLoading}
            />
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <ChartBox
              title="Active Connections"
              loading={isLoading}
              empty={connData.length === 0}
              hint="Requires the configured nginx status endpoint to be enabled and scraped"
            >
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={connData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                    <Tooltip contentStyle={chartStyle.tooltip} />
                    <Legend wrapperStyle={{ fontSize: '11px' }} />
                    <Line dataKey="active" dot={false} isAnimationActive={false} name="Active" stroke="#2563eb" strokeWidth={2} type="monotone" />
                    <Line dataKey="waiting" dot={false} isAnimationActive={false} name="Waiting" stroke="#94a3b8" strokeWidth={1.5} type="monotone" />
                    <Line dataKey="reading" dot={false} isAnimationActive={false} name="Reading" stroke="#22c55e" strokeWidth={1.5} type="monotone" />
                    <Line dataKey="writing" dot={false} isAnimationActive={false} name="Writing" stroke="#f59e0b" strokeWidth={1.5} type="monotone" />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>

            <ChartBox
              title="Total Connections Over Time"
              loading={isLoading}
              empty={totalConnData.length === 0}
              hint="Sum of all nginx connection states"
            >
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={totalConnData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} />
                    <Tooltip contentStyle={chartStyle.tooltip} />
                    <Line dataKey="total" dot={false} isAnimationActive={false} name="Total" stroke="#2563eb" strokeWidth={2} type="monotone" />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>
          </div>

          <ChartBox
            title="Total Requests Over Time"
            loading={isLoading}
            empty={requestsData.length === 0}
            hint="Cumulative requests served by nginx"
          >
            <div className="h-64">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={requestsData}>
                  <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                  <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                  <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={50} tickFormatter={(v) => formatNumber(v)} />
                  <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? formatNumber(v) : v)} />
                  <Line dataKey="requests" dot={false} isAnimationActive={false} name="Requests" stroke="#22c55e" strokeWidth={2} type="monotone" />
                </LineChart>
              </ResponsiveContainer>
            </div>
          </ChartBox>
        </>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            <StatCard
              title="P95 Response Time"
              icon="⏱️"
              value={p95LatestMs !== null ? `${Math.round(p95LatestMs)}ms` : '—'}
              status={p95LatestMs !== null && p95LatestMs > 500 ? 'red' : p95LatestMs !== null && p95LatestMs > 100 ? 'yellow' : 'green'}
              loading={isLoading}
            />
          </div>

          {/* Data tables section */}
          <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
            <h3 className="mb-1 text-sm font-semibold text-foreground">Response Status Breakdown</h3>
            <p className="mb-4 text-xs text-muted-foreground">
              Full selected range via Loki count queries. These totals are aligned with the current time picker.
            </p>
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
              {[
                { label: '2xx Success', value: total2xx, percent: successPct, tone: 'green' as const },
                { label: '3xx Redirect', value: total3xx, percent: redirectPct, tone: 'blue' as const },
                { label: '4xx Client Error', value: total4xx, percent: rate4xxPct, tone: 'amber' as const },
                { label: '5xx Server Error', value: total5xx, percent: rate5xxPct, tone: 'red' as const },
                { label: 'Total', value: totalRequests, percent: null, tone: 'blue' as const },
              ].map((item) => (
                <div key={item.label} className={cn('rounded-lg border p-4 text-center', STATUS_SUMMARY_STYLES[item.tone].card)}>
                  <p className="text-xs font-medium text-muted-foreground">{item.label}</p>
                  <p className={cn('mt-2 text-2xl font-bold', STATUS_SUMMARY_STYLES[item.tone].text)}>
                    {isLoading ? '—' : totalRequests > 0 || item.label === 'Total' ? formatNumber(Math.round(item.value)) : '—'}
                  </p>
                  {!isLoading && item.percent !== null ? (
                    <p className="text-xs text-muted-foreground">{formatPercent(item.percent)}</p>
                  ) : null}
                </div>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-4">
            <DataTable
              title="Top Paths"
              subtitle={aggTotalsLabel}
              loading={isLoading}
              rows={aggTopPaths.map((item) => ({
                label: item.label,
                value: formatNumber(Math.round(item.count), 0),
                percent: totalRequests > 0 ? (item.count / totalRequests) * 100 : 0,
              }))}
            />
            <DataTable
              title="Top Status Codes"
              subtitle={aggTotalsLabel}
              loading={isLoading}
              rows={aggTopStatuses.map((item) => ({
                label: item.label,
                value: formatNumber(Math.round(item.count), 0),
                percent: totalRequests > 0 ? (item.count / totalRequests) * 100 : 0,
                badgeClass: getStatusBadgeClass(Number(item.label)),
              }))}
            />
            <DataTable
              title="Top Client IPs"
              subtitle={aggTotalsLabel}
              loading={isLoading}
              rows={aggTopIPs.map((item) => ({
                label: item.label,
                value: formatNumber(Math.round(item.count), 0),
                percent: totalRequests > 0 ? (item.count / totalRequests) * 100 : 0,
              }))}
            />
            <DataTable
              title="Top Normalized Paths"
              subtitle={aggTotalsLabel}
              loading={isLoading}
              rows={aggTopNormalizedPaths.map((item) => ({
                label: item.label,
                value: formatNumber(Math.round(item.count), 0),
                percent: totalRequests > 0 ? (item.count / totalRequests) * 100 : 0,
              }))}
            />
          </div>

          {/* Slow Requests — both tables side by side */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <NginxSlowRequestTable
              rows={slowRequests}
              total={slowRequestsTotal}
              minRT={slowMinRT}
              thresholdValue={slowThreshold}
              onThresholdChange={setSlowThreshold}
              onThresholdBlur={() => setSlowThreshold(String(slowMinRT))}
              viewMoreHref={slowRequestsHref}
              loading={isLoading}
            />
            <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
              <h3 className="mb-1 text-sm font-semibold text-foreground">Slow Requests (Normalized Paths)</h3>
              <p className="mb-3 text-xs text-muted-foreground">
                Showing {formatNumber(slowRequestsNormalized.length, 0)} of {formatNumber(slowRequestsNormalizedTotal, 0)} normalized paths above {slowMinRT}s.
              </p>
              <div className="max-h-64 space-y-1 overflow-y-auto">
                {isLoading ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
                ) : slowRequestsNormalized.length === 0 ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">No slow requests</p>
                ) : (
                  slowRequestsNormalized.slice(0, 20).map((req, idx) => {
                    const displayPath = req.normalized_path && req.normalized_path !== req.path
                      ? req.normalized_path
                      : req.normalized_path || req.path
                    const isNormalized = req.normalized_path && req.normalized_path !== req.path
                    return (
                      <div key={idx} className="flex items-center justify-between rounded px-2 py-1.5 hover:bg-muted/50 transition-colors text-xs">
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1">
                            {isNormalized ? (
                              <span className="rounded bg-blue-100 px-1 py-0.5 text-[10px] font-semibold text-blue-700 dark:bg-blue-900/30 dark:text-blue-300 flex-shrink-0">
                                norm
                              </span>
                            ) : null}
                            <span className="truncate font-mono text-xs" title={displayPath}>{displayPath}</span>
                          </div>
                          <div className="text-muted-foreground text-xs mt-0.5">{req.method} • {req.status}</div>
                        </div>
                        <div className={cn(
                          'flex-shrink-0 ml-2 font-semibold tabular-nums',
                          req.rt >= 5 ? 'text-red-500' : req.rt >= 2 ? 'text-amber-500' : 'text-primary',
                        )}>
                          {(req.rt * 1000).toFixed(0)}ms
                        </div>
                      </div>
                    )
                  })
                )}
              </div>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
              <h3 className="mb-1 text-sm font-semibold text-foreground">HTTP Method Distribution</h3>
              <p className="mb-4 text-xs text-muted-foreground">{aggTotalsLabel}</p>
              <div className="space-y-2">
                {isLoading ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
                ) : aggMethods.length === 0 ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
                ) : (
                  aggMethods.map((item) => {
                    const pct = aggMethodTotal > 0 ? (item.count / aggMethodTotal) * 100 : 0
                    return (
                      <div key={item.label} className="flex items-center gap-2">
                        <span
                          className="w-16 rounded px-2 py-1 text-center text-xs font-semibold text-white flex-shrink-0"
                          style={{ background: methodBadgeColor(item.label) }}
                        >
                          {item.label}
                        </span>
                        <div className="h-2 flex-1 rounded-full bg-muted">
                          <div className="h-2 rounded-full bg-primary transition-all duration-300" style={{ width: `${Math.max(3, pct)}%` }} />
                        </div>
                        <span className="w-24 flex-shrink-0 text-right text-xs">
                          <span className="font-semibold">{formatNumber(Math.round(item.count), 0)}</span>
                          <span className="ml-1 text-muted-foreground">{pct.toFixed(1)}%</span>
                        </span>
                      </div>
                    )
                  })
                )}
              </div>
            </div>

            <div className="rounded-lg border border-red-200 bg-red-50 p-5 dark:border-red-900 dark:bg-red-950/20 shadow-sm">
              <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold">IPs with High 4xx/5xx Rate</h3>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {isLoading ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
                ) : aggSuspiciousIPs.length > 0 ? (
                  aggSuspiciousIPs.map((item) => (
                    <div key={item.ip} className="flex items-center justify-between rounded-lg border-l-2 border-red-500 bg-white px-3 py-2 dark:bg-muted/30 transition-colors hover:bg-red-50 dark:hover:bg-red-950/10">
                      <div className="flex items-center gap-2">
                        <span className="rounded bg-red-100 px-2 py-1 text-xs font-semibold text-red-700 dark:bg-red-900/40 dark:text-red-300">{item.rate.toFixed(0)}%</span>
                        <span className="font-mono text-sm">{item.ip}</span>
                      </div>
                      <span className="text-xs text-muted-foreground">
                        {formatNumber(Math.round(item.errors), 0)}/{formatNumber(Math.round(item.total), 0)} errors
                      </span>
                    </div>
                  ))
                ) : (
                  <p className="py-4 text-center text-sm text-muted-foreground">No suspicious IPs found in the selected range.</p>
                )}
              </div>
            </div>
          </div>

          {/* Top Error Paths (raw and normalized) side by side */}
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
              <h3 className="mb-1 text-sm font-semibold text-foreground">Top Paths by Error Rate</h3>
              <p className="mb-4 text-xs text-muted-foreground">{aggTotalsLabel}</p>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {isLoading ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
                ) : aggTopErrorPaths.length > 0 ? (
                  aggTopErrorPaths.map((item, index) => (
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
              <p className="mb-4 text-xs text-muted-foreground">{aggTotalsLabel}</p>
              <div className="max-h-64 space-y-2 overflow-y-auto">
                {isLoading ? (
                  <p className="py-4 text-center text-sm text-muted-foreground">Loading...</p>
                ) : aggTopNormalizedErrorPaths.length > 0 ? (
                  aggTopNormalizedErrorPaths.map((item) => (
                    <div key={item.path} className="flex items-center gap-2 rounded px-2 py-2 text-xs hover:bg-muted/50 transition-colors">
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

          {/* Full-width charts section */}
          <div className="space-y-6">
            {/* Request Rate - full width */}
            <ChartBox title="Request Rate (req/s)" loading={isLoading} empty={reqRateData.length === 0}>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={reqRateData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} tickFormatter={(v) => (v < 0.01 ? v.toFixed(4) : v.toFixed(2))} />
                    <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? v.toFixed(4) : v)} />
                    <Line dataKey="req" dot={false} isAnimationActive={false} name="req/s" stroke="#2563eb" strokeWidth={2} type="monotone" />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>

            {/* Response Time — full width */}
            <ChartBox
              title={rtChartTitle}
              loading={isLoading}
              empty={rtData.length === 0 || !rtData.some((pt) => rtPercentileKeys.some((key) => pt[key] !== null))}
              hint="Requires nginx log_format to include rt=$request_time"
              actions={
                <div className="flex flex-wrap items-center gap-2">
                  {customRTPercentiles.map((percentile, index) => (
                    <button
                      key={percentile}
                      type="button"
                      onClick={() => setCustomRTPercentiles((current) => current.filter((item) => item !== percentile))}
                      className="rounded-md border border-border px-2 py-1 text-xs font-semibold"
                      style={{ color: CUSTOM_RT_COLORS[index % CUSTOM_RT_COLORS.length] }}
                    >
                      P{percentile} x
                    </button>
                  ))}
                  <form
                    className="flex items-center gap-2"
                    onSubmit={(event) => {
                      event.preventDefault()
                      addRTPercentile()
                    }}
                  >
                    <input
                      type="number"
                      min="1"
                      max="99"
                      step="1"
                      value={rtPercentileInput}
                      onChange={(event) => setRtPercentileInput(event.target.value)}
                      placeholder="80"
                      disabled={customRTPercentiles.length >= 10}
                      className="h-8 w-20 rounded-md border border-border bg-background px-2 text-sm text-foreground outline-none focus:border-primary disabled:opacity-50"
                    />
                    <button
                      type="submit"
                      disabled={customRTPercentiles.length >= 10}
                      className="h-8 rounded-md border border-border bg-background px-3 text-xs font-semibold hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
                    >
                      Add
                    </button>
                  </form>
                </div>
              }
            >
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={rtData} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis
                      tick={chartStyle.tick}
                      tickLine={false}
                      axisLine={false}
                      width={52}
                      tickFormatter={(v) => `${Math.round(Number(v))}ms`}
                      domain={['auto', 'auto']}
                    />
                    <Tooltip
                      contentStyle={chartStyle.tooltip}
                      formatter={(v: unknown) =>
                        typeof v === 'number' ? [`${v.toFixed(1)}ms`, ''] : ['—', '']
                      }
                    />
                    <Legend wrapperStyle={{ fontSize: '11px' }} />
                    {rtPercentileKeys.map((key, index) => (
                      <Line
                        key={key}
                        dataKey={key}
                        dot={false}
                        isAnimationActive={false}
                        name={key.toUpperCase()}
                        stroke={DEFAULT_RT_COLORS[key] ?? CUSTOM_RT_COLORS[Math.max(0, index - DEFAULT_RT_PERCENTILES.length) % CUSTOM_RT_COLORS.length]}
                        strokeWidth={2}
                        type="monotone"
                      />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>

            {/* Bandwidth Over Time — full width */}
            <ChartBox title="Bandwidth Over Time" loading={isLoading} empty={bandwidthData.length === 0}>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={bandwidthData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis
                      tick={chartStyle.tick}
                      tickLine={false}
                      axisLine={false}
                      width={50}
                      tickFormatter={(v) => v >= 1048576 ? `${(v / 1048576).toFixed(1)} MB/s` : `${(v / 1024).toFixed(0)} KB/s`}
                    />
                    <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? [`${formatBytes(v)}/s`, 'Bandwidth'] : v)} />
                    <Line dataKey="bytes" dot={false} isAnimationActive={false} name="Bandwidth" stroke="#2563eb" strokeWidth={2} type="monotone" />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>

            {/* HTTP Status Breakdown - full width */}
            <ChartBox
              title="HTTP Status Breakdown Over Time"
              loading={isLoading}
              empty={statusData.length === 0}
            >
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={statusData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis
                      dataKey="time"
                      tick={chartStyle.tick}
                      tickLine={false}
                      axisLine={false}
                    />
                    <YAxis
                      tick={chartStyle.tick}
                      tickLine={false}
                      axisLine={false}
                      width={40}
                      allowDecimals={false}
                    />
                    <Tooltip
                      contentStyle={chartStyle.tooltip}
                      formatter={(v: any, name: any) => [
                        `${Math.round(Number(v))} req/s`,
                        name,
                      ]}
                    />
                    <Legend wrapperStyle={{ fontSize: '11px' }} />

                    <Line
                      type="monotone"
                      dataKey="2xx"
                      stroke={STATUS_COLORS['2xx'].hex}
                      strokeWidth={2.5}
                      dot={{ r: 3, fill: STATUS_COLORS['2xx'].hex }}
                      activeDot={{ r: 5, strokeWidth: 0 }}
                      name="2xx"
                      isAnimationActive={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="3xx"
                      stroke={STATUS_COLORS['3xx'].hex}
                      strokeWidth={2.5}
                      dot={{ r: 3, fill: STATUS_COLORS['3xx'].hex }}
                      activeDot={{ r: 5, strokeWidth: 0 }}
                      name="3xx"
                      isAnimationActive={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="4xx"
                      stroke={STATUS_COLORS['4xx'].hex}
                      strokeWidth={2.5}
                      dot={{ r: 3, fill: STATUS_COLORS['4xx'].hex }}
                      activeDot={{ r: 5, strokeWidth: 0 }}
                      name="4xx"
                      isAnimationActive={false}
                    />
                    <Line
                      type="monotone"
                      dataKey="5xx"
                      stroke={STATUS_COLORS['5xx'].hex}
                      strokeWidth={2.5}
                      dot={{ r: 3, fill: STATUS_COLORS['5xx'].hex }}
                      activeDot={{ r: 5, strokeWidth: 0 }}
                      name="5xx"
                      isAnimationActive={false}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </ChartBox>

            {/* Bot vs Human — full width */}
            <ChartBox
              title="Bot vs Human Request Rate"
              loading={isLoading}
              empty={botTrendData.length === 0}
              hint="Estimated from request strings and user agents in the selected range"
            >
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={botTrendData}>
                    <CartesianGrid stroke={chartStyle.grid} strokeDasharray="3 3" />
                    <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} axisLine={false} />
                    <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} width={40} tickFormatter={(v) => v.toFixed(2)} />
                    <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => (typeof v === 'number' ? `${v.toFixed(3)} req/s` : v)} />
                    <Legend wrapperStyle={{ fontSize: '11px' }} />
                    <Line type="monotone" dataKey="humanRate" stroke="#22c55e" strokeWidth={2.5} dot={{ r: 3, fill: '#22c55e' }} activeDot={{ r: 5, strokeWidth: 0 }} name="Human" isAnimationActive={false} />
                    <Line type="monotone" dataKey="botRate" stroke="#ef4444" strokeWidth={2.5} dot={{ r: 3, fill: '#ef4444' }} activeDot={{ r: 5, strokeWidth: 0 }} name="Bot / Automated" isAnimationActive={false} />
                  </LineChart>
                </ResponsiveContainer>
              </div>
              <div className="mt-3 flex items-center justify-between text-xs text-muted-foreground">
                <span>Human share: {humanPct.toFixed(1)}%</span>
                <span>Automated share: {botPct.toFixed(1)}%</span>
              </div>
            </ChartBox>
          </div>

          <div className="rounded-lg border border-blue-200 bg-blue-50 p-5 dark:border-blue-900 dark:bg-blue-950/20 shadow-sm text-sm text-blue-800 dark:text-blue-300">
            <p className="mb-1 font-semibold">Data notes</p>
            <p className="text-xs">
              Domain tabs use server-side Loki and VictoriaMetrics queries for the selected domain and time range.
              Slow Requests are now expandable so you can inspect each request inline.
            </p>
          </div>

          <ChartBox
            title="RT Distribution by Status"
            loading={rtDistributionQuery.isLoading}
            empty={!rtDistributionQuery.data}
            hint="Uses the dashboard selected time range, including manual ranges"
          >
            <NginxRTDistributionChart
              data={rtDistributionQuery.data}
              error={rtDistributionQuery.isError ? rtDistributionQuery.error?.message ?? 'Failed to load RT distribution.' : null}
            />
          </ChartBox>

          <NginxPerformanceSection id={id} domain={selectedDomain} />

          <IPInvestigationBlock id={id} domain={selectedDomain} />
        </>
      )}
    </div>
  )
}
