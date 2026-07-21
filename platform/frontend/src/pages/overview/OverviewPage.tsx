import { useState, useCallback } from "react"
import { useQuery } from "@tanstack/react-query"
import { RefreshCw } from "lucide-react"

import { getOverview } from "@/api/overview"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { GlobalHealthStrip } from "./components/GlobalHealthStrip"
import { HostCard } from "./components/HostCard"
import { DiagnosedIssues } from "./components/DiagnosedIssues"
import { CorrelationFindings } from "./components/CorrelationFindings"
import { IncidentTimeline } from "./components/IncidentTimeline"
import { DependencyGraph } from "./components/DependencyGraph"

export default function OverviewPage() {
  const [timelineRange, setTimelineRange] = useState<{ start: number; end: number } | null>(null)

  const q = useQuery({
    queryKey: ["overview", timelineRange?.start, timelineRange?.end],
    queryFn: () => getOverview(timelineRange?.start, timelineRange?.end),
    refetchInterval: 30_000,
    refetchOnWindowFocus: false,
  })

  const data = q.data
  const loading = q.isLoading

  const handleTimeRangeChange = useCallback((start: number, end: number) => {
    setTimelineRange({ start, end })
  }, [])

  return (
    <Page>
      <PageHeader
        title="Overview & Correlation"
        description="Cross-service observability: fleet health, dependency correlation, and incident timeline in one place."
        actions={
          <Button
            variant="outline"
            size="sm"
            onClick={() => q.refetch()}
            disabled={q.isRefetching}
          >
            <RefreshCw
              className={`h-4 w-4 ${q.isRefetching ? "animate-spin" : ""}`}
            />
            Refresh
          </Button>
        }
      />

      {/* Zone A — Live Fleet Health */}
      <SectionCard title="Fleet Health" description="Global health snapshot across all hosts and services">
        {loading ? (
          <div className="grid grid-cols-3 gap-3 md:grid-cols-6">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="rounded-lg border border-border bg-card p-3 animate-pulse">
                <div className="h-3 w-16 bg-muted rounded" />
                <div className="mt-2 h-6 w-10 bg-muted rounded" />
              </div>
            ))}
          </div>
        ) : data ? (
          <GlobalHealthStrip data={data.global} />
        ) : (
          <div className="py-8 text-center text-sm text-muted-foreground">
            Unable to load fleet health data.
          </div>
        )}
      </SectionCard>

      <SectionCard
        title="Hosts"
        description="Per-host status, resource usage, and installed services"
      >
        {loading ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="rounded-lg border border-border bg-card p-4 animate-pulse"
              >
                <div className="h-4 w-24 bg-muted rounded" />
                <div className="mt-3 flex gap-1.5">
                  <div className="h-5 w-14 bg-muted rounded" />
                  <div className="h-5 w-14 bg-muted rounded" />
                </div>
                <div className="mt-3 space-y-2">
                  <div className="h-4 bg-muted rounded" />
                  <div className="h-4 bg-muted rounded" />
                  <div className="h-4 bg-muted rounded" />
                </div>
              </div>
            ))}
          </div>
        ) : data ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {(data.hosts ?? []).map((host) => (
              <HostCard key={host.machine_id} host={host} />
            ))}
          </div>
        ) : null}
      </SectionCard>

      {/* Diagnosed Issues + Incident Timeline side by side */}
      <div className="grid gap-6 md:grid-cols-2">
        <SectionCard
          title="Diagnosed Issues"
          description="Live-detected issues with what/why/impact/action for each service"
        >
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-16 bg-muted rounded animate-pulse" />
              ))}
            </div>
          ) : data ? (
            <DiagnosedIssues issues={data.issues} />
          ) : null}
        </SectionCard>

        <SectionCard
          title="Incident Timeline"
          description="Recent log events across all services (adjustable window)"
        >
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-10 bg-muted rounded animate-pulse" />
              ))}
            </div>
          ) : data ? (
            <IncidentTimeline
              events={data.timeline?.events ?? null}
              onTimeRangeChange={handleTimeRangeChange}
            />
          ) : null}
        </SectionCard>
      </div>

      {/* Zone B — Persisted Correlation Findings */}
      <SectionCard
        title="Correlation Findings"
        description="Persisted cross-service correlation rules detecting dependency chain issues"
      >
        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="h-24 bg-muted rounded animate-pulse" />
            ))}
          </div>
        ) : data ? (
          <CorrelationFindings findings={data.findings} />
        ) : null}
      </SectionCard>

      <SectionCard
        title="Dependency Graph"
        description="Service dependency topology with live health overlay and edge coloring from findings"
      >
        {loading ? (
          <div className="h-48 bg-muted rounded animate-pulse" />
        ) : data ? (
          <DependencyGraph data={data.dep_graph} />
        ) : null}
      </SectionCard>
    </Page>
  )
}
