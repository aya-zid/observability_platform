import { useCallback, useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { Activity, FileText, RefreshCw, Server, Sparkles } from "lucide-react"

import { getOverview, type OverviewResponse } from "@/api/overview"
import { Button } from "@/components/ui/button"
import { GlobalHealthStrip } from "./components/GlobalHealthStrip"
import { HostCard } from "./components/HostCard"
import { IncidentCards } from "./components/IncidentCards"
import { IncidentTimeline } from "./components/IncidentTimeline"

function SectionShell({
  title,
  eyebrow,
  icon: Icon,
  children,
  action,
}: {
  title: string
  eyebrow?: string
  icon: typeof Activity
  children: ReactNode
  action?: ReactNode
}) {
  return (
    <section className="rounded-lg border border-border bg-card shadow-card">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-2">
        <div className="flex min-w-0 items-center gap-2">
          <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border bg-muted/35 text-primary">
            <Icon className="h-3 w-3" />
          </span>
          <div className="min-w-0">
            <h2 className="truncate text-xs font-semibold text-foreground">{title}</h2>
            {eyebrow ? <p className="mt-0.5 text-[11px] text-muted-foreground">{eyebrow}</p> : null}
          </div>
        </div>
        {action}
      </div>
      <div className="p-3">{children}</div>
    </section>
  )
}

function SkeletonGrid({ count, className }: { count: number; className?: string }) {
  return (
    <div className={className}>
      {Array.from({ length: count }).map((_, index) => (
        <div key={index} className="h-40 rounded-lg border border-border bg-muted/30 animate-pulse" />
      ))}
    </div>
  )
}

export default function OverviewPage() {
  const [timelineRange, setTimelineRange] = useState<{ start: number; end: number }>(() => {
    const now = Date.now()
    return { start: now - 5 * 60 * 1000, end: now }
  })

  const q = useQuery({
    queryKey: ["overview", timelineRange.start, timelineRange.end],
    queryFn: () => getOverview(timelineRange.start, timelineRange.end),
    refetchInterval: 30_000,
    refetchOnWindowFocus: false,
    placeholderData: (prev: OverviewResponse | undefined) => prev,
  })

  const data = q.data
  const loading = q.isLoading && !data
  const hostCount = data?.hosts?.length ?? 0
  const hasTimelineEvents = (data?.timeline?.events?.length ?? 0) > 0

  const handleTimeRangeChange = useCallback((start: number, end: number) => {
    setTimelineRange({ start, end })
  }, [])

  return (
    <div className="animate-fade-in">
      <header className="relative border-b border-border bg-card/50 backdrop-blur-sm px-6 py-3.5 overflow-hidden -mx-4 md:-mx-6">
        <div className="absolute inset-0 gradient-hero pointer-events-none" />
        <div className="absolute inset-0 bg-grid opacity-40 pointer-events-none" />
        <div className="relative flex items-end justify-between gap-4 flex-wrap">
          <div>
            <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-[0.2em] text-muted-foreground font-semibold mb-1">
              <span className="w-1 h-1 rounded-full bg-accent shadow-[0_0_6px_hsl(var(--accent))]" />
              Medianet Observability
            </div>
            <h1 className="text-xl font-bold tracking-tight">
              Infrastructure <span className="text-primary">Overview</span>
            </h1>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-[11px] text-muted-foreground">Auto-refresh 30s</span>
            <Button variant="outline" size="sm" disabled>
              <FileText className="h-3.5 w-3.5" />
              Report
            </Button>
            <Button variant="outline" size="sm" onClick={() => q.refetch()} disabled={q.isRefetching}>
              <RefreshCw className={`h-3.5 w-3.5 ${q.isRefetching ? "animate-spin" : ""}`} />
              Refresh
            </Button>
          </div>
        </div>
      </header>

      <div className="px-4 md:px-6 pb-6 space-y-4 pt-4">
        <SectionShell
          title="Fleet Health"
          eyebrow="Live operational posture from agents, service status, containers, alerts, and CVEs."
          icon={Activity}
        >
          {loading ? (
            <SkeletonGrid count={5} className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5" />
          ) : data ? (
            <GlobalHealthStrip data={data.global} />
          ) : (
            <div className="rounded-lg border border-dashed border-border bg-muted/20 p-6 text-center text-sm text-muted-foreground">
              Unable to load fleet health data.
            </div>
          )}
        </SectionShell>

        <SectionShell
          title="Incident Correlation"
          eyebrow={`${data?.incidents?.length ?? 0} active incident${data?.incidents?.length === 1 ? "" : "s"} across ${hostCount} host${hostCount === 1 ? "" : "s"}. Data refreshes every 30s.`}
          icon={Sparkles}
        >
          {loading ? (
            <SkeletonGrid count={2} className="grid gap-3 md:grid-cols-2" />
          ) : (
            <IncidentCards incidents={data?.incidents} />
          )}
        </SectionShell>

        <SectionShell
          title="Infrastructure Snapshot"
          eyebrow="Per-host service inventory, heartbeat freshness, and resource pressure."
          icon={Server}
        >
          {loading ? (
            <SkeletonGrid count={3} className="grid gap-3 md:grid-cols-2 xl:grid-cols-3" />
          ) : data && (data.hosts ?? []).length > 0 ? (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {data.hosts.map((host) => (
                <HostCard key={host.machine_id} host={host} />
              ))}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-border bg-muted/20 p-6 text-center">
              <p className="text-sm font-medium text-foreground">No hosts discovered yet</p>
              <p className="mt-1 text-sm text-muted-foreground">Install the agent on a machine and this section will become your live infrastructure inventory.</p>
            </div>
          )}
        </SectionShell>

        <SectionShell
          title="Recent Activity"
          eyebrow={hasTimelineEvents ? "Filtered log events from the selected time window." : "A quiet timeline is a useful signal too."}
          icon={Activity}
        >
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 5 }).map((_, index) => (
                <div key={index} className="h-14 rounded-lg border border-border bg-muted/30 animate-pulse" />
              ))}
            </div>
          ) : (
            <IncidentTimeline events={data?.timeline?.events ?? []} onTimeRangeChange={handleTimeRangeChange} />
          )}
        </SectionShell>
      </div>
    </div>
  )
}
