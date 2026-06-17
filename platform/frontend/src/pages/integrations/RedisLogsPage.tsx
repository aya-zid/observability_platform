import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getRedisLogs, type RedisLogEvent } from "@/api/redis"
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
  "warning",
  "error",
  "auth",
  "security",
  "startup",
  "shutdown",
  "persistence",
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
    return { start: getStartSeconds(selection.preset.seconds), end, error: "", isManual: false }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null) return { start: 0, end: 0, error: "Use DD-MM-YYYY and HH:mm.", isManual: true }
  if (end <= start) return { start, end, error: "End time must be after start time.", isManual: true }
  return { start, end, error: "", isManual: true }
}

function categoryTone(event: RedisLogEvent) {
  const key = event.category.toLowerCase()
  if (event.severity === "ERROR" || event.severity === "WARNING" || key === "warning")
    return "bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (key === "error") return "bg-red-500/10 text-red-500"
  if (key === "shutdown") return "bg-red-500/10 text-red-500"
  if (key === "startup") return "bg-teal-500/10 text-teal-600 dark:text-teal-300"
  if (key === "persistence") return "bg-indigo-500/10 text-indigo-600 dark:text-indigo-300"
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
            selected.mode === "manual" ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted",
          )}
        >
          {t("shared.manual")}
        </button>
      </div>
      {selected.mode === "manual" ? (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-1">
          <input type="text" placeholder="DD-MM-YYYY" value={selected.date} onChange={(e) => onChange({ ...selected, date: e.target.value })} className="h-8 w-28 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.startTime} onChange={(e) => onChange({ ...selected, startTime: e.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
          <input type="time" value={selected.endTime} onChange={(e) => onChange({ ...selected, endTime: e.target.value })} className="h-8 rounded border border-input bg-background px-2 text-sm" />
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
        {total === 0 ? t("redisLogs.paginationNoResults") : t("redisLogs.paginationOf", formatNumber(from, 0), formatNumber(to, 0), formatNumber(total, 0))}
      </span>
      <div className="flex items-center gap-1">
        <button type="button" onClick={() => onChange(1)} disabled={page <= 1} className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted">«</button>
        <button type="button" onClick={() => onChange(page - 1)} disabled={page <= 1} className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted">‹</button>
        <span className="px-3 text-xs">{t("redisLogs.pageOf", String(page), String(totalPages))}</span>
        <button type="button" onClick={() => onChange(page + 1)} disabled={!hasMore} className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted">›</button>
        <button type="button" onClick={() => onChange(totalPages)} disabled={!hasMore} className="rounded border border-border px-2 py-1 text-xs disabled:opacity-40 hover:bg-muted">»</button>
      </div>
    </div>
  )
}

export function RedisLogsPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()

  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [category, setCategory] = useState<(typeof CATEGORIES)[number]>("all")
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [page, setPage] = useState(1)
  const [openEvent, setOpenEvent] = useState<number | null>(null)

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

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
    queryKey: ["redis-logs", id, timeWindow.start, timeWindow.end, category, search, page],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () =>
      getRedisLogs(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        category,
        search,
        page,
        page_size: PAGE_SIZE,
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
        title={t("redisLogs.title")}
        description={t("redisLogs.description")}
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
            <Link to={`/integrations/redis-dashboard/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              {t("shared.dashboard")}
            </Link>
          </div>
        }
      />

      {timeWindow.error ? (
        <SectionCard title={t("redisLogs.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      <SectionCard title={t("redisLogs.filters")}>
        <div className="grid gap-3 lg:grid-cols-[220px_1fr_auto]">
          <select
            className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
            value={category}
            onChange={(e) => handleCategoryChange(e.target.value as (typeof CATEGORIES)[number])}
          >
            {CATEGORIES.map((item) => (
              <option key={item} value={item}>
                {item === "all" ? t("redisLogs.allCategories") : item}
              </option>
            ))}
          </select>
          <input
            className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && commitSearch()}
            onBlur={commitSearch}
            placeholder={t("redisLogs.searchPlaceholder")}
          />
          <button
            type="button"
            onClick={commitSearch}
            className="h-10 rounded-md border border-border px-4 text-sm hover:bg-muted"
          >
            {t("shared.search")}
          </button>
        </div>
      </SectionCard>

      <SectionCard
        title={t("redisLogs.eventsTitle")}
        description={
          logsQuery.isFetching
            ? t("shared.loading")
            : result
              ? t("redisLogs.totalEvents", formatNumber(result.total, 0))
              : ""
        }
      >
        {logsQuery.isError ? (
          <div className="py-6 text-center text-sm text-red-500">{t("redisLogs.loadError")}</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-2 pr-4 text-left">{t("redisLogs.colTime")}</th>
                    <th className="py-2 pr-4 text-left">{t("redisLogs.colKind")}</th>
                    <th className="py-2 pr-4 text-left">{t("redisLogs.colPID")}</th>
                    <th className="py-2 pr-4 text-left">{t("redisLogs.colRole")}</th>
                    <th className="py-2 text-left">{t("redisLogs.colMessage")}</th>
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
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs text-muted-foreground">{event.timestamp}</td>
                          <td className="whitespace-nowrap py-2 pr-4">
                            <span className={cn("rounded-full px-2 py-1 text-xs", categoryTone(event))}>{event.category}</span>
                          </td>
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs">{event.pid || "-"}</td>
                          <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs">{event.role || "-"}</td>
                          <td className="max-w-[760px] py-2">
                            <div className="truncate font-mono text-xs" title={event.message}>{event.message}</div>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr className="border-b border-border/60 bg-muted/20">
                            <td colSpan={5} className="py-3 pl-4">
                              <div className="space-y-2 text-xs">
                                <div>
                                  <span className="text-muted-foreground">{t("redisLogs.labelSeverity")}</span>{" "}
                                  <span className="font-mono">{event.severity}</span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("redisLogs.labelPIDRole")}</span>{" "}
                                  <span className="font-mono">{event.pid || "-"} / {event.role || "-"}</span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("redisLogs.labelFullMsg")}</span>
                                  <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono">{event.message}</div>
                                </div>
                                {event.labels && Object.keys(event.labels).length ? (
                                  <div>
                                    <span className="text-muted-foreground">{t("redisLogs.labelExtra")}</span>
                                    <div className="mt-1 grid gap-1 sm:grid-cols-2">
                                      {Object.entries(event.labels).map(([key, value]) => (
                                        <div key={key} className="font-mono"><span className="text-muted-foreground">{key}:</span> {value}</div>
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
                    <tr><td colSpan={5} className="py-8 text-center text-sm text-muted-foreground">{t("redisLogs.emptyFilter")}</td></tr>
                  ) : null}
                  {logsQuery.isFetching && events.length === 0 ? (
                    <tr><td colSpan={5} className="py-8 text-center text-sm text-muted-foreground">{t("shared.loading")}</td></tr>
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
                onChange={(p) => { setPage(p); setOpenEvent(null); window.scrollTo({ top: 0, behavior: "smooth" }) }}
              />
            ) : null}
          </>
        )}
      </SectionCard>
    </Page>
  )
}
