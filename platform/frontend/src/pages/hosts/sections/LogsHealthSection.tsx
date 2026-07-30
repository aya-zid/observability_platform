import { useMemo } from "react"
import { useHostLogsHealth } from "@/pages/hosts/hooks/useHostMetric"
import { LineChartCard } from "@/pages/hosts/components/charts/LineChartCard"
import { StackedAreaCard } from "@/pages/hosts/components/charts/StackedAreaCard"
import { StatCard } from "@/components/shared/StatCard"
import type { LogHealthRestartEvent } from "@/api/hosts"

interface Props {
  machineId: string
  params: { start: string; end: string; step: string }
}

function truncate(s: string, max: number): string {
  return s.length > max ? s.slice(0, max) + "..." : s
}

function formatDatumTs<T extends Record<string, any>>(data: T[]): T[] {
  return data.map((pt) => ({
    ...pt,
    time: new Date(pt.time as number).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    }),
  }))
}

export function LogsHealthSection({ machineId, params }: Props) {
  const query = useHostLogsHealth(machineId, params)
  const d = query.data

  const errorRateData = useMemo(() => {
    if (!d?.error_rate) return []
    return d.error_rate.map((b) => ({
      time: b.time,
      errors: b.errors,
      warnings: b.warnings,
    }))
  }, [d])

  const sshData = useMemo(() => {
    if (!d?.ssh_attempts) return []
    return d.ssh_attempts.map((b) => ({
      time: b.time,
      failed: b.failed,
      ok: b.ok,
    }))
  }, [d])

  const diskErrData = useMemo(() => {
    if (!d?.disk_errors) return []
    return d.disk_errors.map((b) => ({
      time: b.time,
      count: b.count,
    }))
  }, [d])

  const formattedErrorRate = useMemo(() => formatDatumTs(errorRateData), [errorRateData])
  const formattedSSH = useMemo(() => formatDatumTs(sshData), [sshData])
  const formattedDisk = useMemo(() => formatDatumTs(diskErrData), [diskErrData])

  const failedRestarts = useMemo(() => {
    if (!d?.restart_events) return []
    return d.restart_events.filter((e) => e.failed)
  }, [d])

  return (
    <div className="grid gap-4">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <StatCard
          title="Log Errors"
          value={d?.total_errors ?? "\u2014"}
          status={(d?.total_errors ?? 0) > 0 ? "red" : "green"}
          loading={query.isLoading}
        />
        <StatCard
          title="Log Warnings"
          value={d?.total_warnings ?? "\u2014"}
          status={(d?.total_warnings ?? 0) > 0 ? "yellow" : "green"}
          loading={query.isLoading}
        />
        <StatCard
          title="Service Failures"
          value={d?.total_restarts ?? "\u2014"}
          status={(d?.total_restarts ?? 0) > 0 ? "red" : "green"}
          loading={query.isLoading}
        />
        <StatCard
          title="OOM Kills"
          value={d?.total_oom_kills ?? "\u2014"}
          status={(d?.total_oom_kills ?? 0) > 0 ? "red" : "green"}
          loading={query.isLoading}
        />
      </div>

      <StackedAreaCard
        title="Error & Warning Rate"
        data={formattedErrorRate}
        areas={[
          { dataKey: "errors", color: "#ef4444", name: "Errors" },
          { dataKey: "warnings", color: "#f59e0b", name: "Warnings" },
        ]}
        loading={query.isLoading}
      />

      {(formattedSSH.length > 0 || formattedDisk.length > 0) && (
        <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
          {formattedSSH.length > 0 && (
            <StackedAreaCard
              title="SSH Auth Attempts"
              data={formattedSSH}
              areas={[
                { dataKey: "failed", color: "#ef4444", name: "Failed" },
                { dataKey: "ok", color: "#22c55e", name: "Accepted" },
              ]}
              loading={query.isLoading}
            />
          )}
          {formattedDisk.length > 0 && (
            <LineChartCard
              title="Disk I/O Errors"
              data={formattedDisk}
              lines={[{ dataKey: "count", color: "#ef4444", name: "I/O Errors" }]}
              loading={query.isLoading}
              yAxisLabel="count"
            />
          )}
        </div>
      )}

      {failedRestarts.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-3">
          <h3 className="mb-2 text-xs font-semibold text-foreground">Recent Service Failures</h3>
          <div className="max-h-48 overflow-y-auto space-y-1.5">
            {failedRestarts.slice(0, 20).map((evt: LogHealthRestartEvent, i: number) => (
              <div key={i} className="flex items-start gap-2 text-xs">
                <span className="shrink-0 font-mono text-muted-foreground">
                  {new Date(evt.time).toLocaleTimeString()}
                </span>
                <span className="shrink-0 rounded bg-red-500/15 px-1.5 py-0.5 font-medium text-red-500">
                  FAIL
                </span>
                <span className="font-medium text-foreground">{evt.unit || "unknown"}</span>
                <span className="text-muted-foreground">{truncate(evt.message, 120)}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {d && d.oom_kills.length > 0 && (
        <div className="rounded-lg border border-border bg-card p-3">
          <h3 className="mb-2 text-xs font-semibold text-foreground">OOM Kill Events</h3>
          <div className="max-h-48 overflow-y-auto space-y-1.5">
            {d.oom_kills.map((evt, i) => (
              <div key={i} className="flex items-start gap-2 text-xs">
                <span className="shrink-0 font-mono text-muted-foreground">
                  {new Date(evt.time).toLocaleTimeString()}
                </span>
                <span className="shrink-0 rounded bg-red-500/15 px-1.5 py-0.5 font-medium text-red-500">
                  OOM
                </span>
                <span className="text-muted-foreground">{truncate(evt.message, 150)}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
