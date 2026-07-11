import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useNavigate, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getPHPFPMDashboardSummary, type PHPFPMDashboardSummary, type PHPFPMTimePoint, type ScriptStat, type LabelCount } from "@/api/phpfpm"
import { PHPFPMSummaryTab } from "./components/PHPFPMSummaryTab"
import {
  DashboardStatCard, DashboardSectionHeader, DashboardNoData,
  DashboardLinePanel, DashboardAreaPanel, DashboardBarPanel,
  DashboardDonutCard, DashboardCustomBarPanel,
} from "@/components/shared/DashboardComponents"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  dateTimeInputToSeconds, formatBytes, formatDuration, formatNumber, formatPercent,
  getNowSeconds, getStartSeconds, secondsToNanos,
} from "@/lib/queryHelpers"
import { mapTimeseries, getStep } from "@/lib/dashboardHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation, LangToggle } from "@/i18n/LanguageContext"

const FPM_COLORS = {
  blue: "#0ea5e9", green: "#22c55e", amber: "#f59e0b", red: "#ef4444",
  violet: "#8b5cf6", teal: "#14b8a6", rose: "#f43f5e", cyan: "#06b6d4",
  indigo: "#6366f1", orange: "#f97316", slate: "#64748b", emerald: "#10b981",
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
      return `/services/phpfpm-logs/${id}?start=${Math.floor(startNs)}&end=${Math.floor(endNs)}`
    }
  }
  return `/services/phpfpm-logs/${id}`
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
  const latencyPercentiles = mapTimeseries(data?.latency_percentiles_timeseries ?? [])
  const opcacheHitTS = mapTimeseries(data?.opcache_hit_rate_timeseries ?? [])
  const opcacheMemTS = mapTimeseries(data?.opcache_memory_timeseries ?? [])
  const opcacheScriptTS = mapTimeseries(data?.opcache_scripts_timeseries ?? [])
  const opcacheWastedTS = mapTimeseries(data?.opcache_wasted_timeseries ?? [])
  const slowReqRate = mapTimeseries(data?.slow_req_rate_timeseries ?? [])
  const scrapeFailRate = mapTimeseries(data?.scrape_fail_rate_timeseries ?? [])
  const childrenRate = mapTimeseries(data?.children_rate_timeseries ?? [])
  const runningProcs = mapTimeseries(data?.running_procs_timeseries ?? [])

  const pools = data?.pools ?? []

  const tabs = [
    { key: "overview", label: "Overview" },
    ...pools.map((p) => ({ key: `pool-${p}`, label: p })),
    { key: "summary", label: "Summary" },
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
            <Link to={`/services/phpfpm-logs/${id}`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
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
          <DashboardSectionHeader title="Status" description="Instance health and exporter connectivity" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DashboardStatCard label="PHP-FPM" value={data?.up === 1 ? "Online" : data?.up === 0 ? "Down" : "-"}
              tone={data?.up === 1 ? "good" : data?.up === 0 ? "bad" : "neutral"} hint="phpfpm_up" />
            <DashboardStatCard label="Scrape Health" value={(data?.scrape_failures ?? 0) === 0 ? "Healthy" : `${formatNumber(data!.scrape_failures!, 0)} failed`}
              tone={(data?.scrape_failures ?? 0) > 0 ? "bad" : "good"} hint="phpfpm_scrape_failures_total" />
            <DashboardStatCard label="Uptime" value={data?.uptime_seconds != null ? formatDuration(data.uptime_seconds) : "-"} hint="Since pool start" />
            <DashboardStatCard label="Total Accepted" value={data?.accepted_connections != null ? formatNumber(data.accepted_connections, 0) : "-"}
              hint={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : undefined} />
          </div>

          {/* ── POOL SATURATION RANKING ── */}
          {pools.length > 1 && data?.pool_details && (
            <>
              <DashboardSectionHeader title="Pool Saturation Ranking" description="Quick overview of which pools are stressed" />
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
          <DashboardSectionHeader title="Worker Pool" description="Active/idle/total workers and pool saturation" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <DashboardStatCard label="Active Workers" value={data?.active_processes != null ? formatNumber(data.active_processes, 0) : "-"} hint="Currently handling requests" />
            <DashboardStatCard label="Idle Workers" value={data?.idle_processes != null ? formatNumber(data.idle_processes, 0) : "-"} hint="Waiting for work" />
            <DashboardStatCard label="Total Workers" value={data?.total_processes != null ? formatNumber(data.total_processes, 0) : "-"}
              hint={data?.max_active_processes != null ? `Max active: ${formatNumber(data.max_active_processes, 0)}` : undefined} />
            <DashboardDonutCard title="Saturation" value={data?.worker_saturation_pct ?? 0}
              color={(data?.worker_saturation_pct ?? 0) > 85 ? FPM_COLORS.red : (data?.worker_saturation_pct ?? 0) > 70 ? FPM_COLORS.amber : FPM_COLORS.green}
              label="active / total × 100" hint=">85% = add workers" />
            <DashboardStatCard label="Worker Efficiency" value={data?.accepted_conn_per_sec != null && (data?.active_processes ?? 0) > 0 ? `${formatNumber(data!.accepted_conn_per_sec! / data!.active_processes!, 2)}/s` : "-"}
              hint="Requests/s per active worker" tone={(data?.accepted_conn_per_sec ?? 0) > 0 && (data?.active_processes ?? 0) > 0 && ((data?.accepted_conn_per_sec ?? 0) / (data?.active_processes ?? 1)) < 0.5 ? "warn" : "good"} />
          </div>
          <div className="grid gap-4">
            <DashboardAreaPanel title="Workers Over Time" data={workers}
              areas={[
                { dataKey: "active", name: "Active", color: FPM_COLORS.green },
                { dataKey: "idle", name: "Idle", color: FPM_COLORS.blue },
                { dataKey: "total", name: "Total", color: FPM_COLORS.slate },
              ]} stacked={false} />
          </div>

          {/* ── CAPACITY PLANNING ── */}
              {data?.total_processes != null && workers.length > 0 && (
            <div className="mt-2">
              <DashboardSectionHeader title="Capacity Planning" description="Worker usage and headroom" />
              <div className="grid gap-3 md:grid-cols-3">
                {(() => {
                  const d = data!
                  const peak = workers.reduce((max, w) => Math.max(max, typeof w.total === "number" ? w.total : 0), 0)
                  const current = d.total_processes!
                  const limit = d.max_active_processes ?? 0
                  const headroomPct = limit > 0 ? ((limit - peak) / limit) * 100 : 0
                  return (
                    <>
                      <DashboardStatCard label="Current Workers" value={formatNumber(current, 0)}
                        tone={current > 0 ? "good" : "neutral"} />
                      <DashboardStatCard label="Peak Workers (window)" value={formatNumber(peak, 0)}
                        hint={limit > 0 ? `Limit: ${formatNumber(limit, 0)}` : undefined}
                        tone={limit > 0 && peak / limit > 0.85 ? "bad" : limit > 0 && peak / limit > 0.7 ? "warn" : "good"} />
                      <DashboardStatCard label="Headroom" value={limit > 0 ? formatPercent(headroomPct) : "—"}
                        tone={headroomPct < 15 ? "bad" : headroomPct < 30 ? "warn" : headroomPct > 0 ? "good" : "neutral"}
                        hint={limit > 0 ? `${formatNumber(limit - peak, 0)} available workers` : undefined} />
                    </>
                  )
                })()}
              </div>
            </div>
          )}

          {/* ── QUEUE & THROUGHPUT ── */}
          <DashboardSectionHeader title="Queue & Throughput" description="Listen queue depth and connection rate" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DashboardStatCard label="Queue Depth" value={data?.listen_queue != null ? formatNumber(data.listen_queue, 0) : "-"}
              tone={(data?.listen_queue ?? 0) > 0 ? "bad" : "good"} hint="Requests waiting for a free worker" />
            <DashboardStatCard label="Max Queue" value={data?.listen_queue_len != null ? formatNumber(data.listen_queue_len, 0) : "-"} hint="Configured listen.queue_max" />
            <DashboardDonutCard title="Queue Utilization" value={data?.queue_utilization_pct ?? 0}
              color={(data?.queue_utilization_pct ?? 0) > 50 ? FPM_COLORS.red : (data?.queue_utilization_pct ?? 0) > 0 ? FPM_COLORS.amber : FPM_COLORS.green}
              label="queue / max × 100" hint="Sustained queue = workers saturated" />
            <DashboardStatCard label="Throughput" value={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : "-"} hint="Accepted connections per second" />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardAreaPanel title="Queue Depth Over Time" data={queue}
              areas={[{ dataKey: "queue", name: "Queue Depth", color: FPM_COLORS.amber }, { dataKey: "max", name: "Max Queue", color: FPM_COLORS.red }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} stacked={false} />
            <DashboardAreaPanel title="Throughput Over Time" data={throughput} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[{ dataKey: "accepted", name: "Accepted/s", color: FPM_COLORS.green }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} stacked={false} />
          </div>
          <div className="mt-4">
            <DashboardLinePanel title="Queue Correlation" data={(() => {
              const merged: Record<string, string | number>[] = []
              for (let i = 0; i < Math.max(queue.length, workers.length, throughput.length); i++) {
                const row: Record<string, string | number> = {}
                if (queue[i]) Object.assign(row, queue[i])
                if (workers[i]) Object.assign(row, workers[i])
                if (throughput[i]) Object.assign(row, throughput[i])
                if (row.time) merged.push(row)
              }
              return merged
            })()}
              lines={[
                { dataKey: "queue", name: "Queue Depth", color: FPM_COLORS.amber },
                { dataKey: "active", name: "Active Workers", color: FPM_COLORS.green },
                { dataKey: "accepted", name: "Accepted/s", color: FPM_COLORS.blue },
              ]}
              formatter={(v, key) => key === "accepted" ? `${formatNumber(v, 2)}/s` : formatNumber(v, 0)} />
          </div>

          {/* ── REQUEST PERFORMANCE ── */}
          <DashboardSectionHeader title="Request Performance" description="Duration, memory, CPU, and latency percentiles from metrics + access logs" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
            <DashboardStatCard label="Avg Duration" value={data?.avg_duration_ms != null ? `${formatNumber(data.avg_duration_ms, 1)}ms` : "-"}
              tone={(data?.avg_duration_ms ?? 0) > 1000 ? "bad" : (data?.avg_duration_ms ?? 0) > 500 ? "warn" : "good"} />
            <DashboardStatCard label="Avg Memory" value={data?.avg_memory_kb != null ? `${formatNumber(data.avg_memory_kb, 0)} KB` : data?.avg_memory_bytes != null ? formatBytes(data.avg_memory_bytes) : "-"} />
            <DashboardStatCard label="Avg CPU" value={data?.avg_cpu_pct != null ? `${formatNumber(data.avg_cpu_pct, 2)}%` : "-"}
              tone={(data?.avg_cpu_pct ?? 0) > 25 ? "bad" : (data?.avg_cpu_pct ?? 0) > 5 ? "warn" : "good"} />
            <DashboardStatCard label="Slow Requests" value={data?.slow_requests != null ? formatNumber(data.slow_requests, 0) : "-"}
              tone={(data?.slow_requests ?? 0) > 0 ? "bad" : "good"} hint="Cumulative slowlog violations" />
            <DashboardStatCard label="Max Children Reached" value={data?.max_children_reached != null ? formatNumber(data.max_children_reached, 0) : "-"}
              tone={(data?.max_children_reached ?? 0) > 0 ? "bad" : "good"} hint="Pool capacity exhausted" />
          </div>
          <div className="grid gap-4 xl:grid-cols-3">
            <DashboardAreaPanel title="Duration Over Time" data={duration} formatter={(v) => `${formatNumber(v, 1)}ms`}
              areas={[{ dataKey: "duration", name: "Duration (ms)", color: FPM_COLORS.blue }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} stacked={false} />
            <DashboardAreaPanel title="Memory Per Process" data={memory} formatter={(v) => formatBytes(v)}
              areas={[{ dataKey: "memory", name: "Memory (bytes)", color: FPM_COLORS.violet }]} stacked={false} />
            <DashboardAreaPanel title="CPU Per Process" data={cpu} formatter={(v) => `${formatNumber(v, 2)}%`}
              areas={[{ dataKey: "cpu", name: "CPU (%)", color: FPM_COLORS.amber }]} stacked={false} />
          </div>

          {/* ── ERROR SIGNALS OVER TIME ── */}
          <DashboardSectionHeader title="Error Signals Over Time" description="Slow requests, pool exhaustion, scrape health, and running workers" />
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardAreaPanel title="Slow Requests/s" data={slowReqRate} formatter={(v) => `${formatNumber(v, 3)}/s`}
              areas={[{ dataKey: "slow_req_rate", name: "Slow Requests/s", color: FPM_COLORS.red }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} stacked={false} />
            <DashboardAreaPanel title="Max Children Reached/s" data={childrenRate} formatter={(v) => `${formatNumber(v, 3)}/s`}
              areas={[{ dataKey: "children_rate", name: "Children Reached/s", color: FPM_COLORS.rose }]} stacked={false} />
            <DashboardAreaPanel title="Scrape Failures/s" data={scrapeFailRate} formatter={(v) => `${formatNumber(v, 3)}/s`}
              areas={[{ dataKey: "scrape_fail_rate", name: "Scrape Failures/s", color: FPM_COLORS.orange }]} stacked={false} />
            <DashboardAreaPanel title="Running Processes" data={runningProcs} formatter={(v) => formatNumber(v, 0)}
              areas={[{ dataKey: "running", name: "Running", color: FPM_COLORS.teal }]} stacked={false} />
          </div>

          {data?.latency_p50 != null ? (
            <>
              <DashboardSectionHeader title="Latency Percentiles" description="From access log duration field (p50/p95/p99)" />
              <div className="grid gap-3 md:grid-cols-3">
                <DashboardStatCard label="p50 Latency" value={`${formatNumber(data.latency_p50, 1)}ms`} hint="Median" />
                <DashboardStatCard label="p95 Latency" value={`${formatNumber(data.latency_p95 ?? 0, 1)}ms`}
                  tone={(data.latency_p95 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 20 slower" />
                <DashboardStatCard label="p99 Latency" value={`${formatNumber(data.latency_p99 ?? 0, 1)}ms`}
                  tone={(data.latency_p99 ?? 0) > 2000 ? "bad" : (data.latency_p99 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 100 slower" />
              </div>
              <div className="mt-4">
                <DashboardLinePanel title="Latency Percentiles Over Time" data={latencyPercentiles}
                  formatter={(v) => `${formatNumber(v, 1)}ms`}
                  lines={[
                    { dataKey: "p50", name: "p50", color: FPM_COLORS.green },
                    { dataKey: "p95", name: "p95", color: FPM_COLORS.amber },
                    { dataKey: "p99", name: "p99", color: FPM_COLORS.red },
                  ]}
                  onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} />
              </div>
            </>
          ) : null}

          {/* ── TRAFFIC & STATUS CODES ── */}
          <DashboardSectionHeader title="Request Traffic" description="Request rate and status code breakdown from access logs" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            <DashboardStatCard label="2xx Rate" value={data?.status_2xx != null ? `${formatNumber(data.status_2xx, 2)}/s` : "-"} tone="good" />
            <DashboardStatCard label="4xx Rate" value={data?.status_4xx != null ? `${formatNumber(data.status_4xx, 2)}/s` : "-"} tone={(data?.status_4xx ?? 0) > 1 ? "warn" : "neutral"} />
            <DashboardStatCard label="5xx Rate" value={data?.status_5xx != null ? `${formatNumber(data.status_5xx, 2)}/s` : "-"} tone={(data?.status_5xx ?? 0) > 0 ? "bad" : "neutral"} />
            <DashboardStatCard label="Request Rate" value={data?.accepted_conn_per_sec != null ? `${formatNumber(data.accepted_conn_per_sec, 2)}/s` : "-"} hint="From VM metrics" />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardAreaPanel title="Status Codes Over Time" data={statusTS} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[
                { dataKey: "2xx", name: "2xx", color: FPM_COLORS.green },
                { dataKey: "4xx", name: "4xx", color: FPM_COLORS.amber },
                { dataKey: "5xx", name: "5xx", color: FPM_COLORS.red },
              ]} stacked={false} />
            <DashboardAreaPanel title="Request Rate Over Time" data={reqRateTS} formatter={(v) => `${formatNumber(v, 2)}/s`}
              areas={[{ dataKey: "req", name: "Requests/s", color: FPM_COLORS.blue }]}
              onClickTime={(timeLabel) => navigate(chartClickToLogs(id, String(timeLabel)))} stacked={false} />
          </div>

          <DashboardSectionHeader title="Top URIs & Failing Endpoints" description="Most requested URIs and top error rates" />
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardBarPanel title="By Request Count" data={(data?.top_uris ?? []).map((u: LabelCount) => ({ name: u.label, value: u.count }))}
              formatter={(v) => formatNumber(v, 0)} />
            <div className="rounded-lg border border-border bg-card p-4">
              <h3 className="mb-4 text-sm font-semibold">Top Failing Endpoints</h3>
              {data?.top_uris && data?.top_error_uris ? (() => {
                const d = data!
                const errorMap = new Map(d.top_error_uris!.map((e: LabelCount) => [e.label, e.count]))
                const merged = d.top_uris!
                  .filter((u: LabelCount) => errorMap.has(u.label))
                  .map((u: LabelCount) => ({
                    uri: u.label,
                    total: u.count,
                    errors: errorMap.get(u.label) ?? 0,
                    errorPct: u.count > 0 ? (errorMap.get(u.label) ?? 0) / u.count * 100 : 0,
                  }))
                  .sort((a: { errorPct: number }, b: { errorPct: number }) => b.errorPct - a.errorPct)
                  .slice(0, 10)
                return merged.length ? (
                  <div className="overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="text-left text-muted-foreground border-b border-border">
                          <th className="pb-2 pr-3 font-medium">URI</th>
                          <th className="pb-2 pr-3 font-medium text-right">Error %</th>
                          <th className="pb-2 font-medium text-right">Total Req</th>
                        </tr>
                      </thead>
                      <tbody>
                        {merged.map((r: { uri: string; total: number; errors: number; errorPct: number }) => (
                          <tr key={r.uri} className="border-b border-border/50">
                            <td className="py-2 pr-3 font-mono text-xs truncate max-w-[180px]" title={r.uri}>{r.uri}</td>
                            <td className={cn("py-2 pr-3 text-right tabular-nums", r.errorPct > 10 ? "text-red-500" : r.errorPct > 5 ? "text-amber-500" : "")}>
                              {formatPercent(r.errorPct)}
                            </td>
                            <td className="py-2 text-right tabular-nums">{formatNumber(r.total, 0)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                ) : <DashboardNoData />
              })() : <DashboardNoData />}
            </div>
          </div>

          {/* ── ERRORS & SIGNALS ── */}
          <DashboardSectionHeader title="Errors & Signals" description="Warning/error events and slow request traces from logs" />
          <div className="grid gap-3 md:grid-cols-3 xl:grid-cols-5">
            <DashboardStatCard label="Warnings" value={formatNumber(data?.log_summary.warnings ?? 0, 0)} tone={(data?.log_summary.warnings ?? 0) > 0 ? "warn" : "good"} />
            <DashboardStatCard label="Errors" value={formatNumber(data?.log_summary.errors ?? 0, 0)} tone={(data?.log_summary.errors ?? 0) > 0 ? "bad" : "good"} />
            <DashboardStatCard label="Slow Request Traces" value={formatNumber(data?.log_summary.slow_requests ?? 0, 0)} tone={(data?.log_summary.slow_requests ?? 0) > 0 ? "warn" : "good"} />
            <DashboardStatCard label="Notices" value={formatNumber(data?.log_summary.notices ?? 0, 0)} />
            <DashboardStatCard label="Startups" value={formatNumber(data?.log_summary.startups ?? 0, 0)} />
          </div>

          {/* ── TOP SCRIPTS ── */}
          <DashboardSectionHeader title="Top Scripts" description="Worst-performing scripts from access logs" />
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardCustomBarPanel title="Slowest Scripts (avg ms)" data={(data?.top_slow_scripts ?? []).map((s: ScriptStat) => ({ name: s.script, value: Math.round(s.avg_duration_ms) }))} formatter={(v) => `${v}ms`} />
            <DashboardCustomBarPanel title="Highest Memory Scripts (avg KB)" data={(data?.top_mem_scripts ?? []).map((s: ScriptStat) => ({ name: s.script, value: Math.round(s.avg_memory_kb) }))} formatter={(v) => `${formatNumber(v, 0)} KB`} />
          </div>

          {/* ── LOG ANALYTICS ── */}
          <DashboardSectionHeader title="Log Analytics" description="Request method distribution, duration buckets, and error breakdown from access + error logs" />
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardCustomBarPanel title="Method Distribution" data={(data?.method_distribution ?? []).map((m: LabelCount) => ({ name: m.label, value: m.count }))} />
            <DashboardCustomBarPanel title="Status Distribution" data={(data?.status_distribution ?? []).map((s: LabelCount) => ({ name: s.label, value: s.count }))} />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardCustomBarPanel title="Duration Histogram" data={(data?.duration_histogram ?? []).map((b: LabelCount) => ({ name: b.label, value: b.count }))} />
            <DashboardCustomBarPanel title="Memory Histogram" data={(data?.memory_histogram ?? []).map((b: LabelCount) => ({ name: b.label, value: b.count }))} formatter={(v) => formatNumber(v, 0)} />
          </div>
          <div className="grid gap-4 xl:grid-cols-2">
            <DashboardCustomBarPanel title="CPU Histogram" data={(data?.cpu_histogram ?? []).map((b: LabelCount) => ({ name: b.label, value: b.count }))} formatter={(v) => formatNumber(v, 0)} />
            <DashboardCustomBarPanel title="Error Type Breakdown" data={(data?.error_type_breakdown ?? []).map((e: LabelCount) => ({ name: e.label, value: e.count }))} />
          </div>

          {/* ── OPCACHE ── */}
          <DashboardSectionHeader title="OPcache" description="Cache performance, memory usage, and script statistics" />
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
            <DashboardDonutCard title="Hit Rate" value={(data?.opcache_hit_rate ?? 0) * 100}
              color={(data?.opcache_hit_rate ?? 0) >= 0.9 ? FPM_COLORS.green : (data?.opcache_hit_rate ?? 0) >= 0.8 ? FPM_COLORS.amber : FPM_COLORS.red}
              label={`${formatNumber((data?.opcache_hit_rate ?? 0) * 100, 1)}%`} hint={data?.opcache_hit_rate != null && data?.opcache_hit_rate < 0.9 ? "Target ≥ 90%" : undefined} />
            <DashboardDonutCard title="Memory Used" value={(data?.opcache_memory_used_ratio ?? 0) * 100}
              color={(data?.opcache_memory_used_ratio ?? 0) > 0.8 ? FPM_COLORS.red : (data?.opcache_memory_used_ratio ?? 0) > 0.6 ? FPM_COLORS.amber : FPM_COLORS.green}
              label={data?.opcache_memory_total_bytes ? formatBytes(data.opcache_memory_total_bytes) : ""}
              hint={data?.opcache_memory_free_bytes ? `${formatBytes(data.opcache_memory_free_bytes)} free` : undefined} />
            <DashboardDonutCard title="Wasted Memory" value={(data?.opcache_wasted_ratio ?? 0) * 100}
              color={(data?.opcache_wasted_ratio ?? 0) > 0.05 ? FPM_COLORS.red : (data?.opcache_wasted_ratio ?? 0) > 0.02 ? FPM_COLORS.amber : FPM_COLORS.green}
              label={`${formatNumber((data?.opcache_wasted_ratio ?? 0) * 100, 2)}%`}
              hint=">5% may trigger OOM restarts" />
            <DashboardStatCard label="Cached Scripts" value={data?.opcache_cached_scripts != null ? formatNumber(data.opcache_cached_scripts, 0) : "-"} />
            <DashboardStatCard label="Cache Hits" value={data?.opcache_hits_total != null ? formatNumber(data.opcache_hits_total, 0) : "-"}
              hint={data?.opcache_misses_total != null ? `${formatNumber(data.opcache_misses_total, 0)} misses` : undefined} />
            <DashboardStatCard label="Restarts" value={data?.opcache_restarts_total != null ? formatNumber(data.opcache_restarts_total, 0) : "-"}
              tone={(data?.opcache_restarts_total ?? 0) > 0 ? "warn" : "good"} hint="OOM restarts = cache thrashing" />
          </div>
          <DashboardSectionHeader title="OPcache Timeseries" description="Hit rate, memory usage, cached scripts, and fragmentation over time" />
          <div className="grid gap-4 xl:grid-cols-4">
            <DashboardLinePanel title="Hit Rate Over Time" data={opcacheHitTS}
              formatter={(v) => `${formatNumber(v * 100, 1)}%`}
              lines={[{ dataKey: "hit_rate", name: "Hit Rate", color: FPM_COLORS.green }]} />
            <DashboardLinePanel title="Memory Used Ratio" data={opcacheMemTS}
              formatter={(v) => `${formatNumber(v * 100, 1)}%`}
              lines={[{ dataKey: "used_ratio", name: "Used Ratio", color: FPM_COLORS.blue }]} />
            <DashboardLinePanel title="Fragmentation Trend" data={opcacheWastedTS}
              formatter={(v) => `${formatNumber(v * 100, 2)}%`}
              lines={[{ dataKey: "wasted", name: "Wasted", color: FPM_COLORS.red }]} />
            <DashboardAreaPanel title="Cached Scripts" data={opcacheScriptTS}
              formatter={(v) => formatNumber(v, 0)}
              areas={[{ dataKey: "cached", name: "Cached", color: FPM_COLORS.violet }]} stacked={false} />
          </div>
        </>
      )}

      {/* ════════════════════════════════════════════════════════════════ */}
      {/* TAB: SUMMARY                                                */}
      {/* ════════════════════════════════════════════════════════════════ */}
      {activeTab === "summary" && data && (
        <div className="mt-4">
          <PHPFPMSummaryTab data={data} />
        </div>
      )}

      {/* ════════════════════════════════════════════════════════════════ */}
      {/* PER-POOL TABS                                                */}
      {/* ════════════════════════════════════════════════════════════════ */}
      {pools.map((pool) => {
        if (activeTab !== `pool-${pool}`) return null
        const pd = data?.pool_details?.[pool]
        return (
          <div key={pool}>
            <DashboardSectionHeader title={`Pool: ${pool}`} description="Pool-specific metrics and log analytics" />
            {pd ? (
              <>
                {/* ── WORKER POOL ── */}
                <DashboardSectionHeader title="Worker Pool" description="Active, idle, and total process counts" />
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <DashboardStatCard label="Active Workers" value={formatNumber(pd.active_processes ?? 0, 0)} hint="Handling requests now" />
                  <DashboardStatCard label="Idle Workers" value={formatNumber(pd.idle_processes ?? 0, 0)} hint="Waiting for work" />
                  <DashboardStatCard label="Total Workers" value={formatNumber(pd.total_processes ?? 0, 0)} />
                  <DashboardStatCard label="Saturation" value={pd.saturation_pct != null ? `${formatNumber(pd.saturation_pct, 1)}%` : "-"}
                    hint="Active / Total × 100"
                    tone={(pd.saturation_pct ?? 0) > 85 ? "bad" : (pd.saturation_pct ?? 0) > 70 ? "warn" : "good"} />
                </div>
                {(pd.workers_timeseries ?? []).length > 0 && (
                  <div className="mt-4">
                    <DashboardLinePanel title="Workers Over Time" data={mapTimeseries(pd.workers_timeseries!)}
                      formatter={(v) => formatNumber(v, 0)}
                      lines={[
                        { dataKey: "active", name: "Active", color: FPM_COLORS.green },
                        { dataKey: "idle", name: "Idle", color: FPM_COLORS.amber },
                        { dataKey: "total", name: "Total", color: FPM_COLORS.blue },
                      ]} />
                  </div>
                )}

                {/* ── PERFORMANCE AVERAGES ── */}
                {(pd.avg_duration_ms != null || pd.avg_memory_kb != null || pd.avg_cpu_pct != null) && (
                  <>
                    <DashboardSectionHeader title="Performance Averages" description="Mean request duration, memory, and CPU from access logs" />
                    <div className="grid gap-3 md:grid-cols-3">
                      <DashboardStatCard label="Avg Duration" value={pd.avg_duration_ms != null ? `${formatNumber(pd.avg_duration_ms, 1)}ms` : "-"} />
                      <DashboardStatCard label="Avg Memory" value={pd.avg_memory_kb != null ? `${formatNumber(pd.avg_memory_kb, 0)} KB` : "-"} />
                      <DashboardStatCard label="Avg CPU" value={pd.avg_cpu_pct != null ? `${formatNumber(pd.avg_cpu_pct, 2)}%` : "-"} />
                    </div>
                    <div className="grid gap-4 mt-4 xl:grid-cols-2">
                      {(pd.duration_histogram ?? []).length > 0 && (
                        <DashboardCustomBarPanel title="Duration Histogram" data={pd.duration_histogram!.map((b: LabelCount) => ({ name: b.label, value: b.count }))} formatter={(v) => formatNumber(v, 0)} />
                      )}
                      {(pd.memory_histogram ?? []).length > 0 && (
                        <DashboardCustomBarPanel title="Memory Histogram" data={pd.memory_histogram!.map((b: LabelCount) => ({ name: b.label, value: b.count }))} formatter={(v) => formatNumber(v, 0)} />
                      )}
                    </div>
                    {(pd.memory_timeseries ?? []).length > 0 || (pd.cpu_timeseries ?? []).length > 0 ? (
                      <div className="grid gap-4 mt-4 xl:grid-cols-2">
                        {(pd.memory_timeseries ?? []).length > 0 && (
                          <DashboardLinePanel title="Memory Over Time" data={mapTimeseries(pd.memory_timeseries!)}
                            formatter={(v) => formatBytes(v)}
                            lines={[{ dataKey: "memory", name: "Memory", color: FPM_COLORS.violet }]} />
                        )}
                        {(pd.cpu_timeseries ?? []).length > 0 && (
                          <DashboardLinePanel title="CPU Over Time" data={mapTimeseries(pd.cpu_timeseries!)}
                            formatter={(v) => `${formatNumber(v, 2)}%`}
                            lines={[{ dataKey: "cpu", name: "CPU", color: FPM_COLORS.orange }]} />
                        )}
                      </div>
                    ) : null}
                  </>
                )}

                {/* ── REQUEST STATS ── */}
                <DashboardSectionHeader title="Request Stats" description="Request count, throughput, and issues" />
                <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                  <DashboardStatCard label="Request Count" value={pd.request_count != null ? formatNumber(pd.request_count, 0) : "-"} />
                  <DashboardStatCard label="Throughput" value={pd.accepted_conn_per_sec != null ? `${formatNumber(pd.accepted_conn_per_sec, 2)}/s` : "-"} hint="Accepted connections/s" />
                  <DashboardStatCard label="Slow Requests" value={pd.slow_requests != null ? formatNumber(pd.slow_requests, 0) : "-"}
                    tone={(pd.slow_requests ?? 0) > 0 ? "warn" : "good"} />
                  <DashboardStatCard label="Max Children" value={pd.max_children_reached != null ? formatNumber(pd.max_children_reached, 0) : "-"}
                    tone={(pd.max_children_reached ?? 0) > 0 ? "bad" : "good"} hint="Capacity exhausted" />
                </div>
                {(pd.req_rate_timeseries ?? []).length > 0 && (
                  <div className="mt-4">
                    <DashboardLinePanel title="Request Rate" data={mapTimeseries(pd.req_rate_timeseries!)}
                      formatter={(v) => formatNumber(v, 0)}
                      lines={[{ dataKey: "req", name: "Requests", color: FPM_COLORS.blue }]} />
                  </div>
                )}

                {/* ── STATUS CODES ── */}
                {(pd.status_2xx != null || pd.status_4xx != null || pd.status_5xx != null) && (
                  <>
                    <DashboardSectionHeader title="Status Codes" description="HTTP response status breakdown" />
                    <div className="grid gap-3 md:grid-cols-3">
                      <DashboardStatCard label="2xx" value={pd.status_2xx != null ? formatNumber(pd.status_2xx, 0) : "-"} tone="good" />
                      <DashboardStatCard label="4xx" value={pd.status_4xx != null ? formatNumber(pd.status_4xx, 0) : "-"} tone={(pd.status_4xx ?? 0) > 0 ? "warn" : "neutral"} />
                      <DashboardStatCard label="5xx" value={pd.status_5xx != null ? formatNumber(pd.status_5xx, 0) : "-"} tone={(pd.status_5xx ?? 0) > 0 ? "bad" : "neutral"} />
                    </div>
                  </>
                )}

                {/* ── LATENCY PERCENTILES ── */}
                {pd.latency_p50 != null && (
                  <>
                    <DashboardSectionHeader title="Latency Percentiles" description="From access log duration" />
                    <div className="grid gap-3 md:grid-cols-3">
                      <DashboardStatCard label="p50" value={`${formatNumber(pd.latency_p50, 1)}ms`} hint="Median" />
                      <DashboardStatCard label="p95" value={`${formatNumber(pd.latency_p95 ?? 0, 1)}ms`}
                        tone={(pd.latency_p95 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 20 slower" />
                      <DashboardStatCard label="p99" value={`${formatNumber(pd.latency_p99 ?? 0, 1)}ms`}
                        tone={(pd.latency_p99 ?? 0) > 2000 ? "bad" : (pd.latency_p99 ?? 0) > 1000 ? "warn" : "good"} hint="1 in 100 slower" />
                    </div>
                    {(pd.latency_percentiles_timeseries ?? []).length > 0 && (
                      <div className="mt-4">
                        <DashboardLinePanel title="Latency Over Time" data={mapTimeseries(pd.latency_percentiles_timeseries!)}
                          formatter={(v) => `${formatNumber(v, 1)}ms`}
                          lines={[
                            { dataKey: "p50", name: "p50", color: FPM_COLORS.green },
                            { dataKey: "p95", name: "p95", color: FPM_COLORS.amber },
                            { dataKey: "p99", name: "p99", color: FPM_COLORS.red },
                          ]} />
                      </div>
                    )}
                  </>
                )}

                {/* ── SCRIPTS, URIs & METHODS ── */}
                <div className="grid gap-4 mt-4 xl:grid-cols-2">
                  {(pd.top_slow_scripts ?? []).length > 0 && (
                    <DashboardCustomBarPanel title="Slowest Scripts (avg ms)" data={pd.top_slow_scripts!.map((s: ScriptStat) => ({ name: s.script, value: Math.round(s.avg_duration_ms) }))} formatter={(v) => `${v}ms`} />
                  )}
                  {(pd.top_memory_scripts ?? []).length > 0 && (
                    <DashboardCustomBarPanel title="Top Scripts by Memory (avg KB)" data={pd.top_memory_scripts!.map((s: ScriptStat) => ({ name: s.script, value: Math.round(s.avg_memory_kb) }))} formatter={(v) => `${v} KB`} />
                  )}
                  {(pd.top_uris ?? []).length > 0 && (
                    <DashboardCustomBarPanel title="Top URIs by Request Count" data={pd.top_uris!.map((u: LabelCount) => ({ name: u.label, value: u.count }))} formatter={(v) => formatNumber(v, 0)} />
                  )}
                  {(pd.method_distribution ?? []).length > 0 && (
                    <DashboardCustomBarPanel title="Method Distribution" data={pd.method_distribution!.map((m: LabelCount) => ({ name: m.label, value: m.count }))} formatter={(v) => formatNumber(v, 0)} />
                  )}
                  {(pd.top_error_uris ?? []).length > 0 && (
                    <DashboardCustomBarPanel title="Top Error URIs (4xx/5xx)" data={pd.top_error_uris!.map((u: LabelCount) => ({ name: u.label, value: u.count }))} formatter={(v) => formatNumber(v, 0)} />
                  )}
                </div>

                {/* ── ERROR SUMMARY ── */}
                {(pd.error_count != null || pd.warning_count != null || pd.notice_count != null) && (
                  <>
                    <DashboardSectionHeader title="Error Summary" description="Errors, warnings, and notices from error logs" />
                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                      <DashboardStatCard label="Errors" value={formatNumber(pd.error_count ?? 0, 0)} tone={(pd.error_count ?? 0) > 0 ? "bad" : "good"} />
                      <DashboardStatCard label="Warnings" value={formatNumber(pd.warning_count ?? 0, 0)} tone={(pd.warning_count ?? 0) > 0 ? "warn" : "good"} />
                      <DashboardStatCard label="Notices" value={formatNumber(pd.notice_count ?? 0, 0)} />
                      <DashboardStatCard label="Slow Traces" value={formatNumber(pd.slow_request_count ?? 0, 0)} tone={(pd.slow_request_count ?? 0) > 0 ? "warn" : "good"} />
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
