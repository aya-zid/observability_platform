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

export function IncidentTimeline({
  events,
  hostname,
}: {
  events: TimelineEvent[] | null | undefined
  hostname?: string
}) {
  const safe = events ?? []
  const filtered = hostname
    ? safe.filter((e) => e.hostname === hostname)
    : safe

  if (filtered.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
        No recent log events in the selected window
      </div>
    )
  }

  return (
    <div className="relative">
      {/* Vertical line */}
      <div className="absolute left-[7px] top-2 bottom-2 w-0.5 bg-border" />

      <div className="space-y-2">
        {filtered.map((ev, i) => {
          const cfg = typeConfig[ev.event_type] ?? typeConfig.warning
          const ts = new Date(ev.timestamp).toLocaleTimeString()

          return (
            <div key={i} className="relative flex gap-3 pl-5">
              {/* Dot on timeline */}
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
  )
}
