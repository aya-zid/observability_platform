import { useQuery } from "@tanstack/react-query"
import { RefreshCw, Server, Activity, GitBranch, Clock, Sparkles } from "lucide-react"

import { getOverview } from "@/api/overview"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { GlobalHealthStrip } from "./components/GlobalHealthStrip"
import { HostCard } from "./components/HostCard"
import { TopIssuesFeed } from "./components/TopIssuesFeed"
import { CorrelationFindings } from "./components/CorrelationFindings"
import { IncidentTimeline } from "./components/IncidentTimeline"
import { DependencyGraph } from "./components/DependencyGraph"
import { AIInsightPanel } from "./components/AIInsightPanel"

export default function OverviewPage() {
  const q = useQuery({
    queryKey: ["overview"],
    queryFn: getOverview,
    refetchInterval: 30_000,
    refetchOnWindowFocus: false,
  })

  const data = q.data
  const loading = q.isLoading

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

      {/* Zone A — Overview */}
      <SectionCard title="Fleet Health" description="Global health snapshot across all hosts and services">
        {loading ? (
          <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="rounded-lg border border-border bg-card p-4 animate-pulse">
                <div className="h-3 w-16 bg-muted rounded" />
                <div className="mt-2 h-7 w-12 bg-muted rounded" />
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
                <div className="mt-3 grid grid-cols-3 gap-2">
                  <div className="h-3 bg-muted rounded" />
                  <div className="h-3 bg-muted rounded" />
                  <div className="h-3 bg-muted rounded" />
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

      <div className="grid gap-6 md:grid-cols-2">
        <SectionCard
          title="Top Issues"
          description="Ranked cross-service issues requiring attention"
        >
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-12 bg-muted rounded animate-pulse" />
              ))}
            </div>
          ) : data ? (
            <TopIssuesFeed issues={data.top_issues} />
          ) : null}
        </SectionCard>

        <SectionCard
          title="Incident Timeline"
          description="Recent log events across all services"
        >
          {loading ? (
            <div className="space-y-2">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-10 bg-muted rounded animate-pulse" />
              ))}
            </div>
          ) : data ? (
            <IncidentTimeline events={data.timeline?.events ?? null} />
          ) : null}
        </SectionCard>
      </div>

      {/* Zone B — Correlation */}
      <SectionCard
        title="Correlation Findings"
        description="Automated cross-service correlation rules detecting dependency chain issues"
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
        description="Service dependency topology with live health overlay"
      >
        {loading ? (
          <div className="h-48 bg-muted rounded animate-pulse" />
        ) : data ? (
          <DependencyGraph data={data.dep_graph} />
        ) : null}
      </SectionCard>

      {/* AI Insight (on-demand) */}
      {data ? <AIInsightPanel overview={data} /> : null}
    </Page>
  )
}
