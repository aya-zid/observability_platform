import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getContainerService, getContainerDashboardSummary, type ContainerDashboardSummary } from "@/api/container"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  DashboardStatCard,
  DashboardSectionHeader,
  DashboardLinePanel,
  DASHBOARD_COLORS,
} from "@/components/shared/DashboardComponents"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import { getStep, mapTimeseries } from "@/lib/dashboardHelpers"
import {
  dateTimeInputToSeconds,
  formatBytes,
  formatNumber,
  formatPercent,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"

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

export function ContainerDashboardPage() {
  const { id = "" } = useParams<{ id?: string }>()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })

  const serviceQuery = useQuery({
    queryKey: ["discovery-container", id],
    queryFn: () => getContainerService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const dashQuery = useQuery<ContainerDashboardSummary>({
    queryKey: ["container-dashboard-summary", id, timeSelection, timeWindow.start, timeWindow.end],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getContainerDashboardSummary(id, {
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

  const cpu = mapTimeseries(data?.cpu_timeseries ?? [])
  const mem = mapTimeseries(data?.mem_timeseries ?? [])
  const memPercent = mapTimeseries(data?.mem_percent_timeseries ?? [])
  const memLimit = mapTimeseries(data?.mem_limit_timeseries ?? [])
  const net = mapTimeseries(data?.net_timeseries ?? [])
  const blockIO = mapTimeseries(data?.block_timeseries ?? [])
  const pids = mapTimeseries(data?.pids_timeseries ?? [])

  if (!id) return <div className="p-6 text-sm text-red-600">Missing container ID.</div>
  if (serviceQuery.isLoading) return <div className="p-6 text-sm text-muted-foreground">Loading...</div>
  if (!service) return <div className="p-6 text-sm text-red-600">Container not found for ID {id}.</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={`Container: ${service.container_name}`}
        description={service.hostname ? `${service.hostname} • ${service.container_image || service.service_name}` : service.container_image || service.service_name}
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
            <Link to={`/containers/${id}/logs`} className="rounded-md bg-sky-600 px-3 py-2 text-sm font-semibold text-white hover:bg-sky-500">
              View Logs
            </Link>
          </div>
        }
      />

      {dashQuery.isError ? (
        <SectionCard title="Dashboard Unavailable">
          <div className="text-sm text-red-600">Failed to fetch container dashboard data.</div>
        </SectionCard>
      ) : null}
      {timeWindow.error ? (
        <SectionCard title="Invalid Time">
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      {/* CPU */}
      <DashboardSectionHeader title="CPU" description="Container CPU usage percent" />
      <div className="grid gap-3 md:grid-cols-1">
        <DashboardStatCard label="CPU Usage" value={data?.cpu_percent != null ? formatPercent(data.cpu_percent / 100) : "-"} tone={(data?.cpu_percent ?? 0) > 80 ? "bad" : (data?.cpu_percent ?? 0) > 50 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-1">
        <DashboardLinePanel title="CPU % Over Time" data={cpu} formatter={(value) => formatNumber(value, 1)} lines={[{ dataKey: "cpu_percent", name: "CPU %", color: DASHBOARD_COLORS.blue }]} />
      </div>

      {/* Memory */}
      <DashboardSectionHeader title="Memory" description="Memory usage, limit, and percentage" />
      <div className="grid gap-3 md:grid-cols-3">
        <DashboardStatCard label="Memory Usage" value={data?.mem_usage_bytes != null ? formatBytes(data.mem_usage_bytes) : "-"} />
        <DashboardStatCard label="Memory Limit" value={data?.mem_limit_bytes != null ? formatBytes(data.mem_limit_bytes) : "-"} />
        <DashboardStatCard label="Memory %" value={data?.mem_percent != null ? formatPercent(data.mem_percent / 100) : "-"} tone={(data?.mem_percent ?? 0) > 80 ? "bad" : (data?.mem_percent ?? 0) > 50 ? "warn" : "good"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-3">
        <DashboardLinePanel title="Memory Usage" data={mem} formatter={(value) => formatBytes(value)} lines={[{ dataKey: "used", name: "Used", color: DASHBOARD_COLORS.blue }]} />
        <DashboardLinePanel title="Memory Limit" data={memLimit} formatter={(value) => formatBytes(value)} lines={[{ dataKey: "limit", name: "Limit", color: DASHBOARD_COLORS.orange }]} />
        <DashboardLinePanel title="Memory %" data={memPercent} formatter={(value) => formatPercent(value / 100)} lines={[{ dataKey: "percent", name: "%", color: DASHBOARD_COLORS.purple }]} />
      </div>

      {/* Network */}
      <DashboardSectionHeader title="Network" description="Network I/O rates" />
      <div className="grid gap-3 md:grid-cols-2">
        <DashboardStatCard label="RX Total" value={data?.net_rx_bytes_total != null ? formatBytes(data.net_rx_bytes_total) : "-"} />
        <DashboardStatCard label="TX Total" value={data?.net_tx_bytes_total != null ? formatBytes(data.net_tx_bytes_total) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-1">
        <DashboardLinePanel title="Network I/O (rate)" data={net} formatter={(value) => `${formatBytes(value)}/s`} lines={[
          { dataKey: "rx", name: "RX", color: DASHBOARD_COLORS.green },
          { dataKey: "tx", name: "TX", color: DASHBOARD_COLORS.blue },
        ]} />
      </div>

      {/* Block I/O */}
      <DashboardSectionHeader title="Block I/O" description="Block device read/write rates" />
      <div className="grid gap-3 md:grid-cols-2">
        <DashboardStatCard label="Block Read Total" value={data?.block_read_bytes_total != null ? formatBytes(data.block_read_bytes_total) : "-"} />
        <DashboardStatCard label="Block Write Total" value={data?.block_write_bytes_total != null ? formatBytes(data.block_write_bytes_total) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-1">
        <DashboardLinePanel title="Block I/O (rate)" data={blockIO} formatter={(value) => `${formatBytes(value)}/s`} lines={[
          { dataKey: "read", name: "Read", color: DASHBOARD_COLORS.blue },
          { dataKey: "write", name: "Write", color: DASHBOARD_COLORS.orange },
        ]} />
      </div>

      {/* Processes */}
      <DashboardSectionHeader title="Processes" description="Number of processes inside the container" />
      <div className="grid gap-3 md:grid-cols-1">
        <DashboardStatCard label="PIDs" value={data?.pids != null ? formatNumber(data.pids, 0) : "-"} />
      </div>
      <div className="grid gap-4 xl:grid-cols-1">
        <DashboardLinePanel title="Process Count Over Time" data={pids} formatter={(value) => formatNumber(value, 0)} lines={[{ dataKey: "pids", name: "PIDs", color: DASHBOARD_COLORS.purple }]} />
      </div>
    </Page>
  )
}
