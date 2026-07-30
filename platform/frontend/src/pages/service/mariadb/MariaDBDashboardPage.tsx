import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams, useSearchParams } from "react-router-dom"

import { MariaDBSummaryTab } from "./components/MariaDBSummaryTab"
import { MariaDBConfigTab } from "./components/MariaDBConfigTab"
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts"

import { getDiscoveryService } from "@/api/discovery"
import { getMariaDBDashboardSummary, type MariaDBDashboardSummary, type MariaDBTimePoint } from "@/api/mariadb"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  DashboardStatCard,
  DashboardSectionHeader,
  DashboardNoData,
  DashboardLinePanel,
  DashboardAreaPanel,
  DashboardCustomBarPanel,
  DashboardDonutCard,
  DASHBOARD_CHART_STYLE,
} from "@/components/shared/DashboardComponents"
import { TimeRangeSelector, TIME_RANGES, type TimeSelection } from "@/components/shared/TimeRangeSelector"
import {
  dateTimeInputToSeconds,
  formatBytes,
  formatDuration,
  formatNumber,
  formatPercent,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation } from "@/i18n/LanguageContext"
import { MariaDBDigestsTable } from "./components/MariaDBDigestsTable"

function getStep(rangeSeconds: number) {
  if (rangeSeconds <= 900) return "15s"
  if (rangeSeconds <= 3600) return "30s"
  if (rangeSeconds <= 21600) return "2m"
  return "5m"
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

const MARIADB_COLORS = {
  blue: "#0ea5e9", green: "#22c55e", amber: "#f59e0b", red: "#ef4444",
  violet: "#8b5cf6", teal: "#14b8a6", rose: "#f43f5e", cyan: "#06b6d4",
  indigo: "#6366f1", orange: "#f97316",
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
    { name: "SELECT", value: data?.selects_per_sec ?? 0, fill: MARIADB_COLORS.blue },
    { name: "INSERT", value: data?.inserts_per_sec ?? 0, fill: MARIADB_COLORS.green },
    { name: "UPDATE", value: data?.updates_per_sec ?? 0, fill: MARIADB_COLORS.amber },
    { name: "DELETE", value: data?.deletes_per_sec ?? 0, fill: MARIADB_COLORS.red },
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
            <button
              type="button"
              onClick={() => dashQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/services/mariadb-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
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
      <DashboardSectionHeader title="Status" description="Instance health at a glance" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="MySQL Up" value={data?.up === 1 ? "Online" : data?.up === 0 ? "Down" : "-"} tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint="mysql_up — 1 = reachable" />
        <DashboardStatCard label="Exporter Scrape" value={data?.scrape_error === 0 ? "OK" : data?.scrape_error === 1 ? "Error" : "-"} tone={data?.scrape_error === 1 ? "bad" : data?.scrape_error === 0 ? "good" : "neutral"} hint="Last scrape success" />
        <DashboardStatCard label="Uptime" value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} hint="Time since last restart" />
        <DashboardStatCard label="Replication Lag" value={data?.replication_lag_seconds != null ? formatDuration(data.replication_lag_seconds) : "N/A"} tone={(data?.replication_lag_seconds ?? 0) > 30 ? "bad" : (data?.replication_lag_seconds ?? 0) > 5 ? "warn" : "good"} hint={data?.slave_running === 1 ? "Replica running" : "No active replica"} />
      </div>
      {(data?.wsrep_cluster_size != null) && (
        <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4 mt-2">
          <DashboardStatCard label="Galera Cluster Size" value={data?.wsrep_cluster_size != null ? formatNumber(data.wsrep_cluster_size, 0) : "-"} hint="Number of Galera nodes" />
          <DashboardStatCard label="Galera Connected" value={data?.wsrep_connected === 1 ? "Yes" : data?.wsrep_connected === 0 ? "No" : "-"} tone={data?.wsrep_connected === 1 ? "good" : data?.wsrep_connected === 0 ? "bad" : "neutral"} hint="Is this node connected to the cluster?" />
          <DashboardStatCard label="Galera Ready" value={data?.wsrep_ready === 1 ? "Yes" : data?.wsrep_ready === 0 ? "No" : "-"} tone={data?.wsrep_ready === 1 ? "good" : data?.wsrep_ready === 0 ? "bad" : "neutral"} hint="Is this node ready to accept queries?" />
        </div>
      )}

      {/* ═══════════════════════════════ 2. QUERY THROUGHPUT ════════════════ */}
      <DashboardSectionHeader title="Query Throughput" description="Queries per second, rows read/written, and read/write breakdown" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="QPS" value={data?.qps != null ? `${formatNumber(data.qps, 1)}/s` : "-"} hint="Questions/sec (all statements)" />
        <DashboardStatCard label="Slow Queries/min" value={data?.slow_queries_per_min != null ? formatNumber(data.slow_queries_per_min, 2) : "-"} tone={(data?.slow_queries_per_min ?? 0) > 0 ? "warn" : "good"} hint="Queries exceeding long_query_time" />
        <DashboardStatCard label="SELECTs/sec" value={data?.selects_per_sec != null ? `${formatNumber(data.selects_per_sec, 1)}/s` : "-"} hint="Read workload" />
        <DashboardStatCard label="Writes/sec" value={data != null ? `${formatNumber((data.inserts_per_sec ?? 0) + (data.updates_per_sec ?? 0) + (data.deletes_per_sec ?? 0), 1)}/s` : "-"} hint="INSERT + UPDATE + DELETE" />
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Rows Read/sec" value={data?.rows_read_per_sec != null ? `${formatNumber(data.rows_read_per_sec, 0)}/s` : "-"} hint="Handler rows read (all engines)" />
        <DashboardStatCard label="Rows Modified/sec" value={data?.rows_modified_per_sec != null ? `${formatNumber(data.rows_modified_per_sec, 0)}/s` : "-"} hint="Handler writes/updates/deletes" />
        <DashboardStatCard label="Sort Merge Passes/sec" value={data?.sort_merge_passes_per_sec != null ? formatNumber(data.sort_merge_passes_per_sec, 2) : "-"} tone={(data?.sort_merge_passes_per_sec ?? 0) > 0.5 ? "warn" : "good"} hint="Disk-based sort merges — increase sort_buffer_size" />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="xl:col-span-2">
<DashboardAreaPanel title="Read / Write Breakdown Over Time" data={rw} stacked={false} areas={[
          { dataKey: "selects", name: "SELECT", color: MARIADB_COLORS.blue },
          { dataKey: "inserts", name: "INSERT", color: MARIADB_COLORS.green },
          { dataKey: "updates", name: "UPDATE", color: MARIADB_COLORS.amber },
          { dataKey: "deletes", name: "DELETE", color: MARIADB_COLORS.red },
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
                  <Tooltip contentStyle={DASHBOARD_CHART_STYLE.tooltip} formatter={(v) => `${formatNumber(Number(v), 2)}/s`} />
                </PieChart>
              </ResponsiveContainer>
              <div className="mt-2 grid grid-cols-2 gap-1 text-sm">
                {rwPieData.map((d) => (
                  <div key={d.name} className="flex items-center gap-1.5">
                    <div className="h-2 w-2 rounded-full" style={{ background: d.fill }} />
                    <span className="text-muted-foreground">{d.name}:</span>
                    <span className="tabular-nums">{formatNumber(d.value, 2)}/s</span>
                  </div>
                ))}
              </div>
            </>
          ) : <DashboardNoData />}
        </div>
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="QPS Over Time" data={qps} lines={[{ dataKey: "qps", name: "QPS", color: MARIADB_COLORS.green }]} formatter={(v) => `${formatNumber(v, 1)}/s`} />
        <DashboardLinePanel title="Aborted Connects & Clients / min" data={slow} lines={[
          { dataKey: "aborted_connects_per_min", name: "Aborted Connects", color: MARIADB_COLORS.red },
          { dataKey: "aborted_clients_per_min", name: "Aborted Clients", color: MARIADB_COLORS.rose },
          { dataKey: "slow_queries_per_min", name: "Slow Queries/min", color: MARIADB_COLORS.amber },
        ]} formatter={(v) => formatNumber(v, 2)} />
      </div>

      {/* ═══════════════════════════════ 3. CONNECTIONS ═════════════════════ */}
      <DashboardSectionHeader title="Connections" description="Thread pool saturation, peak usage, and error rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Connection Saturation" value={data?.connection_saturation_pct != null ? formatPercent(data.connection_saturation_pct) : "-"} tone={(data?.connection_saturation_pct ?? 0) > 85 ? "bad" : (data?.connection_saturation_pct ?? 0) > 70 ? "warn" : "good"} hint={`${data?.threads_connected ?? "-"} / ${data?.max_connections ?? "-"} used`} />
        <DashboardStatCard label="Threads Running" value={data?.threads_running != null ? formatNumber(data.threads_running, 0) : "-"} tone={(data?.threads_running ?? 0) > (data?.max_connections ?? 100) * 0.5 ? "warn" : "neutral"} hint="Active query threads" />
        <DashboardStatCard label="Max Used Connections" value={data?.max_used_connections != null ? formatNumber(data.max_used_connections, 0) : "-"} hint="Peak concurrent connections since server start" />
        <DashboardStatCard label="Aborted Connects/min" value={data?.aborted_connects_per_min != null ? formatNumber(data.aborted_connects_per_min, 2) : "-"} tone={(data?.aborted_connects_per_min ?? 0) > 1 ? "warn" : "good"} hint="Failed TCP handshakes or auth" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="Connections Over Time" data={connections} stacked={false} areas={[
          { dataKey: "connected", name: "Connected", color: MARIADB_COLORS.blue },
          { dataKey: "running", name: "Running", color: MARIADB_COLORS.green },
          { dataKey: "cached", name: "Cached", color: MARIADB_COLORS.teal },
        ]} />
      </div>

      {/* ═══════════════════════════════ 4. INNODB BUFFER POOL ═══════════════ */}
      <DashboardSectionHeader title="InnoDB Buffer Pool" description="The most important memory structure — aim for >99% hit ratio" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardDonutCard title="Hit Ratio" value={data?.buffer_pool_hit_ratio_pct ?? 0} color={(data?.buffer_pool_hit_ratio_pct ?? 100) < 95 ? MARIADB_COLORS.red : MARIADB_COLORS.green} label="cache effectiveness" hint="< 95% = needs more memory" />
        <DashboardDonutCard title="Used" value={data?.buffer_pool_used_pct ?? 0} color={MARIADB_COLORS.blue} label="of pool allocated" hint="High = pool is active" />
        <DashboardDonutCard title="Dirty Pages" value={data?.buffer_pool_dirty_pct ?? 0} color={(data?.buffer_pool_dirty_pct ?? 0) > 75 ? MARIADB_COLORS.amber : MARIADB_COLORS.teal} label="of data bytes modified" hint="High sustained = I/O pressure" />
        <DashboardStatCard label="Pool Size" value={data?.innodb_buffer_pool_size_bytes != null ? formatBytes(data.innodb_buffer_pool_size_bytes) : "-"} hint="innodb_buffer_pool_size" />
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Wait Free/sec" value={data?.buffer_pool_wait_free_per_sec != null ? `${formatNumber(data.buffer_pool_wait_free_per_sec, 2)}/s` : "-"} tone={(data?.buffer_pool_wait_free_per_sec ?? 0) > 0 ? "warn" : "good"} hint="Non-zero ⇒ InnoDB waiting for free pages (pool too small)" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="Buffer Pool % Over Time" data={bufferPool} formatter={(v) => formatPercent(v)} lines={[
          { dataKey: "hit_ratio", name: "Hit Ratio %", color: MARIADB_COLORS.teal },
          { dataKey: "dirty_pct", name: "Dirty %", color: MARIADB_COLORS.amber },
          { dataKey: "used_pct", name: "Used %", color: MARIADB_COLORS.blue },
        ]} />
      </div>

      {/* ═══════════════════════════════ 4b. INNODB HEALTH ════════════════════ */}
      <DashboardSectionHeader title="InnoDB Health" description="History list length, log waits, and pending I/O indicate internal pressure" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="History List Length" value={data?.innodb_history_list_length != null ? formatNumber(data.innodb_history_list_length, 0) : "-"} tone={(data?.innodb_history_list_length ?? 0) > 100000 ? "bad" : (data?.innodb_history_list_length ?? 0) > 10000 ? "warn" : "good"} hint="High sustained = long-running tx blocking purge" />
        <DashboardStatCard label="Log Waits/sec" value={data?.innodb_log_waits_per_sec != null ? `${formatNumber(data.innodb_log_waits_per_sec, 2)}/s` : "-"} tone={(data?.innodb_log_waits_per_sec ?? 0) > 0 ? "warn" : "good"} hint="Non-zero ⇒ innodb_log_buffer_size too small" />
        <DashboardStatCard label="Pending Reads" value={data?.innodb_pending_reads != null ? formatNumber(data.innodb_pending_reads, 0) : "-"} tone={(data?.innodb_pending_reads ?? 0) > 0 ? "warn" : "good"} hint="I/O operations queued for reads" />
        <DashboardStatCard label="Pending Writes" value={data?.innodb_pending_writes != null ? formatNumber(data.innodb_pending_writes, 0) : "-"} tone={(data?.innodb_pending_writes ?? 0) > 0 ? "warn" : "good"} hint="I/O operations queued for writes" />
        <DashboardStatCard label="Pending fsyncs" value={data?.innodb_pending_fsyncs != null ? formatNumber(data.innodb_pending_fsyncs, 0) : "-"} hint="Pending fsync I/O operations" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="Pending I/O Operations Over Time" data={pendingIO} formatter={(v) => formatNumber(v, 0)} lines={[
          { dataKey: "pending_reads", name: "Pending Reads", color: MARIADB_COLORS.blue },
          { dataKey: "pending_writes", name: "Pending Writes", color: MARIADB_COLORS.orange },
          { dataKey: "pending_fsyncs", name: "Pending fsyncs", color: MARIADB_COLORS.violet },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5. INNODB I/O ═══════════════════════ */}
      <DashboardSectionHeader title="InnoDB Disk I/O" description="Physical reads/writes indicate storage pressure" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Data Reads/sec" value={data?.innodb_data_reads_per_sec != null ? `${formatBytes(data.innodb_data_reads_per_sec)}/s` : "-"} hint="Physical disk reads" />
        <DashboardStatCard label="Data Writes/sec" value={data?.innodb_data_writes_per_sec != null ? `${formatBytes(data.innodb_data_writes_per_sec)}/s` : "-"} hint="Physical disk writes" />
        <DashboardStatCard label="fsyncs/sec" value={data?.innodb_fsyncs_per_sec != null ? formatNumber(data.innodb_fsyncs_per_sec, 1) : "-"} tone={(data?.innodb_fsyncs_per_sec ?? 0) > 100 ? "warn" : "neutral"} hint="Disk flush calls/sec" />
        <DashboardStatCard label="Log Writes/sec" value={data?.innodb_log_writes_per_sec != null ? formatNumber(data.innodb_log_writes_per_sec, 1) : "-"} hint="Redo log write rate" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="InnoDB I/O Bytes Over Time" data={io} stacked={false} formatter={(v, key) => {
          if (typeof v !== "number" || !Number.isFinite(v)) return "0"
          if (key === "log_writes") return `${formatNumber(v, 1)}/s`
          return `${formatBytes(v)}/s`
        }} areas={[
          { dataKey: "reads_bps", name: "Reads", color: MARIADB_COLORS.blue },
          { dataKey: "writes_bps", name: "Writes", color: MARIADB_COLORS.orange },
          { dataKey: "log_writes", name: "Log Writes", color: MARIADB_COLORS.violet },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5b. FULL SCAN DETECTION ═══════════════ */}
      <DashboardSectionHeader title="Full Scan Detection" description="High read_rnd_next vs read_key ⇒ sequential scans (missing indexes)" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Table Scan %" value={data?.table_scan_ratio_pct != null ? formatPercent(data.table_scan_ratio_pct) : "-"} tone={(data?.table_scan_ratio_pct ?? 0) > 50 ? "bad" : (data?.table_scan_ratio_pct ?? 0) > 20 ? "warn" : "good"} hint="% handler ops that are full table scans" />
        <DashboardStatCard label="Handler Read Key/sec" value={data?.handler_read_key_per_sec != null ? `${formatNumber(data.handler_read_key_per_sec, 0)}/s` : "-"} hint="Index-based row lookups" />
        <DashboardStatCard label="Handler Read Rnd Next/sec" value={data?.handler_read_rnd_next_per_sec != null ? `${formatNumber(data.handler_read_rnd_next_per_sec, 0)}/s` : "-"} tone={(data?.handler_read_rnd_next_per_sec ?? 0) > (data?.handler_read_key_per_sec ?? 1) * 5 ? "warn" : "good"} hint="Full scan row reads — high relative to read_key = missing index" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="Handler Read Operations Over Time" data={handlerRead} stacked={false} formatter={(v) => `${formatNumber(v, 0)}/s`} areas={[
          { dataKey: "read_rnd_next", name: "Full Scans (read_rnd_next)", color: MARIADB_COLORS.red },
          { dataKey: "read_key", name: "Index Lookups (read_key)", color: MARIADB_COLORS.green },
          { dataKey: "read_first", name: "First Entry (read_first)", color: MARIADB_COLORS.blue },
        ]} />
      </div>

      {/* ═══════════════════════════════ 5c. SORT OPERATIONS ════════════════════ */}
      <DashboardSectionHeader title="Sort Operations" description="Sorts by type — merge passes indicate disk spill (small sort_buffer_size)" />
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="Sort Operations Over Time" data={sortOps} formatter={(v) => `${formatNumber(v, 1)}/s`} stacked={false} areas={[
          { dataKey: "range", name: "Range Sorts", color: MARIADB_COLORS.teal },
          { dataKey: "scan", name: "Scan Sorts", color: MARIADB_COLORS.blue },
          { dataKey: "merge_passes", name: "Merge Passes (disk)", color: MARIADB_COLORS.red },
        ]} />
      </div>

      {/* ═══════════════════════════════ 6. LOCKS & CONTENTION ═══════════════ */}
      <DashboardSectionHeader title="Locks & Contention" description="Deadlocks and waits indicate query or schema issues" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Table Locks Waited/min" value={data?.table_locks_waited_per_min != null ? formatNumber(data.table_locks_waited_per_min, 2) : "-"} tone={(data?.table_locks_waited_per_min ?? 0) > 0 ? "warn" : "good"} hint="MyISAM or explicit table locks" />
        <DashboardStatCard label="InnoDB Deadlocks/min" value={data?.innodb_deadlocks_per_min != null ? formatNumber(data.innodb_deadlocks_per_min, 2) : "-"} tone={(data?.innodb_deadlocks_per_min ?? 0) > 0 ? "bad" : "good"} hint="Cyclic row lock conflicts" />
        <DashboardStatCard label="Row Lock Waits/min" value={data?.innodb_row_lock_waits_per_min != null ? formatNumber(data.innodb_row_lock_waits_per_min, 2) : "-"} tone={(data?.innodb_row_lock_waits_per_min ?? 0) > 10 ? "warn" : "good"} hint="InnoDB row-level contention" />
        <DashboardStatCard label="Avg Row Lock Time" value={data?.innodb_row_lock_time_avg_ms != null ? `${formatNumber(data.innodb_row_lock_time_avg_ms, 1)} ms` : "-"} tone={(data?.innodb_row_lock_time_avg_ms ?? 0) > 50 ? "warn" : "neutral"} hint="Average wait per lock event" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="Lock Contention Over Time" data={locks} formatter={(v) => `${formatNumber(v, 2)}/min`} lines={[
          { dataKey: "table_locks_waited_pm", name: "Table Locks Waited", color: MARIADB_COLORS.amber },
          { dataKey: "innodb_deadlocks_pm", name: "Deadlocks", color: MARIADB_COLORS.red },
          { dataKey: "row_lock_waits_pm", name: "Row Lock Waits", color: MARIADB_COLORS.rose },
        ]} />
        <DashboardLinePanel title="InnoDB Deadlocks Over Time" data={deadlocks} lines={[{ dataKey: "deadlocks_pm", name: "Deadlocks/min", color: MARIADB_COLORS.red }]} formatter={(v) => `${formatNumber(v, 2)}/min`} />
      </div>

      {/* ═══════════════════════════════ 7. TEMP TABLES ══════════════════════ */}
      <DashboardSectionHeader title="Temporary Tables" description="Disk-based temp tables are expensive — tune tmp_table_size" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Tmp Tables/sec (memory)" value={data?.tmp_tables_per_sec != null ? `${formatNumber(data.tmp_tables_per_sec, 2)}/s` : "-"} hint="In-memory temp tables (fast)" />
        <DashboardStatCard label="Tmp Disk Tables/sec" value={data?.tmp_disk_tables_per_sec != null ? `${formatNumber(data.tmp_disk_tables_per_sec, 2)}/s` : "-"} tone={(data?.tmp_disk_tables_per_sec ?? 0) > 0.5 ? "warn" : "good"} hint="Disk spills — increase tmp_table_size" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="Temporary Tables Created Over Time" data={tmpTables} stacked={false} formatter={(v) => `${formatNumber(v, 3)}/s`} areas={[
          { dataKey: "memory_per_sec", name: "Memory tmp tables", color: MARIADB_COLORS.teal },
          { dataKey: "disk_per_sec", name: "Disk tmp tables", color: MARIADB_COLORS.red },
        ]} />
      </div>

      {/* ═══════════════════════════════ 8. NETWORK ═══════════════════════════ */}
      <DashboardSectionHeader title="Network" description="Client ↔ server data transfer rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="RX (client → server)" value={data?.network_rx_bytes_per_sec != null ? `${formatBytes(data.network_rx_bytes_per_sec)}/s` : "-"} hint="Incoming query bytes" />
        <DashboardStatCard label="TX (server → client)" value={data?.network_tx_bytes_per_sec != null ? `${formatBytes(data.network_tx_bytes_per_sec)}/s` : "-"} hint="Outgoing result bytes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardAreaPanel title="Network Throughput Over Time" data={network} stacked={false} formatter={(v) => `${formatBytes(v)}/s`} areas={[
          { dataKey: "rx_bps", name: "Received", color: MARIADB_COLORS.blue },
          { dataKey: "tx_bps", name: "Sent", color: MARIADB_COLORS.green },
        ]} />
      </div>

      {/* ═══════════════════════════════ 8b. STORAGE ═══════════════════════════ */}
      <DashboardSectionHeader title="Storage" description="Binary log growth and top tables by estimated size" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Binlog Size" value={data?.binlog_size_bytes != null ? formatBytes(data.binlog_size_bytes) : "-"} hint="Current total binary log disk usage" />
        <DashboardStatCard label="Binlog Files" value={data?.binlog_files != null ? formatNumber(data.binlog_files, 0) : "-"} hint="Number of binlog files on disk" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="Binary Log Size Over Time" data={binlog} formatter={(v) => formatBytes(v)} lines={[{ dataKey: "size_bytes", name: "Binlog Size", color: MARIADB_COLORS.cyan }]} />
        <DashboardCustomBarPanel title="Top Tables by Estimated Size" data={(data?.top_table_sizes ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatBytes(v)} />
      </div>

      {/* ═══════════════════════════════ 9. CACHE EFFICIENCY ═════════════════ */}
      <DashboardSectionHeader title="Cache Efficiency" description="Query cache and table open-cache effectiveness" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="Query Cache Hit %" value={data?.qcache_hit_ratio_pct != null ? formatPercent(data.qcache_hit_ratio_pct) : "N/A"} tone={data?.qcache_hit_ratio_pct != null && data.qcache_hit_ratio_pct < 50 ? "warn" : "neutral"} hint="Disabled in MySQL 8+ / MariaDB 10.6+" />
        <DashboardStatCard label="Query Cache Free Mem" value={data?.qcache_free_mem_bytes != null ? formatBytes(data.qcache_free_mem_bytes) : "N/A"} hint="Remaining space in query cache" />
        <DashboardStatCard label="Table Cache Hit %" value={data?.table_cache_hit_ratio_pct != null ? formatPercent(data.table_cache_hit_ratio_pct) : "-"} tone={(data?.table_cache_hit_ratio_pct ?? 100) < 90 ? "warn" : "good"} hint="Increase table_open_cache if low" />
        <DashboardStatCard label="Open Tables" value={data?.open_tables != null ? formatNumber(data.open_tables, 0) : "-"} hint="Currently open table handles" />
      </div>

      {/* ═══════════════════════════════ 10. QUERY ANALYSIS ═══════════════════ */}
      <DashboardSectionHeader title="Query Analysis" description="Performance Schema digest data — requires performance_schema=ON" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-2">
        <DashboardStatCard label="Total Query Time (digests)" value={data?.query_digest_total_time_seconds != null ? formatDuration(data.query_digest_total_time_seconds) : "-"} hint="Cumulative time across all digest classes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title="Total Query Time (digests) Over Time" data={queryTime} formatter={(v) => formatDuration(v)} lines={[{ dataKey: "total_seconds", name: "Total Time", color: MARIADB_COLORS.violet }]} />
        <DashboardLinePanel title="Query Execution Count Over Time" data={queryCount} formatter={(v) => formatNumber(v)} lines={[{ dataKey: "exec_count", name: "Exec Count", color: MARIADB_COLORS.cyan }]} />
      </div>

      <MariaDBDigestsTable title="Top Queries by Total Time" digests={data?.top_query_digests ?? []} valueFn={(d) => d.total_time_seconds} valueLabel="Total Time" valueFormatter={(v) => formatDuration(v)} />
      <MariaDBDigestsTable title="Inefficient Queries (Rows Examined / Sent)" digests={data?.inefficient_digests ?? []} valueFn={(d) => d.rows_examined_per_sent} valueLabel="Rows Exam/Sent" valueFormatter={(v) => formatNumber(v, 1)} />
      <MariaDBDigestsTable title="Queries Without Index" digests={data?.no_index_digests ?? []} valueFn={(d) => d.no_index_used_count} valueLabel="No-Index Count" valueFormatter={(v) => formatNumber(v, 0)} />

      {/* ═══════════════════════════════ 11. PER-DATABASE ACTIVITY ═════════════ */}
      <DashboardSectionHeader title="Per-Database Activity" description="Query volume, cost, and storage grouped by schema — aggregated from digest metrics and slow query logs" />
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardCustomBarPanel title="Top DBs by Query Rate" data={(data?.top_dbs_by_queries ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => `${formatNumber(v, 1)}/s`} />
        <DashboardCustomBarPanel title="Top DBs by Total Query Time" data={(data?.top_dbs_by_query_time ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatDuration(v)} />
        <DashboardCustomBarPanel title="Top DBs by Rows Examined" data={(data?.top_dbs_by_rows_examined ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatNumber(v, 0)} />
        <DashboardCustomBarPanel title="Top DBs by No-Index Queries" data={(data?.top_dbs_by_no_index ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatNumber(v, 0)} />
        <DashboardCustomBarPanel title="Storage per Database" data={(data?.db_storage_sizes ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatBytes(v)} />
        <DashboardCustomBarPanel title="Slow Queries per Database (from logs)" data={(data?.slow_queries_by_db ?? []).map(d => ({ name: d.label, value: d.count }))} formatter={(v) => formatNumber(v, 0)} />
      </div>

      {/* ═══════════════════════════════ 12. LOG SIGNALS ═════════════════════ */}
      <DashboardSectionHeader title="Log Signals" description="Events parsed from MariaDB logs via Loki" />
      <div className="mb-4 flex items-center gap-3">
        <label className="text-sm font-medium text-muted-foreground whitespace-nowrap">Min Slow Query Duration (s):</label>
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
          className="h-8 rounded-md border border-border px-3 text-sm hover:bg-muted"
        >
          Apply
        </button>
        {minDurationSec && (
          <button
            type="button"
            onClick={() => { setMinDurationSec(""); setMinDurationSecInput("") }}
            className="h-8 rounded-md border border-border px-3 text-sm text-muted-foreground hover:bg-muted"
          >
            Clear
          </button>
        )}
      </div>
      <SectionCard title="Log Event Counters">
        <div className="grid gap-3 md:grid-cols-4">
          <DashboardStatCard label="Slow Queries" value={formatNumber(data?.log_summary.slow_queries ?? 0, 0)} tone={(data?.log_summary.slow_queries ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label="Errors" value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label="Warnings" value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label="Auth Failures" value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label="Deadlocks" value={formatNumber(data?.log_summary.deadlocks ?? 0, 0)} tone={(data?.log_summary.deadlocks ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label="Replication Events" value={formatNumber(data?.log_summary.replication ?? 0, 0)} tone={(data?.log_summary.replication ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label="Startups" value={formatNumber(data?.log_summary.startup ?? 0, 0)} />
          <DashboardStatCard label="Shutdowns" value={formatNumber(data?.log_summary.shutdown ?? 0, 0)} />
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
