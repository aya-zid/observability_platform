import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"
import { ArrowLeft, RefreshCw, Search } from "lucide-react"

import { getDiscoveryHosts } from "@/api/discovery"
import { getHostLogs, type HostLogEvent } from "@/api/hosts"
import { LogPagination } from "@/components/shared/LogPagination"
import { TimeRangeSelector, TIME_RANGES, type TimeSelection } from "@/components/shared/TimeRangeSelector"
import { cn } from "@/lib/utils"
import { dateTimeInputToSeconds, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"

const PAGE_SIZE = 100

const PRIORITIES = [
  { value: "", label: "All levels" },
  { value: "0", label: "Emergency" },
  { value: "1", label: "Alert" },
  { value: "2", label: "Critical" },
  { value: "3", label: "Error" },
  { value: "4", label: "Warning" },
  { value: "5", label: "Notice" },
  { value: "6", label: "Info" },
  { value: "7", label: "Debug" },
]

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const end = getNowSeconds()
    return { start: getStartSeconds(selection.preset.seconds), end, error: "", isManual: false }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null) return { start: 0, end: 0, error: "Use DD-MM-YYYY and HH:mm.", isManual: true }
  if (end <= start) return { start, end, error: "End time must be after start time.", isManual: true }
  return { start, end, error: "", isManual: true }
}

function severityClass(severity: string) {
  const key = severity.toLowerCase()
  if (["emergency", "alert", "critical", "error"].includes(key)) return "bg-red-500/10 text-red-500"
  if (key === "warning") return "bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (key === "notice") return "bg-blue-500/10 text-blue-600 dark:text-blue-300"
  if (key === "debug") return "bg-muted text-muted-foreground"
  return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
}

function Stat({ label, value, tone = "default" }: { label: string; value: number | string; tone?: "default" | "red" | "amber" }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <div className="text-xs font-medium uppercase text-muted-foreground">{label}</div>
      <div className={cn("mt-2 text-2xl font-semibold tabular-nums", tone === "red" && "text-red-500", tone === "amber" && "text-amber-500")}>{value}</div>
    </div>
  )
}

function formatLocalTimestamp(ns: string): string {
  const ms = Number(BigInt(ns) / 1_000_000n)
  const d = new Date(ms)
  return d.getFullYear() + "-"
    + String(d.getMonth() + 1).padStart(2, "0") + "-"
    + String(d.getDate()).padStart(2, "0") + " "
    + String(d.getHours()).padStart(2, "0") + ":"
    + String(d.getMinutes()).padStart(2, "0") + ":"
    + String(d.getSeconds()).padStart(2, "0") + "."
    + String(d.getMilliseconds()).padStart(3, "0")
}

function compactRaw(raw?: Record<string, string>) {
  if (!raw) return []
  const hidden = new Set(["MESSAGE"])
  return Object.entries(raw)
    .filter(([key]) => !hidden.has(key))
    .sort(([a], [b]) => a.localeCompare(b))
}

export function HostLogsPage() {
  const { machineId = "" } = useParams<{ machineId?: string }>()
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [priority, setPriority] = useState("")
  const [unit, setUnit] = useState("")
  const [identifier, setIdentifier] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)
  const [openRow, setOpenRow] = useState<number | null>(null)

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const host = useMemo(
    () => (hostsQuery.data ?? []).find((h) => h.machine_id === machineId),
    [hostsQuery.data, machineId],
  )
  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const logsQuery = useQuery({
    queryKey: ["host-logs", machineId, timeWindow.start, timeWindow.end, priority, unit, identifier, search, page],
    enabled: !!machineId && !timeWindow.error,
    queryFn: () => getHostLogs(machineId, {
      start: secondsToNanos(timeWindow.start),
      end: secondsToNanos(timeWindow.end),
      priority,
      unit,
      identifier,
      search,
      page,
      page_size: PAGE_SIZE,
    }),
    placeholderData: (prev) => prev,
    staleTime: 15_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
    refetchOnWindowFocus: false,
  })

  function resetPage() {
    setPage(1)
    setOpenRow(null)
  }

  function commitSearch() {
    setSearch(searchInput)
    resetPage()
  }

  function renderRow(event: HostLogEvent, index: number) {
    const isOpen = openRow === index
    return (
      <Fragment key={`${event.timestamp_ns}-${index}`}>
        <tr className="cursor-pointer border-b border-border/60 hover:bg-muted/40" onClick={() => setOpenRow(isOpen ? null : index)}>
          <td className="whitespace-nowrap py-2 pr-3 font-mono text-xs text-muted-foreground">{formatLocalTimestamp(event.timestamp_ns)}</td>
          <td className="whitespace-nowrap py-2 pr-3">
            <span className={cn("rounded-full px-2 py-1 text-xs", severityClass(event.severity))}>{event.severity}</span>
          </td>
          <td className="max-w-[180px] truncate py-2 pr-3 font-mono text-xs" title={event.unit || "-"}>{event.unit || "-"}</td>
          <td className="max-w-[140px] truncate py-2 pr-3 font-mono text-xs" title={event.identifier || "-"}>{event.identifier || "-"}</td>
          <td className="py-2">
            <div className="truncate font-mono text-xs" title={event.message}>{event.message}</div>
          </td>
        </tr>
        {isOpen ? (
          <tr className="border-b border-border/60 bg-muted/20">
            <td colSpan={5} className="p-4">
              <div className="grid gap-3 text-xs">
                <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
                  <div><span className="text-muted-foreground">PID: </span><span className="font-mono">{event.pid || "-"}</span></div>
                  <div><span className="text-muted-foreground">Transport: </span><span className="font-mono">{event.transport || "-"}</span></div>
                  <div><span className="text-muted-foreground">Priority: </span><span className="font-mono">{event.priority || "-"}</span></div>
                  <div><span className="text-muted-foreground">Timestamp ns: </span><span className="font-mono">{event.timestamp_ns}</span></div>
                </div>
                {event.command ? <div><span className="text-muted-foreground">Command: </span><span className="font-mono">{event.command}</span></div> : null}
                <div>
                  <div className="mb-1 text-muted-foreground">Message</div>
                  <div className="whitespace-pre-wrap break-words rounded border border-border bg-background p-3 font-mono">{event.message}</div>
                </div>
                {compactRaw(event.raw).length ? (
                  <div className="grid gap-1 rounded border border-border bg-background p-3 font-mono">
                    {compactRaw(event.raw).map(([key, value]) => (
                      <div key={key} className="grid gap-2 sm:grid-cols-[220px_1fr]">
                        <span className="text-muted-foreground">{key}</span>
                        <span className="break-all">{value}</span>
                      </div>
                    ))}
                  </div>
                ) : null}
              </div>
            </td>
          </tr>
        ) : null}
      </Fragment>
    )
  }

  if (!machineId) return <div className="p-6 text-sm text-red-600">Missing machine ID.</div>

  const data = logsQuery.data
  const events = data?.events ?? []
  const summary = data?.summary
  const errors = (summary?.by_level.error ?? 0) + (summary?.by_level.critical ?? 0) + (summary?.by_level.alert ?? 0) + (summary?.by_level.emergency ?? 0)
  const warnings = summary?.by_level.warning ?? 0
  const units = summary?.units ?? []

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3 pb-2">
        <div className="flex items-center gap-4">
          <Link to={`/hosts/${machineId}`} className="rounded-lg p-2 text-muted-foreground transition-colors hover:bg-primary/10 hover:text-primary">
            <ArrowLeft className="h-5 w-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold">Host Logs</h1>
            <p className="mt-0.5 text-sm text-muted-foreground">
              <span>{host?.hostname ?? machineId}</span>
              <span className="ml-2 font-mono">{machineId}</span>
            </p>
          </div>
        </div>
        <button
          type="button"
          onClick={() => logsQuery.refetch()}
          className="inline-flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm hover:bg-muted"
        >
          <RefreshCw className={cn("h-4 w-4", logsQuery.isFetching && "animate-spin")} />
          Refresh
        </button>
      </div>

      <div className="flex flex-wrap items-center gap-3 rounded-lg border border-border bg-card p-4">
        <TimeRangeSelector selected={timeSelection} onChange={(value) => { setTimeSelection(value); resetPage() }} />
        <select value={priority} onChange={(e) => { setPriority(e.target.value); resetPage() }} className="rounded-md border border-border bg-background px-3 py-2 text-sm">
          {PRIORITIES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}
        </select>
        <select value={unit} onChange={(e) => { setUnit(e.target.value); resetPage() }} className="rounded-md border border-border bg-background px-3 py-2 text-sm">
          <option value="">All units</option>
          {units.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
        <input
          value={identifier}
          onChange={(e) => { setIdentifier(e.target.value); resetPage() }}
          placeholder="Identifier"
          className="w-36 rounded-md border border-border bg-background px-3 py-2 text-sm"
        />
        <div className="flex min-w-[240px] flex-1 items-center gap-2">
          <input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") commitSearch() }}
            placeholder="Search message, unit, PID..."
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
          />
          <button type="button" onClick={commitSearch} className="inline-flex items-center gap-2 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground">
            <Search className="h-4 w-4" />
            Search
          </button>
        </div>
      </div>

      {timeWindow.error ? <div className="rounded-md border border-red-500/30 bg-red-500/10 p-3 text-sm text-red-500">{timeWindow.error}</div> : null}

      <div className="grid gap-4 md:grid-cols-4">
        <Stat label="Total" value={summary?.total ?? 0} />
        <Stat label="Errors" value={errors} tone="red" />
        <Stat label="Warnings" value={warnings} tone="amber" />
        <Stat label="Units" value={units.length} />
      </div>

      <div className="rounded-lg border border-border bg-card">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border p-4">
          <div>
            <div className="font-medium">Journald Events</div>
            <div className="mt-1 text-xs text-muted-foreground">
              {summary?.first_seen && summary?.last_seen ? `${summary.first_seen} to ${summary.last_seen}` : "Newest entries first"}
            </div>
          </div>
          {logsQuery.isFetching ? <div className="text-xs text-muted-foreground">Updating logs...</div> : null}
        </div>

        <div className="overflow-x-auto">
          <table className="w-full min-w-[900px] text-left">
            <thead className="border-b border-border bg-muted/40 text-xs uppercase text-muted-foreground">
              <tr>
                <th className="py-2 pl-4 pr-3 font-medium">Time</th>
                <th className="py-2 pr-3 font-medium">Level</th>
                <th className="py-2 pr-3 font-medium">Unit</th>
                <th className="py-2 pr-3 font-medium">Identifier</th>
                <th className="py-2 pr-4 font-medium">Message</th>
              </tr>
            </thead>
            <tbody>
              {logsQuery.isLoading ? (
                <tr><td colSpan={5} className="p-6 text-center text-sm text-muted-foreground">Loading logs...</td></tr>
              ) : events.length === 0 ? (
                <tr><td colSpan={5} className="p-6 text-center text-sm text-muted-foreground">No host logs match the selected filters.</td></tr>
              ) : (
                events.map(renderRow)
              )}
            </tbody>
          </table>
        </div>

        <div className="px-4 pb-4">
          <LogPagination
            page={data?.page ?? page}
            total={data?.total ?? 0}
            pageSize={data?.page_size ?? PAGE_SIZE}
            hasMore={data?.has_more ?? false}
            onChange={(next) => { setPage(next); setOpenRow(null) }}
          />
        </div>
      </div>
    </div>
  )
}
