import { useEffect, useMemo, useRef, useState } from "react"
import { AlertCircle, AlertTriangle, Search, ShieldAlert, X } from "lucide-react"

import type { TimelineEvent } from "@/api/overview"
import { cn } from "@/lib/utils"

const typeConfig: Record<string, { dot: string; badge: string; label: string; icon: typeof AlertCircle }> = {
  error: {
    dot: "bg-red-500",
    badge: "border-red-500/25 bg-red-500/[0.06] text-red-600 dark:text-red-400",
    label: "Error",
    icon: AlertCircle,
  },
  warning: {
    dot: "bg-amber-500",
    badge: "border-amber-500/25 bg-amber-500/[0.06] text-amber-600 dark:text-amber-400",
    label: "Warning",
    icon: AlertTriangle,
  },
  finding: {
    dot: "bg-violet-500",
    badge: "border-violet-500/25 bg-violet-500/[0.06] text-violet-600 dark:text-violet-400",
    label: "Finding",
    icon: ShieldAlert,
  },
}

type TimeRange = "5m" | "15m" | "1h" | "6h"

const RANGE_MS: Record<TimeRange, number> = {
  "5m": 5 * 60 * 1000,
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
}

function formatTime(value: string) {
  const date = new Date(value)
  if (!Number.isFinite(date.getTime())) return "unknown"
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
}

// Strip a leading ts=… field (the log-level timestamp) since the row already
// shows its own timestamp above the message.
function stripLeadingLogTimestamp(message: string): string {
  return message.replace(/^\s*ts="?[^\s"]+"?\s*/i, "")
}

function uniqueSorted(values: string[]) {
  return Array.from(new Set(values.filter(Boolean))).sort((a, b) => a.localeCompare(b))
}

function TimelineMessage({
  message,
  expanded,
  onToggle,
}: {
  message: string
  expanded: boolean
  onToggle: () => void
}) {
  const ref = useRef<HTMLParagraphElement>(null)
  const [overflows, setOverflows] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    const check = () => setOverflows(el.scrollHeight > el.clientHeight + 1)
    check()
    const observer = new ResizeObserver(check)
    observer.observe(el)
    return () => observer.disconnect()
  }, [expanded, message])

  return (
    <div>
      <p
        ref={ref}
        className={cn(
          "mt-0.5 whitespace-pre-wrap text-xs leading-4 text-muted-foreground",
          !expanded && "line-clamp-2"
        )}
      >
        {message}
      </p>
      {overflows ? (
        <button
          type="button"
          onClick={onToggle}
          className="mt-1 text-xs font-medium text-primary hover:underline"
        >
          {expanded ? "Show less" : "Show more"}
        </button>
      ) : null}
    </div>
  )
}

export function IncidentTimeline({
  events,
  hostname: externalHostFilter,
  onTimeRangeChange,
}: {
  events: TimelineEvent[] | null | undefined
  hostname?: string
  onTimeRangeChange?: (start: number, end: number) => void
}) {
  const [timeRange, setTimeRange] = useState<TimeRange>("5m")
  const [hostFilter, setHostFilter] = useState<string>(externalHostFilter ?? "")
  const [typeFilter, setTypeFilter] = useState<string>("")
  const [query, setQuery] = useState("")
  const [expandedKey, setExpandedKey] = useState<string | null>(null)

  const safe = events ?? []
  const hostnames = useMemo(() => uniqueSorted(safe.map((event) => event.hostname)), [safe])
  const types = useMemo(() => uniqueSorted(safe.map((event) => event.event_type)), [safe])

  const filtered = useMemo(() => {
    const cutoff = Date.now() - RANGE_MS[timeRange]
    const q = query.trim().toLowerCase()
    return safe.filter((event) => {
      const ts = new Date(event.timestamp).getTime()
      if (Number.isFinite(ts) && ts < cutoff) return false
      if (hostFilter && event.hostname !== hostFilter) return false
      if (typeFilter && event.event_type !== typeFilter) return false
      if (q) {
        const haystack = `${event.hostname} ${event.service} ${event.event_type} ${event.message}`.toLowerCase()
        if (!haystack.includes(q)) return false
      }
      return true
    })
  }, [safe, timeRange, hostFilter, typeFilter, query])

  const counts = useMemo(() => {
    return filtered.reduce(
      (acc, event) => {
        if (event.event_type === "error") acc.errors += 1
        else if (event.event_type === "warning") acc.warnings += 1
        else acc.other += 1
        return acc
      },
      { errors: 0, warnings: 0, other: 0 }
    )
  }, [filtered])

  const handleRangeChange = (range: TimeRange) => {
    setTimeRange(range)
    const now = Date.now()
    onTimeRangeChange?.(now - RANGE_MS[range], now)
  }

  const clearFilters = () => {
    setHostFilter("")
    setTypeFilter("")
    setQuery("")
  }

  return (
    <div className="space-y-3">
      <div className="grid gap-2 sm:grid-cols-3">
        <div className="rounded-lg border border-border bg-muted/20 p-2.5">
          <div className="text-base font-semibold tabular-nums text-foreground">{filtered.length}</div>
          <div className="text-xs text-muted-foreground">matching events</div>
        </div>
        <div className="rounded-lg border border-red-500/20 bg-red-500/[0.04] p-2.5">
          <div className="text-base font-semibold tabular-nums text-red-500">{counts.errors}</div>
          <div className="text-xs text-muted-foreground">errors</div>
        </div>
        <div className="rounded-lg border border-amber-500/20 bg-amber-500/[0.04] p-2.5">
          <div className="text-base font-semibold tabular-nums text-amber-500">{counts.warnings}</div>
          <div className="text-xs text-muted-foreground">warnings</div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        <div className="flex items-center gap-0.5 rounded-md border border-border bg-card p-0.5">
          {(["5m", "15m", "1h", "6h"] as TimeRange[]).map((range) => (
            <button
              key={range}
              type="button"
              onClick={() => handleRangeChange(range)}
              className={cn(
                "h-6 rounded px-2 text-xs font-medium transition-colors",
                timeRange === range ? "bg-accent text-accent-foreground" : "text-muted-foreground hover:text-foreground"
              )}
            >
              {range}
            </button>
          ))}
        </div>

        <div className="relative min-w-[160px] flex-1 sm:max-w-[200px]">
          <Search className="pointer-events-none absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-muted-foreground" />
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search activity"
            className="h-7 w-full rounded-md border border-border bg-card pl-7 pr-2 text-xs outline-none transition-colors placeholder:text-muted-foreground focus:border-primary/50"
          />
        </div>

        {hostnames.length > 0 ? (
          <select
            value={hostFilter}
            onChange={(event) => setHostFilter(event.target.value)}
            className="h-7 rounded-md border border-border bg-card px-1.5 text-xs text-muted-foreground outline-none"
          >
            <option value="">All hosts</option>
            {hostnames.map((hostname) => (
              <option key={hostname} value={hostname}>{hostname}</option>
            ))}
          </select>
        ) : null}

        {types.length > 0 ? (
          <select
            value={typeFilter}
            onChange={(event) => setTypeFilter(event.target.value)}
            className="h-7 rounded-md border border-border bg-card px-1.5 text-xs text-muted-foreground outline-none"
          >
            <option value="">All types</option>
            {types.map((type) => (
              <option key={type} value={type}>{type}</option>
            ))}
          </select>
        ) : null}

        {hostFilter || typeFilter || query ? (
          <button
            type="button"
            onClick={clearFilters}
            className="inline-flex h-7 items-center gap-1 rounded-md px-1.5 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        ) : null}
      </div>

      {filtered.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-muted/20 p-6 text-center">
          <p className="text-xs font-medium text-foreground">No activity matches this view</p>
          <p className="mt-1 text-xs text-muted-foreground">Try a wider time range or clear the filters.</p>
        </div>
      ) : (
        <div className="relative max-h-[380px] overflow-y-auto pr-1">
          <div className="absolute bottom-2 left-[7px] top-2 w-px bg-border" />
          <div className="space-y-1.5">
            {filtered.map((event, index) => {
              const cfg = typeConfig[event.event_type] ?? typeConfig.warning
              const Icon = cfg.icon
              const key = `${event.timestamp}-${event.hostname}-${index}`
              const expanded = expandedKey === key
              const displayMessage = stripLeadingLogTimestamp(event.message)
              return (
                <div key={key} className="relative flex gap-2 pl-5">
                  <span className={cn("absolute left-0 top-2.5 z-10 h-3 w-3 rounded-full border-2 border-background", cfg.dot)} />
                  <div className="min-w-0 flex-1 rounded-md border border-border bg-card px-2.5 py-1.5 transition-colors hover:bg-muted/20">
                    <div className="flex flex-wrap items-center gap-1.5 text-xs">
                      <span className="font-semibold tabular-nums text-foreground">{formatTime(event.timestamp)}</span>
                      <span className={cn("inline-flex items-center gap-1 rounded-md border px-1 py-0.5 text-[11px] font-semibold uppercase tracking-wide", cfg.badge)}>
                        <Icon className="h-2.5 w-2.5" />
                        {cfg.label}
                      </span>
                      {event.hostname ? <span className="text-muted-foreground">{event.hostname}</span> : null}
                      {event.service ? <span className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">{event.service}</span> : null}
                    </div>
                    <TimelineMessage
                      message={displayMessage}
                      expanded={expanded}
                      onToggle={() => setExpandedKey(expanded ? null : key)}
                    />
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
