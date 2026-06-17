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
import { getRedisDashboardSummary, type RedisDashboardSummary, type RedisTimePoint } from "@/api/redis"
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

function mapTimeseries(points: RedisTimePoint[]): Record<string, number | string>[] {
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
            <Bar dataKey="count" fill="#ef4444" radius={[0, 4, 4, 0]} />
          </BarChart>
        </ResponsiveContainer>
      ) : (
        <div className="flex h-[260px] items-center justify-center text-sm text-muted-foreground">{t("shared.noBreakdown")}</div>
      )}
    </div>
  )
}

export function RedisDashboardPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const dashQuery = useQuery<RedisDashboardSummary>({
    queryKey: ["redis-dashboard-summary", id, timeSelection, timeWindow.start, timeWindow.end],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getRedisDashboardSummary(id, {
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

  const memory = mapTimeseries(data?.memory_timeseries ?? [])
  const clients = mapTimeseries(data?.clients_timeseries ?? [])
  const commands = mapTimeseries(data?.commands_timeseries ?? [])
  const cache = mapTimeseries(data?.cache_timeseries ?? [])
  const errors = mapTimeseries(data?.error_timeseries ?? [])
  const net = mapTimeseries(data?.net_timeseries ?? [])
  const cpu = mapTimeseries(data?.cpu_timeseries ?? [])
  const keys = mapTimeseries(data?.keys_timeseries ?? [])
  const evictExpire = mapTimeseries(data?.evict_expire_timeseries ?? [])
  const frag = mapTimeseries(data?.frag_timeseries ?? [])
  const connRate = mapTimeseries(data?.conn_rate_timeseries ?? [])
  const persistence = mapTimeseries(data?.persistence_timeseries ?? [])
  const persistenceState = !data
    ? "-"
    : data.aof_rewrite_in_progress === 1
    ? "AOF rewrite"
    : data?.rdb_save_in_progress === 1
      ? "RDB saving"
      : data?.aof_enabled === 1
        ? "AOF"
        : "RDB"

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">{t("shared.loading")}</div>
  if (!service) return <div className="p-6 text-sm text-red-600">{t("shared.serviceNotFound", id)}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("redisDashboard.title")}
        description={t("redisDashboard.description", service.service_name, service.hostname || service.machine_id)}
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
            <Link to={`/integrations/redis-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              {t("shared.viewLogs")}
            </Link>
          </div>
        }
      />

      {dashQuery.isError ? (
        <SectionCard title={t("redisDashboard.dashUnavailable")}>
          <div className="text-sm text-red-600">{t("redisDashboard.fetchFailed")}</div>
        </SectionCard>
      ) : null}
      {timeWindow.error ? (
        <SectionCard title={t("redisDashboard.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      {/* ═══════════════════════════════ 1. STATUS ═══════════════════════════ */}
      <SectionHeader title="Status" description="Instance health and uptime" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.up")} value={data?.up === 1 ? "Up" : data?.up === 0 ? "Down" : "-"} tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint={t("redisDashboard.upHint")} />
        <StatCard label={t("redisDashboard.scrape")} value={data?.scrape_error === 1 ? "Error" : data?.scrape_error === 0 ? "Healthy" : "-"} tone={data?.scrape_error === 1 ? "bad" : data?.scrape_error === 0 ? "good" : "neutral"} hint={t("redisDashboard.scrapeHint")} />
        <StatCard label={t("redisDashboard.uptime")} value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} />
        <StatCard label="Connected Slaves" value={data?.connected_slaves != null ? formatNumber(data.connected_slaves, 0) : "-"} tone={(data?.connected_slaves ?? 0) === 0 ? "warn" : "good"} hint="Replication clients connected" />
      </div>

      {/* ═══════════════════════════════ 2. MEMORY ══════════════════════════ */}
      <SectionHeader title="Memory" description="Memory usage, fragmentation, and breakdown by category" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.memUsage")} value={data?.memory_usage_pct != null ? formatPercent(data.memory_usage_pct) : "-"} tone={(data?.memory_usage_pct ?? 0) > 80 ? "bad" : (data?.memory_usage_pct ?? 0) > 60 ? "warn" : "good"} hint={data?.memory_used_bytes != null ? formatBytes(data.memory_used_bytes) : undefined} />
        <StatCard label={t("redisDashboard.memFrag")} value={data?.memory_frag_ratio != null ? formatNumber(data.memory_frag_ratio, 2) : "-"} tone={(data?.memory_frag_ratio ?? 1) > 5 ? "bad" : (data?.memory_frag_ratio ?? 1) > 2 ? "warn" : "good"} hint={t("redisDashboard.memFragHint")} />
        <StatCard label={t("redisDashboard.peakMem")} value={data?.memory_peak_bytes != null ? formatBytes(data.memory_peak_bytes) : "-"} />
        <StatCard label="Lua Memory" value={data?.mem_lua_bytes != null ? formatBytes(data.mem_lua_bytes) : "-"} hint="Memory used by Lua scripts" />
        <StatCard label="Scripts Cache" value={data?.mem_scripts_bytes != null ? formatBytes(data.mem_scripts_bytes) : "-"} hint="Cached script bodies" />
        <StatCard label="Startup Overhead" value={data?.mem_startup_bytes != null ? formatBytes(data.mem_startup_bytes) : "-"} hint="Initial memory overhead" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartMemory")}
          data={memory}
          formatter={(value) => formatBytes(value)}
          lines={[
            { key: "used", name: t("redisDashboard.memUsed"), color: C.red },
            { key: "rss", name: t("redisDashboard.memRss"), color: C.amber },
            { key: "peak", name: t("redisDashboard.memPeak"), color: C.slate },
          ]}
        />
        <LinePanel title={t("redisDashboard.chartFrag")} data={frag} formatter={(value) => formatNumber(value, 2)} lines={[{ key: "ratio", name: t("redisDashboard.fragRatio"), color: C.violet }]} />
      </div>

      {/* ═══════════════════════════════ 3. CLIENTS & CONNECTIONS ═══════════ */}
      <SectionHeader title="Clients & Connections" description="Connection pool saturation and blocked clients" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.clients")} value={data?.connected_clients != null ? formatNumber(data.connected_clients, 0) : "-"} tone={(data?.client_saturation_pct ?? 0) > 80 ? "bad" : (data?.client_saturation_pct ?? 0) > 60 ? "warn" : "good"} hint={data?.client_saturation_pct != null ? t("redisDashboard.clientSatHint", formatPercent(data.client_saturation_pct)) : undefined} />
        <StatCard label={t("redisDashboard.blockedClients")} value={data?.blocked_clients != null ? formatNumber(data.blocked_clients, 0) : "-"} tone={(data?.blocked_clients ?? 0) > 0 ? "bad" : "good"} hint={t("redisDashboard.blockedClientsHint")} />
        <StatCard label={t("redisDashboard.rejectedConns")} value={data?.rejected_conns_per_sec != null ? formatNumber(data.rejected_conns_per_sec, 2) : "-"} tone={(data?.rejected_conns_per_sec ?? 0) > 0 ? "bad" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartClients")}
          data={clients}
          formatter={(value) => formatNumber(value, 0)}
          lines={[
            { key: "clients", name: t("redisDashboard.clients"), color: C.blue },
            { key: "blocked", name: t("redisDashboard.blockedClients"), color: C.red },
          ]}
        />
        <LinePanel
          title={t("redisDashboard.chartConnRate")}
          data={connRate}
          formatter={(value) => formatNumber(value, 2)}
          lines={[
            { key: "received", name: t("redisDashboard.connReceived"), color: C.green },
            { key: "rejected", name: t("redisDashboard.rejectedConns"), color: C.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 4. COMMANDS & THROUGHPUT ═══════════ */}
      <SectionHeader title="Commands & Throughput" description="Command processing rate and failures" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.cmdsPerSec")} value={data?.commands_per_sec != null ? formatNumber(data.commands_per_sec, 0) : "-"} />
        <StatCard label={t("redisDashboard.cmdFailures")} value={data?.command_failures_per_sec != null ? formatNumber(data.command_failures_per_sec, 2) : "-"} tone={(data?.command_failures_per_sec ?? 0) > 0 ? "bad" : "good"} />
        <StatCard label={t("redisDashboard.slowLog")} value={data?.slow_log_length != null ? formatNumber(data.slow_log_length, 0) : "-"} tone={(data?.slow_log_length ?? 0) > 0 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel title={t("redisDashboard.chartCommands")} data={commands} formatter={(value) => `${formatNumber(value)} /s`} lines={[{ key: "commands", name: t("redisDashboard.cmdsPerSec"), color: C.green }]} />
        <LinePanel
          title={t("redisDashboard.chartErrors")}
          data={errors}
          formatter={(value) => `${formatNumber(value, 2)} /s`}
          lines={[
            { key: "errors", name: t("redisDashboard.errorReplies"), color: C.amber },
            { key: "unexpected", name: t("redisDashboard.unexpectedErrors"), color: C.red },
            { key: "failed", name: t("redisDashboard.cmdFailures"), color: C.violet },
            { key: "rejected", name: t("redisDashboard.cmdRejects"), color: C.slate },
          ]}
        />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <BarPanel title={t("redisDashboard.chartCommandRates")} data={data?.command_rates ?? []} valueFormatter={(value) => `${formatNumber(value, 2)}/s`} />
        <BarPanel title={t("redisDashboard.chartTopCommands")} data={data?.commands_top ?? []} valueFormatter={(value) => formatNumber(value, 3)} />
      </div>

      {/* ═══════════════════════════════ 5. CACHE EFFICIENCY ════════════════ */}
      <SectionHeader title="Cache Efficiency" description="Keyspace hit ratio and eviction/expiration rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.cacheHit")} value={data?.cache_hit_ratio_pct != null ? formatPercent(data.cache_hit_ratio_pct) : "-"} tone={(data?.cache_hit_ratio_pct ?? 100) < 90 ? "bad" : (data?.cache_hit_ratio_pct ?? 100) < 95 ? "warn" : "good"} hint={t("redisDashboard.cacheHitHint")} />
        <StatCard label={t("redisDashboard.keys")} value={data?.keys_total != null ? formatNumber(data.keys_total, 0) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartCache")}
          data={cache}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { key: "hits", name: t("redisDashboard.hits"), color: C.green },
            { key: "misses", name: t("redisDashboard.misses"), color: C.red },
          ]}
        />
        <LinePanel title={t("redisDashboard.chartKeys")} data={keys} formatter={(value) => formatNumber(value, 0)} lines={[{ key: "keys", name: t("redisDashboard.keys"), color: C.violet }]} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartEvictExpire")}
          data={evictExpire}
          formatter={(value) => formatNumber(value, 2)}
          lines={[
            { key: "evicted", name: t("redisDashboard.evicted"), color: C.red },
            { key: "expired", name: t("redisDashboard.expired"), color: C.amber },
          ]}
        />
        <BarPanel title={t("redisDashboard.chartKeysByDb")} data={data?.keys_by_db ?? []} />
      </div>

      {/* ═══════════════════════════════ 6. PERSISTENCE & REPLICATION ═══════ */}
      <SectionHeader title="Persistence & Replication" description="RDB/AOF state, fork times, and replication health" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.persistenceState")} value={persistenceState} tone={data?.aof_rewrite_in_progress === 1 || data?.rdb_save_in_progress === 1 ? "warn" : "neutral"} hint={data?.rdb_changes_since_last_save != null ? t("redisDashboard.rdbChangesHint", formatNumber(data.rdb_changes_since_last_save, 0)) : undefined} />
        <StatCard label={t("redisDashboard.latestFork")} value={data?.latest_fork_seconds != null ? `${formatNumber(data.latest_fork_seconds, 3)}s` : "-"} tone={(data?.latest_fork_seconds ?? 0) > 1 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartPersistence")}
          data={persistence}
          formatter={(value, key) => key === "fork_sec" ? `${formatNumber(value, 3)}s` : formatNumber(value, 0)}
          lines={[
            { key: "rdb_changes", name: t("redisDashboard.rdbChanges"), color: C.blue },
            { key: "fork_sec", name: t("redisDashboard.latestFork"), color: C.amber },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 7. NETWORK & CPU ════════════════════ */}
      <SectionHeader title="Network & CPU" description="Data transfer rates and CPU utilization" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <StatCard label={t("redisDashboard.netInput")} value={data?.net_input_bytes_per_sec != null ? `${formatBytes(data.net_input_bytes_per_sec)}/s` : "-"} />
        <StatCard label={t("redisDashboard.netOutput")} value={data?.net_output_bytes_per_sec != null ? `${formatBytes(data.net_output_bytes_per_sec)}/s` : "-"} />
        <StatCard label={t("redisDashboard.cpuUser")} value={data?.cpu_user_per_sec != null ? formatNumber(data.cpu_user_per_sec, 2) : "-"} />
        <StatCard label={t("redisDashboard.cpuSys")} value={data?.cpu_sys_per_sec != null ? formatNumber(data.cpu_sys_per_sec, 2) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <LinePanel
          title={t("redisDashboard.chartNet")}
          data={net}
          formatter={(value, key) => key === "input" || key === "output" ? `${formatBytes(value)}/s` : `${formatNumber(value)}/s`}
          lines={[
            { key: "input", name: t("redisDashboard.netInputShort"), color: C.blue },
            { key: "output", name: t("redisDashboard.netOutputShort"), color: C.amber },
          ]}
        />
        <LinePanel
          title={t("redisDashboard.chartCpu")}
          data={cpu}
          formatter={(value) => formatNumber(value, 3)}
          lines={[
            { key: "user", name: t("redisDashboard.cpuUser"), color: C.green },
            { key: "sys", name: t("redisDashboard.cpuSys"), color: C.blue },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 8. LOG SIGNALS ═════════════════════ */}
      <SectionHeader title="Log Signals" description="Events parsed from Redis logs via Loki" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-7">
          <StatCard label={t("redisDashboard.warnings")} value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label={t("redisDashboard.errors")} value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label={t("redisDashboard.authFailures")} value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label={t("redisDashboard.security")} value={formatNumber(data?.log_summary.security ?? 0, 0)} tone={(data?.log_summary.security ?? 0) > 0 ? "bad" : "good"} />
          <StatCard label={t("redisDashboard.shutdowns")} value={formatNumber(data?.log_summary.shutdowns ?? 0, 0)} tone={(data?.log_summary.shutdowns ?? 0) > 0 ? "warn" : "good"} />
          <StatCard label={t("redisDashboard.startups")} value={formatNumber(data?.log_summary.startups ?? 0, 0)} />
          <StatCard label={t("redisDashboard.persistence")} value={formatNumber(data?.log_summary.persistence ?? 0, 0)} />
        </div>
      </SectionCard>
    </Page>
  )
}
