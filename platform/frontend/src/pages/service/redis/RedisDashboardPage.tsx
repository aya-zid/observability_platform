import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getRedisDashboardSummary, type RedisDashboardSummary } from "@/api/redis"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  DashboardStatCard,
  DashboardSectionHeader,
  DashboardLinePanel,
  DashboardBarPanel,
  DASHBOARD_COLORS,
} from "@/components/shared/DashboardComponents"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import { getStep, mapTimeseries } from "@/lib/dashboardHelpers"
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
import { useTranslation, LangToggle } from "@/i18n/LanguageContext"

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
            <Link to={`/services/redis-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
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
      <DashboardSectionHeader title="Status" description="Instance health and uptime" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.up")} value={data?.up === 1 ? "Up" : data?.up === 0 ? "Down" : "-"} tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint={t("redisDashboard.upHint")} />
        <DashboardStatCard label={t("redisDashboard.scrape")} value={data?.scrape_error === 1 ? "Error" : data?.scrape_error === 0 ? "Healthy" : "-"} tone={data?.scrape_error === 1 ? "bad" : data?.scrape_error === 0 ? "good" : "neutral"} hint={t("redisDashboard.scrapeHint")} />
        <DashboardStatCard label={t("redisDashboard.uptime")} value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} />
        <DashboardStatCard label="Connected Slaves" value={data?.connected_slaves != null ? formatNumber(data.connected_slaves, 0) : "-"} tone={(data?.connected_slaves ?? 0) === 0 ? "warn" : "good"} hint="Replication clients connected" />
      </div>

      {/* ═══════════════════════════════ 2. MEMORY ══════════════════════════ */}
      <DashboardSectionHeader title="Memory" description="Memory usage, fragmentation, and breakdown by category" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.memUsage")} value={data?.memory_usage_pct != null ? formatPercent(data.memory_usage_pct) : "-"} tone={(data?.memory_usage_pct ?? 0) > 80 ? "bad" : (data?.memory_usage_pct ?? 0) > 60 ? "warn" : "good"} hint={data?.memory_used_bytes != null ? formatBytes(data.memory_used_bytes) : undefined} />
        <DashboardStatCard label={t("redisDashboard.memFrag")} value={data?.memory_frag_ratio != null ? formatNumber(data.memory_frag_ratio, 2) : "-"} tone={(data?.memory_frag_ratio ?? 1) > 5 ? "bad" : (data?.memory_frag_ratio ?? 1) > 2 ? "warn" : "good"} hint={t("redisDashboard.memFragHint")} />
        <DashboardStatCard label={t("redisDashboard.peakMem")} value={data?.memory_peak_bytes != null ? formatBytes(data.memory_peak_bytes) : "-"} />
        <DashboardStatCard label="Lua Memory" value={data?.mem_lua_bytes != null ? formatBytes(data.mem_lua_bytes) : "-"} hint="Memory used by Lua scripts" />
        <DashboardStatCard label="Scripts Cache" value={data?.mem_scripts_bytes != null ? formatBytes(data.mem_scripts_bytes) : "-"} hint="Cached script bodies" />
        <DashboardStatCard label="Startup Overhead" value={data?.mem_startup_bytes != null ? formatBytes(data.mem_startup_bytes) : "-"} hint="Initial memory overhead" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartMemory")}
          data={memory}
          formatter={(value) => formatBytes(value)}
          lines={[
            { dataKey: "used", name: t("redisDashboard.memUsed"), color: DASHBOARD_COLORS.red },
            { dataKey: "rss", name: t("redisDashboard.memRss"), color: DASHBOARD_COLORS.orange },
            { dataKey: "peak", name: t("redisDashboard.memPeak"), color: DASHBOARD_COLORS.slate },
          ]}
        />
        <DashboardLinePanel title={t("redisDashboard.chartFrag")} data={frag} formatter={(value) => formatNumber(value, 2)} lines={[{ dataKey: "ratio", name: t("redisDashboard.fragRatio"), color: DASHBOARD_COLORS.purple }]} />
      </div>

      {/* ═══════════════════════════════ 3. CLIENTS & CONNECTIONS ═══════════ */}
      <DashboardSectionHeader title="Clients & Connections" description="Connection pool saturation and blocked clients" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.clients")} value={data?.connected_clients != null ? formatNumber(data.connected_clients, 0) : "-"} tone={(data?.client_saturation_pct ?? 0) > 80 ? "bad" : (data?.client_saturation_pct ?? 0) > 60 ? "warn" : "good"} hint={data?.client_saturation_pct != null ? t("redisDashboard.clientSatHint", formatPercent(data.client_saturation_pct)) : undefined} />
        <DashboardStatCard label={t("redisDashboard.blockedClients")} value={data?.blocked_clients != null ? formatNumber(data.blocked_clients, 0) : "-"} tone={(data?.blocked_clients ?? 0) > 0 ? "bad" : "good"} hint={t("redisDashboard.blockedClientsHint")} />
        <DashboardStatCard label={t("redisDashboard.rejectedConns")} value={data?.rejected_conns_per_sec != null ? formatNumber(data.rejected_conns_per_sec, 2) : "-"} tone={(data?.rejected_conns_per_sec ?? 0) > 0 ? "bad" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartClients")}
          data={clients}
          formatter={(value) => formatNumber(value, 0)}
          lines={[
            { dataKey: "clients", name: t("redisDashboard.clients"), color: DASHBOARD_COLORS.blue },
            { dataKey: "blocked", name: t("redisDashboard.blockedClients"), color: DASHBOARD_COLORS.red },
          ]}
        />
        <DashboardLinePanel
          title={t("redisDashboard.chartConnRate")}
          data={connRate}
          formatter={(value) => formatNumber(value, 2)}
          lines={[
            { dataKey: "received", name: t("redisDashboard.connReceived"), color: DASHBOARD_COLORS.green },
            { dataKey: "rejected", name: t("redisDashboard.rejectedConns"), color: DASHBOARD_COLORS.red },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 4. COMMANDS & THROUGHPUT ═══════════ */}
      <DashboardSectionHeader title="Commands & Throughput" description="Command processing rate and failures" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.cmdsPerSec")} value={data?.commands_per_sec != null ? formatNumber(data.commands_per_sec, 2) : "-"} />
        <DashboardStatCard label={t("redisDashboard.cmdFailures")} value={data?.command_failures_per_sec != null ? formatNumber(data.command_failures_per_sec, 2) : "-"} tone={(data?.command_failures_per_sec ?? 0) > 0 ? "bad" : "good"} />
        <DashboardStatCard label={t("redisDashboard.slowLog")} value={data?.slow_log_length != null ? formatNumber(data.slow_log_length, 0) : "-"} tone={(data?.slow_log_length ?? 0) > 0 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title={t("redisDashboard.chartCommands")} data={commands} formatter={(value) => `${formatNumber(value)} /s`} lines={[{ dataKey: "commands", name: t("redisDashboard.cmdsPerSec"), color: DASHBOARD_COLORS.green }]} />
        <DashboardLinePanel
          title={t("redisDashboard.chartErrors")}
          data={errors}
          formatter={(value) => `${formatNumber(value, 2)} /s`}
          lines={[
            { dataKey: "errors", name: t("redisDashboard.errorReplies"), color: DASHBOARD_COLORS.orange },
            { dataKey: "unexpected", name: t("redisDashboard.unexpectedErrors"), color: DASHBOARD_COLORS.red },
            { dataKey: "failed", name: t("redisDashboard.cmdFailures"), color: DASHBOARD_COLORS.purple },
            { dataKey: "rejected", name: t("redisDashboard.cmdRejects"), color: DASHBOARD_COLORS.slate },
          ]}
        />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardBarPanel title={t("redisDashboard.chartCommandRates")} data={(data?.command_rates ?? []).map((d) => ({ name: d.label, value: d.count }))} formatter={(value) => `${formatNumber(value, 2)}/s`} />
        <DashboardBarPanel title={t("redisDashboard.chartTopCommands")} data={(data?.commands_top ?? []).map((d) => ({ name: d.label, value: d.count }))} formatter={(value) => formatNumber(value, 3)} />
      </div>

      {/* ═══════════════════════════════ 5. CACHE EFFICIENCY ════════════════ */}
      <DashboardSectionHeader title="Cache Efficiency" description="Keyspace hit ratio and eviction/expiration rates" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.cacheHit")} value={data?.cache_hit_ratio_pct != null ? formatPercent(data.cache_hit_ratio_pct) : "-"} tone={(data?.cache_hit_ratio_pct ?? 100) < 90 ? "bad" : (data?.cache_hit_ratio_pct ?? 100) < 95 ? "warn" : "good"} hint={t("redisDashboard.cacheHitHint")} />
        <DashboardStatCard label={t("redisDashboard.keys")} value={data?.keys_total != null ? formatNumber(data.keys_total, 0) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartCache")}
          data={cache}
          formatter={(value) => `${formatNumber(value)} /s`}
          lines={[
            { dataKey: "hits", name: t("redisDashboard.hits"), color: DASHBOARD_COLORS.green },
            { dataKey: "misses", name: t("redisDashboard.misses"), color: DASHBOARD_COLORS.red },
          ]}
        />
        <DashboardLinePanel title={t("redisDashboard.chartKeys")} data={keys} formatter={(value) => formatNumber(value, 0)} lines={[{ dataKey: "keys", name: t("redisDashboard.keys"), color: DASHBOARD_COLORS.purple }]} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartEvictExpire")}
          data={evictExpire}
          formatter={(value) => formatNumber(value, 2)}
          lines={[
            { dataKey: "evicted", name: t("redisDashboard.evicted"), color: DASHBOARD_COLORS.red },
            { dataKey: "expired", name: t("redisDashboard.expired"), color: DASHBOARD_COLORS.orange },
          ]}
        />
        <DashboardBarPanel title={t("redisDashboard.chartKeysByDb")} data={(data?.keys_by_db ?? []).map((d) => ({ name: d.label, value: d.count }))} />
      </div>

      {/* ═══════════════════════════════ 6. PERSISTENCE & REPLICATION ═══════ */}
      <DashboardSectionHeader title="Persistence & Replication" description="RDB/AOF state, fork times, and replication health" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.persistenceState")} value={persistenceState} tone={data?.aof_rewrite_in_progress === 1 || data?.rdb_save_in_progress === 1 ? "warn" : "neutral"} hint={data?.rdb_changes_since_last_save != null ? t("redisDashboard.rdbChangesHint", formatNumber(data.rdb_changes_since_last_save, 0)) : undefined} />
        <DashboardStatCard label={t("redisDashboard.latestFork")} value={data?.latest_fork_seconds != null ? `${formatNumber(data.latest_fork_seconds, 3)}s` : "-"} tone={(data?.latest_fork_seconds ?? 0) > 1 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartPersistence")}
          data={persistence}
          formatter={(value, key) => key === "fork_sec" ? `${formatNumber(value, 3)}s` : formatNumber(value, 0)}
          lines={[
            { dataKey: "rdb_changes", name: t("redisDashboard.rdbChanges"), color: DASHBOARD_COLORS.blue },
            { dataKey: "fork_sec", name: t("redisDashboard.latestFork"), color: DASHBOARD_COLORS.orange },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 7. NETWORK & CPU ════════════════════ */}
      <DashboardSectionHeader title="Network & CPU" description="Data transfer rates and CPU utilization" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("redisDashboard.netInput")} value={data?.net_input_bytes_per_sec != null ? `${formatBytes(data.net_input_bytes_per_sec)}/s` : "-"} />
        <DashboardStatCard label={t("redisDashboard.netOutput")} value={data?.net_output_bytes_per_sec != null ? `${formatBytes(data.net_output_bytes_per_sec)}/s` : "-"} />
        <DashboardStatCard label={t("redisDashboard.cpuUser")} value={data?.cpu_user_per_sec != null ? formatNumber(data.cpu_user_per_sec, 4) : "-"} />
        <DashboardStatCard label={t("redisDashboard.cpuSys")} value={data?.cpu_sys_per_sec != null ? formatNumber(data.cpu_sys_per_sec, 4) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel
          title={t("redisDashboard.chartNet")}
          data={net}
          formatter={(value, key) => key === "input" || key === "output" ? `${formatBytes(value)}/s` : `${formatNumber(value)}/s`}
          lines={[
            { dataKey: "input", name: t("redisDashboard.netInputShort"), color: DASHBOARD_COLORS.blue },
            { dataKey: "output", name: t("redisDashboard.netOutputShort"), color: DASHBOARD_COLORS.orange },
          ]}
        />
        <DashboardLinePanel
          title={t("redisDashboard.chartCpu")}
          data={cpu}
          formatter={(value) => formatNumber(value, 3)}
          lines={[
            { dataKey: "user", name: t("redisDashboard.cpuUser"), color: DASHBOARD_COLORS.green },
            { dataKey: "sys", name: t("redisDashboard.cpuSys"), color: DASHBOARD_COLORS.blue },
          ]}
        />
      </div>

      {/* ═══════════════════════════════ 8. LOG SIGNALS ═════════════════════ */}
      <DashboardSectionHeader title="Log Signals" description="Events parsed from Redis logs via Loki" />
      <SectionCard title="">
        <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-7">
          <DashboardStatCard label={t("redisDashboard.warnings")} value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label={t("redisDashboard.errors")} value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label={t("redisDashboard.authFailures")} value={formatNumber(data?.log_summary.auth_failures ?? 0, 0)} tone={(data?.log_summary.auth_failures ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label={t("redisDashboard.security")} value={formatNumber(data?.log_summary.security ?? 0, 0)} tone={(data?.log_summary.security ?? 0) > 0 ? "bad" : "good"} />
          <DashboardStatCard label={t("redisDashboard.shutdowns")} value={formatNumber(data?.log_summary.shutdowns ?? 0, 0)} tone={(data?.log_summary.shutdowns ?? 0) > 0 ? "warn" : "good"} />
          <DashboardStatCard label={t("redisDashboard.startups")} value={formatNumber(data?.log_summary.startups ?? 0, 0)} />
          <DashboardStatCard label={t("redisDashboard.persistence")} value={formatNumber(data?.log_summary.persistence ?? 0, 0)} />
        </div>
      </SectionCard>
    </Page>
  )
}
