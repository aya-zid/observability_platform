import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getMariaDBLogs, getMariaDBErrorLogs } from "@/api/mariadb"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { LogPagination } from "@/components/shared/LogPagination"
import { TIME_RANGES, TimeRangeSelector, type TimeSelection } from "@/components/shared/TimeRangeSelector"
import {
  dateTimeInputToSeconds,
  formatNumber,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation } from "@/i18n/LanguageContext"

const PAGE_SIZE = 100

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const end = getNowSeconds()
    return {
      start: getStartSeconds(selection.preset.seconds),
      end,
      error: "",
      isManual: false,
    }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null)
    return { start: 0, end: 0, error: "Use DD-MM-YYYY and HH:mm.", isManual: true }
  if (end <= start)
    return { start, end, error: "End time must be after start time.", isManual: true }
  return { start, end, error: "", isManual: true }
}

function NumericRange({ label, min, max, onMinChange, onMaxChange, placeholder, step }: {
  label: string
  min: string
  max: string
  onMinChange: (v: string) => void
  onMaxChange: (v: string) => void
  placeholder?: string
  step?: string
}) {
  return (
    <>
      <span className="text-sm text-muted-foreground whitespace-nowrap">{label}:</span>
      <input type="number" min="0" step={step || "1"} value={min}
        onChange={(e) => onMinChange(e.target.value)}
        placeholder={placeholder ? `min ${placeholder}` : "min"}
        className="h-8 w-[72px] rounded border border-input bg-background/70 px-2 text-sm outline-none" />
      <span className="text-sm text-muted-foreground">-</span>
      <input type="number" min="0" step={step || "1"} value={max}
        onChange={(e) => onMaxChange(e.target.value)}
        placeholder={placeholder ? `max ${placeholder}` : "max"}
        className="h-8 w-[72px] rounded border border-input bg-background/70 px-2 text-sm outline-none" />
    </>
  )
}

export function MariaDBLogsPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()

  const [timeSelection, setTimeSelection] = useState<TimeSelection>({
    mode: "preset",
    preset: TIME_RANGES[1],
  })
  const [logType, setLogType] = useState<"event" | "error">("event")
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [page, setPage] = useState(1)
  const [openEvent, setOpenEvent] = useState<number | null>(null)

  // Numeric filter state (all values in user-facing units: ms for duration & lock time)
  const [minDurationMs, setMinDurationMs] = useState("")
  const [maxDurationMs, setMaxDurationMs] = useState("")
  const [minRowsSent, setMinRowsSent] = useState("")
  const [maxRowsSent, setMaxRowsSent] = useState("")
  const [minRowsExamined, setMinRowsExamined] = useState("")
  const [maxRowsExamined, setMaxRowsExamined] = useState("")
  const [minLockTimeMs, setMinLockTimeMs] = useState("")
  const [maxLockTimeMs, setMaxLockTimeMs] = useState("")
  const [minRowsAffected, setMinRowsAffected] = useState("")
  const [maxRowsAffected, setMaxRowsAffected] = useState("")
  const [minBytesSent, setMinBytesSent] = useState("")
  const [maxBytesSent, setMaxBytesSent] = useState("")

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const errorLogsQuery = useQuery({
    queryKey: ["mariadb-error-logs", id, timeWindow.start, timeWindow.end, logType],
    queryFn: async () => {
      const endNs = secondsToNanos(timeWindow.end)
      const startNs = secondsToNanos(timeWindow.start)
      return getMariaDBErrorLogs(id, { start: String(startNs), end: String(endNs), limit: 200 })
    },
    enabled: !!id && !!serviceQuery.data && !timeWindow.error && logType === "error",
    refetchInterval: 30_000,
    staleTime: 0,
  })

  function handleTimeChange(value: TimeSelection) {
    setTimeSelection(value)
    setPage(1)
    setOpenEvent(null)
  }
  function commitSearch() {
    setSearch(searchInput)
    setPage(1)
    setOpenEvent(null)
  }
  const logsQuery = useQuery({
    queryKey: ["mariadb-logs", id, timeWindow.start, timeWindow.end, search, page, minDurationMs, maxDurationMs, minRowsSent, maxRowsSent, minRowsExamined, maxRowsExamined, minLockTimeMs, maxLockTimeMs, minRowsAffected, maxRowsAffected, minBytesSent, maxBytesSent],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error && logType === "event",
    queryFn: () =>
      getMariaDBLogs(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        category: "",
        search,
        page,
        page_size: PAGE_SIZE,
        min_duration_ms: minDurationMs ? Number(minDurationMs) : undefined,
        max_duration_ms: maxDurationMs ? Number(maxDurationMs) : undefined,
        min_rows_sent: minRowsSent ? Number(minRowsSent) : undefined,
        max_rows_sent: maxRowsSent ? Number(maxRowsSent) : undefined,
        min_rows_examined: minRowsExamined ? Number(minRowsExamined) : undefined,
        max_rows_examined: maxRowsExamined ? Number(maxRowsExamined) : undefined,
        min_lock_time_s: minLockTimeMs ? Number(minLockTimeMs) / 1000 : undefined,
        max_lock_time_s: maxLockTimeMs ? Number(maxLockTimeMs) / 1000 : undefined,
        min_rows_affected: minRowsAffected ? Number(minRowsAffected) : undefined,
        max_rows_affected: maxRowsAffected ? Number(maxRowsAffected) : undefined,
        min_bytes_sent: minBytesSent ? Number(minBytesSent) : undefined,
        max_bytes_sent: maxBytesSent ? Number(maxBytesSent) : undefined,
      }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
    refetchOnWindowFocus: false,
  })

  const result = logsQuery.data
  const events = result?.events ?? []

  function clearAllFilters() {
    setSearch(""); setSearchInput("")
    setMinDurationMs(""); setMaxDurationMs("")
    setMinRowsSent(""); setMaxRowsSent("")
    setMinRowsExamined(""); setMaxRowsExamined("")
    setMinLockTimeMs(""); setMaxLockTimeMs("")
    setMinRowsAffected(""); setMaxRowsAffected("")
    setMinBytesSent(""); setMaxBytesSent("")
    setPage(1)
    setOpenEvent(null)
  }

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("mdbLogs.title")}
        description={t("mdbLogs.description")}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <TimeRangeSelector selected={timeSelection} onChange={handleTimeChange} />
            <button
              type="button"
              onClick={() => logsQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', logsQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link
              to={`/services/mariadb-dashboard/${id}`}
              className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted"
            >
              {t("shared.dashboard")}
            </Link>
          </div>
        }
      />

      {timeWindow.error ? (
        <SectionCard title={t("mdbLogs.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      {/* Log type toggle — Event Logs vs Error Logs */}
      <div className="flex items-center gap-2">
        <span className="text-xs font-medium text-muted-foreground">{t("mdbLogs.logType")}:</span>
        <div className="flex gap-1 rounded-lg border border-border bg-muted p-0.5">
          {(["event", "error"] as const).map((type) => (
            <button
              key={type}
              type="button"
              onClick={() => { setLogType(type); setPage(1); setOpenEvent(null) }}
              className={cn(
                "rounded-md px-3 py-1 text-xs font-medium capitalize transition-colors",
                logType === type
                  ? type === "error"
                    ? "bg-red-600 text-white"
                    : "bg-primary text-primary-foreground"
                  : "text-muted-foreground hover:text-foreground",
              )}
            >
              {type === "event" ? t("mdbLogs.eventLogs") : t("mdbLogs.errorLogs")}
            </button>
          ))}
        </div>
      </div>

      {/* ── ERROR LOGS VIEW ─────────────────────────────────── */}
      {logType === "error" && (
        <div className="space-y-3 mt-3">
          <div className="rounded-lg border border-red-200 bg-red-50 p-2.5 text-xs text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
            <p className="font-semibold">{t("mdbLogs.errorLogsTitle")}</p>
            <p className="mt-0.5 text-xs">{t("mdbLogs.errorLogsDesc")}</p>
          </div>

          {errorLogsQuery.isFetching ? (
            <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
              {t("shared.loading")}
            </div>
          ) : errorLogsQuery.isError ? (
            <div className="rounded-lg border border-red-300 bg-red-50 p-3 text-xs text-red-700">
              {t("mdbLogs.errorLogsFailed")}
            </div>
          ) : !errorLogsQuery.data?.lines?.length ? (
            <div className="flex items-center justify-center py-8 text-xs text-muted-foreground">
              {t("mdbLogs.errorLogsEmpty")}
            </div>
          ) : (
            <div className="rounded-lg border border-border bg-card overflow-hidden">
              <div className="border-b border-border p-3 flex items-center justify-between">
                <h3 className="text-xs font-semibold">
                  {t("mdbLogs.errorLogsCount", String(errorLogsQuery.data.lines.length))}
                </h3>
              </div>
              <div className="max-h-[600px] overflow-y-auto">
                {errorLogsQuery.data.lines.map((line, idx) => (
                  <div
                    key={idx}
                    className={cn(
                      "border-b border-border/50 px-3 py-1.5 font-mono text-xs leading-relaxed",
                      line.includes("[ERROR]") || line.includes("[FATAL]")
                        ? "text-red-700 dark:text-red-400 bg-red-50/30 dark:bg-red-900/10"
                        : line.includes("[Warning]")
                          ? "text-yellow-700 dark:text-yellow-400 bg-yellow-50/30 dark:bg-yellow-900/10"
                          : "text-foreground hover:bg-muted/30",
                    )}
                  >
                    {line}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── EVENT LOGS VIEW ─────────────────────────────────── */}
      {logType === "event" && (<>
      <SectionCard title="">
        <div className="space-y-2">
          {/* Row 1: Text filters + message search */}
          <div className="flex flex-wrap items-center gap-2">
            <input
              className="h-8 flex-1 rounded-md border border-input bg-background/70 px-2 text-xs outline-none min-w-[150px]"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && commitSearch()}
              onBlur={commitSearch}
              placeholder={t("mdbLogs.searchPlaceholder")}
            />
            <button
              type="button"
              onClick={() => { commitSearch() }}
              className="h-8 rounded-md border border-border px-2 text-xs hover:bg-muted"
            >
              {t("shared.search")}
            </button>
            <button
              type="button"
              onClick={clearAllFilters}
              className="h-8 rounded-md border border-border px-2 text-xs hover:bg-muted"
            >
              Clear
            </button>
          </div>
          {/* Row 2: Numeric range filters (all values in ms except rows/bytes) */}
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
            <NumericRange label="Dur (ms)" placeholder="ms" step="1"
              min={minDurationMs} max={maxDurationMs}
              onMinChange={(v) => { setMinDurationMs(v); setPage(1) }}
              onMaxChange={(v) => { setMaxDurationMs(v); setPage(1) }} />
            <NumericRange label="Rows Sent"
              min={minRowsSent} max={maxRowsSent}
              onMinChange={(v) => { setMinRowsSent(v); setPage(1) }}
              onMaxChange={(v) => { setMaxRowsSent(v); setPage(1) }} />
            <NumericRange label="Rows Examined"
              min={minRowsExamined} max={maxRowsExamined}
              onMinChange={(v) => { setMinRowsExamined(v); setPage(1) }}
              onMaxChange={(v) => { setMaxRowsExamined(v); setPage(1) }} />
            <NumericRange label="Lock (ms)" placeholder="ms" step="1"
              min={minLockTimeMs} max={maxLockTimeMs}
              onMinChange={(v) => { setMinLockTimeMs(v); setPage(1) }}
              onMaxChange={(v) => { setMaxLockTimeMs(v); setPage(1) }} />
            <NumericRange label="Rows Affected"
              min={minRowsAffected} max={maxRowsAffected}
              onMinChange={(v) => { setMinRowsAffected(v); setPage(1) }}
              onMaxChange={(v) => { setMaxRowsAffected(v); setPage(1) }} />
            <NumericRange label="Bytes Sent"
              min={minBytesSent} max={maxBytesSent}
              onMinChange={(v) => { setMinBytesSent(v); setPage(1) }}
              onMaxChange={(v) => { setMaxBytesSent(v); setPage(1) }} />
          </div>
        </div>
      </SectionCard>

      <SectionCard
        title={t("mdbLogs.eventsTitle")}
        description={
          logsQuery.isFetching
            ? t("shared.loading")
            : result
              ? t("mdbLogs.totalEvents", formatNumber(result.total, 0))
              : ""
        }
      >
        {logsQuery.isError ? (
          <div className="py-6 text-center text-sm text-red-500">
            {t("mdbLogs.loadError")}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="text-sm uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">{t("mdbLogs.colTime")}</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">{t("mdbLogs.colUser")}</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">{t("mdbLogs.colHost")}</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">{t("mdbLogs.colThread")}</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">{t("mdbLogs.colDuration")}</th>
                    <th className="py-1.5 pr-3 text-right whitespace-nowrap text-xs">Rows Sent</th>
                    <th className="py-1.5 pr-3 text-right whitespace-nowrap text-xs">Rows Examined</th>
                    <th className="py-1.5 pr-3 text-right whitespace-nowrap text-xs">Lock Time</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">Schema</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">QC Hit</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">Rows Affected</th>
                    <th className="py-1.5 pr-3 text-left whitespace-nowrap text-xs">Bytes Sent</th>
                    <th className="py-1.5 text-left text-xs">{t("mdbLogs.colMessage")}</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((event, index) => {
                    const isOpen = openEvent === index
                    return (
                      <Fragment key={`${event.timestamp}-${index}`}>
                        <tr
                          className="cursor-pointer border-b border-border/60 hover:bg-muted/40"
                          onClick={() => setOpenEvent(isOpen ? null : index)}
                        >
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-muted-foreground">
                            {event.timestamp}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.user || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.host || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.thread_id || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.duration_ms != null
                              ? `${formatNumber(event.duration_ms)} ms`
                              : "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-right tabular-nums">
                            {event.rows_sent != null ? formatNumber(event.rows_sent) : "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-right tabular-nums">
                            {event.rows_examined != null ? formatNumber(event.rows_examined) : "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-right tabular-nums">
                            {event.lock_time_s != null
                              ? `${formatNumber(event.lock_time_s * 1000, 1)} ms`
                              : "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.schema || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.qc_hit || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.rows_affected != null ? formatNumber(event.rows_affected) : "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.bytes_sent != null ? formatNumber(event.bytes_sent) : "-"}
                          </td>
                          <td className="max-w-[240px] py-1.5">
                            <div className="truncate font-mono text-xs" title={event.message}>
                              {event.message}
                            </div>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr className="border-b border-border/60 bg-muted/20">
                            <td colSpan={13} className="py-3 pl-4 pr-4">
                              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-2 text-sm">
                                <div>
                                  <span className="text-muted-foreground">Timestamp</span>
                                  <div className="font-mono mt-0.5">{event.timestamp}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Severity</span>
                                  <div className="font-mono mt-0.5">{event.severity}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Category</span>
                                  <div className="font-mono mt-0.5">{event.category}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">User</span>
                                  <div className="font-mono mt-0.5">{event.user || "-"}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Host</span>
                                  <div className="font-mono mt-0.5">{event.host || "-"}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Thread ID</span>
                                  <div className="font-mono mt-0.5">{event.thread_id || "-"}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Duration</span>
                                  <div className="font-mono mt-0.5">
                                    {event.duration_ms != null ? `${formatNumber(event.duration_ms)} ms` : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Rows Sent</span>
                                  <div className="font-mono mt-0.5">
                                    {event.rows_sent != null ? formatNumber(event.rows_sent) : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Rows Examined</span>
                                  <div className="font-mono mt-0.5">
                                    {event.rows_examined != null ? formatNumber(event.rows_examined) : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Lock Time</span>
                                  <div className="font-mono mt-0.5">
                                    {event.lock_time_s != null ? `${formatNumber(event.lock_time_s * 1000, 1)} ms` : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Schema</span>
                                  <div className="font-mono mt-0.5">{event.schema || "-"}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">QC Hit</span>
                                  <div className="font-mono mt-0.5">{event.qc_hit || "-"}</div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Rows Affected</span>
                                  <div className="font-mono mt-0.5">
                                    {event.rows_affected != null ? formatNumber(event.rows_affected) : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Bytes Sent</span>
                                  <div className="font-mono mt-0.5">
                                    {event.bytes_sent != null ? formatNumber(event.bytes_sent) : "-"}
                                  </div>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">Subsystem</span>
                                  <div className="font-mono mt-0.5">{event.subsystem || "-"}</div>
                                </div>
                                <div className="col-span-2 sm:col-span-3 lg:col-span-4">
                                  <span className="text-muted-foreground">Full Message</span>
                                  <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono">
                                    {event.message}
                                  </div>
                                </div>
                              </div>
                            </td>
                          </tr>
                        ) : null}
                      </Fragment>
                    )
                  })}
                  {!logsQuery.isFetching && events.length === 0 ? (
                    <tr>
                      <td colSpan={13} className="py-8 text-center text-sm text-muted-foreground">
                        {t("mdbLogs.emptyFilter")}
                      </td>
                    </tr>
                  ) : null}
                  {logsQuery.isFetching && events.length === 0 ? (
                    <tr>
                      <td colSpan={13} className="py-8 text-center text-sm text-muted-foreground">
                        {t("shared.loading")}
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>

            {result && result.total > 0 ? (
              <LogPagination
                page={result.page}
                total={result.total}
                pageSize={result.page_size}
                hasMore={result.has_more}
                onChange={(p) => {
                  setPage(p)
                  setOpenEvent(null)
                  window.scrollTo({ top: 0, behavior: "smooth" })
                }}
              />
            ) : null}
          </>
        )}
      </SectionCard>
      </>)}
    </Page>
  )
}
