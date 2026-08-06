import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useParams } from "react-router-dom"
import { AlertCircle, AlertTriangle, CalendarClock, ClipboardList, HeartPulse, History, Lightbulb, ListChecks } from "lucide-react"

import {
  getReport, regenerateReport,
  type ReportDetail, type AIReport, type TodayAIReport, type CompareAIReport,
} from "@/api/reports"
import { cn } from "@/lib/utils"
import { ReportHeader } from "./components/ReportHeader"
import { ReportStatusBanner } from "./components/ReportStatusBanner"
import { ReportKPIGrid, ServiceDeltaTable } from "./components/ReportKPIGrid"
import { RegionCard } from "./components/RegionCard"
import { CompareRegionCard } from "./components/CompareRegionCard"
import { DayTimeline } from "./components/DayTimeline"
import { AlertsIncidentsSection } from "./components/AlertsIncidentsSection"
import { ServiceHealthTable } from "./components/ServiceHealthTable"
import { RecommendationsSection } from "./components/RecommendationsSection"
import { ServiceComparisonTable } from "./components/ServiceComparisonTable"
import { ReportSection } from "./components/ReportSection"
import { KeyFindingsSection } from "./components/KeyFindingsSection"

function isCompareAI(ai: AIReport | null | undefined): ai is CompareAIReport {
  return !!ai && "service_comparisons" in ai
}

function GeneratingSkeleton() {
  return (
    <div className="animate-fade-in space-y-4">
      <div className="h-20 rounded-lg border border-border bg-card animate-pulse" />
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="h-20 rounded-lg border border-border bg-muted/30 animate-pulse" />
        ))}
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-64 rounded-lg border border-border bg-muted/30 animate-pulse" />
        ))}
      </div>
    </div>
  )
}

export default function ReportViewPage() {
  const { id } = useParams()
  const queryClient = useQueryClient()
  const reportId = Number(id)

  const reportQuery = useQuery({
    queryKey: ["reports", reportId],
    queryFn: () => getReport(reportId),
    enabled: Number.isFinite(reportId),
    refetchInterval: (query) => {
      const report = query.state.data as ReportDetail | undefined
      return report?.status === "generating" ? 3000 : false
    },
  })

  const regenerateMutation = useMutation({
    mutationFn: () => regenerateReport(reportId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["reports", reportId] }),
  })

  const report = reportQuery.data

  if (reportQuery.isError) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <AlertCircle className="mb-3 h-10 w-10 text-destructive opacity-70" />
        <p className="text-sm font-medium text-foreground">Failed to load this report.</p>
        <p className="mt-1 text-sm text-muted-foreground">{reportQuery.error?.message ?? "Please try again."}</p>
      </div>
    )
  }

  if (!report) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-center">
        <div className="h-10 w-10 animate-pulse rounded-full bg-muted" />
        <p className="mt-3 text-sm text-muted-foreground">Loading report…</p>
      </div>
    )
  }

  const ai = report.ai_report as AIReport | null | undefined
  const compare = isCompareAI(ai)
  const regions = report.numbers?.regions ?? []
  const aiRegions = ai?.regions ?? []
  const aiByRegion = new Map(aiRegions.map((r) => [r.region, r]))

  return (
    <div className="animate-fade-in space-y-4">
      <ReportHeader
        report={report}
        onRegenerate={() => regenerateMutation.mutate()}
        regenerating={regenerateMutation.isPending}
        onPrint={() => window.print()}
      />

      {report.status === "generating" ? (
        <GeneratingSkeleton />
      ) : report.status === "error" ? (
        <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
          <p className="font-semibold">Generation failed</p>
          <p className="mt-1">{report.error || "Unknown error. Try regenerating."}</p>
        </div>
      ) : (
        <>
          {/* Status banner */}
          {ai && (
            <ReportStatusBanner
              overallStatus={(ai as TodayAIReport).overall_status}
              executiveSummary={(ai as TodayAIReport).executive_summary}
            />
          )}

          {/* Key findings */}
          {ai?.key_findings && ai.key_findings.length > 0 && (
            <ReportSection title="Key Findings" eyebrow="The most important signals the AI identified." icon={AlertTriangle}>
              <KeyFindingsSection findings={ai.key_findings} />
            </ReportSection>
          )}

          {/* KPI strip */}
          {report.numbers && (
            <ReportSection title="Key Metrics" eyebrow="Whole-day snapshot from gathered telemetry." icon={HeartPulse}>
              <ReportKPIGrid numbers={report.numbers} />
            </ReportSection>
          )}

          {/* Regions */}
          {regions.length > 0 && (
            <ReportSection
              title={compare ? "Regions — Day A vs Day B" : "Time Regions"}
              eyebrow="Fixed UTC windows: Night, Morning, Afternoon, Evening."
              icon={CalendarClock}
            >
              <div className={cn("grid gap-4", compare ? "md:grid-cols-2" : "md:grid-cols-2 xl:grid-cols-4")}>
                {regions.map((region) =>
                  compare ? (
                    <CompareRegionCard key={region.region} region={region} ai={aiByRegion.get(region.region)} />
                  ) : (
                    <RegionCard key={region.region} region={region} ai={aiByRegion.get(region.region)} />
                  )
                )}
              </div>
            </ReportSection>
          )}

          {/* Day timeline */}
          {!compare && (
            <ReportSection
              title="Day Timeline"
              eyebrow="Notable events, concerns and alerts per region."
              icon={History}
            >
              <DayTimeline regions={regions} aiRegions={aiRegions} alerts={report.numbers?.alerts_a} />
            </ReportSection>
          )}

          {/* Alerts & incidents */}
          {(report.numbers?.alerts_a?.length || report.numbers?.incidents?.length) ? (
            <ReportSection title="Alerts & Incidents" eyebrow="Alert events and correlated incidents overlapping the report day." icon={ClipboardList}>
              <AlertsIncidentsSection
                alertsA={report.numbers?.alerts_a}
                alertsB={report.numbers?.alerts_b}
                incidents={report.numbers?.incidents}
              />
            </ReportSection>
          ) : null}

          {/* Service health */}
          {(report.numbers?.service_health?.length ?? 0) > 0 && (
            <ReportSection title="Service Health" eyebrow="Per-service-kind presence and up-instance counts." icon={HeartPulse}>
              <ServiceHealthTable health={report.numbers?.service_health} />
            </ReportSection>
          )}

          {/* Compare: service comparisons + deltas */}
          {compare && (ai as CompareAIReport).service_comparisons?.length > 0 && (
            <ReportSection title="Service Comparison" eyebrow="AI-picked per-service metric differences (Day A vs Day B)." icon={ListChecks}>
              <ServiceComparisonTable comparisons={(ai as CompareAIReport).service_comparisons} />
            </ReportSection>
          )}
          {compare && report.numbers?.deltas && (
            <ReportSection title="Computed Deltas" eyebrow="Ground-truth percentage changes precomputed by the backend." icon={ListChecks}>
              <ServiceDeltaTable numbers={report.numbers} />
            </ReportSection>
          )}

          {/* Recommendations */}
          {ai?.recommendations && ai.recommendations.length > 0 && (
            <ReportSection title="Recommendations" eyebrow="Prioritized, evidence-linked actions from the AI analysis." icon={Lightbulb}>
              <RecommendationsSection recommendations={ai.recommendations} />
            </ReportSection>
          )}

          {/* Alerts insight prose */}
          {compare && (ai as CompareAIReport).alerts_diff && (
            <ReportSection title="Alerts Difference" icon={ClipboardList}>
              <p className="text-sm leading-relaxed text-foreground/90">{(ai as CompareAIReport).alerts_diff}</p>
            </ReportSection>
          )}
          {!compare && (ai as TodayAIReport).alerts_insight && (
            <ReportSection title="Alerts Insight" icon={ClipboardList}>
              <p className="text-sm leading-relaxed text-foreground/90">{(ai as TodayAIReport).alerts_insight}</p>
            </ReportSection>
          )}
        </>
      )}
    </div>
  )
}
