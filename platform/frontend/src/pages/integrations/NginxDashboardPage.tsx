// src/pages/integrations/NginxDashboardPage.tsx

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import {
  LineChart, Line, AreaChart, Area, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, ReferenceLine, PieChart, Pie, Cell, Legend,
} from 'recharts'
import apiClient from '@/api/client'
import { getDiscoveryService } from '@/api/discovery'
import {
  parseLogLines,
  formatNumber,
  formatPercent,
  formatBytes,
  getStep,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from '@/lib/queryHelpers'

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const TIME_RANGES = [
  { label: '15m', seconds: 900   },
  { label: '1h',  seconds: 3600  },
  { label: '6h',  seconds: 21600 },
  { label: '24h', seconds: 86400 },
]

// ─────────────────────────────────────────────────────────────────────────────
// Local helpers — defined here so behaviour is explicit and never guessed
// ─────────────────────────────────────────────────────────────────────────────

/**
 * Parse a Loki / VictoriaMetrics metric query_range response body.
 *
 * Our Go backend proxies both services and returns the raw JSON unchanged.
 * Axios stores that JSON in response.data, so inside a queryFn:
 *
 *   res.data  →  { status: "success", data: { resultType: "matrix", result: [...] } }
 *
 * This function accepts res.data (the full JSON body) and returns typed series.
 */
function parseMatrix(
  body: unknown,
): Array<{ metric: Record<string, string>; values: Array<{ time: number; value: number }> }> {
  if (!body || typeof body !== 'object') return []
  const inner = (body as Record<string, unknown>)['data'] as Record<string, unknown> | undefined
  if (!inner) return []
  const result = inner['result'] as Array<unknown> | undefined
  if (!Array.isArray(result)) return []

  return result.map((series) => {
    const s       = series as Record<string, unknown>
    const metric  = (s['metric'] as Record<string, string>) ?? {}
    const rawVals = (s['values'] as Array<[number | string, string]>) ?? []
    const values  = rawVals.map(([ts, v]) => ({
      time:  typeof ts === 'number' ? ts : parseFloat(ts as string),
      value: parseFloat(v) || 0,
    }))
    return { metric, values }
  })
}

/** Get the most-recent value of a matrix series. Returns null if no data. */
function latestVal(body: unknown): number | null {
  const series = parseMatrix(body)
  if (!series.length) return null
  const vals = series[0].values
  if (!vals.length) return null
  return vals[vals.length - 1].value
}

// ─────────────────────────────────────────────────────────────────────────────
// Small UI components
// ─────────────────────────────────────────────────────────────────────────────

function StatCard({
  title, value, unit, status = 'green', loading, icon,
}: {
  title: string; value: string | number; unit?: string
  status?: 'green' | 'yellow' | 'red'; loading?: boolean; icon?: string
}) {
  const color = { green: 'text-green-500', yellow: 'text-yellow-500', red: 'text-red-500' }[status]
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-muted-foreground">{title}</p>
          <div className="mt-2 flex items-baseline gap-2">
            <p className={`text-3xl font-bold ${loading ? 'text-muted' : color}`}>
              {loading ? '—' : value}
            </p>
            {unit && !loading && <p className="text-sm text-muted-foreground">{unit}</p>}
          </div>
        </div>
        {icon && <div className="text-4xl opacity-40">{icon}</div>}
      </div>
    </div>
  )
}

function TimeRangeSelector({
  selected, onChange,
}: {
  selected: (typeof TIME_RANGES)[0]
  onChange: (r: (typeof TIME_RANGES)[0]) => void
}) {
  return (
    <div className="flex gap-1 rounded-lg border border-border bg-muted p-1">
      {TIME_RANGES.map((r) => (
        <button key={r.label} onClick={() => onChange(r)}
          className={`px-3 py-1.5 rounded text-sm font-medium transition-colors ${
            selected.seconds === r.seconds
              ? 'bg-primary text-primary-foreground'
              : 'text-muted-foreground hover:text-foreground'
          }`}
        >
          {r.label}
        </button>
      ))}
    </div>
  )
}

function ChartBox({
  title, children, loading, empty, hint,
}: {
  title: string; children: React.ReactNode
  loading?: boolean; empty?: boolean; hint?: string
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 font-semibold">{title}</h3>
      {loading ? (
        <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">Loading…</div>
      ) : empty ? (
        <div className="flex h-64 flex-col items-center justify-center gap-1 text-sm text-muted-foreground">
          <span>No data for selected range</span>
          {hint && <span className="text-xs opacity-70">{hint}</span>}
        </div>
      ) : children}
    </div>
  )
}

function DataTable({
  title, subtitle, rows, loading,
}: {
  title: string; subtitle?: string
  rows: Array<{ label: string; value: string; percent?: number }>
  loading?: boolean
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="font-semibold">{title}</h3>
      {subtitle && <p className="mb-3 text-xs text-muted-foreground">{subtitle}</p>}
      {!subtitle && <div className="mb-3" />}
      <div className="space-y-1 max-h-64 overflow-y-auto">
        {loading ? (
          <p className="py-4 text-center text-sm text-muted-foreground">Loading…</p>
        ) : rows.length === 0 ? (
          <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
        ) : rows.map((row, i) => (
          <div key={i} className="flex items-center justify-between rounded px-2 py-1.5 hover:bg-muted">
            <span className="font-mono text-sm truncate flex-1 mr-2">{row.label}</span>
            <div className="flex items-center gap-2 flex-shrink-0">
              {row.percent !== undefined && (
                <span className="text-xs text-muted-foreground">{row.percent.toFixed(1)}%</span>
              )}
              <span className="font-semibold text-primary">{row.value}</span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Grafana button
//
// MULTI-TENANT NOTE: Grafana uses statically provisioned datasources that are
// scoped to a single tenant (configured in grafana-datasources.yaml).
// This button is intentionally labelled as an admin view.
// Full per-tenant isolation lives in this React dashboard which calls
// the backend proxy (/api/query/*) — those endpoints inject the correct
// tenant headers/paths server-side.
// ─────────────────────────────────────────────────────────────────────────────

function GrafanaButton({ timeRange }: { timeRange: (typeof TIME_RANGES)[0] }) {
  const [busy, setBusy] = useState(false)
  const open = async () => {
    setBusy(true)
    try {
      const res = await apiClient.get('/api/grafana/dashboard-url', {
        params: { dashboard: 'nginx-overview', from: `now-${timeRange.label}`, to: 'now' },
      })
      const url = res.data?.url
      if (url) window.open(url, '_blank', 'noopener,noreferrer')
    } catch (e) { console.error(e) } finally { setBusy(false) }
  }
  return (
    <button onClick={open} disabled={busy}
      title="Opens Grafana admin view (single-tenant datasource)"
      className="inline-flex items-center gap-2 rounded-md border border-border bg-card px-3 py-1.5 text-sm hover:bg-muted transition-colors disabled:opacity-50"
    >
      <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" className="opacity-70">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
      </svg>
      {busy ? 'Opening…' : 'Open in Grafana'}
    </button>
  )
}

// ─────────────────────────────────────────────────────────────────────────────
// Main page
// ─────────────────────────────────────────────────────────────────────────────

export function NginxDashboardPage() {
  const { id = '' } = useParams<{ id?: string }>()
  const [timeRange, setTimeRange] = useState(TIME_RANGES[1]) // default 1h

  const serviceQuery = useQuery({
    queryKey: ['discovery-service', id],
    queryFn:  () => getDiscoveryService(id),
    enabled:  !!id,
  })

  const serviceName = serviceQuery.data?.service_name ?? 'nginx'
  const hostId      = serviceQuery.data?.machine_id   ?? ''

  // Query key — all queries include timeRange so React Query refetches on change
  const qk = (name: string) => [`nginx-${name}`, id, hostId, timeRange.seconds]

  const lokiFilter = `{service_name="${serviceName}"}`

  // ── Shared param builders ─────────────────────────────────────────────────

  /** Build params for a Loki metric query (rate / count / quantile). */
  const lokiMetricParams = (query: string) => {
    const now   = getNowSeconds()
    const start = getStartSeconds(timeRange.seconds)
    const step  = getStep(timeRange.seconds)
    return {
      query,
      start: secondsToNanos(start),
      end:   secondsToNanos(now),
      step,
      limit: '5000', // ignored for metric queries; set high to avoid default cap
    }
  }

  /** Build params for a VictoriaMetrics range query. */
  const vmParams = (query: string) => {
    const now   = getNowSeconds()
    const start = getStartSeconds(timeRange.seconds)
    const step  = getStep(timeRange.seconds)
    return { query, start, end: now, step, type: 'range' }
  }

  // ── Queries ───────────────────────────────────────────────────────────────

  // Log sample — used only for tables (top-N lists)
  const logsQuery = useQuery({
    queryKey: qk('logs-sample'),
    queryFn: async () => {
      const now   = getNowSeconds()
      const start = getStartSeconds(timeRange.seconds)
      const res   = await apiClient.get('/api/query/logs', {
        params: {
          query:     lokiFilter,
          limit:     500,
          start:     secondsToNanos(start),
          end:       secondsToNanos(now),
          direction: 'backward',
        },
      })
      return res.data
    },
    staleTime: 0,
    refetchInterval: 20000,
  })

  // Real totals via count_over_time — single window query, returns one value
  const totalCountQuery = useQuery({
    queryKey: qk('total-count'),
    queryFn: async () => {
      const res = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(count_over_time(${lokiFilter}[${timeRange.seconds}s]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const count2xxQuery = useQuery({
    queryKey: qk('count-2xx'),
    queryFn: async () => {
      const res = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(count_over_time(${lokiFilter} |~ \`" 2\\d\\d "\` [${timeRange.seconds}s]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const count4xxQuery = useQuery({
    queryKey: qk('count-4xx'),
    queryFn: async () => {
      const res = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(count_over_time(${lokiFilter} |~ \`" 4\\d\\d "\` [${timeRange.seconds}s]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const count5xxQuery = useQuery({
    queryKey: qk('count-5xx'),
    queryFn: async () => {
      const res = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(count_over_time(${lokiFilter} |~ \`" 5\\d\\d "\` [${timeRange.seconds}s]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Request rate time series
  const reqRateQuery = useQuery({
    queryKey: qk('req-rate'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter}[${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Error rate % time series
  const errRate4xxQuery = useQuery({
    queryKey: qk('errrate-4xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `(sum(rate(${lokiFilter} |~ \`" 4\\d\\d "\` [${step}])) or vector(0)) / sum(rate(${lokiFilter}[${step}])) * 100`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const errRate5xxQuery = useQuery({
    queryKey: qk('errrate-5xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `(sum(rate(${lokiFilter} |~ \`" 5\\d\\d "\` [${step}])) or vector(0)) / sum(rate(${lokiFilter}[${step}])) * 100`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Response time percentiles
  const rtP50Query = useQuery({
    queryKey: qk('rt-p50'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `quantile_over_time(0.50, ${lokiFilter} | regexp \`rt=(?P<rt>[0-9.]+)$\` | unwrap rt [${step}])`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const rtP95Query = useQuery({
    queryKey: qk('rt-p95'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `quantile_over_time(0.95, ${lokiFilter} | regexp \`rt=(?P<rt>[0-9.]+)$\` | unwrap rt [${step}])`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const rtP99Query = useQuery({
    queryKey: qk('rt-p99'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `quantile_over_time(0.99, ${lokiFilter} | regexp \`rt=(?P<rt>[0-9.]+)$\` | unwrap rt [${step}])`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Status breakdown over time
  const s2xxQuery = useQuery({
    queryKey: qk('s-2xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter} |~ \`" 2\\d\\d "\` [${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const s3xxQuery = useQuery({
    queryKey: qk('s-3xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter} |~ \`" 3\\d\\d "\` [${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const s4xxQuery = useQuery({
    queryKey: qk('s-4xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter} |~ \`" 4\\d\\d "\` [${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const s5xxQuery = useQuery({
    queryKey: qk('s-5xx'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter} |~ \`" 5\\d\\d "\` [${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Nginx connection metrics (VictoriaMetrics)
  const connActiveQ  = useQuery({ queryKey: qk('conn-active'),  queryFn: async () => (await apiClient.get('/api/query/metrics', { params: vmParams('nginx_connections_active')  })).data, staleTime: 0, refetchInterval: 30000 })
  const connWaitingQ = useQuery({ queryKey: qk('conn-waiting'), queryFn: async () => (await apiClient.get('/api/query/metrics', { params: vmParams('nginx_connections_waiting') })).data, staleTime: 0, refetchInterval: 30000 })
  const connReadingQ = useQuery({ queryKey: qk('conn-reading'), queryFn: async () => (await apiClient.get('/api/query/metrics', { params: vmParams('nginx_connections_reading') })).data, staleTime: 0, refetchInterval: 30000 })
  const connWritingQ = useQuery({ queryKey: qk('conn-writing'), queryFn: async () => (await apiClient.get('/api/query/metrics', { params: vmParams('nginx_connections_writing') })).data, staleTime: 0, refetchInterval: 30000 })

  // Bandwidth
  const bwQuery = useQuery({
    queryKey: qk('bandwidth'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(rate(${lokiFilter} | regexp \`^[^ ]+ [^ ]+ [^ ]+ \\[[^\\]]+\\] "[^"]*" \\d+ (?P<bytes>\\d+)\` | unwrap bytes [${step}]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // Bot traffic
  const botQuery = useQuery({
    queryKey: qk('bot'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(
          `sum(rate(${lokiFilter} |~ \`(?i)bot|crawl|spider|curl|wget|python|java|go-http\` [${step}]))`
        ),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  const totalRateQuery = useQuery({
    queryKey: qk('total-rate'),
    queryFn: async () => {
      const step = getStep(timeRange.seconds)
      const res  = await apiClient.get('/api/query/logs/range', {
        params: lokiMetricParams(`sum(rate(${lokiFilter}[${step}]))`),
      })
      return res.data
    },
    staleTime: 0, refetchInterval: 30000,
  })

  // ── Derived stat-card values (from count queries — same source, consistent) ─

  const totalRequests = latestVal(totalCountQuery.data) ?? 0
  const total2xx      = latestVal(count2xxQuery.data)   ?? 0
  const total4xx      = latestVal(count4xxQuery.data)   ?? 0
  const total5xx      = latestVal(count5xxQuery.data)   ?? 0

  // All percentages derived from the same totalRequests — always sum to ≤100%
  const successPct = totalRequests > 0 ? (total2xx / totalRequests) * 100 : 0
  const rate4xxPct = totalRequests > 0 ? (total4xx / totalRequests) * 100 : 0
  const rate5xxPct = totalRequests > 0 ? (total5xx / totalRequests) * 100 : 0

  const p95Latest = latestVal(rtP95Query.data)

  const countLoading = totalCountQuery.isLoading || count2xxQuery.isLoading ||
                       count4xxQuery.isLoading   || count5xxQuery.isLoading

  // ── Log sample for tables ─────────────────────────────────────────────────

  const logLines   = parseLogLines(logsQuery.data)
  const sampleSize = logLines.length

  const pathCounts = logLines.reduce((acc, l) => {
    const m = l.match(/"\s(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s([^"\s?]+)/)
    if (m) acc[m[1]] = (acc[m[1]] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)
  const topPaths = Object.entries(pathCounts).sort((a, b) => b[1] - a[1]).slice(0, 10)
    .map(([path, count]) => ({ path, count }))

  const statusCounts = logLines.reduce((acc, l) => {
    const m = l.match(/"\s(\d{3})\s/)
    if (m) acc[m[1]] = (acc[m[1]] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)
  const topStatuses = Object.entries(statusCounts).sort((a, b) => b[1] - a[1]).slice(0, 10)
    .map(([status, count]) => ({ status, count }))

  const ipCounts = logLines.reduce((acc, l) => {
    const m = l.match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/)
    if (m) acc[m[1]] = (acc[m[1]] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)
  const topIPs = Object.entries(ipCounts).sort((a, b) => b[1] - a[1]).slice(0, 10)
    .map(([ip, count]) => ({ ip, count }))

  const methodCounts = logLines.reduce((acc, l) => {
    const m = l.match(/"\s(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS|CONNECT|TRACE)\s/)
    const method = m ? m[1] : 'OTHER'
    acc[method] = (acc[method] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)
  const methodBreakdown = Object.entries(methodCounts)
    .map(([method, count]) => ({ method, count, pct: sampleSize > 0 ? (count / sampleSize) * 100 : 0 }))
    .sort((a, b) => b.count - a.count)

  const pathErrMap = logLines.reduce((acc, l) => {
    const pm = l.match(/"\s(?:GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s([^"\s?]+)/)
    const sm = l.match(/"\s(\d{3})\s/)
    if (pm && sm) {
      const path = pm[1], code = parseInt(sm[1])
      if (!acc[path]) acc[path] = { ok: 0, err: 0 }
      if (code >= 400) acc[path].err++; else acc[path].ok++
    }
    return acc
  }, {} as Record<string, { ok: number; err: number }>)
  const topErrorPaths = Object.entries(pathErrMap)
    .map(([path, { ok, err }]) => ({ path, total: ok + err, err, errPct: (ok + err) > 0 ? (err / (ok + err)) * 100 : 0 }))
    .filter((p) => p.err > 0)
    .sort((a, b) => b.errPct - a.errPct)
    .slice(0, 10)

  // ── Bot vs Human ─────────────────────────────────────────────────────────
  const rawBot   = latestVal(botQuery.data)       ?? 0
  const rawTotal = latestVal(totalRateQuery.data) ?? 0
  const botPct   = rawTotal > 0 ? Math.min(100, (rawBot / rawTotal) * 100) : 0
  const humanPct = 100 - botPct

  // ── Chart series — all via parseMatrix() ─────────────────────────────────

  const reqRateData = parseMatrix(reqRateQuery.data)[0]?.values.map((p) => ({
    time: new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    req:  p.value,
  })) ?? []

  const e4s = parseMatrix(errRate4xxQuery.data)[0]?.values ?? []
  const e5s = parseMatrix(errRate5xxQuery.data)[0]?.values ?? []
  const errTrendData = e4s.map((p, i) => ({
    time:   new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    pct4xx: p.value,
    pct5xx: e5s[i]?.value ?? 0,
  }))

  const p50s = parseMatrix(rtP50Query.data)[0]?.values ?? []
  const p95s = parseMatrix(rtP95Query.data)[0]?.values ?? []
  const p99s = parseMatrix(rtP99Query.data)[0]?.values ?? []
  const rtData = p50s.map((p, i) => ({
    time: new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    p50:  p.value   * 1000,
    p95:  (p95s[i]?.value ?? 0) * 1000,
    p99:  (p99s[i]?.value ?? 0) * 1000,
  }))

  const sv2  = parseMatrix(s2xxQuery.data)[0]?.values ?? []
  const sv3  = parseMatrix(s3xxQuery.data)[0]?.values ?? []
  const sv4  = parseMatrix(s4xxQuery.data)[0]?.values ?? []
  const sv5  = parseMatrix(s5xxQuery.data)[0]?.values ?? []
  const statusData = sv2.map((p, i) => ({
    time: new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    '2xx': p.value, '3xx': sv3[i]?.value ?? 0, '4xx': sv4[i]?.value ?? 0, '5xx': sv5[i]?.value ?? 0,
  }))

  const ca  = parseMatrix(connActiveQ.data)[0]?.values  ?? []
  const cw  = parseMatrix(connWaitingQ.data)[0]?.values ?? []
  const cr  = parseMatrix(connReadingQ.data)[0]?.values ?? []
  const cwr = parseMatrix(connWritingQ.data)[0]?.values ?? []
  const connData = ca.map((p, i) => ({
    time:    new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    active:  p.value,
    waiting: cw[i]?.value  ?? 0,
    reading: cr[i]?.value  ?? 0,
    writing: cwr[i]?.value ?? 0,
  }))

  const bwData = parseMatrix(bwQuery.data)[0]?.values.map((p) => ({
    time:  new Date(p.time * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    bytes: p.value,
  })) ?? []

  // ── Guards ────────────────────────────────────────────────────────────────

  if (!id)                    return <div className="p-6 text-sm text-red-600">Missing service id.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading…</div>
  if (!serviceQuery.data)     return <div className="p-6 text-sm text-red-600">Service not found for id {id}.</div>

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="space-y-6 p-6">

      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-3xl font-bold">Nginx Dashboard</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Service: <span className="font-mono font-semibold">{serviceName}</span>
            {hostId && <> • Host: <span className="font-mono font-semibold">{hostId}</span></>}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          <GrafanaButton timeRange={timeRange} />
          <TimeRangeSelector selected={timeRange} onChange={setTimeRange} />
          <Link to={`/integrations/nginx-logs/${id}`}
            className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-500">
            View Logs →
          </Link>
        </div>
      </div>

      {/* ── Stat cards — all derived from count_over_time (same source) ── */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
        <StatCard
          title="Total Requests" icon="📊" status="green"
          value={totalRequests > 0 ? formatNumber(Math.round(totalRequests)) : '—'}
          loading={countLoading}
        />
        <StatCard
          title="2xx Success Rate" icon="✅"
          value={formatPercent(successPct)}
          status={successPct > 95 ? 'green' : successPct > 80 ? 'yellow' : 'red'}
          loading={countLoading}
        />
        <StatCard
          title="4xx Error Rate" icon="⚠️"
          value={formatPercent(rate4xxPct)}
          status={rate4xxPct > 5 ? 'red' : rate4xxPct > 1 ? 'yellow' : 'green'}
          loading={countLoading}
        />
        <StatCard
          title="5xx Error Rate" icon="❌"
          value={formatPercent(rate5xxPct)}
          status={rate5xxPct > 1 ? 'red' : rate5xxPct > 0.1 ? 'yellow' : 'green'}
          loading={countLoading}
        />
        <StatCard
          title="P95 Response Time" icon="⏱️" status="green"
          value={p95Latest !== null ? `${(p95Latest * 1000).toFixed(0)}ms` : '—'}
          loading={rtP95Query.isLoading}
        />
        <StatCard
          title="Log Sample" icon="🔍" status="green" unit="lines"
          value={formatNumber(sampleSize)}
          loading={logsQuery.isLoading}
        />
      </div>

      {/* ── Row 1: Request Rate | Error Rate Trend ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox title="Request Rate (req/s)" loading={reqRateQuery.isLoading} empty={reqRateData.length === 0}>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={reqRateData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => v < 0.01 ? v.toFixed(4) : v.toFixed(3)} />
              <Tooltip formatter={(v) => typeof v === 'number' ? v.toFixed(4) : v} />
              <Line type="monotone" dataKey="req" stroke="#6366f1" dot={false} strokeWidth={2} isAnimationActive={false} name="req/s" />
            </LineChart>
          </ResponsiveContainer>
        </ChartBox>

        <ChartBox title="Error Rate Trend (%)" loading={errRate4xxQuery.isLoading || errRate5xxQuery.isLoading} empty={errTrendData.length === 0}>
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={errTrendData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `${v.toFixed(1)}%`} />
              <Tooltip formatter={(v) => typeof v === 'number' ? `${v.toFixed(2)}%` : v} />
              <Legend />
              <ReferenceLine y={5} stroke="#f59e0b" strokeDasharray="4 4" label={{ value: '5%', fontSize: 10 }} />
              <ReferenceLine y={1} stroke="#ef4444" strokeDasharray="4 4" label={{ value: '1%', fontSize: 10 }} />
              <Line type="monotone" dataKey="pct4xx" stroke="#f59e0b" dot={false} strokeWidth={2} name="4xx %" isAnimationActive={false} />
              <Line type="monotone" dataKey="pct5xx" stroke="#ef4444" dot={false} strokeWidth={2} name="5xx %" isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </ChartBox>
      </div>

      {/* ── Row 2: Response Time | Status Breakdown ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox
          title="Response Time P50 / P95 / P99 (ms)"
          loading={rtP50Query.isLoading || rtP95Query.isLoading || rtP99Query.isLoading}
          empty={rtData.length === 0}
          hint="Requires nginx log_format to include rt=$request_time"
        >
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={rtData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => `${v.toFixed(0)}ms`} />
              <Tooltip formatter={(v) => typeof v === 'number' ? `${v.toFixed(1)}ms` : v} />
              <Legend />
              <Line type="monotone" dataKey="p50" stroke="#22c55e" dot={false} strokeWidth={2} name="P50" isAnimationActive={false} />
              <Line type="monotone" dataKey="p95" stroke="#f59e0b" dot={false} strokeWidth={2} name="P95" isAnimationActive={false} />
              <Line type="monotone" dataKey="p99" stroke="#ef4444" dot={false} strokeWidth={2} name="P99" isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </ChartBox>

        <ChartBox
          title="HTTP Status Breakdown Over Time"
          loading={s2xxQuery.isLoading || s3xxQuery.isLoading || s4xxQuery.isLoading || s5xxQuery.isLoading}
          empty={statusData.length === 0}
        >
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={statusData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              <Area type="monotone" dataKey="2xx" stackId="1" stroke="#22c55e" fill="#22c55e" fillOpacity={0.6} name="2xx Success"  isAnimationActive={false} />
              <Area type="monotone" dataKey="3xx" stackId="1" stroke="#3b82f6" fill="#3b82f6" fillOpacity={0.6} name="3xx Redirect" isAnimationActive={false} />
              <Area type="monotone" dataKey="4xx" stackId="1" stroke="#f59e0b" fill="#f59e0b" fillOpacity={0.6} name="4xx Error"    isAnimationActive={false} />
              <Area type="monotone" dataKey="5xx" stackId="1" stroke="#ef4444" fill="#ef4444" fillOpacity={0.6} name="5xx Error"    isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </ChartBox>
      </div>

      {/* ── Row 3: Connections | Bandwidth ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <ChartBox
          title="Active Connections"
          loading={connActiveQ.isLoading}
          empty={connData.length === 0}
          hint="Requires nginx stub_status module enabled"
        >
          <ResponsiveContainer width="100%" height={250}>
            <LineChart data={connData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} />
              <Tooltip />
              <Legend />
              <Line type="monotone" dataKey="active"  stroke="#6366f1" dot={false} strokeWidth={2} name="Active"  isAnimationActive={false} />
              <Line type="monotone" dataKey="waiting" stroke="#94a3b8" dot={false} strokeWidth={1} name="Waiting" isAnimationActive={false} />
              <Line type="monotone" dataKey="reading" stroke="#22c55e" dot={false} strokeWidth={1} name="Reading" isAnimationActive={false} />
              <Line type="monotone" dataKey="writing" stroke="#f59e0b" dot={false} strokeWidth={1} name="Writing" isAnimationActive={false} />
            </LineChart>
          </ResponsiveContainer>
        </ChartBox>

        <ChartBox title="Bandwidth Over Time" loading={bwQuery.isLoading} empty={bwData.length === 0}>
          <ResponsiveContainer width="100%" height={250}>
            <AreaChart data={bwData}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" />
              <XAxis dataKey="time" tick={{ fontSize: 11 }} />
              <YAxis tick={{ fontSize: 11 }} tickFormatter={(v) => v > 1048576 ? `${(v/1048576).toFixed(1)}MB/s` : `${(v/1024).toFixed(0)}KB/s`} />
              <Tooltip formatter={(v) => typeof v === 'number' ? [`${(v/1024).toFixed(1)} KB/s`, 'Bandwidth'] : v} />
              <Area type="monotone" dataKey="bytes" stroke="#6366f1" fill="#6366f1" fillOpacity={0.2} isAnimationActive={false} />
            </AreaChart>
          </ResponsiveContainer>
        </ChartBox>
      </div>

      {/* ── Status Summary ── */}
      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="font-semibold">Response Status Breakdown</h3>
        <p className="mb-4 text-xs text-muted-foreground">Full time range via Loki count_over_time — totals and rates are mutually consistent</p>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {[
            { label: '2xx Success',      value: total2xx,   color: 'green'  },
            { label: '4xx Client Error', value: total4xx,   color: 'orange' },
            { label: '5xx Server Error', value: total5xx,   color: 'red'    },
            { label: 'Total',            value: totalRequests, color: 'blue' },
          ].map(({ label, value, color }) => (
            <div key={label} className={`rounded-lg bg-${color}-50 dark:bg-${color}-900/20 p-4 text-center`}>
              <p className="text-xs text-muted-foreground">{label}</p>
              <p className={`mt-1 text-2xl font-bold text-${color}-600 dark:text-${color}-400`}>
                {countLoading ? '—' : totalRequests > 0 ? formatNumber(Math.round(value)) : '—'}
              </p>
              {!countLoading && totalRequests > 0 && value !== totalRequests && (
                <p className="text-xs text-muted-foreground">{formatPercent((value / totalRequests) * 100)}</p>
              )}
            </div>
          ))}
        </div>
      </div>

      {/* ── Tables (from log sample) ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <DataTable
          title="Top Paths" subtitle={`${formatNumber(sampleSize)}-line sample`}
          loading={logsQuery.isLoading}
          rows={topPaths.map((p) => ({ label: p.path, value: formatNumber(p.count), percent: sampleSize > 0 ? (p.count / sampleSize) * 100 : 0 }))}
        />
        <DataTable
          title="Top Status Codes" subtitle={`${formatNumber(sampleSize)}-line sample`}
          loading={logsQuery.isLoading}
          rows={topStatuses.map((s) => ({ label: s.status, value: formatNumber(s.count), percent: sampleSize > 0 ? (s.count / sampleSize) * 100 : 0 }))}
        />
        <DataTable
          title="Top Client IPs" subtitle={`${formatNumber(sampleSize)}-line sample`}
          loading={logsQuery.isLoading}
          rows={topIPs.map((ip) => ({ label: ip.ip, value: formatNumber(ip.count), percent: sampleSize > 0 ? (ip.count / sampleSize) * 100 : 0 }))}
        />
      </div>

      {/* ── Method distribution + Bot pie ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-border bg-card p-4">
          <h3 className="font-semibold">HTTP Method Distribution</h3>
          <p className="mb-4 text-xs text-muted-foreground">{formatNumber(sampleSize)}-line sample</p>
          <div className="space-y-2">
            {methodBreakdown.length === 0
              ? <p className="py-4 text-center text-sm text-muted-foreground">No data</p>
              : methodBreakdown.map((m) => (
                <div key={m.method} className="flex items-center gap-2">
                  <span className="w-16 rounded px-2 py-1 text-center text-xs font-semibold text-white"
                    style={{ background: m.method === 'GET' ? '#22c55e' : m.method === 'POST' ? '#3b82f6' : m.method === 'DELETE' ? '#ef4444' : '#a855f7' }}>
                    {m.method}
                  </span>
                  <div className="flex-1 rounded-full bg-muted h-2">
                    <div className="h-2 rounded-full bg-primary" style={{ width: `${Math.max(3, m.pct)}%` }} />
                  </div>
                  <span className="w-24 text-right text-xs">
                    <span className="font-semibold">{formatNumber(m.count)}</span>
                    <span className="ml-1 text-muted-foreground">{m.pct.toFixed(1)}%</span>
                  </span>
                </div>
              ))
            }
          </div>
        </div>

        <ChartBox title="Bot vs Human Traffic" loading={botQuery.isLoading || totalRateQuery.isLoading} empty={rawTotal === 0}>
          <ResponsiveContainer width="100%" height={250}>
            <PieChart>
              <Pie
                data={[
                  { name: 'Human',         value: humanPct, label: `${humanPct.toFixed(1)}%` },
                  { name: 'Bot/Automated', value: botPct,   label: `${botPct.toFixed(1)}%`   },
                ]}
                cx="50%" cy="50%" innerRadius={50} outerRadius={80} paddingAngle={3} dataKey="value"
                label={({ name, label }: { name: string; label: string }) => `${name} ${label}`}
              >
                <Cell fill="#22c55e" />
                <Cell fill="#94a3b8" />
              </Pie>
              <Tooltip formatter={(_v: unknown, name: unknown, p: { payload?: { label?: string } }) =>
                [`${p.payload?.label ?? '0%'}`, name as string]} />
            </PieChart>
          </ResponsiveContainer>
        </ChartBox>
      </div>

      {/* ── Top Error Paths ── */}
      <div className="rounded-lg border border-border bg-card p-4">
        <h3 className="font-semibold">Top Paths by Error Rate</h3>
        <p className="mb-4 text-xs text-muted-foreground">{formatNumber(sampleSize)}-line sample</p>
        <div className="space-y-2 max-h-64 overflow-y-auto">
          {topErrorPaths.length > 0 ? topErrorPaths.map((p, i) => (
            <div key={i} className="flex items-center gap-2 rounded px-2 py-2 hover:bg-muted text-xs">
              <span className={`flex-shrink-0 rounded px-2 py-0.5 font-medium ${p.errPct > 10 ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400' : 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'}`}>
                {p.errPct > 10 ? 'ERROR' : 'WARN'}
              </span>
              <span className="font-mono flex-1 truncate">{p.path}</span>
              <span className="text-muted-foreground flex-shrink-0">{p.err}/{p.total}</span>
              <span className="font-semibold flex-shrink-0 w-14 text-right">{p.errPct.toFixed(1)}%</span>
            </div>
          )) : (
            <p className="py-4 text-center text-sm text-muted-foreground">✅ No error paths in sample</p>
          )}
        </div>
      </div>

      {/* ── Security ── */}
      <div className="rounded-lg border border-red-200 bg-red-50 dark:border-red-900 dark:bg-red-900/20 p-4">
        <h3 className="mb-4 text-sm font-semibold flex items-center gap-2">🔒 IPs with High 4xx Rate (sample)</h3>
        <div className="space-y-2 max-h-48 overflow-y-auto">
          {(() => {
            const susp = topIPs.map(({ ip }) => {
              const lines = logLines.filter((l) => l.startsWith(ip))
              const errs  = lines.filter((l) => /"\s4\d{2}\s/.test(l))
              const rate  = lines.length > 0 ? (errs.length / lines.length) * 100 : 0
              return { ip, total: lines.length, errors: errs.length, rate }
            }).filter((x) => x.rate > 30).sort((a, b) => b.errors - a.errors).slice(0, 5)
            return susp.length > 0 ? susp.map((x) => (
              <div key={x.ip} className="flex items-center justify-between rounded border-l-2 border-red-500 bg-white dark:bg-muted px-2 py-2">
                <div className="flex items-center gap-2">
                  <span className="rounded px-2 py-1 text-xs font-semibold bg-red-100 text-red-700">{x.rate.toFixed(0)}%</span>
                  <span className="font-mono text-sm">{x.ip}</span>
                </div>
                <span className="text-xs text-muted-foreground">{x.errors}/{x.total} errors</span>
              </div>
            )) : <p className="py-4 text-center text-sm text-muted-foreground">✅ No suspicious IPs in sample</p>
          })()}
        </div>
      </div>

      {/* Info */}
      <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-300">
        <p className="mb-1 font-semibold">💡 Data sources</p>
        <p className="text-xs">
          Stat cards and time-series charts use Loki metric queries over the full selected time window — numbers are accurate and consistent.
          Table sections (top paths, IPs, status codes) use a 500-line sample from the same window and are labelled accordingly.
          Refreshes every 20–30 s.
        </p>
      </div>

    </div>
  )
}