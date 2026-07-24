import { useState, useCallback } from "react"
import { useQuery } from "@tanstack/react-query"
import { RefreshCw, LayoutDashboard, Server, AlertTriangle, AlertCircle, Activity, Zap, Link2 } from "lucide-react"

import { getOverview } from "@/api/overview"
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
    <div className="space-y-8 animate-fade-in">
      {/* Gradient header */}
      <div className="relative overflow-hidden rounded-2xl border border-border/60 bg-gradient-to-br from-primary/10 via-card to-card/80 p-6 md:p-8">
        <div className="absolute inset-0 bg-grid opacity-[0.03]" />
        <div className="relative">
          <div className="flex items-start justify-between gap-4">
            <div>
              <div className="flex items-center gap-3 mb-1">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-primary to-primary/60 flex items-center justify-center shadow-accent-glow">
                  <LayoutDashboard className="w-5 h-5 text-primary-foreground" />
                </div>
                <h1 className="text-2xl md:text-3xl font-bold text-foreground tracking-tight">
                  <span className="text-gradient-brand">Overview</span>
                </h1>
              </div>
              <p className="text-sm text-muted-foreground mt-1 max-w-xl">
                Cross-service observability: fleet health, dependency correlation, and incident timeline in one place.
              </p>
            </div>
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
          </div>
        </div>
      </div>

      {/* Zone A — Live Fleet Health */}
      <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <Activity className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-foreground">Fleet Health</h2>
          <span className="text-[10px] text-muted-foreground ml-auto uppercase tracking-wider">Live</span>
        </div>
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
      </section>

      <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <Server className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-foreground">Hosts</h2>
          <span className="text-[10px] text-muted-foreground ml-auto uppercase tracking-wider">Per-host status</span>
        </div>
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
      </section>

      {/* Diagnosed Issues + Incident Timeline side by side */}
      <div className="grid gap-6 md:grid-cols-2">
        <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
          <div className="flex items-center gap-2 mb-4">
            <AlertTriangle className="w-4 h-4 text-amber-500" />
            <h2 className="text-sm font-semibold text-foreground">Diagnosed Issues</h2>
          </div>
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-16 bg-muted rounded animate-pulse" />
              ))}
            </div>
          ) : data ? (
            <DiagnosedIssues issues={data.issues} />
          ) : null}
        </section>

        <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
          <div className="flex items-center gap-2 mb-4">
            <Activity className="w-4 h-4 text-primary" />
            <h2 className="text-sm font-semibold text-foreground">Incident Timeline</h2>
          </div>
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
        </section>
      </div>

      {/* Zone B — Persisted Correlation Findings */}
      <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <Link2 className="w-4 h-4 text-primary" />
          <h2 className="text-sm font-semibold text-foreground">Correlation Findings</h2>
          <span className="text-[10px] text-muted-foreground ml-auto uppercase tracking-wider">Persisted</span>
        </div>
        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 2 }).map((_, i) => (
              <div key={i} className="h-24 bg-muted rounded animate-pulse" />
            ))}
          </div>
        ) : data ? (
          <CorrelationFindings findings={data.findings} />
        ) : null}
      </section>

      <section className="rounded-xl border border-border bg-card shadow-card p-4 md:p-6">
        <div className="flex items-center gap-2 mb-4">
          <Zap className="w-4 h-4 text-amber-500" />
          <h2 className="text-sm font-semibold text-foreground">Dependency Graph</h2>
        </div>
        {loading ? (
          <div className="h-48 bg-muted rounded animate-pulse" />
        ) : data ? (
          <DependencyGraph data={data.dep_graph} />
        ) : null}
      </section>
    </div>
  )
}
