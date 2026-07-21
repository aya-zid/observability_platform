import { useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getContainerService, getContainerLogs } from "@/api/container"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { TimeRangeSelector, type TimeSelection, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import { LogPagination } from "@/components/shared/LogPagination"
import { dateTimeInputToSeconds, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"
import { RefreshCw } from "lucide-react"

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

export function ContainerLogsPage() {
  const { id = "" } = useParams<{ id?: string }>()

  const [timeSelection, setTimeSelection] = useState<TimeSelection>({ mode: "preset", preset: TIME_RANGES[1] })
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [page, setPage] = useState(1)

  const serviceQuery = useQuery({
    queryKey: ["discovery-container", id],
    queryFn: () => getContainerService(id),
    enabled: !!id,
  })

  const timeWindow = useMemo(() => getTimeWindow(timeSelection), [timeSelection])

  function commitSearch() {
    setSearch(searchInput)
    setPage(1)
  }

  const logsQuery = useQuery({
    queryKey: ["container-logs", id, timeWindow.start, timeWindow.end, search, page],
    enabled: !!id && !!serviceQuery.data && !timeWindow.error,
    queryFn: () =>
      getContainerLogs(id, {
        start: secondsToNanos(timeWindow.start),
        end: secondsToNanos(timeWindow.end),
        search,
        page,
        page_size: PAGE_SIZE,
      }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: timeWindow.isManual ? false : 30_000,
    refetchOnWindowFocus: false,
  })

  function handleTimeChange(value: TimeSelection) {
    setTimeSelection(value)
    setPage(1)
  }

  const result = logsQuery.data
  const events = result?.events ?? []

  if (!id) return <div className="p-6 text-sm text-red-600">Missing container ID.</div>

  return (
    <Page className="max-w-none">
      <PageHeader
        title="Container Logs"
        description={serviceQuery.data ? `Logs for ${serviceQuery.data.container_name}` : ""}
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
            <Link to={`/containers/${id}/dashboard`} className="rounded-md border border-border px-3 py-2 text-sm font-medium hover:bg-muted">
              Dashboard
            </Link>
          </div>
        }
      />

      {timeWindow.error ? (
        <SectionCard title="Invalid Time">
          <div className="text-sm text-red-600">{timeWindow.error}</div>
        </SectionCard>
      ) : null}

      <SectionCard title="Filters">
        <div className="grid gap-3 lg:grid-cols-[1fr_auto]">
          <input
            className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && commitSearch()}
            onBlur={commitSearch}
            placeholder="Search log lines..."
          />
          <button
            type="button"
            onClick={commitSearch}
            className="h-10 rounded-md border border-border px-4 text-sm hover:bg-muted"
          >
            Search
          </button>
        </div>
      </SectionCard>

      <SectionCard
        title="Logs"
        description={
          logsQuery.isFetching
            ? "Loading..."
            : result
              ? `${result.total.toLocaleString()} lines`
              : ""
        }
      >
        {logsQuery.isError ? (
          <div className="py-6 text-center text-sm text-red-500">Failed to load logs.</div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="text-xs uppercase text-muted-foreground">
                  <tr className="border-b border-border">
                    <th className="py-2 pr-4 text-left w-44">Timestamp</th>
                    <th className="py-2 text-left">Message</th>
                  </tr>
                </thead>
                <tbody>
                  {events.map((event, index) => (
                    <tr key={`${event.timestamp}-${index}`} className="border-b border-border/60 hover:bg-muted/20">
                      <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs text-muted-foreground align-top">
                        {formatTimestamp(event.timestamp)}
                      </td>
                      <td className="py-2">
                        <div className="font-mono text-xs whitespace-pre-wrap break-all">{event.line}</div>
                      </td>
                    </tr>
                  ))}
                  {!logsQuery.isFetching && events.length === 0 ? (
                    <tr><td colSpan={2} className="py-8 text-center text-sm text-muted-foreground">No log lines found.</td></tr>
                  ) : null}
                  {logsQuery.isFetching && events.length === 0 ? (
                    <tr><td colSpan={2} className="py-8 text-center text-sm text-muted-foreground">Loading...</td></tr>
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
                onChange={(p) => { setPage(p); window.scrollTo({ top: 0, behavior: "smooth" }) }}
              />
            ) : null}
          </>
        )}
      </SectionCard>
    </Page>
  )
}

function formatTimestamp(ts: string): string {
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    return d.toLocaleString()
  } catch {
    return ts
  }
}
