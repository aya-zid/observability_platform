import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getRedisLogs, type RedisLogEvent } from "@/api/redis"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import { LogPagination } from "@/components/shared/LogPagination"
import { dateTimeInputToSeconds, formatNumber, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation } from "@/i18n/LanguageContext"

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
            <button
              type="button"
              onClick={() => logsQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted"
              title="Refresh"
            >
              <RefreshCw className={cn('h-4 w-4', logsQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/services/redis-dashboard/${id}`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
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
        <div className="grid gap-2 lg:grid-cols-[180px_1fr_auto]">
          <select
            className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none"
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
            className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && commitSearch()}
            onBlur={commitSearch}
            placeholder={t("redisLogs.searchPlaceholder")}
          />
          <button
            type="button"
            onClick={commitSearch}
            className="h-8 rounded-md border border-border px-2 text-xs hover:bg-muted"
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
                <thead className="text-sm uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-1.5 pr-3 text-left text-xs">{t("redisLogs.colTime")}</th>
                    <th className="py-1.5 pr-3 text-left text-xs">{t("redisLogs.colKind")}</th>
                    <th className="py-1.5 pr-3 text-left text-xs">{t("redisLogs.colPID")}</th>
                    <th className="py-1.5 pr-3 text-left text-xs">{t("redisLogs.colRole")}</th>
                    <th className="py-1.5 text-left text-xs">{t("redisLogs.colMessage")}</th>
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
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-muted-foreground">{event.timestamp}</td>
                          <td className="whitespace-nowrap py-1.5 pr-3">
                            <span className={cn("rounded-full px-1.5 py-0.5 text-xs", categoryTone(event))}>{event.category}</span>
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{event.pid || "-"}</td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{event.role || "-"}</td>
                          <td className="max-w-[760px] py-1.5">
                            <div className="truncate font-mono text-xs" title={event.message}>{event.message}</div>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr className="border-b border-border/60 bg-muted/20">
                            <td colSpan={5} className="py-3 pl-4">
                              <div className="space-y-2 text-sm">
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
              <LogPagination
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
