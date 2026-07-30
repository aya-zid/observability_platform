import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getPostgresDashboardSummary, type PostgresDashboardSummary, type PostgresTimePoint } from "@/api/postgres"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { DashboardStatCard, DashboardSectionHeader, DashboardLinePanel, DashboardBarPanel } from "@/components/shared/DashboardComponents"
import { TimeRangeSelector, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import type { TimeSelection } from "@/components/shared/TimeRangeSelector"
import { mapTimeseries, latestFromSeries } from "@/lib/dashboardHelpers"
import { dateTimeInputToSeconds, formatBytes, formatDuration, formatNumber, formatPercent, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { PG_CHART_COLORS } from "./components/PostgresColors"
import { PostgresStatusSection } from "./components/PostgresStatusSection"
import { PostgresConnectionsSection } from "./components/PostgresConnectionsSection"
import { PostgresDatabaseHealthTable } from "./components/PostgresDatabaseHealthTable"
import { RefreshCw } from "lucide-react"
import { useTranslation } from "@/i18n/LanguageContext"

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const end = getNowSeconds()
    const start = getStartSeconds(selection.preset.seconds)
    return { start, end, step: selection.preset.seconds <= 900 ? "15s" : selection.preset.seconds <= 3600 ? "30s" : selection.preset.seconds <= 21600 ? "2m" : "5m", error: "", isManual: false }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null) return { start: 0, end: 0, step: "30s", error: "Use DD-MM-YYYY and HH:mm.", isManual: true }
  if (end <= start) return { start, end, step: "30s", error: "End time must be after start time.", isManual: true }
  const step = end - start <= 900 ? "15s" : end - start <= 3600 ? "30s" : end - start <= 21600 ? "2m" : "5m"
  return { start, end, step, error: "", isManual: true }
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
  const idleInTx = latestFromSeries(data?.activity_timeseries, "idle in transaction")
  const sizeFirstLast = totalFirstAndLast(data?.size_timeseries)
  const sizeGrowthPerHour = sizeFirstLast && sizeFirstLast.end.time > sizeFirstLast.start.time
    ? ((sizeFirstLast.end.value - sizeFirstLast.start.value) / ((sizeFirstLast.end.time - sizeFirstLast.start.time) / 3600))
    : null
  const seqReturned = latestFromSeries(data?.seq_vs_index_timeseries, "returned") ?? 0
  const indexFetched = latestFromSeries(data?.seq_vs_index_timeseries, "fetched") ?? 0
  const seqScanRatio = seqReturned + indexFetched > 0 ? (100 * seqReturned) / (seqReturned + indexFetched) : null
  const bgwriterClean = latestFromSeries(data?.bgwriter_timeseries, "clean") ?? 0
  const bgwriterAlloc = latestFromSeries(data?.bgwriter_timeseries, "alloc") ?? 0
  const bgwriterPressure = bgwriterClean + bgwriterAlloc > 0 ? (100 * bgwriterAlloc) / (bgwriterClean + bgwriterAlloc) : null
  const tempBytesLatest = latestFromSeries(data?.temp_timeseries, "bytes")
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
            <button
              type="button"
              onClick={() => dashQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', dashQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/services/postgres-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              {t("shared.viewLogs")}
            </Link>
            <Link to={`/services/postgres-config/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
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
      <PostgresStatusSection up={data?.up} scrapeError={data?.scrape_error} replicaRole={data?.replica_role} cacheHitRatioPct={data?.cache_hit_ratio_pct} />

      {/* ═══════════════════════════════ 2. CONNECTIONS ═════════════════════ */}
      <PostgresConnectionsSection
        connectionSaturationPct={data?.connection_saturation_pct}
        maxConnections={data?.max_connections}
        idleInTx={idleInTx}
        maxTxDurationSeconds={data?.max_tx_duration_seconds}
        connectionsData={connections}
        activityData={activity}
        connByDatabase={data?.conn_by_database ?? []}
        connSaturationByDb={data?.conn_saturation_by_db ?? []}
      />

      {/* ═══════════════════════════════ 3. TRANSACTIONS ════════════════════ */}
      <DashboardSectionHeader title="Transactions" description="Commit, rollback, and deadlock rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.rollbackRate")} value={data?.rollback_rate_pct != null ? formatPercent(data.rollback_rate_pct) : "-"} tone={(data?.rollback_rate_pct ?? 0) > 2 ? "bad" : (data?.rollback_rate_pct ?? 0) > 1 ? "warn" : "good"} hint={t("pgDashboard.rollbackRateHint")} />
        <DashboardStatCard label={t("pgDashboard.deadlocks")} value={data?.deadlocks_per_min != null ? formatNumber(data.deadlocks_per_min, 2) : "-"} tone={(data?.deadlocks_per_min ?? 0) > 0 ? "bad" : "good"} hint={t("pgDashboard.deadlocksHint")} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("pgDashboard.chartTransactions")}
          data={transactions}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "commits", name: "commits/s", color: PG_CHART_COLORS.green },
            { dataKey: "rollbacks", name: "rollbacks/s", color: PG_CHART_COLORS.amber },
            { dataKey: "deadlocks", name: "deadlocks/s", color: PG_CHART_COLORS.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 4. CACHE ═══════════════════════════ */}
      <DashboardSectionHeader title="Cache" description="Buffer cache hit ratio and disk read/write effectiveness" />
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("pgDashboard.chartCache")}
          data={cache}
          formatter={(value, name) => name === "hit_ratio" ? formatPercent(value) : `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "hit_ratio", name: "hit ratio %", color: PG_CHART_COLORS.teal },
            { dataKey: "hits", name: "buffer hits/s", color: PG_CHART_COLORS.green },
            { dataKey: "reads", name: "disk reads/s", color: PG_CHART_COLORS.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 5. STORAGE & WAL ═══════════════════ */}
      <DashboardSectionHeader title="Storage & WAL" description="Database size, WAL generation rate, and growth trends" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.dbSize")} value={data?.database_size_bytes != null ? formatBytes(data.database_size_bytes) : "-"} hint={t("pgDashboard.dbSizeHint")} />
        <DashboardStatCard label={t("pgDashboard.walSize")} value={data?.wal_size_bytes != null ? formatBytes(data.wal_size_bytes) : "-"} tone={(data?.wal_size_bytes ?? 0) > 1024 * 1024 * 1024 ? "warn" : "neutral"} hint={t("pgDashboard.walSizeHint")} />
        <DashboardStatCard label="WAL Rate" value={data?.wal_rate_per_sec != null ? `${formatBytes(data.wal_rate_per_sec)}/s` : "-"} tone={(data?.wal_rate_per_sec ?? 0) > 10 * 1024 * 1024 ? "warn" : "neutral"} hint="WAL generation rate" />
        <DashboardStatCard label={t("pgDashboard.dbGrowth")} value={sizeGrowthPerHour != null ? `${formatBytes(Math.abs(sizeGrowthPerHour))}/h` : "-"} tone={(sizeGrowthPerHour ?? 0) > 100 * 1024 * 1024 ? "warn" : "neutral"} hint={totalSizeLatest != null ? `${t("pgDashboard.totalInRange", formatBytes(totalSizeLatest))}` : undefined} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title={t("pgDashboard.chartWAL")} data={wal} formatter={(value) => formatBytes(value)} lines={[{ dataKey: "bytes", name: "WAL bytes", color: PG_CHART_COLORS.violet }]} />
        <DashboardLinePanel title="Temp Spill" data={temp} formatter={(value, name) => name === "bytes" ? `${formatBytes(value)}/s` : `${formatNumber(value)} /s`} lines={[
          { dataKey: "files", name: "temp files/s", color: PG_CHART_COLORS.amber },
          { dataKey: "bytes", name: "temp bytes/s", color: PG_CHART_COLORS.red },
        ]} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardBarPanel title={t("pgDashboard.chartDbSizes")} data={(data?.database_sizes ?? []).map((d) => ({ name: d.label, value: d.count }))} formatter={formatBytes} />
      </div>

      {/* ═══════════════════════════════ 5b. DATABASE HEALTH ════════════════ */}
      <DashboardSectionHeader title="Database Health" description="Per-database metrics: size, connections, cache, temp spills, rollback rate" />
      <PostgresDatabaseHealthTable rows={data?.database_health ?? []} />

      {/* ═══════════════════════════════ 6. ROW THROUGHPUT ══════════════════ */}
      <DashboardSectionHeader title="Row Throughput" description="Row operations and sequential vs. index scan ratio" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.seqScan")} value={seqScanRatio != null ? formatPercent(seqScanRatio) : "-"} tone={(seqScanRatio ?? 0) > 60 ? "bad" : (seqScanRatio ?? 0) > 30 ? "warn" : "good"} hint={t("pgDashboard.seqScanHint")} />
        <DashboardStatCard label={t("pgDashboard.tempSpill")} value={tempBytesLatest != null ? `${formatBytes(tempBytesLatest)}/s` : "-"} tone={(tempBytesLatest ?? 0) > 0 ? "warn" : "good"} hint={t("pgDashboard.tempSpillHint")} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("pgDashboard.chartRowThroughput")}
          data={rowOps}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "inserts", name: "inserts/s", color: PG_CHART_COLORS.green },
            { dataKey: "updates", name: "updates/s", color: PG_CHART_COLORS.blue },
            { dataKey: "deletes", name: "deletes/s", color: PG_CHART_COLORS.red },
          ]}
        />
        <DashboardLinePanel
          title={t("pgDashboard.chartSeqVsIndex")}
          data={seqVsIndex}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "returned", name: "seq returned/s", color: PG_CHART_COLORS.amber },
            { dataKey: "fetched", name: "index fetched/s", color: PG_CHART_COLORS.green },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 7. REPLICATION & WAL ═══════════════ */}
      <DashboardSectionHeader title="Replication" description="Replication lag, conflicts, and WAL generation" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.replLag")} value={data?.replication_lag_seconds != null ? formatDuration(data.replication_lag_seconds) : "-"} tone={(data?.replication_lag_seconds ?? 0) > 30 ? "bad" : "good"} hint={t("pgDashboard.replLagHint")} />
        <DashboardStatCard label="Repl Lag (bytes)" value={data?.replication_lag_bytes != null ? formatBytes(data.replication_lag_bytes) : "-"} tone={(data?.replication_lag_bytes ?? 0) > 100 * 1024 * 1024 ? "warn" : "good"} hint="Replication lag in WAL bytes" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title="Replication Conflicts"
          data={replicationConflicts}
          formatter={(value) => `${formatNumber(value, 2)} /s`}
          lines={[
            { dataKey: "conflicts", name: "conflicts/s", color: PG_CHART_COLORS.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 8. LOCKS ═════════════════════════ */}
      <DashboardSectionHeader title="Locks & Contention" description="Lock counts by mode and lock-wait events from logs" />
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("pgDashboard.chartLocks")}
          data={locks}
          lines={[
            { dataKey: "AccessShareLock", name: "AccessShare", color: PG_CHART_COLORS.slate },
            { dataKey: "RowExclusiveLock", name: "RowExclusive", color: PG_CHART_COLORS.blue },
            { dataKey: "ExclusiveLock", name: "Exclusive", color: PG_CHART_COLORS.red },
            { dataKey: "AccessExclusiveLock", name: "AccessExclusive", color: "#dc2626" },
          ]}
        />
        <DashboardBarPanel title={t("pgDashboard.chartLocksByMode")} data={(data?.locks_by_mode ?? []).map((d) => ({ name: d.label, value: d.count }))} />
      </div>

      {/* ═══════════════════════════════ 9. BACKGROUND WRITER & ARCHIVER ════ */}
      <DashboardSectionHeader title="Background Writer & Archiver" description="Buffer management and WAL archiving activity" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.bgwriterPressure")} value={bgwriterPressure != null ? formatPercent(bgwriterPressure) : "-"} tone={(bgwriterPressure ?? 0) > 50 ? "warn" : "good"} hint={t("pgDashboard.bgwriterPressureHint")} />
        <DashboardStatCard label={t("pgDashboard.archiveFailed")} value={data?.archive_failed != null ? formatNumber(data.archive_failed, 4) : "-"} tone={(data?.archive_failed ?? 0) > 0 ? "bad" : "good"} hint="WAL archive failures/s" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("pgDashboard.chartBgwriter")}
          data={bgwriter}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "clean", name: "buffers clean/s", color: PG_CHART_COLORS.green },
            { dataKey: "alloc", name: "buffers alloc/s", color: PG_CHART_COLORS.red },
            { dataKey: "maxwritten", name: "maxwritten clean/s", color: PG_CHART_COLORS.blue },
          ]}
        />
        <DashboardLinePanel
          title={t("pgDashboard.chartArchiver")}
          data={archiver}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "archived", name: "archived/s", color: PG_CHART_COLORS.green },
            { dataKey: "failed", name: "failed/s", color: PG_CHART_COLORS.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 10. DISK I/O ════════════════════════ */}
      <DashboardSectionHeader title="Disk I/O" description="Backend-driven buffer writes and fsyncs indicate I/O pressure from query execution" />
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title="Backend I/O"
          data={diskIO}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "backend_writes", name: "backend writes/s", color: PG_CHART_COLORS.orange },
            { dataKey: "backend_fsync", name: "backend fsync/s", color: PG_CHART_COLORS.red },
          ]}
        />
        <DashboardLinePanel
          title="Active Session Duration"
          data={activeSession}
          formatter={(value) => formatDuration(value)}
          lines={[
            { dataKey: "seconds", name: "max tx duration", color: PG_CHART_COLORS.amber },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 11. CONFIG REFERENCE ═══════════════ */}
      <DashboardSectionHeader title="Config Reference" description="Key memory settings from the running configuration" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3">
          <DashboardStatCard label="shared_buffers" value={data?.shared_buffers_bytes != null ? formatBytes(data.shared_buffers_bytes) : "-"} />
          <DashboardStatCard label="work_mem" value={data?.work_mem_bytes != null ? formatBytes(data.work_mem_bytes) : "-"} />
          <DashboardStatCard label="effective_cache_size" value={data?.effective_cache_size_bytes != null ? formatBytes(data.effective_cache_size_bytes) : "-"} />
        </div>
      </SectionCard>

      {/* ═══════════════════════════════ 12. LOG SIGNALS ════════════════════ */}
      <DashboardSectionHeader title="Log Signals" description="Events parsed from PostgreSQL logs via Loki" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-7">
          <DashboardStatCard label={t("pgDashboard.slowQueries")} value={formatNumber(data?.log_summary.slow_queries ?? 0, 0)} tone={(data?.log_summary.slow_queries ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label={t("pgDashboard.errors")} value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label={t("pgDashboard.authFailures")} value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label={t("pgDashboard.lockWaits")} value={formatNumber(data?.log_summary.lock_waits ?? 0, 0)} tone={(data?.log_summary.lock_waits ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label={t("pgDashboard.tempFiles")} value={formatNumber(data?.log_summary.temp_files ?? 0, 0)} tone={(data?.log_summary.temp_files ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label={t("pgDashboard.schemaChanges")} value={formatNumber(data?.log_summary.schema_changes ?? 0, 0)} />
          <DashboardStatCard label={t("pgDashboard.checkpoints")} value={formatNumber(data?.log_summary.checkpoints ?? 0, 0)} tone={(data?.log_summary.checkpoints ?? 0) > 0 ? "warn" : "good"} />
        </div>
      </SectionCard>

    </Page>
  )
}
