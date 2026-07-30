import { Fragment, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryService } from "@/api/discovery"
import {
  getPHPFPMLogs,
  getPHPFPMLogsMeta,
  type PHPFPMLogEvent,
  type PHPFPMLogsTypeMeta,
} from "@/api/phpfpm"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { LogPagination } from "@/components/shared/LogPagination"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import {
  dateTimeInputToSeconds,
  formatNumber,
  getNowSeconds,
  getStartSeconds,
  secondsToNanos,
} from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"
import { useTranslation, LangToggle } from "@/i18n/LanguageContext"

const LOG_TABS = [
  { key: "access", label: "Access Logs" },
  { key: "error", label: "Error Logs" },
  { key: "slow", label: "Slow Logs" },
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

function categoryBadge(event: PHPFPMLogEvent) {
  const key = event.category.toLowerCase()
  if (key === "warning") return "bg-amber-500/10 text-amber-600 dark:text-amber-400"
  if (key === "error") return "bg-destructive/10 text-destructive"
  if (key === "slow_request") return "bg-orange-500/10 text-orange-600 dark:text-orange-400"
  if (key === "startup") return "bg-teal-500/10 text-teal-600 dark:text-teal-300"
  if (key === "notice") return "bg-blue-500/10 text-blue-600 dark:text-blue-300"
  if (key === "access") return "bg-muted text-muted-foreground"
  return "bg-muted text-muted-foreground"
}

function parseSlowLogMessage(msg: string): { pid: string; script: string; backtrace: string[] } {
  const lines = msg.split("\n")
  const header = lines[0] || ""
  const pid = header.match(/pid\s+(\d+)/)?.[1] || ""
  const scriptLine = lines.find((l) => l.includes("script_filename ="))
  const script = scriptLine?.match(/script_filename\s*=\s*(\S+)/)?.[1] || ""
  const backtrace = lines.filter((l) => !l.includes("script_filename =") && l !== header).filter(Boolean)
  return { pid, script, backtrace }
}

export function PHPFPMLogsPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id?: string }>()

  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [logTab, setLogTab] = useState<(typeof LOG_TABS)[number]["key"]>("access")
  const [poolFilter, setPoolFilter] = useState("")
  const [methodFilter, setMethodFilter] = useState("")
  const [statusFilter, setStatusFilter] = useState("")
  const [minDuration, setMinDuration] = useState("")
  const [maxDuration, setMaxDuration] = useState("")
  const [minMemory, setMinMemory] = useState("")
  const [maxMemory, setMaxMemory] = useState("")
  const [minCPU, setMinCPU] = useState("")
  const [maxCPU, setMaxCPU] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)
  const [openEvent, setOpenEvent] = useState<number | null>(null)

  const serviceQuery = useQuery({
    queryKey: ["discovery-service", id],
    queryFn: () => getDiscoveryService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  const logsMetaQuery = useQuery<PHPFPMLogsTypeMeta>({
    queryKey: ["phpfpm-logs-meta", id, timeWindow.start, timeWindow.end],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getPHPFPMLogsMeta(id, {
      start: secondsToNanos(timeWindow.start),
      end: secondsToNanos(timeWindow.end),
    }),
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
  })

  const logsQuery = useQuery({
    queryKey: ["phpfpm-logs", id, timeWindow.start, timeWindow.end, logTab, poolFilter, methodFilter, statusFilter, minDuration, maxDuration, minMemory, maxMemory, minCPU, maxCPU, search, page],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () => getPHPFPMLogs(id, {
      start: secondsToNanos(timeWindow.start),
      end: secondsToNanos(timeWindow.end),
      log_file: logTab,
      pool: poolFilter,
      method: methodFilter,
      status_code: statusFilter,
      min_duration: minDuration,
      max_duration: maxDuration,
      min_memory: minMemory,
      max_memory: maxMemory,
      min_cpu: minCPU,
      max_cpu: maxCPU,
      search,
      page,
      page_size: PAGE_SIZE,
    }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
    refetchOnWindowFocus: false,
  })

  function handleTabChange(tab: (typeof LOG_TABS)[number]["key"]) {
    setLogTab(tab)
    setPage(1)
    setOpenEvent(null)
    setPoolFilter("")
    setMethodFilter("")
    setStatusFilter("")
    setMinDuration("")
    setMaxDuration("")
    setMinMemory("")
    setMaxMemory("")
    setMinCPU("")
    setMaxCPU("")
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

  const result = logsQuery.data
  const events = result?.events ?? []
  const logMeta = logsMetaQuery.data

  if (!id) return <div className="p-6 text-sm text-red-600">{t("shared.missingServiceId")}</div>

  // Extract unique pools from events for dropdown
  const availablePools = useMemo(() => {
    const s = new Set<string>()
    for (const ev of events) {
      if (ev.pool) s.add(ev.pool)
    }
    return Array.from(s).sort()
  }, [events])

  // Extract unique status codes from events for dropdown
  const availableStatuses = useMemo(() => {
    const s = new Set<string>()
    for (const ev of events) {
      if (ev.status_code) s.add(String(ev.status_code))
    }
    return Array.from(s).sort()
  }, [events])

  const METHODS = ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"]

  function renderLogRow(event: PHPFPMLogEvent, index: number) {
    const isOpen = openEvent === index
    const isAccess = logTab === "access"
    const isSlow = logTab === "slow"
    const colCount = isAccess ? 8 : isSlow ? 5 : 4
    const slowParsed = isSlow ? parseSlowLogMessage(event.message) : null

    return (
      <Fragment key={`${event.timestamp}-${index}`}>
        <tr className="cursor-pointer border-b border-border/60 hover:bg-muted/40" onClick={() => setOpenEvent(isOpen ? null : index)}>
          <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs text-muted-foreground">{event.timestamp}</td>
          {isAccess ? (
            <>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{event.pool || "-"}</td>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs font-semibold">{event.method || "-"}</td>
              <td className="max-w-[200px] truncate py-1.5 pr-3 font-mono text-xs" title={event.uri}>{event.uri || "-"}</td>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">
                <span className={cn("rounded px-1 py-0.5 text-xs",
                  event.status_code != null && event.status_code >= 500 ? "bg-destructive/15 text-destructive" :
                  event.status_code != null && event.status_code >= 400 ? "bg-amber-500/15 text-amber-600 dark:text-amber-400" :
                  event.status_code != null && event.status_code >= 300 ? "bg-blue-500/15 text-blue-600 dark:text-blue-300" :
                  "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400")}>
                  {event.status_code ?? "-"}
                </span>
              </td>
              <td className="py-1.5 pr-3 text-right font-mono text-xs tabular-nums">{event.duration_ms != null ? `${formatNumber(event.duration_ms, 1)}ms` : "-"}</td>
              <td className="py-1.5 pr-3 text-right font-mono text-xs tabular-nums">{event.memory_kb != null ? `${formatNumber(event.memory_kb, 0)}KB` : "-"}</td>
              <td className="py-1.5 text-right font-mono text-xs tabular-nums">{event.cpu != null ? `${formatNumber(event.cpu, 2)}%` : "-"}</td>
            </>
          ) : isSlow ? (
            <>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{slowParsed?.pid || "-"}</td>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{event.pool || "-"}</td>
              <td className="max-w-[300px] truncate py-1.5 pr-3 font-mono text-xs" title={slowParsed?.script}>{slowParsed?.script || "-"}</td>
              <td className="py-1.5 font-mono text-xs text-muted-foreground">
                {slowParsed && slowParsed.backtrace.length > 0
                  ? `${slowParsed.backtrace.length} frame(s)`
                  : event.message.split("\n")[0]?.slice(0, 60) || "-"}
              </td>
            </>
          ) : (
            <>
              <td className="whitespace-nowrap py-1.5 pr-3">
                <span className={cn("rounded-full px-1.5 py-0.5 text-xs", categoryBadge(event))}>{event.category}</span>
              </td>
              <td className="whitespace-nowrap py-1.5 pr-3 font-mono text-xs">{event.pool || "-"}</td>
              <td className="max-w-[600px] py-1.5">
                <div className="truncate font-mono text-xs" title={event.message}>{event.message}</div>
              </td>
            </>
          )}
        </tr>
        {isOpen && (
          <tr className="border-b border-border/60 bg-muted/20">
            <td colSpan={colCount} className="py-3 pl-4">
              <div className="space-y-2 text-sm">
                {isAccess && (
                  <>
                    <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                      <div><span className="text-muted-foreground">Script: </span><span className="font-mono">{event.script || "-"}</span></div>
                      <div><span className="text-muted-foreground">CPU: </span><span className="font-mono">{event.cpu != null ? `${formatNumber(event.cpu, 2)}%` : "-"}</span></div>
                      <div><span className="text-muted-foreground">Severity: </span><span className="font-mono">{event.severity}</span></div>
                    </div>
                    <div>
                      <span className="text-muted-foreground">Full Line: </span>
                      <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono text-sm">{event.message}</div>
                    </div>
                  </>
                )}
                {isSlow && (
                  <>
                    <div className="grid gap-2 sm:grid-cols-2">
                      <div><span className="text-muted-foreground">PID: </span><span className="font-mono">{slowParsed?.pid || "-"}</span></div>
                      <div><span className="text-muted-foreground">Script: </span><span className="font-mono">{slowParsed?.script || "-"}</span></div>
                    </div>
                    {slowParsed && slowParsed.backtrace.length > 0 && (
                      <div>
                        <span className="text-muted-foreground">Stack Trace:</span>
                        <div className="mt-1 space-y-0.5 rounded border border-border bg-background p-2 font-mono text-sm">
                          {slowParsed.backtrace.map((frame, fi) => (
                            <div key={fi} className="break-all text-muted-foreground">{frame}</div>
                          ))}
                        </div>
                      </div>
                    )}
                    <div>
                      <span className="text-muted-foreground">Raw Entry:</span>
                      <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono text-sm">{event.message}</div>
                    </div>
                  </>
                )}
                {!isAccess && !isSlow && (
                  <div>
                    <span className="text-muted-foreground">Full Message:</span>
                    <div className="mt-1 whitespace-pre-wrap break-all rounded border border-border bg-background p-2 font-mono">{event.message}</div>
                  </div>
                )}
                {event.labels && Object.keys(event.labels).length > 0 && (
                  <div>
                    <span className="text-muted-foreground">Metadata:</span>
                    <div className="mt-1 grid gap-1 sm:grid-cols-2">
                      {Object.entries(event.labels).map(([key, value]) => (
                        <div key={key} className="font-mono">
                          <span className="text-muted-foreground">{key}:</span> {value}
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </td>
          </tr>
        )}
      </Fragment>
    )
  }

  return (
    <Page className="max-w-none">
      <PageHeader
        title={t("phpfpmLogs.title")}
        description={t("phpfpmLogs.description")}
        actions={
          <div className="flex flex-wrap items-center gap-3">
            <TimeRangeSelector selected={timeSelection} onChange={handleTimeChange} />
            <LangToggle />
            <button type="button" onClick={() => logsQuery.refetch()}
              className="rounded-md border border-border bg-background p-2 text-foreground hover:bg-muted" title="Refresh">
              <RefreshCw className={cn('h-4 w-4', logsQuery.isRefetching && 'animate-spin')} />
            </button>
            <Link to={`/services/phpfpm-dashboard/${id}`}
              className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              {t("shared.dashboard")}
            </Link>
          </div>
        }
      />

      {timeWindow.error && (
        <SectionCard title={t("phpfpmLogs.invalidTime")}>
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      )}

      {/* ── Log file type tabs ── */}
      <div className="mt-4 flex gap-1 rounded-lg border border-border bg-card p-1">
        {LOG_TABS.map((tab) => {
          const count = tab.key === "access" ? logMeta?.access_count : tab.key === "error" ? logMeta?.error_count : logMeta?.slow_count
          return (
            <button key={tab.key} type="button" onClick={() => handleTabChange(tab.key)}
              className={cn("rounded-md px-3 py-1.5 text-xs font-medium transition-colors",
                logTab === tab.key ? "bg-foreground text-background" : "text-muted-foreground hover:bg-muted")}>
              {tab.label}
              {count != null && <span className="ml-1.5 text-xs opacity-60">({formatNumber(count, 0)})</span>}
            </button>
          )
        })}
      </div>

      {/* ── Filters ── */}
      <SectionCard title="">
        <div className="space-y-3">
          {/* Row 1: Dropdown filters + search */}
          <div className="flex flex-wrap items-center gap-2">
            <select value={poolFilter} onChange={(e) => { setPoolFilter(e.target.value); setPage(1) }}
              className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none min-w-[100px]">
              <option value="">All Pools</option>
              {availablePools.map((p) => <option key={p} value={p}>{p}</option>)}
            </select>
            {logTab === "access" && (
              <>
                <select value={methodFilter} onChange={(e) => { setMethodFilter(e.target.value); setPage(1) }}
                  className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none min-w-[80px]">
                  <option value="">All Methods</option>
                  {METHODS.map((m) => <option key={m} value={m}>{m}</option>)}
                </select>
                <select value={statusFilter} onChange={(e) => { setStatusFilter(e.target.value); setPage(1) }}
                  className="h-8 rounded-md border border-input bg-background/70 px-2 text-xs outline-none min-w-[100px]">
                  <option value="">All Status</option>
                  <option value="2">2xx</option>
                  <option value="3">3xx</option>
                  <option value="4">4xx</option>
                  <option value="5">5xx</option>
                  {availableStatuses.map((s) => <option key={s} value={s}>{s}</option>)}
                </select>
              </>
            )}
            <input className="h-8 flex-1 rounded-md border border-input bg-background/70 px-2 text-xs outline-none min-w-[150px]"
              value={searchInput} onChange={(e) => setSearchInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && commitSearch()} onBlur={commitSearch}
              placeholder="Search logs..." />
            <button type="button" onClick={commitSearch}
              className="h-8 rounded-md border border-border px-2 text-xs hover:bg-muted">
              {t("shared.search")}
            </button>
            <button type="button" onClick={() => {
              setPoolFilter(""); setMethodFilter(""); setStatusFilter(""); setSearch(""); setSearchInput("")
              setMinDuration(""); setMaxDuration(""); setMinMemory(""); setMaxMemory(""); setMinCPU(""); setMaxCPU("")
              setPage(1)
            }} className="h-8 rounded-md border border-border px-2 text-xs hover:bg-muted">
              Clear
            </button>
          </div>
          {/* Row 2: Numeric filters (access only) */}
          {logTab === "access" && (
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm text-muted-foreground">Duration:</span>
              <input type="number" value={minDuration} onChange={(e) => setMinDuration(e.target.value)} placeholder="min ms"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
              <span className="text-sm text-muted-foreground">-</span>
              <input type="number" value={maxDuration} onChange={(e) => setMaxDuration(e.target.value)} placeholder="max ms"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
              <span className="ml-2 text-sm text-muted-foreground">Memory:</span>
              <input type="number" value={minMemory} onChange={(e) => setMinMemory(e.target.value)} placeholder="min KB"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
              <span className="text-sm text-muted-foreground">-</span>
              <input type="number" value={maxMemory} onChange={(e) => setMaxMemory(e.target.value)} placeholder="max KB"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
              <span className="ml-2 text-sm text-muted-foreground">CPU:</span>
              <input type="number" value={minCPU} onChange={(e) => setMinCPU(e.target.value)} placeholder="min %"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
              <span className="text-sm text-muted-foreground">-</span>
              <input type="number" value={maxCPU} onChange={(e) => setMaxCPU(e.target.value)} placeholder="max %"
                className="h-8 w-20 rounded border border-input bg-background/70 px-2 text-sm outline-none" />
            </div>
          )}
        </div>
      </SectionCard>

      <SectionCard title={t("phpfpmLogs.eventsTitle")}
        description={logsQuery.isFetching ? t("shared.loading") : result ? t("phpfpmLogs.totalEvents", formatNumber(result.total, 0)) : ""}>
        {logsQuery.isError ? (
          <div className="py-6 text-center text-sm text-red-500">{t("phpfpmLogs.loadError")}</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-sm uppercase text-muted-foreground">
                  {logTab === "access" ? (
                    <tr className="border-b border-border">
                      <th className="py-1.5 pr-3 text-left text-xs">Time</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Pool</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Method</th>
                      <th className="py-1.5 pr-3 text-left text-xs">URI</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Status</th>
                      <th className="py-1.5 pr-3 text-right text-xs">Duration</th>
                      <th className="py-1.5 pr-3 text-right text-xs">Memory</th>
                      <th className="py-1.5 text-right text-xs">CPU</th>
                    </tr>
                  ) : logTab === "slow" ? (
                    <tr className="border-b border-border">
                      <th className="py-1.5 pr-3 text-left text-xs">Time</th>
                      <th className="py-1.5 pr-3 text-left text-xs">PID</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Pool</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Script</th>
                      <th className="py-1.5 text-left text-xs">Frames</th>
                    </tr>
                  ) : (
                    <tr className="border-b border-border">
                      <th className="py-1.5 pr-3 text-left text-xs">Time</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Level</th>
                      <th className="py-1.5 pr-3 text-left text-xs">Pool</th>
                      <th className="py-1.5 text-left text-xs">Message</th>
                    </tr>
                  )}
                </thead>
                <tbody>
                  {events.map((event, index) => renderLogRow(event, index))}
                  {!logsQuery.isFetching && events.length === 0 && (
                    <tr><td colSpan={logTab === "access" ? 8 : logTab === "slow" ? 5 : 4} className="py-8 text-center text-sm text-muted-foreground">{t("phpfpmLogs.emptyFilter")}</td></tr>
                  )}
                  {logsQuery.isFetching && events.length === 0 && (
                    <tr><td colSpan={logTab === "access" ? 8 : logTab === "slow" ? 5 : 4} className="py-8 text-center text-sm text-muted-foreground">{t("shared.loading")}</td></tr>
                  )}
                </tbody>
              </table>
            </div>
            {result && result.total > 0 && (
              <LogPagination page={result.page} total={result.total} pageSize={result.page_size}
                hasMore={result.has_more}
                onChange={(p) => { setPage(p); setOpenEvent(null); window.scrollTo({ top: 0, behavior: "smooth" }) }} />
            )}
          </>
        )}
      </SectionCard>
    </Page>
  )
}
