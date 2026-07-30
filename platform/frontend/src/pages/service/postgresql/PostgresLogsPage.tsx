import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import { getPostgresLogs, type PostgresLogEvent } from "@/api/postgres"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { TimeRangeSelector, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import type { TimeSelection } from "@/components/shared/TimeRangeSelector"
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

// ── Constants ────────────────────────────────────────────────────────────────

const CATEGORIES = [
  "all",
  "event",
  "slow query",
  "error",
  "auth failure",
  "lock wait",
  "schema change",
  "temp file",
  "connection",
  "disconnection",
  "checkpoint",
] as const

const PAGE_SIZE = 100

// ── Time selection helpers ───────────────────────────────────────────────────

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

// ── Category badge styling ───────────────────────────────────────────────────

function categoryTone(event: PostgresLogEvent) {
  const key = event.category.toLowerCase()
  if (event.severity === "ERROR" || event.severity === "FATAL" || key.includes("auth"))
    return "bg-red-500/10 text-red-500"
  if (key.includes("slow") || key.includes("lock") || key.includes("temp"))
    return "bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (key.includes("schema")) return "bg-sky-500/10 text-sky-500"
  if (key === "event") return "bg-violet-500/10 text-violet-600 dark:text-violet-300"
  if (key.includes("connection") || key.includes("disconnection")) return "bg-teal-500/10 text-teal-600 dark:text-teal-300"
  if (key.includes("checkpoint")) return "bg-indigo-500/10 text-indigo-600 dark:text-indigo-300"
  return "bg-muted text-muted-foreground"
}

// ── Sub-components ───────────────────────────────────────────────────────────

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
    <div className="flex items-center justify-between border-t border-border pt-3">
      <span className="text-xs text-muted-foreground">
        {total === 0 ? t("pgLogs.paginationNoResults") : t("pgLogs.paginationOf", formatNumber(from, 0), formatNumber(to, 0), formatNumber(total, 0))}
      </span>
      <div className="flex items-center gap-1">
        <button
          type="button"
          onClick={() => onChange(1)}
          disabled={page <= 1}
          className="rounded border border-border px-1.5 py-0.5 text-xs disabled:opacity-40 hover:bg-muted"
        >
          «
        </button>
        <button
          type="button"
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className="rounded border border-border px-1.5 py-0.5 text-xs disabled:opacity-40 hover:bg-muted"
        >
          ‹
        </button>
        <span className="px-2 text-xs">
          {t("pgLogs.pageOf", String(page), String(totalPages))}
        </span>
        <button
          type="button"
          onClick={() => onChange(page + 1)}
          disabled={!hasMore}
          className="rounded border border-border px-1.5 py-0.5 text-xs disabled:opacity-40 hover:bg-muted"
        >
          ›
        </button>
        <button
          type="button"
          onClick={() => onChange(totalPages)}
          disabled={!hasMore}
          className="rounded border border-border px-1.5 py-0.5 text-xs disabled:opacity-40 hover:bg-muted"
        >
          »
        </button>
      </div>
    </div>
  )
}

// ── Main page ────────────────────────────────────────────────────────────────

export function PostgresLogsPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()

  const [timeSelection, setTimeSelection] = useState<TimeSelection>({
    mode: "preset",
    preset: TIME_RANGES[1],
  })
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
    queryKey: ["postgres-logs", id, timeWindow.start, timeWindow.end, category, search, page],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () =>
      getPostgresLogs(id, {
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
        title={t("pgLogs.title")}
        description={t("pgLogs.description")}
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
              to={`/services/postgres-dashboard/${id}`}
              className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted"
            >
              {t("shared.dashboard")}
            </Link>
          </div>
        }
      />

      {timeWindow.error ? (
        <SectionCard title={t("pgLogs.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      {/* ── Filters ── */}
      <SectionCard title={t("pgLogs.filters")}>
        <div className="grid gap-2 lg:grid-cols-[180px_1fr_auto_auto]">
          <select
            className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none"
            value={category}
            onChange={(e) => handleCategoryChange(e.target.value as (typeof CATEGORIES)[number])}
          >
            {CATEGORIES.map((item) => (
              <option key={item} value={item}>
                {item === "all" ? t("pgLogs.allCategories") : item}
              </option>
            ))}
          </select>
          <input
            className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && commitSearch()}
            onBlur={commitSearch}
            placeholder={t("pgLogs.searchPlaceholder")}
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

      {/* ── Events table ── */}
      <SectionCard
        title={t("pgLogs.eventsTitle")}
        description={
          logsQuery.isFetching
            ? t("shared.loading")
            : result
              ? t("pgLogs.totalEvents", formatNumber(result.total, 0))
              : ""
        }
      >
        {logsQuery.isError ? (
          <div className="py-6 text-center text-sm text-red-500">
            {t("pgLogs.loadError")}
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-sm uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colTime")}</th>
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colKind")}</th>
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colUser")}</th>
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colDatabase")}</th>
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colClient")}</th>
                    <th className="py-1.5 pr-3 text-left">{t("pgLogs.colDuration")}</th>
                    <th className="py-1.5 text-left">{t("pgLogs.colMessage")}</th>
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
                          <td className="whitespace-nowrap py-1.5 pr-3">
                            <span className={cn("rounded-full px-1.5 py-0.5 text-xs", categoryTone(event))}>
                              {event.category}
                            </span>
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.user || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.database || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                            {event.client || "-"}
                          </td>
                          <td className="whitespace-nowrap py-1.5 pr-3 text-xs">
                            {event.duration_ms != null
                              ? `${formatNumber(event.duration_ms)} ms`
                              : "-"}
                          </td>
                          <td className="max-w-[760px] py-1.5">
                            <div className="truncate font-mono text-xs" title={event.message}>
                              {event.message}
                            </div>
                          </td>
                        </tr>
                        {isOpen ? (
                          <tr className="border-b border-border/60 bg-muted/20">
                            <td colSpan={7} className="py-3 pl-4">
                              <div className="space-y-2 text-sm">
                                <div>
                                  <span className="text-muted-foreground">{t("pgLogs.labelSeverity")}</span>{" "}
                                  <span className="font-mono">{event.severity}</span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("pgLogs.labelUserDb")}</span>{" "}
                                  <span className="font-mono">
                                    {event.user || "-"} / {event.database || "-"}
                                  </span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("pgLogs.labelClientApp")}</span>{" "}
                                  <span className="font-mono">
                                    {event.client || "-"} / {event.app || "-"}
                                  </span>
                                </div>
                                <div>
                                  <span className="text-muted-foreground">{t("pgLogs.labelFullMsg")}</span>
                                  <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono">
                                    {event.message}
                                  </div>
                                </div>
                                {event.labels && Object.keys(event.labels).length ? (
                                  <div>
                                    <span className="text-muted-foreground">{t("pgLogs.labelExtra")}</span>
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
                        {t("pgLogs.emptyFilter")}
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
    </Page>
  )
}
