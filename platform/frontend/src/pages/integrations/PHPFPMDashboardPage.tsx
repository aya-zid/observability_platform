import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useNavigate, useParams } from "react-router-dom"
import {
  Area, AreaChart, Bar, BarChart, CartesianGrid, Cell, Legend, Line, LineChart,
  Pie, PieChart, ResponsiveContainer, Tooltip, XAxis, YAxis,
} from "recharts"

import { getDiscoveryService } from "@/api/discovery"
import { getPHPFPMDashboardSummary, type PHPFPMDashboardSummary, type PHPFPMTimePoint, type ScriptStat, type LabelCount } from "@/api/phpfpm"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  dateTimeInputToSeconds, formatBytes, formatDateInputValue, formatDuration,
  formatNumber, formatPercent, formatTimeInputValue, getNowSeconds, getStartSeconds, secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation, LangToggle } from "@/i18n/LanguageContext"

const TIME_RANGES = [
  { label: "15m", seconds: 900 },
  { label: "1h", seconds: 3600 },
  { label: "6h", seconds: 21600 },
  { label: "24h", seconds: 86400 },
] as const

function getStep(rangeSeconds: number) {
  if (rangeSeconds <= 900) return "15s"
  if (rangeSeconds <= 3600) return "30s"
  if (rangeSeconds <= 21600) return "2m"
  return "5m"
}

type TimeSelection =
  | { mode: "preset"; preset: (typeof TIME_RANGES)[number] }
  | { mode: "manual"; date: string; startTime: string; endTime: string }

function createDefaultManualSelection(): TimeSelection {
  const end = new Date()
  const start = new Date(end.getTime() - 60 * 60 * 1000)
  return { mode: "manual", date: formatDateInputValue(end), startTime: formatTimeInputValue(start), endTime: formatTimeInputValue(end) }
}

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const end = getNowSeconds()
    const start = getStartSeconds(selection.preset.seconds)
    return { start, end, step: getStep(selection.preset.seconds), error: "", isManual: false }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null) return { start: 0, end: 0, step: "30s", error: "Use DD-MM-YYYY and HH:mm.", isManual: true }
  if (end <= start) return { start, end, step: "30s", error: "End time must be after start time.", isManual: true }
  return { start, end, step: getStep(end - start), error: "", isManual: true }
}

function formatChartTime(unixSeconds: number) {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

function mapTimeseries(points: PHPFPMTimePoint[]): Record<string, number | string>[] {
  return (points ?? []).map((pt) => {
    const row: Record<string, number | string> = {}
    for (const [key, value] of Object.entries(pt)) {
      if (key === "time" && typeof value === "number") row.time = formatChartTime(value)
      else if (typeof value === "number" && Number.isFinite(value)) row[key] = value
    }
    return row
  })
}

function chartClickToLogs(id: string, chartTime: string) {
  const now = getNowSeconds()
  const dayStart = now - (now % 86400)
  const parts = chartTime.split(":")
  if (parts.length === 2) {
    const h = parseInt(parts[0], 10)
    const m = parseInt(parts[1], 10)
    if (!isNaN(h) && !isNaN(m)) {
      const clickSec = dayStart + h * 3600 + m * 60
      const startNs = (clickSec - 300) * 1e9
      const endNs = (clickSec + 300) * 1e9
      return `/integrations/phpfpm-logs/${id}?start=${Math.floor(startNs)}&end=${Math.floor(endNs)}`
    }
  }
  return `/integrations/phpfpm-logs/${id}`
}

const C = {
  blue: "#0ea5e9", green: "#22c55e", amber: "#f59e0b", red: "#ef4444",
  violet: "#8b5cf6", teal: "#14b8a6", rose: "#f43f5e", cyan: "#06b6d4",
  indigo: "#6366f1", orange: "#f97316", slate: "#64748b", emerald: "#10b981",
}

const chartStyle = {
  grid: "hsl(var(--border))",
  tick: { fill: "#94a3b8", fontSize: 10 },
  tooltip: {
    background: "hsl(var(--card))",
    border: "1px solid hsl(var(--border))",
    borderRadius: "6px",
    color: "hsl(var(--foreground))",
    fontSize: "12px",
  },
}

function StatCard({ label, value, hint, tone = "neutral", icon }: {
  label: string; value: string; hint?: string; tone?: "neutral" | "good" | "warn" | "bad"; icon?: string
}) {
  const tones = {
    neutral: "border-border bg-card",
    good: "border-emerald-300/50 bg-emerald-500/10",
    warn: "border-amber-300/50 bg-amber-500/10",
    bad: "border-red-300/50 bg-red-500/10",
  }
  return (
    <div className={cn("rounded-lg border p-4", tones[tone])}>
      <div className="flex items-center gap-1.5 text-xs uppercase tracking-wide text-muted-foreground">
        {icon && <span>{icon}</span>}{label}
      </div>
      <div className="mt-2 text-2xl font-semibold tabular-nums">{value}</div>
      {hint ? <div className="mt-1 text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  )
}

function SectionHeader({ title, description }: { title: string; description?: string }) {
  return (
    <div className="mb-1 mt-6">
      <h2 className="text-base font-semibold">{title}</h2>
      {description && <p className="text-xs text-muted-foreground">{description}</p>}
    </div>
  )
}

function NoData() {
  const { t } = useTranslation()
  return <div className="flex h-[220px] items-center justify-center text-sm text-muted-foreground">{t("shared.noData")}</div>
}

function LinePanel({ title, data, lines, formatter, height = 220, onClickTime }: {
  title: string; data: Record<string, number | string>[]; lines: Array<{ key: string; name: string; color: string }>; formatter?: (value: number, key: string) => string; height?: number; onClickTime?: (timeLabel: string) => void
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={height}>
          <LineChart data={data} onClick={onClickTime ? (e) => { if (e?.activeLabel) onClickTime(String(e.activeLabel)) } : undefined}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.5} />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} />
            <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <Tooltip contentStyle={chartStyle.tooltip} formatter={(v, n) => typeof v === "number" ? formatter?.(v, String(n)) ?? formatNumber(v) : v} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {lines.map((l) => <Line key={l.key} type="monotone" dataKey={l.key} name={l.name} stroke={l.color} strokeWidth={2} dot={false} />)}
          </LineChart>
        </ResponsiveContainer>
      ) : <NoData />}
      {onClickTime ? <div className="mt-1 text-[10px] text-muted-foreground">Click a data point to view logs</div> : null}
    </div>
  )
}

function AreaPanel({ title, data, areas, formatter, height = 220, stacked = false, onClickTime }: {
  title: string; data: Record<string, number | string>[]; areas: Array<{ key: string; name: string; color: string }>; formatter?: (value: number, key: string) => string; height?: number; stacked?: boolean; onClickTime?: (timeLabel: string) => void
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={height}>
          <AreaChart data={data} onClick={onClickTime ? (e) => { if (e?.activeLabel) onClickTime(String(e.activeLabel)) } : undefined}>
            <defs>
              {areas.map((a) => (
                <linearGradient key={a.key} id={`grad-${a.key}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={a.color} stopOpacity={0.3} />
                  <stop offset="95%" stopColor={a.color} stopOpacity={0.02} />
                </linearGradient>
              ))}
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.5} />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} />
            <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <Tooltip contentStyle={chartStyle.tooltip} formatter={(v, n) => typeof v === "number" ? formatter?.(v, String(n)) ?? formatNumber(v) : v} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {areas.map((a) => (
              <Area key={a.key} type="monotone" dataKey={a.key} name={a.name} stroke={a.color} fill={`url(#grad-${a.key})`} strokeWidth={2} dot={false} stackId={stacked ? "s" : undefined} />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      ) : <NoData />}
      {onClickTime ? <div className="mt-1 text-[10px] text-muted-foreground">Click a data point to view logs at that time</div> : null}
    </div>
  )
}

function BarPanel({ title, data, formatter, height = 200 }: {
  title: string; data: { label: string; count: number }[]; formatter?: (v: number) => string; height?: number
}) {
  const maxVal = Math.max(...data.map((d) => d.count), 1)
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <div className="space-y-2" style={{ minHeight: height }}>
          {data.map((d) => (
            <div key={d.label} className="flex items-center gap-3">
              <span className="w-32 truncate text-xs text-muted-foreground text-right shrink-0" title={d.label}>{d.label}</span>
              <div className="flex-1 h-5 rounded bg-muted overflow-hidden">
                <div className="h-full rounded bg-sky-500/70 transition-all" style={{ width: `${Math.max((d.count / maxVal) * 100, 2)}%` }} />
              </div>
              <span className="w-20 text-right text-xs tabular-nums text-muted-foreground shrink-0">{formatter ? formatter(d.count) : d.count.toLocaleString()}</span>
            </div>
          ))}
        </div>
      ) : <NoData />}
    </div>
  )
}

function HorzBarChart({ title, data, formatter, color = C.blue, height = 250 }: {
  title: string; data: { label: string; count: number }[]; formatter?: (v: number) => string; color?: string; height?: number
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={height}>
          <BarChart data={data} layout="vertical" margin={{ left: 100, right: 20 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.5} horizontal={false} />
            <XAxis type="number" tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <YAxis type="category" dataKey="label" tick={chartStyle.tick} tickLine={false} axisLine={false} width={90} tickFormatter={(v) => v.length > 25 ? v.slice(0, 22) + "..." : v} />
            <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => typeof v === "number" ? (formatter ? formatter(v) : formatNumber(v)) : v} />
            <Bar dataKey="count" fill={color} radius={[0, 3, 3, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : <NoData />}
    </div>
  )
}

function DonutCard({ title, value, label, color, hint }: {
  title: string; value: number; label: string; color: string; hint?: string
}) {
  const rest = Math.max(0, 100 - value)
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-2 text-sm font-semibold">{title}</h3>
      <div className="flex items-center gap-4">
        <PieChart width={80} height={80}>
          <Pie data={[{ value }, { value: rest }]} cx={35} cy={35} innerRadius={22} outerRadius={35} startAngle={90} endAngle={-270} dataKey="value" stroke="none">
            <Cell fill={color} />
            <Cell fill="hsl(var(--border))" />
          </Pie>
        </PieChart>
        <div>
          <div className="text-2xl font-semibold tabular-nums" style={{ color }}>{formatPercent(value)}</div>
          <div className="text-xs text-muted-foreground">{label}</div>
          {hint && <div className="mt-0.5 text-xs text-muted-foreground">{hint}</div>}
        </div>
      </div>
    </div>
  )
}

function TimeRangeSelector({ selected, onChange }: { selected: TimeSelection; onChange: (value: TimeSelection) => void }) {
  const { t } = useTranslation()
  const manual = selected.mode === "manual" ? selected : createDefaultManualSelection()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex rounded-lg border border-border bg-card p-1">
        {TIME_RANGES.map((range) => (
          <button key={range.label} type="button" onClick={() => onChange({ mode: "preset", preset: range })}
            className={cn("rounded-md px-3 py-1.5 text-sm transition-colors",
              selected.mode === "preset" && selected.preset.label === range.label ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted")}>
            {range.label}
          </button>
        ))}
        <button type="button" onClick={() => onChange(manual)}
          className={cn("rounded-md px-3 py-1.5 text-sm transition-colors",
            selected.mode === "manual" ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted")}>
          {t("shared.manual")}
        </button>
      </div>
      {selected.mode === "manual" && (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-1">
          <input type="text" placeholder="DD-MM-YYYY" value={selected.date} onChange={(e) => onChange({ ...selected, date: e.target.value })} className="h-8 w-28 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.startTime} onChange={(e) => onChange({ ...selected, startTime: e.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.endTime} onChange={(e) => onChange({ ...selected, endTime: e.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
        </div>
      )}
    </div>
  )
}

export function PHPFPMDashboardPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()
  const navigate = useNavigate()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [activeTab, setActiveTab] = useState("overview")

  const serviceQuery = useQuery({ queryKey: ["discovery-service", id], queryFn: () => getDiscoveryService(id), enabled: !!id })
  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const dashQuery = useQuery<PHPFPMDashboardSummary>({
    queryKey: ["phpfpm-dashboard-summary", id, timeWindow.start, timeWindow.end],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getPHPFPMDashboardSummary(id, { start: secondsToNanos(timeWindow.start), end: secondsToNanos(timeWindow.end), step: timeWindow.step }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 15_000,
    refetchOnWindowFocus: false,
  })

  const data = dashQuery.data
  const service = serviceQuery.data

  const workers = mapTimeseries(data?.workers_timeseries ?? [])
  const queue = mapTimeseries(data?.queue_timeseries ?? [])
  const throughput = mapTimeseries(data?.throughput_timeseries ?? [])
  const duration = mapTimeseries(data?.duration_timeseries ?? [])
  const memory = mapTimeseries(data?.memory_timeseries ?? [])
  const cpu = mapTimeseries(data?.cpu_timeseries ?? [])
  const statusTS = mapTimeseries(data?.status_timeseries ?? [])
  const reqRateTS = mapTimeseries(data?.req_rate_timeseries ?? [])
  const opcacheHitTS = mapTimeseries(data?.opcache_hit_rate_timeseries ?? [])
  const opcacheMemTS = mapTimeseries(data?.opcache_memory_timeseries ?? [])
  const opcacheScriptTS = mapTimeseries(data?.opcache_scripts_timeseries ?? [])

  const pools = data?.pools ?? []

  const tabs = [
    { key: "overview", label: "Overview" },
    ...pools.map((p) => ({ key: `pool-${p}`, label: p })),
  ]

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">{t("shared.loading")}</div>
  if (!service) return <div className="p-6 text-sm text-red-600">{t("shared.serviceNotFound", id)}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("phpfpmDashboard.title")}
        description={t("phpfpmDashboard.description", service.service_name, service.hostname || service.machine_id)}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <TimeRangeSelector selected={timeSelection} onChange={setTimeSelection} />
            <LangToggle />
            <button type="button" onClick={() => dashQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted" title="Refresh">
              <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/integrations/phpfpm-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              View Logs
            </Link>
          </div>
        }
      />

      {dashQuery.isError && (
        <SectionCard title={t("phpfpmDashboard.dashUnavailable")}>
          <div className="text-sm text-red-600">{t("phpfpmDashboard.fetchFailed")}</div>
        </SectionCard>
      )}
      {timeWindow.error && (
        <SectionCard title={t("phpfpmDashboard.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      )}

      {/* ── Tab Navigation ── */}
      <div className="mt-4 flex gap-1 rounded-lg border border-border bg-card p-1">
        {tabs.map((tab) => (
          <button key={tab.key} type="button" onClick={() => setActiveTab(tab.key)}
            className={cn("rounded-md px-4 py-2 text-sm font-medium transition-colors",
              activeTab === tab.key ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted")}>
            {tab.label}
          </button>
        ))}
      </div>

      {/* ════════════════════════════════════════════════════════════════ */}
      {/* TAB: OVERVIEW (merged)                                       */}
      {/* ════════════════════════════════════════════════════════════════ */}
      {activeTab === "overview" && (
        <>
          {/* ── STATUS ROW ── */}
          <SectionHeader title="Status" description="Instance health and exporter connectivity" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <StatCard label="PHP-FPM" value={data?.up === 1 ? "Online" : data?.up === 0 ? "Down" : "-"}
              tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint="phpfpm_up" />
            <StatCard label="Scrape Health" value={(data?.scrape_failures ?? 0) === 0 ? "Healthy" : `${formatNumber(data!.scrape_failures!, 0)} failed`}
              tone={(data?.scrape_failures ?? 0) > 0 ? "bad" : "good"} hint="phpfpm_scrape_failures_total" />
            <StatCard label="Uptime" value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} hint="Since pool start" />
            <StatCard label="Total Accepted" value={data?.accepted_connections != null ? formatNumber(data.accepted_connections, 0) : "-"}
              hint={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : undefined} />
          </div>

          {/* ── HEALTH SCORE ── */}
          {data?.health_score != null ? (
            <div className={cn("mt-3 rounded-xl border p-5", data.health_score >= 80 ? "border-emerald-300/50 bg-emerald-500/10" : data.health_score >= 50 ? "border-amber-300/50 bg-amber-500/10" : "border-red-300/50 bg-red-500/10")}>
              <div className="flex items-center gap-4">
                <div className={cn("text-5xl font-bold tabular-nums", data.health_score >= 80 ? "text-emerald-600 dark:text-emerald-400" : data.health_score >= 50 ? "text-amber-600 dark:text-amber-400" : "text-red-600 dark:text-red-400")}>
                  {data.health_score}/100
                </div>
                <div>
                  <div className="text-base font-semibold">Pool Health</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    {data.health_score >= 80 ? "Healthy — all metrics normal" :
                     data.health_score >= 50 ? "Degraded — one or more metrics need attention" :
                     "Critical — immediate action required"}
                  </div>
                </div>
              </div>
            </div>
          ) : null}

          {/* ── POOL SATURATION RANKING ── */}
          {pools.length > 1 && data?.pool_details && (
            <>
              <SectionHeader title="Pool Saturation Ranking" description="Quick overview of which pools are stressed" />
              <div className="rounded-lg border border-border bg-card p-4">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-left text-muted-foreground border-b border-border">
                      <th className="pb-3 pr-4">Pool</th>
                      <th className="pb-3 pr-4 text-right">Saturation</th>
                      <th className="pb-3 pr-4 text-right">Active</th>
                      <th className="pb-3 pr-4 text-right">Idle</th>
                      <th className="pb-3 text-right">Req/s</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pools.map((pool) => {
                      const pd = data.pool_details![pool]
                      return (
                        <tr key={pool} className="border-b border-border/50">
                          <td className="py-2 pr-4 font-mono">{pool}</td>
                          <td className={cn("py-2 pr-4 text-right tabular-nums", (pd?.saturation_pct ?? 0) > 85 ? "text-red-500" : (pd?.saturation_pct ?? 0) > 70 ? "text-amber-500" : "")}>
                            {pd?.saturation_pct != null ? formatPercent(pd.saturation_pct) : "-"}
                          </td>
                          <td className="py-2 pr-4 text-right tabular-nums">{pd?.active_processes != null ? formatNumber(pd.active_processes, 0) : "-"}</td>
                          <td className="py-2 pr-4 text-right tabular-nums">{pd?.idle_processes != null ? formatNumber(pd.idle_processes, 0) : "-"}</td>
                          <td className="py-2 text-right tabular-nums">{pd?.accepted_conn_per_sec != null ? formatNumber(pd.accepted_conn_per_sec, 2) : "-"}</td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {/* ── WORKER POOL ── */}
          <SectionHeader title="Worker Pool" description="Active/idle/total workers and pool saturation" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <StatCard label="Active Workers" value={data?.active_processes != null ? formatNumber(data.active_processes, 0) : "-"} hint="Currently handling requests" />
            <StatCard label="Idle Workers" value={data?.idle_processes != null ? formatNumber(data.idle_processes, 0) : "-"} hint="Waiting for work" />
            <StatCard label="Total Workers" value={data?.total_processes != null ? formatNumber(data.total_processes, 0) : "-"}
              hint={data?.max_active_processes != null ? `Max active: ${formatNumber(data.max_active_processes, 0)}` : undefined} />
            <DonutCard title="Saturation" value={data?.worker_saturation_pct ?? 0}
              color={(data?.worker_saturation_pct ?? 0) > 85 ? C.red : (data?.worker_saturation_pct ?? 0) > 70 ? C.amber : C.green}
              label="active / total × 100" hint=">85% = add workers" />
          </div>
          <div className="grid gap-4">
            <AreaPanel title="Workers Over Time" data={workers}
              areas={[
                { key: "active", name: "Active", color: C.green },
                { key: "idle", name: "Idle", color: C.blue },
                { key: "total", name: "Total", color: C.slate },
              ]} />
          </div>

          {/* ── QUEUE & THROUGHPUT ── */}
          <SectionHeader title="Queue & Throughput" description="Listen queue depth and connection rate" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <StatCard label="Queue Depth" value={data?.listen_queue != null ? formatNumber(data.listen_queue, 0) : "-"}
              tone={(data?.listen_queue ?? 0) > 0 ? "bad" : "good"} hint="Requests waiting for a free worker" />
            <StatCard label="Max Queue" value={data?.listen_queue_len != null ? formatNumber(data.listen_queue_len, 0) : "-"} hint="Configured listen.queue_max" />
            <DonutCard title="Queue Utilization" value={data?.queue_utilization_pct ?? 0}
              color={(data?.queue_utilization_pct ?? 0) > 50 ? C.red : (data?.queue_utilization_pct ?? 0) > 0 ? C.amber : C.green}
              label="queue / max × 100" hint="Sustained queue = workers saturated" />
            <StatCard label="Throughput" value={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : "-"} hint="Accepted connections per second" />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <AreaPanel title="Queue Depth Over Time" data={queue}
              areas={[{ key: "queue", name: "Queue Depth", color: C.amber }, { key: "max", name: "Max Queue", color: C.red }]} />
            <AreaPanel title="Throughput Over Time" data={throughput} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[{ key: "accepted", name: "Accepted/s", color: C.green }]} />
          </div>

          {/* ── REQUEST PERFORMANCE ── */}
          <SectionHeader title="Request Performance" description="Duration, memory, CPU, and latency percentiles from metrics + access logs" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <StatCard label="Avg Duration" value={data?.avg_duration_ms != null ? `${formatNumber(data.avg_duration_ms, 1)}ms` : "-"}
              tone={(data?.avg_duration_ms ?? 0) > 1000 ? "bad" : (data?.avg_duration_ms ?? 0) > 500 ? "warn" : "good"} />
            <StatCard label="Avg Memory" value={data?.avg_memory_kb != null ? `${formatNumber(data.avg_memory_kb, 0)} KB` : data?.avg_memory_bytes != null ? formatBytes(data.avg_memory_bytes) : "-"}
              hint={data?.avg_cpu_pct != null ? `CPU: ${formatNumber(data.avg_cpu_pct, 2)}%` : undefined} />
            <StatCard label="Slow Requests" value={data?.slow_requests != null ? formatNumber(data.slow_requests, 0) : "-"}
              tone={(data?.slow_requests ?? 0) > 0 ? "bad" : "good"} hint="Cumulative slowlog violations" />
            <StatCard label="Max Children Reached" value={data?.max_children_reached != null ? formatNumber(data.max_children_reached, 0) : "-"}
              tone={(data?.max_children_reached ?? 0) > 0 ? "bad" : "good"} hint="Pool capacity exhausted" />
          </div>
          <div className="grid gap-4 xl:grid-cols-3">
            <AreaPanel title="Duration Over Time" data={duration} formatter={(v) => `${formatNumber(v, 1)}ms`}
              areas={[{ key: "duration", name: "Duration (ms)", color: C.blue }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, timeLabel))} />
            <AreaPanel title="Memory Per Process" data={memory} formatter={(v) => formatBytes(v)}
              areas={[{ key: "memory", name: "Memory (bytes)", color: C.violet }]} />
            <AreaPanel title="CPU Per Process" data={cpu} formatter={(v) => `${formatNumber(v, 2)}%`}
              areas={[{ key: "cpu", name: "CPU (%)", color: C.amber }]} />
          </div>

          {data?.latency_p50 != null ? (
            <>
              <SectionHeader title="Latency Percentiles" description="From access log duration field (p50/p95/p99)" />
              <div className="grid gap-3 md:grid-cols-3">
                <StatCard label="p50 Latency" value={`${formatNumber(data.latency_p50, 1)}ms`} hint="Median" />
                <StatCard label="p95 Latency" value={`${formatNumber(data.latency_p95 ?? 0, 1)}ms`}
                  tone={(data.latency_p95 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 20 slower" />
                <StatCard label="p99 Latency" value={`${formatNumber(data.latency_p99 ?? 0, 1)}ms`}
                  tone={(data.latency_p99 ?? 0) > 2000 ? "bad" : (data.latency_p99 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 100 slower" />
              </div>
            </>
          ) : null}

          {/* ── TRAFFIC & STATUS CODES ── */}
          <SectionHeader title="Request Traffic" description="Request rate and status code breakdown from access logs" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <StatCard label="2xx Rate" value={data?.status_2xx != null ? `${formatNumber(data.status_2xx, 2)}/s` : "-"} tone="good" />
            <StatCard label="4xx Rate" value={data?.status_4xx != null ? `${formatNumber(data.status_4xx, 2)}/s` : "-"} tone={(data?.status_4xx ?? 0) > 1 ? "warn" : "neutral"} />
            <StatCard label="5xx Rate" value={data?.status_5xx != null ? `${formatNumber(data.status_5xx, 2)}/s` : "-"} tone={(data?.status_5xx ?? 0) > 0 ? "bad" : "neutral"} />
            <StatCard label="Request Rate" value={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : "-"} hint="From VM metrics" />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <AreaPanel title="Status Codes Over Time" data={statusTS} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[
                { key: "2xx", name: "2xx", color: C.green },
                { key: "4xx", name: "4xx", color: C.amber },
                { key: "5xx", name: "5xx", color: C.red },
              ]} />
            <AreaPanel title="Request Rate Over Time" data={reqRateTS} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[{ key: "req", name: "Requests/s", color: C.blue }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, timeLabel))} />
          </div>

          {(data?.top_uris ?? []).length > 0 ? (
            <>
              <SectionHeader title="Top URIs" description="Most frequently accessed URIs from access logs" />
              <div className="grid gap-4 xl:grid-cols-2">
                <HorzBarChart title="By Request Count" data={(data?.top_uris ?? []).map((u: LabelCount) => ({ label: u.label, count: u.count }))}
                  formatter={(v) => formatNumber(v, 0)} color={C.blue} />
                {(data?.top_error_uris ?? []).length > 0 ? (
                  <HorzBarChart title="By Error Count (4xx/5xx)" data={(data?.top_error_uris ?? []).map((u: LabelCount) => ({ label: u.label, count: u.count }))}
                    formatter={(v) => formatNumber(v, 0)} color={C.red} />
                ) : null}
              </div>
            </>
          ) : null}

          {/* ── ERRORS & SIGNALS ── */}
          <SectionHeader title="Errors & Signals" description="Warning/error events and slow request traces from logs" />
          <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
            <StatCard label="Warnings" value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
            <StatCard label="Errors" value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
            <StatCard label="Slow Request Traces" value={formatNumber(data?.log_summary.slow_requests ?? 0, 0)} tone={(data?.log_summary.slow_requests ?? 0) > 0 ? "warn" : "good"} />
            <StatCard label="Notices" value={formatNumber(data?.log_summary.notices ?? 0, 0)} />
            <StatCard label="Startups" value={formatNumber(data?.log_summary.startups ?? 0, 0)} />
          </div>

          {/* ── TOP SCRIPTS ── */}
          {(data?.top_slow_scripts ?? []).length > 0 || (data?.top_mem_scripts ?? []).length > 0 ? (
            <>
              <SectionHeader title="Top Scripts" description="Worst-performing scripts from access logs" />
              <div className="grid gap-4 xl:grid-cols-2">
                <BarPanel title="Slowest Scripts (avg ms)" data={(data?.top_slow_scripts ?? []).map((s: ScriptStat) => ({ label: s.script, count: Math.round(s.avg_duration_ms) }))} formatter={(v) => `${v}ms`} />
                <BarPanel title="Highest Memory Scripts (avg KB)" data={(data?.top_mem_scripts ?? []).map((s: ScriptStat) => ({ label: s.script, count: Math.round(s.avg_memory_kb) }))} formatter={(v) => `${formatNumber(v, 0)} KB`} />
              </div>
            </>
          ) : null}

          {/* ── LOG ANALYTICS ── */}
          {(data?.method_distribution ?? []).length > 0 && (
            <>
              <SectionHeader title="Log Analytics" description="Request method distribution, duration buckets, and error breakdown from access + error logs" />
              <div className="grid gap-4 xl:grid-cols-2">
                <BarPanel title="Method Distribution" data={(data?.method_distribution ?? []).map((m: LabelCount) => ({ label: m.label, count: m.count }))} />
                <BarPanel title="Status Distribution" data={(data?.status_distribution ?? []).map((s: LabelCount) => ({ label: s.label, count: s.count }))} />
              </div>
              <div className="grid gap-4 xl:grid-cols-2">
                <BarPanel title="Duration Histogram" data={(data?.duration_histogram ?? []).map((b: LabelCount) => ({ label: b.label, count: b.count }))} />
                <BarPanel title="Memory Histogram" data={(data?.memory_histogram ?? []).map((b: LabelCount) => ({ label: b.label, count: b.count }))} formatter={(v) => formatNumber(v, 0)} />
              </div>
              <div className="grid gap-4 xl:grid-cols-2">
                <BarPanel title="CPU Histogram" data={(data?.cpu_histogram ?? []).map((b: LabelCount) => ({ label: b.label, count: b.count }))} formatter={(v) => formatNumber(v, 0)} />
              </div>
            </>
          )}
          {(data?.error_type_breakdown ?? []).length > 0 && (
            <>
              <SectionHeader title="Error Log Analytics" description="Error type breakdown and most frequent error messages" />
              <div className="grid gap-4 xl:grid-cols-2">
                <BarPanel title="Error Type Breakdown" data={(data?.error_type_breakdown ?? []).map((e: LabelCount) => ({ label: e.label, count: e.count }))} />
                {(data?.top_error_messages ?? []).length > 0 && (
                  <BarPanel title="Top Error Messages" data={(data?.top_error_messages ?? []).map((e: LabelCount) => ({ label: e.label.length > 50 ? e.label.slice(0, 47) + "..." : e.label, count: e.count }))} />
                )}
              </div>
            </>
          )}

          {/* ── OPCACHE ── */}
          {data?.opcache_enabled != null && (
            <>
              <SectionHeader title="OPcache" description="Cache performance, memory usage, and script statistics" />
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
                <DonutCard title="Hit Rate" value={(data.opcache_hit_rate ?? 0) * 100}
                  color={(data.opcache_hit_rate ?? 0) >= 0.9 ? C.green : (data.opcache_hit_rate ?? 0) >= 0.8 ? C.amber : C.red}
                  label={`${formatNumber((data.opcache_hit_rate ?? 0) * 100, 1)}%`} hint={data.opcache_hit_rate != null && data.opcache_hit_rate < 0.9 ? "Target ≥ 90%" : undefined} />
                <DonutCard title="Memory Used" value={(data.opcache_memory_used_ratio ?? 0) * 100}
                  color={(data.opcache_memory_used_ratio ?? 0) > 0.8 ? C.red : (data.opcache_memory_used_ratio ?? 0) > 0.6 ? C.amber : C.green}
                  label={data.opcache_memory_total_bytes != null ? formatBytes(data.opcache_memory_total_bytes) : ""}
                  hint={data.opcache_memory_free_bytes != null ? `${formatBytes(data.opcache_memory_free_bytes)} free` : undefined} />
                <StatCard label="Cached Scripts" value={data.opcache_cached_scripts != null ? formatNumber(data.opcache_cached_scripts, 0) : "-"} />
                <StatCard label="Cache Hits" value={data.opcache_hits_total != null ? formatNumber(data.opcache_hits_total, 0) : "-"}
                  hint={data.opcache_misses_total != null ? `${formatNumber(data.opcache_misses_total, 0)} misses` : undefined} />
                <StatCard label="Restarts" value={data.opcache_restarts_total != null ? formatNumber(data.opcache_restarts_total, 0) : "-"}
                  tone={(data.opcache_restarts_total ?? 0) > 0 ? "warn" : "good"} hint="OOM restarts = cache thrashing" />
              </div>
              <SectionHeader title="OPcache Timeseries" description="Hit rate, memory usage, and cached scripts over time" />
              <div className="grid gap-4 xl:grid-cols-3">
                <LinePanel title="Hit Rate Over Time" data={opcacheHitTS}
                  formatter={(v) => `${formatNumber(v * 100, 1)}%`}
                  lines={[{ key: "hit_rate", name: "Hit Rate", color: C.green }]} />
                <LinePanel title="Memory Used Ratio" data={opcacheMemTS}
                  formatter={(v) => `${formatNumber(v * 100, 1)}%`}
                  lines={[{ key: "used_ratio", name: "Used Ratio", color: C.blue }]} />
                <AreaPanel title="Cached Scripts" data={opcacheScriptTS}
                  formatter={(v) => formatNumber(v, 0)}
                  areas={[{ key: "cached", name: "Cached", color: C.violet }]} />
              </div>
            </>
          )}
        </>
      )}

      {/* ════════════════════════════════════════════════════════════════ */}
      {/* PER-POOL TABS                                                */}
      {/* ════════════════════════════════════════════════════════════════ */}
      {pools.map((pool) => {
        if (activeTab !== `pool-${pool}`) return null
        const pd = data?.pool_details?.[pool]
        return (
          <div key={pool}>
            <SectionHeader title={`Pool: ${pool}`} description="Pool-specific metrics and log analytics" />
            {pd ? (
              <>
                {/* ── POOL STAT CARDS ── */}
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <StatCard label="Active Workers" value={formatNumber(pd.active_processes ?? 0, 0)} hint="Handling requests now" />
                  <StatCard label="Idle Workers" value={formatNumber(pd.idle_processes ?? 0, 0)} hint="Waiting for work" />
                  <StatCard label="Total Workers" value={formatNumber(pd.total_processes ?? 0, 0)} />
                  <DonutCard title="Saturation" value={pd.saturation_pct ?? 0}
                    color={(pd.saturation_pct ?? 0) > 85 ? C.red : (pd.saturation_pct ?? 0) > 70 ? C.amber : C.green}
                    label="active / total × 100" hint=">85% = add workers" />
                </div>

                {/* ── REQUEST & STATUS ── */}
                <SectionHeader title="Request Stats" description="Request count, throughput, and status codes" />
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <StatCard label="Request Count" value={pd.request_count != null ? formatNumber(pd.request_count, 0) : "-"} />
                  <StatCard label="Throughput" value={pd.accepted_conn_per_sec != null ? `${formatNumber(pd.accepted_conn_per_sec, 2)}/s` : "-"} hint="Accepted connections/s" />
                  <StatCard label="Slow Requests" value={pd.slow_requests != null ? formatNumber(pd.slow_requests, 0) : "-"}
                    tone={(pd.slow_requests ?? 0) > 0 ? "warn" : "good"} />
                  <StatCard label="Max Children" value={pd.max_children_reached != null ? formatNumber(pd.max_children_reached, 0) : "-"}
                    tone={(pd.max_children_reached ?? 0) > 0 ? "bad" : "good"} hint="Capacity exhausted" />
                </div>

                {/* ── STATUS CODE BREAKDOWN ── */}
                {(pd.status_2xx != null || pd.status_4xx != null || pd.status_5xx != null) && (
                  <div className="grid gap-3 md:grid-cols-3">
                    <StatCard label="2xx" value={pd.status_2xx != null ? formatNumber(pd.status_2xx, 0) : "-"} tone="good" />
                    <StatCard label="4xx" value={pd.status_4xx != null ? formatNumber(pd.status_4xx, 0) : "-"} tone={(pd.status_4xx ?? 0) > 0 ? "warn" : "neutral"} />
                    <StatCard label="5xx" value={pd.status_5xx != null ? formatNumber(pd.status_5xx, 0) : "-"} tone={(pd.status_5xx ?? 0) > 0 ? "bad" : "neutral"} />
                  </div>
                )}

                {/* ── LATENCY PERCENTILES ── */}
                {pd.latency_p50 != null && (
                  <>
                    <SectionHeader title="Latency Percentiles" description="From access log duration" />
                    <div className="grid gap-3 md:grid-cols-3">
                      <StatCard label="p50" value={`${formatNumber(pd.latency_p50, 1)}ms`} hint="Median" />
                      <StatCard label="p95" value={`${formatNumber(pd.latency_p95 ?? 0, 1)}ms`}
                        tone={(pd.latency_p95 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 20 slower" />
                      <StatCard label="p99" value={`${formatNumber(pd.latency_p99 ?? 0, 1)}ms`}
                        tone={(pd.latency_p99 ?? 0) > 2000 ? "bad" : (pd.latency_p99 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 100 slower" />
                    </div>
                  </>
                )}

                {/* ── TOP SLOW SCRIPTS ── */}
                {(pd.top_slow_scripts ?? []).length > 0 && (
                  <>
                    <SectionHeader title="Slowest Scripts" description="Worst-performing scripts in this pool" />
                    <div className="grid gap-4 xl:grid-cols-1">
                      <BarPanel title="Slowest Scripts (avg ms)" data={pd.top_slow_scripts!.map((s: ScriptStat) => ({ label: s.script, count: Math.round(s.avg_duration_ms) }))} formatter={(v) => `${v}ms`} />
                    </div>
                  </>
                )}

                {/* ── TOP URIs ── */}
                {(pd.top_uris ?? []).length > 0 && (
                  <>
                    <SectionHeader title="Top URIs" description="Most accessed endpoints in this pool" />
                    <HorzBarChart title="By Request Count" data={pd.top_uris!.map((u: LabelCount) => ({ label: u.label, count: u.count }))}
                      formatter={(v) => formatNumber(v, 0)} color={C.blue} />
                  </>
                )}

                {/* ── ERROR SUMMARY ── */}
                {(pd.error_count != null || pd.warning_count != null || pd.notice_count != null) && (
                  <>
                    <SectionHeader title="Error Summary" description="Errors, warnings, and notices from error logs" />
                    <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
                      <StatCard label="Errors" value={formatNumber(pd.error_count ?? 0, 0)} tone={(pd.error_count ?? 0) > 0 ? "bad" : "good"} />
                      <StatCard label="Warnings" value={formatNumber(pd.warning_count ?? 0, 0)} tone={(pd.warning_count ?? 0) > 0 ? "warn" : "good"} />
                      <StatCard label="Notices" value={formatNumber(pd.notice_count ?? 0, 0)} />
                      <StatCard label="Slow Traces" value={formatNumber(pd.slow_request_count ?? 0, 0)} tone={(pd.slow_request_count ?? 0) > 0 ? "warn" : "good"} />
                    </div>
                  </>
                )}

                {/* ── TOP ERROR MESSAGES ── */}
                {(pd.top_error_messages ?? []).length > 0 && (
                  <>
                    <SectionHeader title="Top Error Messages" description="Most frequent error messages in this pool" />
                    <div className="rounded-lg border border-border bg-card p-4">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="text-left text-muted-foreground border-b border-border">
                            <th className="pb-3 pr-4">Message</th>
                            <th className="pb-3 text-right">Count</th>
                          </tr>
                        </thead>
                        <tbody>
                          {pd.top_error_messages!.map((m: LabelCount, i: number) => (
                            <tr key={i} className="border-b border-border/50">
                              <td className="py-2 pr-4 font-mono text-xs max-w-[500px] truncate" title={m.label}>{m.label}</td>
                              <td className="py-2 text-right tabular-nums">{formatNumber(m.count, 0)}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </>
                )}

              </>
            ) : (
              <div className="text-sm text-muted-foreground py-4">No pool-specific data available yet.</div>
            )}
          </div>
        )
      })}
    </Page>
  )
}
