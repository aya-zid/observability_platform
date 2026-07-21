import { useState, useMemo } from "react"
import { Filter, X } from "lucide-react"

import type { TimelineEvent } from "@/api/overview"
import { cn } from "@/lib/utils"

const typeConfig: Record<string, { dot: string; bg: string; label: string }> = {
  error: {
    dot: "bg-red-500",
    bg: "bg-red-100 dark:bg-red-900/30",
    label: "error",
  },
  warning: {
    dot: "bg-amber-500",
    bg: "bg-amber-100 dark:bg-amber-900/30",
    label: "warning",
  },
  finding: {
    dot: "bg-purple-500",
    bg: "bg-purple-100 dark:bg-purple-900/30",
    label: "finding",
  },
}

type TimeRange = "5m" | "15m" | "1h" | "6h"

const RANGE_MS: Record<TimeRange, number> = {
  "5m": 5 * 60 * 1000,
  "15m": 15 * 60 * 1000,
  "1h": 60 * 60 * 1000,
  "6h": 6 * 60 * 60 * 1000,
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

  const safe = events ?? []

  const hostnames = useMemo(() => {
    const set = new Set<string>()
    for (const e of safe) if (e.hostname) set.add(e.hostname)
    return Array.from(set).sort()
  }, [safe])

  const types = useMemo(() => {
    const set = new Set<string>()
    for (const e of safe) if (e.event_type) set.add(e.event_type)
    return Array.from(set).sort()
  }, [safe])

  const filtered = useMemo(() => {
    let result = safe
    const now = Date.now()
    const rangeMs = RANGE_MS[timeRange]
    const cutoff = now - rangeMs

    result = result.filter((e) => {
      const ts = new Date(e.timestamp).getTime()
      return ts >= cutoff
    })

    if (hostFilter) {
      result = result.filter((e) => e.hostname === hostFilter)
    }
    if (typeFilter) {
      result = result.filter((e) => e.event_type === typeFilter)
    }

    // Update parent with the effective time window (last 5m by default, or selected range)
    if (onTimeRangeChange) {
      // The parent uses the selected time range for the API call
    }

    return result
  }, [safe, timeRange, hostFilter, typeFilter, onTimeRangeChange])

  const handleRangeChange = (range: TimeRange) => {
    setTimeRange(range)
    const now = Date.now()
    const rangeMs = RANGE_MS[range]
    if (onTimeRangeChange) {
      onTimeRangeChange(now - rangeMs, now)
    }
  }

  if (safe.length === 0 && !hostFilter) {
    return (
      <div className="space-y-3">
        <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
          No recent log events in the selected window
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {/* Filters bar */}
      <div className="flex flex-wrap items-center gap-2">
        {/* Time range selector */}
        <div className="flex items-center gap-1 rounded-md border border-border bg-card p-0.5">
          {(["5m", "15m", "1h", "6h"] as TimeRange[]).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => handleRangeChange(r)}
              className={cn(
                "rounded px-2 py-0.5 text-xs font-medium transition-colors",
                timeRange === r
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              {r}
            </button>
          ))}
        </div>

        <Filter className="h-3.5 w-3.5 text-muted-foreground" />

        {/* Host filter */}
        {hostnames.length > 0 ? (
          <select
            value={hostFilter}
            onChange={(e) => setHostFilter(e.target.value)}
            className="h-7 rounded-md border border-border bg-card px-2 text-xs text-muted-foreground"
          >
            <option value="">All hosts</option>
            {hostnames.map((h) => (
              <option key={h} value={h}>{h}</option>
            ))}
          </select>
        ) : null}

        {/* Type filter */}
        {types.length > 0 ? (
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="h-7 rounded-md border border-border bg-card px-2 text-xs text-muted-foreground"
          >
            <option value="">All types</option>
            {types.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>
        ) : null}

        {/* Clear filters */}
        {(hostFilter || typeFilter) ? (
          <button
            type="button"
            onClick={() => { setHostFilter(""); setTypeFilter("") }}
            className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground"
          >
            <X className="h-3 w-3" />
            Clear
          </button>
        ) : null}
      </div>

      {/* Timeline */}
      {filtered.length === 0 ? (
        <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
          No matching events in the selected window
        </div>
      ) : (
        <div className="relative">
          <div className="absolute left-[7px] top-2 bottom-2 w-0.5 bg-border" />

          <div className="space-y-2">
            {filtered.map((ev, i) => {
              const cfg = typeConfig[ev.event_type] ?? typeConfig.warning
              const ts = new Date(ev.timestamp).toLocaleTimeString()

              return (
                <div key={i} className="relative flex gap-3 pl-5">
                  <span
                    className={cn(
                      "absolute left-0 top-1.5 z-10 inline-block h-3.5 w-3.5 rounded-full border-2 border-background",
                      cfg.dot
                    )}
                  />

                  <div className="min-w-0 flex-1 rounded-md border border-border bg-card/50 px-3 py-1.5">
                    <div className="flex items-center gap-2 text-xs">
                      <span className="font-medium">{ts}</span>
                      <span className={cn("rounded px-1 py-0.5 text-[10px] uppercase", cfg.bg, cfg.dot.replace("bg-", "text-"))}>
                        {cfg.label}
                      </span>
                      {ev.hostname ? (
                        <>
                          <span className="text-muted-foreground">·</span>
                          <span className="text-muted-foreground">{ev.hostname}</span>
                        </>
                      ) : null}
                      {ev.service ? (
                        <>
                          <span className="text-muted-foreground">·</span>
                          <span className="text-muted-foreground">{ev.service}</span>
                        </>
                      ) : null}
                    </div>
                    <p className="mt-0.5 text-xs text-muted-foreground line-clamp-1">
                      {ev.message}
                    </p>
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
