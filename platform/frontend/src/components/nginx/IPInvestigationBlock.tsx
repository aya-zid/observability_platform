// src/components/nginx/IPInvestigationBlock.tsx
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
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
import { ChevronDown, ChevronRight, Clock } from 'lucide-react'

import { getNginxIPInvestigation } from '@/api/nginx'
import type { NginxIPInvestigationResult, NginxTimePoint } from '@/api/nginx'
import {
  formatNumber,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

// ─── Constants ────────────────────────────────────────────────────────────────

const IP_TIME_RANGES = [
  { label: '15m', seconds: 900 },
  { label: '30m', seconds: 1800 },
  { label: '1h', seconds: 3600 },
  { label: '2h', seconds: 7200 },
  { label: '6h', seconds: 21600 },
  { label: '12h', seconds: 43200 },
  { label: '24h', seconds: 86400 },
] as const

type IPTimeRange = (typeof IP_TIME_RANGES)[number]

// step only affects X-axis tick density; bucket size is always 1m server-side
function getStep(rangeSeconds: number): string {
  if (rangeSeconds <= 900) return '15s'
  if (rangeSeconds <= 3600) return '30s'
  if (rangeSeconds <= 21600) return '2m'
  return '5m'
}

// ─── Shared chart styles ──────────────────────────────────────────────────────

const chartStyle = {
  grid: 'hsl(var(--border))',
  tick: { fill: '#94a3b8', fontSize: 10 },
  tooltip: {
    background: 'hsl(var(--card))',
    border: '1px solid hsl(var(--border))',
    borderRadius: '6px',
    padding: '8px 12px',
    fontSize: '12px',
    color: 'hsl(var(--foreground))',
  },
} as const

// Status class palette — shared between chart and path UI
const STATUS_COLORS = {
  '2xx': { hex: '#22c55e', text: 'text-green-400',  bg: 'bg-green-500/15 border-green-500/30' },
  '3xx': { hex: '#3b82f6', text: 'text-blue-400',   bg: 'bg-blue-500/15  border-blue-500/30'  },
  '4xx': { hex: '#f59e0b', text: 'text-amber-400',  bg: 'bg-amber-500/15 border-amber-500/30' },
  '5xx': { hex: '#ef4444', text: 'text-red-400',    bg: 'bg-red-500/15   border-red-500/30'   },
} as const

type StatusClass = keyof typeof STATUS_COLORS

const METHOD_COLORS: Record<string, string> = {
  GET:    'bg-blue-500/15  text-blue-400   border-blue-500/30',
  POST:   'bg-green-500/15 text-green-400  border-green-500/30',
  PUT:    'bg-amber-500/15 text-amber-400  border-amber-500/30',
  PATCH:  'bg-purple-500/15 text-purple-400 border-purple-500/30',
  DELETE: 'bg-red-500/15   text-red-400    border-red-500/30',
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function fmtTime(unixSeconds: number) {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function mapTs(points: NginxTimePoint[]): Record<string, number | string>[] {
  return (points ?? []).map((pt) => {
    const row: Record<string, number | string> = {}
    for (const [k, v] of Object.entries(pt)) {
      row[k] = k === 'time' ? fmtTime(Number(v)) : v
    }
    return row
  })
}

function classOf(code: string): StatusClass {
  const n = parseInt(code, 10)
  if (n >= 500) return '5xx'
  if (n >= 400) return '4xx'
  if (n >= 300) return '3xx'
  return '2xx'
}

// ─── Path row sub-component ───────────────────────────────────────────────────

interface PathRowProps {
  path: string
  method: string
  count: number
  errors: number
  avgRt: number
  statusBreakdown: Record<string, number>
  timeWindow: string
}

function PathRow({ path, method, count, avgRt, statusBreakdown }: PathRowProps) {
  const [open, setOpen] = useState(false)
  const [activeClass, setActiveClass] = useState<StatusClass | null>(null)

  // Aggregate breakdown into status classes
  const classAgg: Record<StatusClass, number> = { '2xx': 0, '3xx': 0, '4xx': 0, '5xx': 0 }
  Object.entries(statusBreakdown ?? {}).forEach(([code, n]) => {
    classAgg[classOf(code)] += n
  })

  // Segments sorted largest → smallest for the stacked bar
  const segments = (Object.entries(classAgg) as [StatusClass, number][])
    .filter(([, n]) => n > 0)
    .sort((a, b) => b[1] - a[1])

  // Individual codes grouped by class, sorted by count
  const codesByClass: Record<string, [string, number][]> = {}
  Object.entries(statusBreakdown ?? {}).forEach(([code, n]) => {
    const c = classOf(code)
    ;(codesByClass[c] ??= []).push([code, n])
  })
  Object.values(codesByClass).forEach((arr) => arr.sort((a, b) => b[1] - a[1]))

  const total = count || 1

  return (
    <div className="bg-card border border-border rounded-lg p-3 hover:border-primary/40 transition-colors">
      {/* Row header */}
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={() => setOpen((o) => !o)}
          className="shrink-0 text-muted-foreground hover:text-foreground transition-colors"
          aria-label="toggle details"
        >
          {open ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
        </button>

        {/* Method badge */}
        <span
          className={cn(
            'shrink-0 px-2 py-0.5 text-[10px] font-mono font-semibold rounded border',
            METHOD_COLORS[method] ?? 'bg-secondary text-foreground border-border',
          )}
        >
          {method || '?'}
        </span>

        {/* Path */}
        <span className="font-mono text-xs text-foreground flex-1 truncate min-w-[120px]" title={path}>
          {path}
        </span>

        {/* Avg RT */}
        {avgRt > 0 && (
          <span className="shrink-0 text-xs text-muted-foreground">avg {Math.round(avgRt)}ms</span>
        )}

        {/* Total */}
        <span className="shrink-0 text-sm font-bold text-foreground tabular-nums">
          {count.toLocaleString()}
          <span className="text-[10px] text-muted-foreground font-normal ml-1">req</span>
        </span>
      </div>

      {/* Stacked bar */}
      {segments.length > 0 && (
        <div className="mt-2.5 flex w-full h-2.5 rounded-full overflow-hidden bg-secondary/50">
          {segments.map(([cls, n]) => {
            const pct = (n / total) * 100
            return (
              <button
                key={cls}
                title={`${cls}: ${n.toLocaleString()} (${pct.toFixed(1)}%)`}
                onClick={() => {
                  setOpen(true)
                  setActiveClass(cls)
                }}
                style={{ width: `${pct}%`, backgroundColor: STATUS_COLORS[cls].hex }}
                className="h-full hover:opacity-75 transition-opacity first:rounded-l-full last:rounded-r-full"
              />
            )
          })}
        </div>
      )}

      {/* Mini class labels */}
      {segments.length > 0 && (
        <div className="mt-2 flex items-center gap-1.5 flex-wrap">
          {segments.map(([cls, n]) => {
            const pct = ((n / total) * 100).toFixed(1)
            return (
              <button
                key={cls}
                onClick={() => {
                  setOpen(true)
                  setActiveClass(cls)
                }}
                className={cn(
                  'px-1.5 py-0.5 rounded border text-[10px] font-mono transition hover:opacity-80',
                  STATUS_COLORS[cls].bg,
                  STATUS_COLORS[cls].text,
                )}
              >
                {cls} · {n.toLocaleString()}{' '}
                <span className="opacity-60">({pct}%)</span>
              </button>
            )
          })}
        </div>
      )}

      {/* Expanded per-code breakdown */}
      {open && (
        <div className="mt-3 pt-3 border-t border-border/50 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-2">
          {(['2xx', '3xx', '4xx', '5xx'] as const).map((cls) => {
            const codes = codesByClass[cls] ?? []
            if (!codes.length) return null
            const isActive = activeClass === cls
            return (
              <div
                key={cls}
                className={cn(
                  'rounded-md border p-2',
                  STATUS_COLORS[cls].bg,
                  isActive && 'ring-1 ring-primary/60',
                )}
              >
                <div className={cn('text-[11px] font-semibold mb-1.5', STATUS_COLORS[cls].text)}>
                  {cls} responses
                </div>
                <div className="space-y-1">
                  {codes.map(([code, n]) => (
                    <div key={code} className="flex items-center justify-between text-[11px] font-mono">
                      <span className={STATUS_COLORS[cls].text}>{code}</span>
                      <span className="text-foreground tabular-nums">{n.toLocaleString()}</span>
                    </div>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}

// ─── Main component ───────────────────────────────────────────────────────────

export function IPInvestigationBlock({
  id,
  domain,
}: {
  id: string
  domain?: string
}) {
  const [inputValue, setInputValue] = useState('')
  const [committedIP, setCommittedIP] = useState('')
  const [timeRange, setTimeRange] = useState<IPTimeRange>(IP_TIME_RANGES[2]) // 1h default
  const [methodFilter, setMethodFilter] = useState<string>('all')

  const handleSubmit = () => {
    const trimmed = inputValue.trim()
    if (trimmed) setCommittedIP(trimmed)
  }

  const investigationQuery = useQuery<NginxIPInvestigationResult>({
    queryKey: ['nginx-ip-investigation', id, committedIP, timeRange.seconds, domain ?? ''],
    enabled: !!committedIP,
    queryFn: () => {
      const end = getNowSeconds()
      const start = getStartSeconds(timeRange.seconds)
      return getNginxIPInvestigation(id, {
        ip: committedIP,
        start: secondsToNanos(start),
        end: secondsToNanos(end),
        step: getStep(timeRange.seconds),
        domain: domain || undefined,
      })
    },
    staleTime: 25_000,
    refetchOnWindowFocus: false,
  })

  const d = investigationQuery.data
  const isLoading = investigationQuery.isFetching

  const reqRateData = useMemo(() => mapTs(d?.req_rate_timeseries ?? []), [d])
  const statusData  = useMemo(() => mapTs(d?.status_timeseries  ?? []), [d])
  const rtData      = useMemo(() => mapTs(d?.rt_timeseries      ?? []), [d])

  const totalRequests = d?.total_requests ?? 0
  const errorRate     = d?.error_rate     ?? 0

  // Available methods for the filter pill row
  const availableMethods = useMemo(() => {
    if (!d?.paths?.length) return []
    return Array.from(new Set(d.paths.map((p) => p.method).filter(Boolean))).sort()
  }, [d])

  const filteredPaths = useMemo(() => {
    if (!d?.paths) return []
    return d.paths
      .filter((p) => methodFilter === 'all' || p.method === methodFilter)
      .slice()
      .sort((a, b) => b.count - a.count)
  }, [d, methodFilter])

  return (
    <div className="rounded-xl border border-border bg-card shadow-sm overflow-hidden">
      {/* ── Header ───────────────────────────────────────────────────────── */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-muted/40 px-5 py-4">
        <div className="flex items-center gap-2">
          <span className="flex h-7 w-7 items-center justify-center rounded-full bg-primary/10 text-primary text-sm">
            🔍
          </span>
          <h2 className="text-base font-semibold text-foreground">IP Investigation</h2>
        </div>

        {/* Time-range pills */}
        <div className="flex gap-1 rounded-lg border border-border bg-background p-1 shadow-sm">
          {IP_TIME_RANGES.map((r) => (
            <button
              key={r.label}
              type="button"
              onClick={() => setTimeRange(r)}
              className={cn(
                'rounded px-2.5 py-1 text-xs font-medium transition-all duration-150',
                timeRange.seconds === r.seconds
                  ? 'bg-primary text-primary-foreground shadow-sm'
                  : 'text-muted-foreground hover:text-foreground hover:bg-muted/70',
              )}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      <div className="p-5 space-y-6">
        {/* ── IP Input ──────────────────────────────────────────────────── */}
        <div className="flex gap-2">
          <input
            type="text"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleSubmit()}
            placeholder="Enter IP address (e.g. 203.0.113.42)"
            className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm font-mono placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/50"
          />
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!inputValue.trim()}
            className="rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground disabled:opacity-40 hover:bg-primary/90 transition-colors"
          >
            Investigate
          </button>
        </div>

        {/* ── Empty state ───────────────────────────────────────────────── */}
        {!committedIP && (
          <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border bg-muted/30 py-14 text-center">
            <p className="text-2xl mb-2">🕵️</p>
            <p className="text-sm font-medium text-foreground">Enter an IP address above</p>
            <p className="mt-1 text-xs text-muted-foreground">
              Precise timeline, status breakdown, response times and path analysis
            </p>
          </div>
        )}

        {/* ── Results ───────────────────────────────────────────────────── */}
        {committedIP && (
          <>
            {/* IP info bar */}
            <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/30 px-4 py-3">
              <span className="rounded bg-primary/10 px-2 py-1 font-mono text-sm font-semibold text-primary">
                {committedIP}
              </span>
              <span className="text-xs text-muted-foreground">
                — {timeRange.label} window{domain ? ` • ${domain}` : ''}
              </span>
              {isLoading && (
                <span className="ml-auto text-xs text-muted-foreground animate-pulse">Loading…</span>
              )}
            </div>

            {/* ── Stat cards ────────────────────────────────────────────── */}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <div className="rounded-lg border border-border bg-background p-4">
                <p className="text-xs text-muted-foreground">Total Requests</p>
                <p className="mt-1 text-2xl font-bold tabular-nums text-foreground">
                  {isLoading ? '—' : formatNumber(Math.round(totalRequests), 0)}
                </p>
              </div>
              <div className="rounded-lg border border-border bg-background p-4">
                <p className="text-xs text-muted-foreground">Error Rate (4xx+5xx)</p>
                <p
                  className={cn(
                    'mt-1 text-2xl font-bold tabular-nums',
                    errorRate >= 50
                      ? 'text-red-500'
                      : errorRate >= 10
                        ? 'text-amber-500'
                        : 'text-green-500',
                  )}
                >
                  {isLoading ? '—' : `${errorRate.toFixed(1)}%`}
                </p>
              </div>
              <div className="rounded-lg border border-border bg-background p-4">
                <p className="text-xs text-muted-foreground">First Seen Today</p>
                <p className="mt-1 font-mono text-2xl font-bold tabular-nums text-foreground">
                  {isLoading ? '—' : d?.first_seen_today || '—'}
                </p>
              </div>
              <div className="rounded-lg border border-border bg-background p-4">
                <p className="text-xs text-muted-foreground">Last Seen Today</p>
                <p className="mt-1 font-mono text-2xl font-bold tabular-nums text-foreground">
                  {isLoading ? '—' : d?.last_seen_today || '—'}
                </p>
              </div>
            </div>

            {/* ── Charts ────────────────────────────────────────────────── */}
            <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">

              {/* Requests per minute */}
              <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
                <h3 className="mb-3 text-xs font-semibold text-foreground uppercase tracking-wide">
                  Requests / min
                </h3>
                {isLoading ? (
                  <div className="h-56 flex items-center justify-center text-xs text-muted-foreground">
                    Loading…
                  </div>
                ) : reqRateData.length === 0 ? (
                  <div className="h-56 flex items-center justify-center text-xs text-muted-foreground">
                    No data
                  </div>
                ) : (
                  <div className="h-56">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={reqRateData}>
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
                          width={36}
                          allowDecimals={false}
                        />
                        <Tooltip
                          contentStyle={chartStyle.tooltip}
                          formatter={(v: any) => [
                            `${Math.round(Number(v))} req`,
                            'Count',
                          ]}
                        />
                        <Line
                          type="monotone"
                          dataKey="req"
                          stroke="#6366f1"           // indigo — always visible in dark mode
                          strokeWidth={2.5}
                          dot={false}               // no dots → clean line, no invisible specks
                          activeDot={{ r: 4, strokeWidth: 0, fill: '#6366f1' }}
                          isAnimationActive={false}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </div>

              {/* Status Codes / min — Multi Line Chart (exactly like req/min) */}
              <div className="rounded-lg border border-border bg-card p-4 shadow-sm lg:col-span-1">
                <h3 className="mb-3 text-xs font-semibold text-foreground uppercase tracking-wide">
                  Status Codes / min
                </h3>
                {isLoading || statusData.length === 0 ? (
                  <div className="h-56 flex items-center justify-center text-xs text-muted-foreground">
                    {isLoading ? "Loading…" : "No data"}
                  </div>
                ) : (
                  <div className="h-56">
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
                          width={36}
                          allowDecimals={false}
                        />
                        <Tooltip
                          contentStyle={chartStyle.tooltip}
                          formatter={(v: any, name: any) => [
                            `${Math.round(Number(v))} req`,
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
                )}
              </div>

              {/* Avg Response Time */}
              <div className="rounded-lg border border-border bg-card p-4 shadow-sm">
                <h3 className="mb-3 text-xs font-semibold text-foreground uppercase tracking-wide">
                  Avg Response Time (ms)
                </h3>
                {isLoading || rtData.length === 0 ? (
                  <div className="h-56 flex items-center justify-center text-xs text-muted-foreground">
                    No rt= data
                  </div>
                ) : (
                  <div className="h-56">
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={rtData}>
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
                          width={50}
                          tickFormatter={(v) => `${Math.round(v)}ms`}
                        />
                        <Tooltip
                          contentStyle={chartStyle.tooltip}
                          formatter={(v: any) => [`${Number(v).toFixed(1)}ms`, 'Avg RT']}
                        />
                        <Line
                          type="monotone"
                          dataKey="avg_rt"
                          stroke="#a78bfa"          // violet — visible in dark mode
                          strokeWidth={2.5}
                          dot={false}
                          activeDot={{ r: 4, strokeWidth: 0, fill: '#a78bfa' }}
                          isAnimationActive={false}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  </div>
                )}
              </div>
            </div>

            {/* ── Paths section ─────────────────────────────────────────── */}
            <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
              {/* Section header */}
              <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
                <div>
                  <h3 className="text-sm font-semibold text-foreground">Paths This IP Hit</h3>
                  <p className="mt-0.5 text-xs text-muted-foreground">
                    Top paths sorted by frequency • {timeRange.label} window
                  </p>
                </div>

                {/* Method filter pills */}
                {availableMethods.length > 0 && (
                  <div className="flex items-center gap-1.5 flex-wrap">
                    <span className="text-xs text-muted-foreground">Method:</span>
                    <button
                      onClick={() => setMethodFilter('all')}
                      className={cn(
                        'px-2 py-0.5 text-[10px] rounded-md border font-mono transition',
                        methodFilter === 'all'
                          ? 'bg-primary text-primary-foreground border-primary'
                          : 'bg-secondary text-muted-foreground border-border hover:text-foreground',
                      )}
                    >
                      ALL
                    </button>
                    {availableMethods.map((m) => (
                      <button
                        key={m}
                        onClick={() => setMethodFilter(m)}
                        className={cn(
                          'px-2 py-0.5 text-[10px] rounded-md border font-mono transition',
                          methodFilter === m
                            ? 'bg-primary text-primary-foreground border-primary'
                            : cn(
                                METHOD_COLORS[m] ?? 'bg-secondary text-muted-foreground border-border',
                                'hover:opacity-80',
                              ),
                        )}
                      >
                        {m}
                      </button>
                    ))}
                    <div className="flex items-center gap-1 text-[11px] text-muted-foreground ml-1">
                      <Clock className="w-3 h-3" />
                      {timeRange.label}
                    </div>
                  </div>
                )}
              </div>

              {/* Path rows */}
              {isLoading ? (
                <p className="py-12 text-center text-sm text-muted-foreground">Loading paths…</p>
              ) : !filteredPaths.length ? (
                <p className="py-12 text-center text-sm text-muted-foreground">
                  No requests found for this IP in the selected range.
                </p>
              ) : (
                <div className="space-y-1.5 max-h-[520px] overflow-y-auto pr-1">
                  {filteredPaths.map((p, i) => (
                    <PathRow
                      key={`${p.method}__${p.path}__${i}`}
                      path={p.path}
                      method={p.method}
                      count={Math.round(p.count)}
                      errors={Math.round(p.errors)}
                      avgRt={p.avg_rt}
                      statusBreakdown={
                        // The backend now returns status_breakdown as a map.
                        // Fall back gracefully if older backend version sends nothing.
                        (p as any).status_breakdown ?? {}
                      }
                      timeWindow={timeRange.label}
                    />
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  )
}