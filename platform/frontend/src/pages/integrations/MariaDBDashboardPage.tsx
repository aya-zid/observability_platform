import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { MariaDBSummaryTab } from "@/components/mariadb/MariaDBSummaryTab"
import { MariaDBConfigTab } from "@/components/mariadb/MariaDBConfigTab"
import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

import { getDiscoveryService } from "@/api/discovery"
import { getMariaDBDashboardSummary, type MariaDBDashboardSummary, type MariaDBTimePoint } from "@/api/mariadb"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  dateTimeInputToSeconds,
  formatBytes,
  formatDateInputValue,
  formatDuration,
  formatNumber,
  formatPercent,
  formatTimeInputValue,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
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

function mapTimeseries(points: MariaDBTimePoint[]): Record<string, number | string>[] {
  return (points ?? []).map((pt) => {
    const row: Record<string, number | string> = {}
    for (const [key, value] of Object.entries(pt)) {
      if (key === "time" && typeof value === "number") row.time = formatChartTime(value)
      else if (typeof value === "number" && Number.isFinite(value)) row[key] = value
    }
    return row
  })
}

const C = {
  blue: "#0ea5e9", green: "#22c55e", amber: "#f59e0b", red: "#ef4444",
  violet: "#8b5cf6", teal: "#14b8a6", rose: "#f43f5e", cyan: "#06b6d4",
  indigo: "#6366f1", orange: "#f97316",
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

function LinePanel({ title, data, lines, formatter, height = 220 }: {
  title: string; data: Record<string, number | string>[]; lines: Array<{ key: string; name: string; color: string }>; formatter?: (value: number, key: string) => string; height?: number
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={height}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.5} />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} />
            <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <Tooltip contentStyle={chartStyle.tooltip} formatter={(v, n) => typeof v === "number" ? formatter?.(v, String(n)) ?? formatNumber(v) : v} />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {lines.map((l) => <Line key={l.key} type="monotone" dataKey={l.key} name={l.name} stroke={l.color} strokeWidth={2} dot={false} />)}
          </LineChart>
        </ResponsiveContainer>
      ) : <NoData />}
    </div>
  )
}

function AreaPanel({ title, data, areas, formatter, height = 220, stacked = false }: {
  title: string; data: Record<string, number | string>[]; areas: Array<{ key: string; name: string; color: string }>; formatter?: (value: number, key: string) => string; height?: number; stacked?: boolean
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={height}>
          <AreaChart data={data}>
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
              <span className="w-32 truncate text-xs text-muted-foreground text-right shrink-0" title={d.label}>
                {d.label}
              </span>
              <div className="flex-1 h-5 rounded bg-muted overflow-hidden">
                <div
                  className="h-full rounded bg-sky-500/70 transition-all"
                  style={{ width: `${Math.max((d.count / maxVal) * 100, 2)}%` }}
                />
              </div>
              <span className="w-20 text-right text-xs tabular-nums text-muted-foreground shrink-0">
                {formatter ? formatter(d.count) : d.count.toLocaleString()}
              </span>
            </div>
          ))}
        </div>
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

function DigestsTable({ title, digests, valueFn, valueLabel, valueFormatter }: {
  title: string; digests: MariaDBDashboardSummary["top_query_digests"]; valueFn: (d: MariaDBDashboardSummary["top_query_digests"][number]) => number; valueLabel: string; valueFormatter: (v: number) => string
}) {
  const { t } = useTranslation()
  return (
    <SectionCard title={title}>
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs uppercase text-muted-foreground">
            <tr className="border-b border-border">
              <th className="py-2 pr-4 text-left">Query</th>
              <th className="py-2 pr-4 text-left">DB</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.execs")}</th>
              <th className="py-2 pr-4 text-right">{valueLabel}</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.avg")}</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.rowsExam")}</th>
              <th className="py-2 text-right">{t("mdbDashboard.rowsSent")}</th>
            </tr>
          </thead>
          <tbody>
            {digests.length ? digests.map((d, i) => (
              <tr key={d.digest || i} className="border-b border-border/60 hover:bg-muted/30">
                <td className="max-w-[360px] truncate py-2 pr-4 font-mono text-xs" title={d.digest_text || d.digest}>{d.digest_text || `${d.digest.slice(0, 20)}…`}</td>
                <td className="py-2 pr-4 font-mono text-xs">{d.database}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatNumber(d.exec_count, 0)}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{valueFormatter(valueFn(d))}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatDuration(d.avg_time_seconds)}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatNumber(d.rows_examined, 0)}</td>
                <td className="py-2 text-right tabular-nums">{formatNumber(d.rows_sent, 0)}</td>
              </tr>
            )) : (
              <tr><td colSpan={7} className="py-6 text-center text-sm text-muted-foreground">{t("mdbDashboard.noDigests")}</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </SectionCard>
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

export function MariaDBDashboardPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get("tab") as "dashboard" | "config" | "summary") || "dashboard"
  const setActiveTab = (tab: string) => setSearchParams((prev) => { prev.set("tab", tab); return prev }, { replace: true })
  const [minDurationSec, setMinDurationSec] = useState("")
  const [minDurationSecInput, setMinDurationSecInput] = useState("")

  const serviceQuery = useQuery({ queryKey: ["discovery-service", id], queryFn: () => getDiscoveryService(id), enabled: !!id })
  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const dashQuery = useQuery<MariaDBDashboardSummary>({
    queryKey: ["mariadb-dashboard-summary", id, timeWindow.start, timeWindow.end, minDurationSec],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getMariaDBDashboardSummary(id, { start: secondsToNanos(timeWindow.start), end: secondsToNanos(timeWindow.end), step: timeWindow.step, min_duration_seconds: minDurationSec ? Number(minDurationSec) : undefined }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 15_000,
    refetchOnWindowFocus: false,
  })

  const data = dashQuery.data
  const service = serviceQuery.data

  const connections = mapTimeseries(data?.connections_timeseries ?? [])
  const qps = mapTimeseries(data?.qps_timeseries ?? [])
  const slow = mapTimeseries(data?.slow_timeseries ?? [])
  const bufferPool = mapTimeseries(data?.buffer_pool_timeseries ?? [])
  const locks = mapTimeseries(data?.locks_timeseries ?? [])
  const queryTime = mapTimeseries(data?.query_time_timeseries ?? [])
  const queryCount = mapTimeseries(data?.query_count_timeseries ?? [])
  const io = mapTimeseries(data?.io_timeseries ?? [])
  const network = mapTimeseries(data?.network_timeseries ?? [])
  const tmpTables = mapTimeseries(data?.tmp_tables_timeseries ?? [])
  const rw = mapTimeseries(data?.rw_timeseries ?? [])
  const deadlocks = mapTimeseries(data?.deadlocks_timeseries ?? [])
  const handlerRead = mapTimeseries(data?.handler_read_timeseries ?? [])
  const sortOps = mapTimeseries(data?.sort_timeseries ?? [])
  const pendingIO = mapTimeseries(data?.innodb_pending_timeseries ?? [])
  const binlog = mapTimeseries(data?.binlog_timeseries ?? [])

  const totalRW = (data?.selects_per_sec ?? 0) + (data?.inserts_per_sec ?? 0) + (data?.updates_per_sec ?? 0) + (data?.deletes_per_sec ?? 0)
  const rwPieData = totalRW > 0 ? [
    { name: "SELECT", value: data?.selects_per_sec ?? 0, fill: C.blue },
    { name: "INSERT", value: data?.inserts_per_sec ?? 0, fill: C.green },
    { name: "UPDATE", value: data?.updates_per_sec ?? 0, fill: C.amber },
    { name: "DELETE", value: data?.deletes_per_sec ?? 0, fill: C.red },
  ] : []

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">{t("shared.loading")}</div>
  if (!service) return <div className="p-6 text-sm text-red-600">{t("shared.serviceNotFound", id)}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("mdbDashboard.title")}
        description={t("mdbDashboard.description", service.service_name, service.hostname || service.machine_id)}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            {activeTab === "dashboard" && <TimeRangeSelector selected={timeSelection} onChange={setTimeSelection} />}
            <LangToggle />
            <button
              type="button"
              onClick={() => dashQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/integrations/mariadb-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              View Logs
            </Link>
          </div>
        }
      />

      {/* ── Tab bar ──────────────────────────────────────── */}
      <div className="mb-6 border-b border-border">
        <div className="flex gap-0">
          {(["dashboard", "config", "summary"] as const).map((tab) => (
            <button
              key={tab}
              type="button"
              onClick={() => setActiveTab(tab)}
              className={cn(
                "px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px",
                activeTab === tab
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground",
              )}
            >
              {tab === "dashboard" && "Dashboard"}
              {tab === "config" && "Configuration"}
              {tab === "summary" && "Summary"}
            </button>
          ))}
        </div>
      </div>

      {dashQuery.isError && (
        <SectionCard title={t("mdbDashboard.dashUnavailable")}>
          <div className="text-sm text-red-600">{t("mdbDashboard.fetchFailed")}</div>
        </SectionCard>
      )}
      {timeWindow.error && (
        <SectionCard title={t("mdbDashboard.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      )}

      {activeTab === "dashboard" && (
        <>
      {/* ═══════════════════════════════ 1. STATUS BAR ═══════════════════════ */}
      <SectionHeader title="Status" description="Instance health at a glance" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="MySQL Up" icon="🔌" value={data?.up === 1 ? "Online" : data?.up === 0 ? "Down" : "-"} tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint="mysql_up — 1 = reachable" />
        <StatCard label="Exporter Scrape" icon="📡" value={data?.scrape_error === 0 ? "OK" : data?.scrape_error === 1 ? "Error" : "-"} tone={data?.scrape_error === 1 ? "bad" : data?.scrape_error === 0 ? "good" : "neutral"} hint="Last scrape success" />
        <StatCard label="Uptime" icon="⏱️" value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} hint="Time since last restart" />
        <StatCard label="Replication Lag" icon="🔁" value={data?.replication_lag_seconds != null ? formatDuration(data.replication_lag_seconds) : "N/A"} tone={(data?.replication_lag_seconds ?? 0) > 30 ? "bad" : (data?.replication_lag_seconds ?? 0) > 5 ? "warn" : "good"} hint={data?.slave_running === 1 ? "Replica running" : "No active replica"} />
      </div>
      {(data?.wsrep_cluster_size != null) && (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4 mt-2">
          <StatCard label="Galera Cluster Size" icon="🌐" value={data?.wsrep_cluster_size != null ? formatNumber(data.wsrep_cluster_size, 0) : "-"} hint="Number of Galera nodes" />
          <StatCard label="Galera Connected" icon="🔗" value={data?.wsrep_connected === 1 ? "Yes" : data?.wsrep_connected === 0 ? "No" : "-"} tone={data?.wsrep_connected === 1 ? "good" : data?.wsrep_connected === 0 ? "bad" : "neutral"} hint="Is this node connected to the cluster?" />
          <StatCard label="Galera Ready" icon="✅" value={data?.wsrep_ready === 1 ? "Yes" : data?.wsrep_ready === 0 ? "No" : "-"} tone={data?.wsrep_ready === 1 ? "good" : data?.wsrep_ready === 0 ? "bad" : "neutral"} hint="Is this node ready to accept queries?" />
        </div>
      )}

      {/* ═══════════════════════════════ 2. QUERY THROUGHPUT ════════════════ */}
      <SectionHeader title="Query Throughput" description="Queries per second, rows read/written, and read/write breakdown" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="QPS" icon="⚡" value={data?.qps != null ? `${formatNumber(data.qps, 1)}/s` : "-"} hint="Questions/sec (all statements)" />
        <StatCard label="Slow Queries/min" icon="🐢" value={data?.slow_queries_per_min != null ? formatNumber(data.slow_queries_per_min, 2) : "-"} tone={(data?.slow_queries_per_min ?? 0) > 0 ? "warn" : "good"} hint="Queries exceeding long_query_time" />
        <StatCard label="SELECTs/sec" icon="📖" value={data?.selects_per_sec != null ? `${formatNumber(data.selects_per_sec, 1)}/s` : "-"} hint="Read workload" />
        <StatCard label="Writes/sec" icon="✏️" value={data != null ? `${formatNumber((data.inserts_per_sec ?? 0) + (data.updates_per_sec ?? 0) + (data.deletes_per_sec ?? 0), 1)}/s` : "-"} hint="INSERT + UPDATE + DELETE" />
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Rows Read/sec" icon="📖" value={data?.rows_read_per_sec != null ? `${formatNumber(data.rows_read_per_sec, 0)}/s` : "-"} hint="Handler rows read (all engines)" />
        <StatCard label="Rows Modified/sec" icon="✏️" value={data?.rows_modified_per_sec != null ? `${formatNumber(data.rows_modified_per_sec, 0)}/s` : "-"} hint="Handler writes/updates/deletes" />
        <StatCard label="Sort Merge Passes/sec" icon="🔄" value={data?.sort_merge_passes_per_sec != null ? formatNumber(data.sort_merge_passes_per_sec, 2) : "-"} tone={(data?.sort_merge_passes_per_sec ?? 0) > 0.5 ? "warn" : "good"} hint="Disk-based sort merges — increase sort_buffer_size" />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <AreaPanel title="Read / Write Breakdown Over Time" data={rw} areas={[
            { key: "selects", name: "SELECT", color: C.blue },
            { key: "inserts", name: "INSERT", color: C.green },
            { key: "updates", name: "UPDATE", color: C.amber },
            { key: "deletes", name: "DELETE", color: C.red },
          ]} formatter={(v) => `${formatNumber(v, 2)}/s`} />
        </div>
        <div className="rounded-lg border border-border bg-card p-4">
          <h3 className="mb-2 text-sm font-semibold">Current R/W Mix</h3>
          {rwPieData.length ? (
            <>
              <ResponsiveContainer width="100%" height={160}>
                <PieChart>
                  <Pie data={rwPieData} cx="50%" cy="50%" innerRadius={40} outerRadius={70} dataKey="value" label={({ name, percent }) => `${name} ${(percent != null ? (percent * 100).toFixed(0) : 0)}%`} labelLine={false} fontSize={10}>
                    {rwPieData.map((d) => <Cell key={d.name} fill={d.fill} />)}
                  </Pie>
                  <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => `${formatNumber(Number(v), 2)}/s`} />
                </PieChart>
              </ResponsiveContainer>
              <div className="mt-2 grid grid-cols-2 gap-1 text-xs">
                {rwPieData.map((d) => (
                  <div key={d.name} className="flex items-center gap-1.5">
                    <div className="h-2 w-2 rounded-full" style={{ background: d.fill }} />
                    <span className="text-muted-foreground">{d.name}:</span>
                    <span className="tabular-nums">{formatNumber(d.value, 2)}/s</span>
                  </div>
                ))}
              </div>
            </>
          ) : <NoData />}
        </div>
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="QPS Over Time" data={qps} lines={[{ key: "qps", name: "QPS", color: C.green }]} formatter={(v) => `${formatNumber(v, 1)}/s`} />
        <LinePanel title="Aborted Connects & Clients / min" data={slow} lines={[
          { key: "aborted_connects_per_min", name: "Aborted Connects", color: C.red },
          { key: "aborted_clients_per_min", name: "Aborted Clients", color: C.rose },
          { key: "slow_queries_per_min", name: "Slow Queries/min", color: C.amber },
        ]} formatter={(v) => formatNumber(v, 2)} />
      </div>

      {/* ═══════════════════════════════ 3. CONNECTIONS ═════════════════════ */}
      <SectionHeader title="Connections" description="Thread pool saturation, peak usage, and error rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Connection Saturation" icon="📊" value={data?.connection_saturation_pct != null ? formatPercent(data.connection_saturation_pct) : "-"} tone={(data?.connection_saturation_pct ?? 0) > 85 ? "bad" : (data?.connection_saturation_pct ?? 0) > 70 ? "warn" : "good"} hint={`${data?.threads_connected ?? "-"} / ${data?.max_connections ?? "-"} used`} />
        <StatCard label="Threads Running" icon="⚡" value={data?.threads_running != null ? formatNumber(data.threads_running, 0) : "-"} tone={(data?.threads_running ?? 0) > (data?.max_connections ?? 100) * 0.5 ? "warn" : "neutral"} hint="Active query threads" />
        <StatCard label="Max Used Connections" icon="📈" value={data?.max_used_connections != null ? formatNumber(data.max_used_connections, 0) : "-"} hint="Peak concurrent connections since server start" />
        <StatCard label="Aborted Connects/min" icon="⛔" value={data?.aborted_connects_per_min != null ? formatNumber(data.aborted_connects_per_min, 2) : "-"} tone={(data?.aborted_connects_per_min ?? 0) > 1 ? "warn" : "good"} hint="Failed TCP handshakes or auth" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="Connections Over Time" data={connections} areas={[
          { key: "connected", name: "Connected", color: C.blue },
          { key: "running", name: "Running", color: C.green },
          { key: "cached", name: "Cached", color: C.teal },
        ]} />
      </div>

      {/* ═══════════════════════════════ 4. INNODB BUFFER POOL ═══════════════ */}
      <SectionHeader title="InnoDB Buffer Pool" description="The most important memory structure — aim for >99% hit ratio" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DonutCard title="Hit Ratio" value={data?.buffer_pool_hit_ratio_pct ?? 0} color={(data?.buffer_pool_hit_ratio_pct ?? 100) < 95 ? C.red : C.green} label="cache effectiveness" hint="< 95% = needs more memory" />
        <DonutCard title="Used" value={data?.buffer_pool_used_pct ?? 0} color={C.blue} label="of pool allocated" hint="High = pool is active" />
        <DonutCard title="Dirty Pages" value={data?.buffer_pool_dirty_pct ?? 0} color={(data?.buffer_pool_dirty_pct ?? 0) > 75 ? C.amber : C.teal} label="of data bytes modified" hint="High sustained = I/O pressure" />
        <StatCard label="Pool Size" icon="💽" value={data?.innodb_buffer_pool_size_bytes != null ? formatBytes(data.innodb_buffer_pool_size_bytes) : "-"} hint="innodb_buffer_pool_size" />
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Wait Free/sec" icon="⏳" value={data?.buffer_pool_wait_free_per_sec != null ? `${formatNumber(data.buffer_pool_wait_free_per_sec, 2)}/s` : "-"} tone={(data?.buffer_pool_wait_free_per_sec ?? 0) > 0 ? "warn" : "good"} hint="Non-zero ⇒ InnoDB waiting for free pages (pool too small)" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="Buffer Pool % Over Time" data={bufferPool} formatter={(v) => formatPercent(v)} lines={[
          { key: "hit_ratio", name: "Hit Ratio %", color: C.teal },
          { key: "dirty_pct", name: "Dirty %", color: C.amber },
          { key: "used_pct", name: "Used %", color: C.blue },
        ]} />
      </div>

      {/* ═══════════════════════════════ 4b. INNODB HEALTH ════════════════════ */}
      <SectionHeader title="InnoDB Health" description="History list length, log waits, and pending I/O indicate internal pressure" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="History List Length" icon="📜" value={data?.innodb_history_list_length != null ? formatNumber(data.innodb_history_list_length, 0) : "-"} tone={(data?.innodb_history_list_length ?? 0) > 100000 ? "bad" : (data?.innodb_history_list_length ?? 0) > 10000 ? "warn" : "good"} hint="High sustained = long-running tx blocking purge" />
        <StatCard label="Log Waits/sec" icon="📝" value={data?.innodb_log_waits_per_sec != null ? `${formatNumber(data.innodb_log_waits_per_sec, 2)}/s` : "-"} tone={(data?.innodb_log_waits_per_sec ?? 0) > 0 ? "warn" : "good"} hint="Non-zero ⇒ innodb_log_buffer_size too small" />
        <StatCard label="Pending Reads" icon="⬇️" value={data?.innodb_pending_reads != null ? formatNumber(data.innodb_pending_reads, 0) : "-"} tone={(data?.innodb_pending_reads ?? 0) > 0 ? "warn" : "good"} hint="I/O operations queued for reads" />
        <StatCard label="Pending Writes" icon="⬆️" value={data?.innodb_pending_writes != null ? formatNumber(data.innodb_pending_writes, 0) : "-"} tone={(data?.innodb_pending_writes ?? 0) > 0 ? "warn" : "good"} hint="I/O operations queued for writes" />
        <StatCard label="Pending fsyncs" icon="🔄" value={data?.innodb_pending_fsyncs != null ? formatNumber(data.innodb_pending_fsyncs, 0) : "-"} hint="Pending fsync I/O operations" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="Pending I/O Operations Over Time" data={pendingIO} formatter={(v) => formatNumber(v, 0)} lines={[
          { key: "pending_reads", name: "Pending Reads", color: C.blue },
          { key: "pending_writes", name: "Pending Writes", color: C.orange },
          { key: "pending_fsyncs", name: "Pending fsyncs", color: C.violet },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5. INNODB I/O ═══════════════════════ */}
      <SectionHeader title="InnoDB Disk I/O" description="Physical reads/writes indicate storage pressure" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Data Reads/sec" icon="📥" value={data?.innodb_data_reads_per_sec != null ? `${formatBytes(data.innodb_data_reads_per_sec)}/s` : "-"} hint="Physical disk reads" />
        <StatCard label="Data Writes/sec" icon="📤" value={data?.innodb_data_writes_per_sec != null ? `${formatBytes(data.innodb_data_writes_per_sec)}/s` : "-"} hint="Physical disk writes" />
        <StatCard label="fsyncs/sec" icon="🔄" value={data?.innodb_fsyncs_per_sec != null ? formatNumber(data.innodb_fsyncs_per_sec, 1) : "-"} tone={(data?.innodb_fsyncs_per_sec ?? 0) > 100 ? "warn" : "neutral"} hint="Disk flush calls/sec" />
        <StatCard label="Log Writes/sec" icon="📝" value={data?.innodb_log_writes_per_sec != null ? formatNumber(data.innodb_log_writes_per_sec, 1) : "-"} hint="Redo log write rate" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="InnoDB I/O Bytes Over Time" data={io} formatter={(v, key) => {
          if (typeof v !== "number" || !Number.isFinite(v)) return "0"
          if (key === "log_writes") return `${formatNumber(v, 1)}/s`
          return `${formatBytes(v)}/s`
        }} areas={[
          { key: "reads_bps", name: "Reads", color: C.blue },
          { key: "writes_bps", name: "Writes", color: C.orange },
          { key: "log_writes", name: "Log Writes", color: C.violet },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5b. FULL SCAN DETECTION ═══════════════ */}
      <SectionHeader title="Full Scan Detection" description="High read_rnd_next vs read_key ⇒ sequential scans (missing indexes)" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Table Scan %" icon="🔍" value={data?.table_scan_ratio_pct != null ? formatPercent(data.table_scan_ratio_pct) : "-"} tone={(data?.table_scan_ratio_pct ?? 0) > 50 ? "bad" : (data?.table_scan_ratio_pct ?? 0) > 20 ? "warn" : "good"} hint="% handler ops that are full table scans" />
        <StatCard label="Handler Read Key/sec" icon="🔑" value={data?.handler_read_key_per_sec != null ? `${formatNumber(data.handler_read_key_per_sec, 0)}/s` : "-"} hint="Index-based row lookups" />
        <StatCard label="Handler Read Rnd Next/sec" icon="📖" value={data?.handler_read_rnd_next_per_sec != null ? `${formatNumber(data.handler_read_rnd_next_per_sec, 0)}/s` : "-"} tone={(data?.handler_read_rnd_next_per_sec ?? 0) > (data?.handler_read_key_per_sec ?? 1) * 5 ? "warn" : "good"} hint="Full scan row reads — high relative to read_key = missing index" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="Handler Read Operations Over Time" data={handlerRead} formatter={(v) => `${formatNumber(v, 0)}/s`} areas={[
          { key: "read_rnd_next", name: "Full Scans (read_rnd_next)", color: C.red },
          { key: "read_key", name: "Index Lookups (read_key)", color: C.green },
          { key: "read_first", name: "First Entry (read_first)", color: C.blue },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5c. SORT OPERATIONS ════════════════════ */}
      <SectionHeader title="Sort Operations" description="Sorts by type — merge passes indicate disk spill (small sort_buffer_size)" />
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="Sort Operations Over Time" data={sortOps} formatter={(v) => `${formatNumber(v, 1)}/s`} stacked areas={[
          { key: "range", name: "Range Sorts", color: C.teal },
          { key: "scan", name: "Scan Sorts", color: C.blue },
          { key: "merge_passes", name: "Merge Passes (disk)", color: C.red },
        ]} />
      </div>

      {/* ═══════════════════════════════ 6. LOCKS & CONTENTION ═══════════════ */}
      <SectionHeader title="Locks & Contention" description="Deadlocks and waits indicate query or schema issues" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Table Locks Waited/min" icon="🔒" value={data?.table_locks_waited_per_min != null ? formatNumber(data.table_locks_waited_per_min, 2) : "-"} tone={(data?.table_locks_waited_per_min ?? 0) > 0 ? "warn" : "good"} hint="MyISAM or explicit table locks" />
        <StatCard label="InnoDB Deadlocks/min" icon="💀" value={data?.innodb_deadlocks_per_min != null ? formatNumber(data.innodb_deadlocks_per_min, 2) : "-"} tone={(data?.innodb_deadlocks_per_min ?? 0) > 0 ? "bad" : "good"} hint="Cyclic row lock conflicts" />
        <StatCard label="Row Lock Waits/min" icon="⏳" value={data?.innodb_row_lock_waits_per_min != null ? formatNumber(data.innodb_row_lock_waits_per_min, 2) : "-"} tone={(data?.innodb_row_lock_waits_per_min ?? 0) > 10 ? "warn" : "good"} hint="InnoDB row-level contention" />
        <StatCard label="Avg Row Lock Time" icon="⌛" value={data?.innodb_row_lock_time_avg_ms != null ? `${formatNumber(data.innodb_row_lock_time_avg_ms, 1)} ms` : "-"} tone={(data?.innodb_row_lock_time_avg_ms ?? 0) > 50 ? "warn" : "neutral"} hint="Average wait per lock event" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="Lock Contention Over Time" data={locks} formatter={(v) => `${formatNumber(v, 2)}/min`} lines={[
          { key: "table_locks_waited_pm", name: "Table Locks Waited", color: C.amber },
          { key: "innodb_deadlocks_pm", name: "Deadlocks", color: C.red },
          { key: "row_lock_waits_pm", name: "Row Lock Waits", color: C.rose },
        ]} />
        <LinePanel title="InnoDB Deadlocks Over Time" data={deadlocks} lines={[{ key: "deadlocks_pm", name: "Deadlocks/min", color: C.red }]} formatter={(v) => `${formatNumber(v, 2)}/min`} />
      </div>

      {/* ═══════════════════════════════ 7. TEMP TABLES ══════════════════════ */}
      <SectionHeader title="Temporary Tables" description="Disk-based temp tables are expensive — tune tmp_table_size" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Tmp Tables/sec (memory)" icon="📋" value={data?.tmp_tables_per_sec != null ? `${formatNumber(data.tmp_tables_per_sec, 2)}/s` : "-"} hint="In-memory temp tables (fast)" />
        <StatCard label="Tmp Disk Tables/sec" icon="💽" value={data?.tmp_disk_tables_per_sec != null ? `${formatNumber(data.tmp_disk_tables_per_sec, 2)}/s` : "-"} tone={(data?.tmp_disk_tables_per_sec ?? 0) > 0.5 ? "warn" : "good"} hint="Disk spills — increase tmp_table_size" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="Temporary Tables Created Over Time" data={tmpTables} formatter={(v) => `${formatNumber(v, 3)}/s`} areas={[
          { key: "memory_per_sec", name: "Memory tmp tables", color: C.teal },
          { key: "disk_per_sec", name: "Disk tmp tables", color: C.red },
        ]} />
      </div>

      {/* ═══════════════════════════════ 8. NETWORK ═══════════════════════════ */}
      <SectionHeader title="Network" description="Client ↔ server data transfer rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="RX (client → server)" icon="⬇️" value={data?.network_rx_bytes_per_sec != null ? `${formatBytes(data.network_rx_bytes_per_sec)}/s` : "-"} hint="Incoming query bytes" />
        <StatCard label="TX (server → client)" icon="⬆️" value={data?.network_tx_bytes_per_sec != null ? `${formatBytes(data.network_tx_bytes_per_sec)}/s` : "-"} hint="Outgoing result bytes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <AreaPanel title="Network Throughput Over Time" data={network} formatter={(v) => `${formatBytes(v)}/s`} areas={[
          { key: "rx_bps", name: "Received", color: C.blue },
          { key: "tx_bps", name: "Sent", color: C.green },
        ]} />
      </div>

      {/* ═══════════════════════════════ 8b. STORAGE ═══════════════════════════ */}
      <SectionHeader title="Storage" description="Binary log growth and top tables by estimated size" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Binlog Size" icon="📦" value={data?.binlog_size_bytes != null ? formatBytes(data.binlog_size_bytes) : "-"} hint="Current total binary log disk usage" />
        <StatCard label="Binlog Files" icon="📄" value={data?.binlog_files != null ? formatNumber(data.binlog_files, 0) : "-"} hint="Number of binlog files on disk" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="Binary Log Size Over Time" data={binlog} formatter={(v) => formatBytes(v)} lines={[{ key: "size_bytes", name: "Binlog Size", color: C.cyan }]} />
        <BarPanel title="Top Tables by Estimated Size" data={data?.top_table_sizes ?? []} formatter={(v) => formatBytes(v)} />
      </div>

      {/* ═══════════════════════════════ 9. CACHE EFFICIENCY ═════════════════ */}
      <SectionHeader title="Cache Efficiency" description="Query cache and table open-cache effectiveness" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label="Query Cache Hit %" icon="🎯" value={data?.qcache_hit_ratio_pct != null ? formatPercent(data.qcache_hit_ratio_pct) : "N/A"} tone={data?.qcache_hit_ratio_pct != null && data.qcache_hit_ratio_pct < 50 ? "warn" : "neutral"} hint="Disabled in MySQL 8+ / MariaDB 10.6+" />
        <StatCard label="Query Cache Free Mem" icon="💾" value={data?.qcache_free_mem_bytes != null ? formatBytes(data.qcache_free_mem_bytes) : "N/A"} hint="Remaining space in query cache" />
        <StatCard label="Table Cache Hit %" icon="📂" value={data?.table_cache_hit_ratio_pct != null ? formatPercent(data.table_cache_hit_ratio_pct) : "-"} tone={(data?.table_cache_hit_ratio_pct ?? 100) < 90 ? "warn" : "good"} hint="Increase table_open_cache if low" />
        <StatCard label="Open Tables" icon="📁" value={data?.open_tables != null ? formatNumber(data.open_tables, 0) : "-"} hint="Currently open table handles" />
      </div>

      {/* ═══════════════════════════════ 10. QUERY ANALYSIS ═══════════════════ */}
      <SectionHeader title="Query Analysis" description="Performance Schema digest data — requires performance_schema=ON" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-2">
        <StatCard label="Total Query Time (digests)" icon="⏱️" value={data?.query_digest_total_time_seconds != null ? formatDuration(data.query_digest_total_time_seconds) : "-"} hint="Cumulative time across all digest classes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title="Total Query Time (digests) Over Time" data={queryTime} formatter={(v) => formatDuration(v)} lines={[{ key: "total_seconds", name: "Total Time", color: C.violet }]} />
        <LinePanel title="Query Execution Count Over Time" data={queryCount} formatter={(v) => formatNumber(v)} lines={[{ key: "exec_count", name: "Exec Count", color: C.cyan }]} />
      </div>

      <DigestsTable title="Top Queries by Total Time" digests={data?.top_query_digests ?? []} valueFn={(d) => d.total_time_seconds} valueLabel="Total Time" valueFormatter={(v) => formatDuration(v)} />
      <DigestsTable title="Inefficient Queries (Rows Examined / Sent)" digests={data?.inefficient_digests ?? []} valueFn={(d) => d.rows_examined_per_sent} valueLabel="Rows Exam/Sent" valueFormatter={(v) => formatNumber(v, 1)} />
      <DigestsTable title="Queries Without Index" digests={data?.no_index_digests ?? []} valueFn={(d) => d.no_index_used_count} valueLabel="No-Index Count" valueFormatter={(v) => formatNumber(v, 0)} />

      {/* ═══════════════════════════════ 11. PER-DATABASE ACTIVITY ═════════════ */}
      <SectionHeader title="Per-Database Activity" description="Query volume, cost, and storage grouped by schema — aggregated from digest metrics and slow query logs" />
      <div className="grid gap-4 xl:grid-cols-2">
        <BarPanel title="Top DBs by Query Rate" data={data?.top_dbs_by_queries ?? []} formatter={(v) => `${formatNumber(v, 1)}/s`} />
        <BarPanel title="Top DBs by Total Query Time" data={data?.top_dbs_by_query_time ?? []} formatter={(v) => formatDuration(v)} />
        <BarPanel title="Top DBs by Rows Examined" data={data?.top_dbs_by_rows_examined ?? []} formatter={(v) => formatNumber(v, 0)} />
        <BarPanel title="Top DBs by No-Index Queries" data={data?.top_dbs_by_no_index ?? []} formatter={(v) => formatNumber(v, 0)} />
        <BarPanel title="Storage per Database" data={data?.db_storage_sizes ?? []} formatter={(v) => formatBytes(v)} />
        <BarPanel title="Slow Queries per Database (from logs)" data={data?.slow_queries_by_db ?? []} formatter={(v) => formatNumber(v, 0)} />
      </div>

      {/* ═══════════════════════════════ 12. LOG SIGNALS ═════════════════════ */}
      <SectionHeader title="Log Signals" description="Events parsed from MariaDB logs via Loki" />
      <div className="mb-4 flex items-center gap-3">
        <label className="text-xs font-medium text-muted-foreground whitespace-nowrap">Min Slow Query Duration (s):</label>
        <input
          type="number"
          min="0"
          step="0.1"
          value={minDurationSecInput}
          onChange={(e) => setMinDurationSecInput(e.target.value)}
          onKeyDown={(e) => { if (e.key === "Enter") { setMinDurationSec(minDurationSecInput) } }}
          onBlur={() => setMinDurationSec(minDurationSecInput)}
          placeholder="e.g. 2"
          className="h-8 w-24 rounded border border-input bg-background px-2 text-sm"
        />
        <button
          type="button"
          onClick={() => setMinDurationSec(minDurationSecInput)}
          className="h-8 rounded-md border border-border px-3 text-xs hover:bg-muted"
        >
          Apply
        </button>
        {minDurationSec && (
          <button
            type="button"
            onClick={() => { setMinDurationSec(""); setMinDurationSecInput("") }}
            className="h-8 rounded-md border border-border px-3 text-xs text-muted-foreground hover:bg-muted"
          >
            Clear
          </button>
        )}
      </div>
      <SectionCard title="Log Event Counters">
        <div className="grid gap-3 md:grid-cols-4">
          <StatCard label="Slow Queries" icon="🐢" value={formatNumber(data?.log_summary.slow_queries ?? 0, 0)} tone={(data?.log_summary.slow_queries ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label="Errors" icon="❌" value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label="Warnings" icon="⚠️" value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label="Auth Failures" icon="🔐" value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label="Deadlocks" icon="💀" value={formatNumber(data?.log_summary.deadlocks ?? 0, 0)} tone={(data?.log_summary.deadlocks ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label="Replication Events" icon="🔁" value={formatNumber(data?.log_summary.replication ?? 0, 0)} tone={(data?.log_summary.replication ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label="Startups" icon="🚀" value={formatNumber(data?.log_summary.startup ?? 0, 0)} />
          <StatCard label="Shutdowns" icon="🛑" value={formatNumber(data?.log_summary.shutdown ?? 0, 0)} />
        </div>
      </SectionCard>
        </>
      )}

      {activeTab === "config" && <MariaDBConfigTab id={id} />}

      {activeTab === "summary" && (
        <>
          {dashQuery.isLoading ? (
            <div className="p-6 text-sm text-muted-foreground">{t("shared.loading")}</div>
          ) : data ? (
            <MariaDBSummaryTab data={data} />
          ) : (
            <div className="p-6 text-sm text-muted-foreground">No data available yet.</div>
          )}
        </>
      )}
    </Page>
  )
}
