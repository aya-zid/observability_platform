import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getMariaDBLogs, getMariaDBErrorLogs, type MariaDBLogEvent } from "@/api/mariadb"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import {
  dateTimeInputToSeconds,
  formatDateInputValue,
  formatNumber,
  formatTimeInputValue,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation, LangToggle } from "@/i18n/LanguageContext"

const TIME_RANGES = [
  { label: "15m", seconds: 900 },
  { label: "1h", seconds: 3600 },
  { label: "6h", seconds: 21600 },
  { label: "24h", seconds: 86400 },
] as const

const CATEGORIES = [
  "all",
  "event",
  "slow query",
  "error",
  "warning",
  "auth failure",
  "deadlock",
  "replication",
  "startup",
  "shutdown",
] as const

const PAGE_SIZE = 100

type TimeSelection =
  | { mode: "preset"; preset: (typeof TIME_RANGES)[number] }
  | { mode: "manual"; date: string; startTime: string; endTime: string }

function createDefaultManualSelection(): TimeSelection {
  const end = new Date()
  const start = new Date(end.getTime() - 60 * 60 * 1000)
  return {
    mode: "manual",
    date: formatDateInputValue(end),
    startTime: formatTimeInputValue(start),
    endTime: formatTimeInputValue(end),
  }
}

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

function categoryTone(event: MariaDBLogEvent) {
  const key = event.category.toLowerCase()
  if (event.severity === "ERROR" || event.severity === "FATAL" || key.includes("auth"))
    return "bg-red-500/10 text-red-500"
  if (key.includes("slow") || key.includes("deadlock") || key.includes("lock"))
    return "bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (key.includes("warning")) return "bg-yellow-500/10 text-yellow-600 dark:text-yellow-400"
  if (key.includes("replication")) return "bg-sky-500/10 text-sky-500"
  if (key.includes("startup") || key.includes("shutdown")) return "bg-violet-500/10 text-violet-600 dark:text-violet-300"
  if (key === "event") return "bg-teal-500/10 text-teal-600 dark:text-teal-300"
  return "bg-muted text-muted-foreground"
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
              selected.mode === "preset" && selected.preset.label === range.label
                ? "bg-foreground text-background"
                : "text-muted-foreground hover:bg-muted",
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
            selected.mode === "manual"
              ? "bg-foreground text-background"
              : "text-muted-foreground hover:bg-muted",
          )}
        >
          {t("shared.manual")}
        </button>
      </div>
      {selected.mode === "manual" ? (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-1">
          <input
            type="text"
            placeholder="DD-MM-YYYY"
            value={selected.date}
            onChange={(e) => onChange({ ...selected, date: e.target.value })}
            className="h-8 w-28 rounded border border-input bg-background px-2 text-sm"
          />
          <input
            type="time"
            value={selected.startTime}
            onChange={(e) => onChange({ ...selected, startTime: e.target.value })}
            className="h-8 rounded border border-input bg-background px-2 text-sm"
          />
          <input
            type="time"
            value={selected.endTime}
            onChange={(e) => onChange({ ...selected, endTime: e.target.value })}
            className="h-8 rounded border border-input bg-background px-2 text-sm"
          />
        </div>
      ) : null}
    </div>
  )
}

function Pagination({
  page,
  total,
  pageSize,
  hasMore,
  onChange,
}: {
  page: number
  total: number
  pageSize: number
  hasMore: boolean
  onChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(page * pageSize, total)

  return (
    <div className="flex items-center justify-between border-t border-border pt-4">
      <span className="text-xs text-muted-foreground">
        {total === 0 ? t("mdbLogs.paginationNoResults") : t("mdbLogs.paginationOf", formatNumber(from, 0), formatNumber(to, 0), formatNumber(total, 0))}
      </span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => onChange(1)}
          disabled={page <= 1}
          className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted"
        >
          «
        </button>
        <button
          type="button"
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted"
        >
          ‹
        </button>
        <span className="px-3 text-xs">
          {t("mdbLogs.pageOf", String(page), String(totalPages))}
        </span>
        <button
          type="button"
          onClick={() => onChange(page + 1)}
          disabled={!hasMore}
          className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted"
        >
          ›
        </button>
        <button
          type="button"
          onClick={() => onChange(totalPages)}
          disabled={!hasMore}
          className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted"
        >
          »
        </button>
      </div>
    </div>
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
  const [category, setCategory] = useState<(typeof CATEGORIES)[number]>("all")
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [page, setPage] = useState(1)
  const [openEvent, setOpenEvent] = useState<number | null>(null)
  const [minDurationSec, setMinDurationSec] = useState("")
  const [minDurationSecInput, setMinDurationSecInput] = useState("")

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

  function handleCategoryChange(value: (typeof CATEGORIES)[number]) {
    setCategory(value)
    setPage(1)
    setOpenEvent(null)
  }
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
    queryKey: ["mariadb-logs", id, timeWindow.start, timeWindow.end, category, search, page, minDurationSec],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () =>
      getMariaDBLogs(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        category,
        search,
        page,
        page_size: PAGE_SIZE,
        min_duration_ms: minDurationSec ? Number(minDurationSec) * 1000 : undefined,
      }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
    refetchOnWindowFocus: false,
  })

  const result = logsQuery.data
  const events = result?.events ?? []

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("mdbLogs.title")}
        description={t("mdbLogs.description")}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <TimeRangeSelector selected={timeSelection} onChange={handleTimeChange} />
            <LangToggle />
            <button
              type="button"
              onClick={() => logsQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', logsQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link
              to={`/integrations/mariadb-dashboard/${id}`}
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
        <span className="text-sm font-medium text-muted-foreground">{t("mdbLogs.logType")}:</span>
        <div className="flex gap-1 rounded-lg border border-border bg-muted p-1">
          {(["event", "error"] as const).map((type) => (
            <button
              key={type}
              type="button"
              onClick={() => { setLogType(type); setPage(1); setOpenEvent(null) }}
              className={cn(
                "rounded-md px-4 py-1.5 text-sm font-medium capitalize transition-colors",
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
        <div className="space-y-4">
          <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
            <p className="font-semibold">{t("mdbLogs.errorLogsTitle")}</p>
            <p className="mt-0.5 text-xs">{t("mdbLogs.errorLogsDesc")}</p>
          </div>

          {errorLogsQuery.isFetching ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              {t("shared.loading")}
            </div>
          ) : errorLogsQuery.isError ? (
            <div className="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-700">
              {t("mdbLogs.errorLogsFailed")}
            </div>
          ) : !errorLogsQuery.data?.lines?.length ? (
            <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
              {t("mdbLogs.errorLogsEmpty")}
            </div>
          ) : (
            <div className="rounded-lg border border-border bg-card overflow-hidden">
              <div className="border-b border-border p-4 flex items-center justify-between">
                <h3 className="font-semibold">
                  {t("mdbLogs.errorLogsCount", String(errorLogsQuery.data.lines.length))}
                </h3>
              </div>
              <div className="max-h-[600px] overflow-y-auto">
                {errorLogsQuery.data.lines.map((line, idx) => (
                  <div
                    key={idx}
                    className={cn(
                      "border-b border-border/50 px-4 py-2 font-mono text-xs leading-relaxed",
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
      <SectionCard title={t("mdbLogs.filters")}>
        <div className="grid gap-3 lg:grid-cols-[220px_1fr_auto]">
          <select
            className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
            value={category}
            onChange={(e) => handleCategoryChange(e.target.value as (typeof CATEGORIES)[number])}
          >
            {CATEGORIES.map((item) => (
              <option key={item} value={item}>
                {item === "all" ? t("mdbLogs.allCategories") : item}
              </option>
            ))}
          </select>
          <input
            className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && commitSearch()}
            onBlur={commitSearch}
            placeholder={t("mdbLogs.searchPlaceholder")}
          />
          <button
            type="button"
            onClick={commitSearch}
            className="h-10 rounded-md border border-border px-4 text-sm hover:bg-muted"
          >
            {t("shared.search")}
          </button>
          {category === "slow query" && (
            <div className="flex items-center gap-2 col-span-full lg:col-span-3">
              <label className="text-xs font-medium text-muted-foreground whitespace-nowrap">Min Duration (s):</label>
              <input
                type="number"
                min="0"
                step="0.1"
                value={minDurationSecInput}
                onChange={(e) => setMinDurationSecInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") { setMinDurationSec(minDurationSecInput); setPage(1) } }}
                onBlur={() => { setMinDurationSec(minDurationSecInput); setPage(1) }}
                placeholder="e.g. 2"
                className="h-8 w-24 rounded border border-input bg-background px-2 text-sm"
              />
              <button
                type="button"
                onClick={() => { setMinDurationSec(minDurationSecInput); setPage(1) }}
                className="h-8 rounded-md border border-border px-3 text-xs hover:bg-muted"
              >
                Apply
              </button>
              {minDurationSec && (
                <button
                  type="button"
                  onClick={() => { setMinDurationSec(""); setMinDurationSecInput(""); setPage(1) }}
                  className="h-8 rounded-md border border-border px-3 text-xs text-muted-foreground hover:bg-muted"
                >
                  Clear
                </button>
              )}
            </div>
          )}
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
              <table className="min-w-full text-sm">
                <thead className="text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colTime")}</th>
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colKind")}</th>
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colUser")}</th>
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colHost")}</th>
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colThread")}</th>
                    <th className="py-2 pr-4 text-left">{t("mdbLogs.colDuration")}</th>
                    <th className="py-2 text-left">{t("mdbLogs.colMessage")}</th>
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
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs text-muted-foreground">
                            {event.timestamp}
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4">
                            <span className={cn("rounded-full px-2 py-1 text-xs", categoryTone(event))}>
                              {event.category}
                            </span>
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs">
                            {event.user || "-"}
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs">
                            {event.host || "-"}
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs">
                            {event.thread_id || "-"}
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4">
                            {event.duration_ms != null
                              ? `${formatNumber(event.duration_ms)} ms`
                              : "-"}
                          </td>
                          <td className="max-w-[760px] py-2">
                            <div className="truncate font-mono text-xs" title={event.message}>
                              {event.message}
                            </div>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr className="border-b border-border/60 bg-muted/20">
                            <td colSpan={7} className="py-3 pl-4">
                              <div className="space-y-2 text-xs">
                                <div>
                                  <span className="text-muted-foreground">{t("mdbLogs.labelSeverity")}</span>{" "}
                                  <span className="font-mono">{event.severity}</span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("mdbLogs.labelUserHost")}</span>{" "}
                                  <span className="font-mono">
                                    {event.user || "-"} / {event.host || "-"}
                                  </span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("mdbLogs.labelSubsystem")}</span>{" "}
                                  <span className="font-mono">{event.subsystem || "-"}</span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("mdbLogs.labelFullMsg")}</span>
                                  <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono">
                                    {event.message}
                                  </div>
                                </div>
                                {event.labels && Object.keys(event.labels).length ? (
                                  <div>
                                    <span className="text-muted-foreground">{t("mdbLogs.labelExtra")}</span>
                                    <div className="mt-1 grid gap-1 sm:grid-cols-2">
                                      {Object.entries(event.labels).map(([key, value]) => (
                                        <div key={key} className="font-mono">
                                          <span className="text-muted-foreground">{key}:</span>{" "}
                                          {value}
                                        </div>
                                      ))}
                                    </div>
                                  </div>
                                ) : null}
                              </div>
                            </td>
                          </tr>
                        ) : null}
                      </Fragment>
                    )
                  })}
                  {!logsQuery.isFetching && events.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-8 text-center text-sm text-muted-foreground">
                        {t("mdbLogs.emptyFilter")}
                      </td>
                    </tr>
                  ) : null}
                  {logsQuery.isFetching && events.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-8 text-center text-sm text-muted-foreground">
                        {t("shared.loading")}
                      </td>
                    </tr>
                  ) : null}
                </tbody>
              </table>
            </div>

            {result && result.total > 0 ? (
              <Pagination
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
