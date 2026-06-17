import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts"

import { getDiscoveryService } from "@/api/discovery"
import { getPostgresDashboardSummary, type PostgresDashboardSummary, type PostgresTimePoint } from "@/api/postgres"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { dateTimeInputToSeconds, formatBytes, formatDateInputValue, formatDuration, formatNumber, formatPercent, formatTimeInputValue, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"
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

function mapTimeseries(points: PostgresTimePoint[]): Record<string, number | string>[] {
  return (points ?? []).map((pt) => {
    const row: Record<string, number | string> = {}
    for (const [key, value] of Object.entries(pt)) {
      if (key === "time" && typeof value === "number") {
        row.time = formatChartTime(value)
      } else if (typeof value === "number" && Number.isFinite(value)) {
        row[key] = value
      }
    }
    return row
  })
}

function latest(points: PostgresTimePoint[] | undefined, key: string) {
  const last = points?.[points.length - 1]?.[key]
  return typeof last === "number" ? last : null
}

function totalLatest(points: PostgresTimePoint[] | undefined) {
  const row = points?.[points.length - 1]
  if (!row) return null
  return Object.entries(row).reduce((sum, [key, value]) => key !== "time" && typeof value === "number" ? sum + value : sum, 0)
}

function totalFirstAndLast(points: PostgresTimePoint[] | undefined) {
  const first = points?.[0]
  const last = points?.[points.length - 1]
  if (typeof first?.time !== "number" || typeof last?.time !== "number") return null
  const total = (row: PostgresTimePoint) => Object.entries(row).reduce((sum, [key, value]) => key !== "time" && typeof value === "number" ? sum + value : sum, 0)
  return { start: { time: first.time, value: total(first) }, end: { time: last.time, value: total(last) } }
}

function StatCard({
  label,
  value,
  hint,
  tone = "neutral",
}: {
  label: string
  value: string
  hint?: string
  tone?: "neutral" | "good" | "warn" | "bad"
}) {
  const tones = {
    neutral: "border-border bg-card",
    good: "border-emerald-300/50 bg-emerald-500/10",
    warn: "border-amber-300/50 bg-amber-500/10",
    bad: "border-red-300/50 bg-red-500/10",
  }
  return (
    <div className={cn("rounded-lg border p-4", tones[tone])}>
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="mt-2 text-2xl font-semibold">{value}</div>
      {hint ? <div className="mt-1 text-xs text-muted-foreground">{hint}</div> : null}
    </div>
  )
}

function TimeRangeSelector({
  selected,
  onChange,
}: {
  selected: TimeSelection
  onChange: (value: TimeSelection) => void
}) {
  const { t } = useTranslation()
  const manual = selected.mode === "manual" ? selected : createDefaultManualSelection()
  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex rounded-lg border border-border bg-card p-1">
        {TIME_RANGES.map((range) => (
          <button
            key={range.label}
            type="button"
            onClick={() => onChange({ mode: "preset", preset: range })}
            className={cn(
              "rounded-md px-3 py-1.5 text-sm transition-colors",
              selected.mode === "preset" && selected.preset.label === range.label ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted",
            )}
          >
            {range.label}
          </button>
        ))}
        <button
          type="button"
          onClick={() => onChange(manual)}
          className={cn(
            "rounded-md px-3 py-1.5 text-sm transition-colors",
            selected.mode === "manual" ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted",
          )}
        >
          {t("shared.manual")}
        </button>
      </div>
      {selected.mode === "manual" ? (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-1">
          <input type="text" placeholder="DD-MM-YYYY" value={selected.date} onChange={(event) => onChange({ ...selected, date: event.target.value })} className="h-8 w-28 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.startTime} onChange={(event) => onChange({ ...selected, startTime: event.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.endTime} onChange={(event) => onChange({ ...selected, endTime: event.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
        </div>
      ) : null}
    </div>
  )
}

const C = {
  blue: "#0ea5e9", green: "#22c55e", amber: "#f59e0b", red: "#ef4444",
  violet: "#8b5cf6", teal: "#14b8a6", rose: "#f43f5e", cyan: "#06b6d4",
  indigo: "#6366f1", orange: "#f97316", slate: "#64748b",
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

function SectionHeader({ title, description }: { title: string; description?: string }) {
  return (
    <div className="mb-1 mt-6">
      <h2 className="text-base font-semibold">{title}</h2>
      {description && <p className="text-xs text-muted-foreground">{description}</p>}
    </div>
  )
}

function LinePanel({
  title,
  data,
  lines,
  formatter,
}: {
  title: string
  data: Record<string, number | string>[]
  lines: Array<{ key: string; name: string; color: string }>
  formatter?: (value: number, key: string) => string
}) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={260}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.55} />
            <XAxis dataKey="time" tick={chartStyle.tick} tickLine={false} />
            <YAxis tick={chartStyle.tick} tickLine={false} axisLine={false} />
            <Tooltip
              contentStyle={chartStyle.tooltip}
              formatter={(value, name) => typeof value === "number" ? formatter?.(value, String(name)) ?? formatNumber(value) : value}
            />
            <Legend wrapperStyle={{ fontSize: 11 }} />
            {lines.map((line) => (
              <Line key={line.key} type="monotone" dataKey={line.key} name={line.name} stroke={line.color} strokeWidth={2} dot={false} />
            ))}
          </LineChart>
        </ResponsiveContainer>
      ) : (
        <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">{t("shared.noData")}</div>
      )}
    </div>
  )
}

function BarPanel({
  title,
  data,
  valueFormatter,
}: {
  title: string
  data: Array<{ label: string; count: number }>
  valueFormatter?: (value: number) => string
}) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <h3 className="mb-4 text-sm font-semibold">{title}</h3>
      {data.length ? (
        <ResponsiveContainer width="100%" height={260}>
          <BarChart data={data} layout="vertical" margin={{ left: 32 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={chartStyle.grid} opacity={0.55} />
            <XAxis type="number" tick={chartStyle.tick} tickFormatter={(v) => valueFormatter?.(Number(v)) ?? formatNumber(Number(v))} />
            <YAxis type="category" dataKey="label" tick={chartStyle.tick} width={110} />
            <Tooltip contentStyle={chartStyle.tooltip} formatter={(v) => typeof v === "number" ? valueFormatter?.(v) ?? formatNumber(v) : v} />
            <Bar dataKey="count" fill="#0ea5e9" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">{t("shared.noBreakdown")}</div>
      )}
    </div>
  )
}

function DatabaseHealthTable({ rows }: { rows: PostgresDashboardSummary["database_health"] }) {
  const { t } = useTranslation()
  return (
    <SectionCard title={t("pgDashboard.healthTitle")} description={t("pgDashboard.healthDesc")}>
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs uppercase text-muted-foreground">
            <tr className="border-b border-border">
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColDb")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColSize")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColConns")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColCache")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColTempFiles")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColTempBytes")}</th>
              <th className="py-2 text-left">{t("pgDashboard.healthColRollback")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length ? rows.map((row) => (
              <tr key={row.database} className="border-b border-border/60">
                <td className="py-2 pr-4 font-mono text-xs">{row.database}</td>
                <td className="py-2 pr-4">{formatBytes(row.size_bytes)}</td>
                <td className="py-2 pr-4">{formatNumber(row.connections, 0)}</td>
                <td className={cn("py-2 pr-4", row.cache_hit_ratio_pct < 95 ? "text-amber-600 dark:text-amber-400" : "")}>{formatPercent(row.cache_hit_ratio_pct)}</td>
                <td className={cn("py-2 pr-4", row.temp_files_per_second > 0 ? "text-amber-600 dark:text-amber-400" : "")}>{formatNumber(row.temp_files_per_second, 3)}</td>
                <td className={cn("py-2 pr-4", row.temp_bytes_per_second > 0 ? "text-amber-600 dark:text-amber-400" : "")}>{formatBytes(row.temp_bytes_per_second)}/s</td>
                <td className={cn("py-2", row.rollback_rate_pct > 2 ? "text-red-500" : row.rollback_rate_pct > 1 ? "text-amber-600 dark:text-amber-400" : "")}>{formatPercent(row.rollback_rate_pct)}</td>
              </tr>
            )) : (
              <tr>
                <td colSpan={7} className="py-6 text-center text-sm text-muted-foreground">{t("pgDashboard.healthEmpty")}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </SectionCard>
  )
}

export function PostgresDashboardPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const dashQuery = useQuery<PostgresDashboardSummary>({
    queryKey: ["postgres-dashboard-summary", id, timeSelection, timeWindow.start, timeWindow.end],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getPostgresDashboardSummary(id, {
      start: secondsToNanos(timeWindow.start),
      end: secondsToNanos(timeWindow.end),
      step: timeWindow.step,
    }),
    placeholderData: (previousData) => previousData,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 25_000,
    refetchOnWindowFocus: false,
  })

  const data = dashQuery.data
  const service = serviceQuery.data

  const connections = mapTimeseries(data?.connections_timeseries ?? [])
  const activity = mapTimeseries(data?.activity_timeseries ?? [])
  const transactions = mapTimeseries(data?.transaction_timeseries ?? [])
  const cache = mapTimeseries(data?.cache_timeseries ?? [])
  const temp = mapTimeseries(data?.temp_timeseries ?? [])
  const locks = mapTimeseries(data?.locks_timeseries ?? [])
  const wal = mapTimeseries(data?.wal_timeseries ?? [])
  const rowOps = mapTimeseries(data?.row_ops_timeseries ?? [])
  const seqVsIndex = mapTimeseries(data?.seq_vs_index_timeseries ?? [])
  const bgwriter = mapTimeseries(data?.bgwriter_timeseries ?? [])
  const archiver = mapTimeseries(data?.archiver_timeseries ?? [])
  const replicationConflicts = mapTimeseries(data?.replication_conflicts_timeseries ?? [])
  const diskIO = mapTimeseries(data?.disk_io_timeseries ?? [])
  const activeSession = mapTimeseries(data?.active_session_timeseries ?? [])
  const idleInTx = latest(data?.activity_timeseries, "idle in transaction")
  const sizeFirstLast = totalFirstAndLast(data?.size_timeseries)
  const sizeGrowthPerHour = sizeFirstLast && sizeFirstLast.end.time > sizeFirstLast.start.time
    ? ((sizeFirstLast.end.value - sizeFirstLast.start.value) / ((sizeFirstLast.end.time - sizeFirstLast.start.time) / 3600))
    : null
  const seqReturned = latest(data?.seq_vs_index_timeseries, "returned") ?? 0
  const indexFetched = latest(data?.seq_vs_index_timeseries, "fetched") ?? 0
  const seqScanRatio = seqReturned + indexFetched > 0 ? (100 * seqReturned) / (seqReturned + indexFetched) : null
  const bgwriterClean = latest(data?.bgwriter_timeseries, "clean") ?? 0
  const bgwriterAlloc = latest(data?.bgwriter_timeseries, "alloc") ?? 0
  const bgwriterPressure = bgwriterClean + bgwriterAlloc > 0 ? (100 * bgwriterAlloc) / (bgwriterClean + bgwriterAlloc) : null
  const tempBytesLatest = latest(data?.temp_timeseries, "bytes")
  const totalSizeLatest = totalLatest(data?.size_timeseries)

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">{t("shared.loading")}</div>
  if (!service) return <div className="p-6 text-sm text-red-600">{t("shared.serviceNotFound", id)}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("pgDashboard.title")}
        description={t("pgDashboard.description", service.service_name, service.hostname || service.machine_id)}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <TimeRangeSelector selected={timeSelection} onChange={setTimeSelection} />
            <LangToggle />
            <button
              type="button"
              onClick={() => dashQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/integrations/postgres-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              {t("shared.viewLogs")}
            </Link>
            <Link to={`/integrations/postgres-config/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              {t("shared.configuration")}
            </Link>
          </div>
        }
      />

      {dashQuery.isError ? (
        <SectionCard title={t("pgDashboard.dashUnavailable")}>
          <div className="text-sm text-red-600">{t("pgDashboard.fetchFailed")}</div>
        </SectionCard>
      ) : null}
      {timeWindow.error ? (
        <SectionCard title={t("pgDashboard.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      {/* ═══════════════════════════════ 1. STATUS ═══════════════════════════ */}
      <SectionHeader title="Status" description="Instance health and replication role at a glance" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.up")} value={data?.up === 1 ? "Up" : data?.up === 0 ? "Down" : "-"} tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint={t("pgDashboard.upHint")} />
        <StatCard label={t("pgDashboard.scrape")} value={data?.scrape_error === 1 ? "Error" : data?.scrape_error === 0 ? "Healthy" : "-"} tone={data?.scrape_error === 1 ? "bad" : data?.scrape_error === 0 ? "good" : "neutral"} hint={t("pgDashboard.scrapeHint")} />
        <StatCard label="Replica Role" value={data?.replica_role != null ? (data.replica_role === 1 ? "Replica" : "Primary") : "-"} tone={data?.replica_role === 1 ? "warn" : "good"} hint={data?.replica_role === 1 ? "This is a read replica" : "This is the primary server"} />
        <StatCard label={t("pgDashboard.cacheHit")} value={data?.cache_hit_ratio_pct != null ? formatPercent(data.cache_hit_ratio_pct) : "-"} tone={(data?.cache_hit_ratio_pct ?? 100) < 95 ? "warn" : "good"} hint={t("pgDashboard.cacheHitHint")} />
      </div>

      {/* ═══════════════════════════════ 2. CONNECTIONS ═════════════════════ */}
      <SectionHeader title="Connections" description="Connection pool saturation, per-database breakdown, and idle-in-transaction tracking" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.connSat")} value={data?.connection_saturation_pct != null ? formatPercent(data.connection_saturation_pct) : "-"} tone={(data?.connection_saturation_pct ?? 0) > 85 ? "bad" : (data?.connection_saturation_pct ?? 0) > 70 ? "warn" : "good"} hint={t("pgDashboard.connSatHint", data?.max_connections ? formatNumber(data.max_connections, 0) : "-")} />
        <StatCard label={t("pgDashboard.idleTx")} value={idleInTx != null ? formatNumber(idleInTx, 0) : "-"} tone={(idleInTx ?? 0) > 0 ? "warn" : "good"} hint={t("pgDashboard.idleTxHint")} />
        <StatCard label={t("pgDashboard.maxTxDuration")} value={data?.max_tx_duration_seconds != null ? formatDuration(data.max_tx_duration_seconds) : "-"} tone={(data?.max_tx_duration_seconds ?? 0) > 30 ? "warn" : "good"} hint="Longest active transaction" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title={t("pgDashboard.chartConnections")} data={connections} lines={[{ key: "connections", name: "connections", color: C.blue }]} />
        <LinePanel
          title={t("pgDashboard.chartActivity")}
          data={activity}
          lines={[
            { key: "active", name: "active", color: C.green },
            { key: "idle", name: "idle", color: C.slate },
            { key: "idle in transaction", name: "idle in transaction", color: C.amber },
          ]}
        />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <BarPanel title={t("pgDashboard.chartConnsByDb")} data={data?.conn_by_database ?? []} />
        <BarPanel title="Conn Saturation % by DB" data={data?.conn_saturation_by_db ?? []} valueFormatter={(v) => formatPercent(v)} />
      </div>

      {/* ═══════════════════════════════ 3. TRANSACTIONS ════════════════════ */}
      <SectionHeader title="Transactions" description="Commit, rollback, and deadlock rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.rollbackRate")} value={data?.rollback_rate_pct != null ? formatPercent(data.rollback_rate_pct) : "-"} tone={(data?.rollback_rate_pct ?? 0) > 2 ? "bad" : (data?.rollback_rate_pct ?? 0) > 1 ? "warn" : "good"} hint={t("pgDashboard.rollbackRateHint")} />
        <StatCard label={t("pgDashboard.deadlocks")} value={data?.deadlocks_per_min != null ? formatNumber(data.deadlocks_per_min, 2) : "-"} tone={(data?.deadlocks_per_min ?? 0) > 0 ? "bad" : "good"} hint={t("pgDashboard.deadlocksHint")} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("pgDashboard.chartTransactions")}
          data={transactions}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "commits", name: "commits/s", color: C.green },
            { key: "rollbacks", name: "rollbacks/s", color: C.amber },
            { key: "deadlocks", name: "deadlocks/s", color: C.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 4. CACHE ═══════════════════════════ */}
      <SectionHeader title="Cache" description="Buffer cache hit ratio and disk read/write effectiveness" />
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("pgDashboard.chartCache")}
          data={cache}
          formatter={(value, key) => key === "hit_ratio" ? formatPercent(value) : `${formatNumber(value)} /s`}
          lines={[
            { key: "hit_ratio", name: "hit ratio %", color: C.teal },
            { key: "hits", name: "buffer hits/s", color: C.green },
            { key: "reads", name: "disk reads/s", color: C.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 5. STORAGE & WAL ═══════════════════ */}
      <SectionHeader title="Storage & WAL" description="Database size, WAL generation rate, and growth trends" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.dbSize")} value={data?.database_size_bytes != null ? formatBytes(data.database_size_bytes) : "-"} hint={t("pgDashboard.dbSizeHint")} />
        <StatCard label={t("pgDashboard.walSize")} value={data?.wal_size_bytes != null ? formatBytes(data.wal_size_bytes) : "-"} tone={(data?.wal_size_bytes ?? 0) > 1024 * 1024 * 1024 ? "warn" : "neutral"} hint={t("pgDashboard.walSizeHint")} />
        <StatCard label="WAL Rate" value={data?.wal_rate_per_sec != null ? `${formatBytes(data.wal_rate_per_sec)}/s` : "-"} tone={(data?.wal_rate_per_sec ?? 0) > 10 * 1024 * 1024 ? "warn" : "neutral"} hint="WAL generation rate" />
        <StatCard label={t("pgDashboard.dbGrowth")} value={sizeGrowthPerHour != null ? `${formatBytes(Math.abs(sizeGrowthPerHour))}/h` : "-"} tone={(sizeGrowthPerHour ?? 0) > 100 * 1024 * 1024 ? "warn" : "neutral"} hint={totalSizeLatest != null ? `${t("pgDashboard.totalInRange", formatBytes(totalSizeLatest))}` : undefined} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title={t("pgDashboard.chartWAL")} data={wal} formatter={(value) => formatBytes(value)} lines={[{ key: "bytes", name: "WAL bytes", color: C.violet }]} />
        <LinePanel title="Temp Spill" data={temp} formatter={(value, key) => key === "bytes" ? `${formatBytes(value)}/s` : `${formatNumber(value)} /s`} lines={[
          { key: "files", name: "temp files/s", color: C.amber },
          { key: "bytes", name: "temp bytes/s", color: C.red },
        ]} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <BarPanel title={t("pgDashboard.chartDbSizes")} data={data?.database_sizes ?? []} valueFormatter={formatBytes} />
      </div>

      {/* ═══════════════════════════════ 5b. DATABASE HEALTH ════════════════ */}
      <SectionHeader title="Database Health" description="Per-database metrics: size, connections, cache, temp spills, rollback rate" />
      <DatabaseHealthTable rows={data?.database_health ?? []} />

      {/* ═══════════════════════════════ 6. ROW THROUGHPUT ══════════════════ */}
      <SectionHeader title="Row Throughput" description="Row operations and sequential vs. index scan ratio" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.seqScan")} value={seqScanRatio != null ? formatPercent(seqScanRatio) : "-"} tone={(seqScanRatio ?? 0) > 60 ? "bad" : (seqScanRatio ?? 0) > 30 ? "warn" : "good"} hint={t("pgDashboard.seqScanHint")} />
        <StatCard label={t("pgDashboard.tempSpill")} value={tempBytesLatest != null ? `${formatBytes(tempBytesLatest)}/s` : "-"} tone={(tempBytesLatest ?? 0) > 0 ? "warn" : "good"} hint={t("pgDashboard.tempSpillHint")} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("pgDashboard.chartRowThroughput")}
          data={rowOps}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "inserts", name: "inserts/s", color: C.green },
            { key: "updates", name: "updates/s", color: C.blue },
            { key: "deletes", name: "deletes/s", color: C.red },
          ]}
        />
        <LinePanel
          title={t("pgDashboard.chartSeqVsIndex")}
          data={seqVsIndex}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "returned", name: "seq returned/s", color: C.amber },
            { key: "fetched", name: "index fetched/s", color: C.green },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 7. REPLICATION & WAL ═══════════════ */}
      <SectionHeader title="Replication" description="Replication lag, conflicts, and WAL generation" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.replLag")} value={data?.replication_lag_seconds != null ? formatDuration(data.replication_lag_seconds) : "-"} tone={(data?.replication_lag_seconds ?? 0) > 30 ? "bad" : "good"} hint={t("pgDashboard.replLagHint")} />
        <StatCard label="Repl Lag (bytes)" value={data?.replication_lag_bytes != null ? formatBytes(data.replication_lag_bytes) : "-"} tone={(data?.replication_lag_bytes ?? 0) > 100 * 1024 * 1024 ? "warn" : "good"} hint="Replication lag in WAL bytes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title="Replication Conflicts"
          data={replicationConflicts}
          formatter={(value) => `${formatNumber(value, 2)} /s`}
          lines={[
            { key: "conflicts", name: "conflicts/s", color: C.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 8. LOCKS ═════════════════════════ */}
      <SectionHeader title="Locks & Contention" description="Lock counts by mode and lock-wait events from logs" />
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("pgDashboard.chartLocks")}
          data={locks}
          lines={[
            { key: "AccessShareLock", name: "AccessShare", color: C.slate },
            { key: "RowExclusiveLock", name: "RowExclusive", color: C.blue },
            { key: "ExclusiveLock", name: "Exclusive", color: C.red },
            { key: "AccessExclusiveLock", name: "AccessExclusive", color: "#b91c1c" },
          ]}
        />
        <BarPanel title={t("pgDashboard.chartLocksByMode")} data={data?.locks_by_mode ?? []} />
      </div>

      {/* ═══════════════════════════════ 9. BACKGROUND WRITER & ARCHIVER ════ */}
      <SectionHeader title="Background Writer & Archiver" description="Buffer management and WAL archiving activity" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("pgDashboard.bgwriterPressure")} value={bgwriterPressure != null ? formatPercent(bgwriterPressure) : "-"} tone={(bgwriterPressure ?? 0) > 50 ? "warn" : "good"} hint={t("pgDashboard.bgwriterPressureHint")} />
        <StatCard label={t("pgDashboard.archiveFailed")} value={data?.archive_failed != null ? formatNumber(data.archive_failed, 4) : "-"} tone={(data?.archive_failed ?? 0) > 0 ? "bad" : "good"} hint="WAL archive failures/s" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("pgDashboard.chartBgwriter")}
          data={bgwriter}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "clean", name: "buffers clean/s", color: C.green },
            { key: "alloc", name: "buffers alloc/s", color: C.red },
            { key: "maxwritten", name: "maxwritten clean/s", color: C.blue },
          ]}
        />
        <LinePanel
          title={t("pgDashboard.chartArchiver")}
          data={archiver}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "archived", name: "archived/s", color: C.green },
            { key: "failed", name: "failed/s", color: C.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 10. DISK I/O ════════════════════════ */}
      <SectionHeader title="Disk I/O" description="Backend-driven buffer writes and fsyncs indicate I/O pressure from query execution" />
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title="Backend I/O"
          data={diskIO}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "backend_writes", name: "backend writes/s", color: C.orange },
            { key: "backend_fsync", name: "backend fsync/s", color: C.red },
          ]}
        />
        <LinePanel
          title="Active Session Duration"
          data={activeSession}
          formatter={(value) => formatDuration(value)}
          lines={[
            { key: "seconds", name: "max tx duration", color: C.amber },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 11. CONFIG REFERENCE ═══════════════ */}
      <SectionHeader title="Config Reference" description="Key memory settings from the running configuration" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3">
          <StatCard label="shared_buffers" value={data?.shared_buffers_bytes != null ? formatBytes(data.shared_buffers_bytes) : "-"} />
          <StatCard label="work_mem" value={data?.work_mem_bytes != null ? formatBytes(data.work_mem_bytes) : "-"} />
          <StatCard label="effective_cache_size" value={data?.effective_cache_size_bytes != null ? formatBytes(data.effective_cache_size_bytes) : "-"} />
        </div>
      </SectionCard>

      {/* ═══════════════════════════════ 12. LOG SIGNALS ════════════════════ */}
      <SectionHeader title="Log Signals" description="Events parsed from PostgreSQL logs via Loki" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-7">
          <StatCard label={t("pgDashboard.slowQueries")} value={formatNumber(data?.log_summary.slow_queries ?? 0, 0)} tone={(data?.log_summary.slow_queries ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label={t("pgDashboard.errors")} value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label={t("pgDashboard.authFailures")} value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label={t("pgDashboard.lockWaits")} value={formatNumber(data?.log_summary.lock_waits ?? 0, 0)} tone={(data?.log_summary.lock_waits ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label={t("pgDashboard.tempFiles")} value={formatNumber(data?.log_summary.temp_files ?? 0, 0)} tone={(data?.log_summary.temp_files ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label={t("pgDashboard.schemaChanges")} value={formatNumber(data?.log_summary.schema_changes ?? 0, 0)} />
          <StatCard label={t("pgDashboard.checkpoints")} value={formatNumber(data?.log_summary.checkpoints ?? 0, 0)} tone={(data?.log_summary.checkpoints ?? 0) > 0 ? "warn" : "good"} />
        </div>
      </SectionCard>

    </Page>
  )
}
