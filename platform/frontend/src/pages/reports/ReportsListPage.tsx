import { useState } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useNavigate } from "react-router-dom"
import {
  CalendarRange, Eye, FileText, Loader2, RefreshCw, Trash2, XCircle,
} from "lucide-react"

import { deleteReport, generateReport, listReports, regenerateReport, type ReportDetail } from "@/api/reports"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { dateLabel } from "./utils"
import CompareDayDialog from "./components/CompareDayDialog"

const STATUS_STYLES: Record<string, { label: string; className: string }> = {
  generating: { label: "Generating", className: "text-blue-600 dark:text-blue-400 bg-blue-500/10 border-blue-500/30" },
  ready: { label: "Ready", className: "text-green-600 dark:text-green-400 bg-green-500/10 border-green-500/30" },
  error: { label: "Error", className: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30" },
}

function formatDateTime(s?: string): string {
  if (!s) return "—"
  const d = new Date(s)
  return d.toLocaleString()
}

function StatusBadge({ status }: { status: string }) {
  const cfg = STATUS_STYLES[status] ?? { label: status, className: "text-muted-foreground bg-muted border-border" }
  return (
    <span className={cn("inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-xs font-medium", cfg.className)}>
      {status === "generating" && <Loader2 className="w-3 h-3 animate-spin" />}
      {status === "error" && <XCircle className="w-3 h-3" />}
      {status === "ready" && <RefreshCw className="w-3 h-3" />}
      {cfg.label}
    </span>
  )
}

function TypeBadge({ type }: { type: string }) {
  return (
    <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-border text-xs font-medium text-muted-foreground">
      {type === "compare" ? <CalendarRange className="w-3 h-3" /> : <FileText className="w-3 h-3" />}
      {type === "compare" ? "Comparison" : "Today"}
    </span>
  )
}

export default function ReportsListPage() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const [generating, setGenerating] = useState(false)
  const [showCompare, setShowCompare] = useState(false)
  const [genError, setGenError] = useState("")

  async function createToday() {
    if (generating) return
    setGenerating(true)
    setGenError("")
    try {
      const report = await generateReport({ type: "today" })
      navigate(`/reports/${report.id}`)
    } catch (e) {
      setGenError(e instanceof Error ? e.message : "Failed to generate report")
      setGenerating(false)
    }
  }

  async function createCompare(dateA: string, dateB: string) {
    if (generating) return
    setGenerating(true)
    setGenError("")
    setShowCompare(false)
    try {
      const report = await generateReport({ type: "compare", date_a: dateA, date_b: dateB })
      navigate(`/reports/${report.id}`)
    } catch (e) {
      setGenError(e instanceof Error ? e.message : "Failed to generate report")
      setGenerating(false)
    }
  }

  const reportsQuery = useQuery({
    queryKey: ["reports"],
    queryFn: () => listReports({ limit: 100 }),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteReport(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["reports"] }),
  })

  const regenerateMutation = useMutation({
    mutationFn: (id: number) => regenerateReport(id),
    onSuccess: (report) => navigate(`/reports/${report.id}`),
  })

  const reports = reportsQuery.data ?? []

  return (
    <Page>
      <PageHeader
        title="Reports"
        description="Saved AI-generated operations reports. Generate a new one anytime — the same report is reused instead of regenerated."
        icon={FileText}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" onClick={createToday} disabled={generating}>
              {generating ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <FileText className="h-3.5 w-3.5" />
              )}
              {generating ? "Generating…" : "Today's Report"}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setShowCompare(true)} disabled={generating}>
              <CalendarRange className="h-3.5 w-3.5" />
              Compare Report
            </Button>
            {genError && (
              <span className="max-w-[220px] truncate text-xs text-destructive" title={genError}>
                {genError}
              </span>
            )}
            {showCompare && (
              <CompareDayDialog
                onClose={() => setShowCompare(false)}
                onGenerate={createCompare}
              />
            )}
          </div>
        }
      />

      <SectionCard>
        {reportsQuery.isLoading ? (
          <div className="space-y-3">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="h-14 rounded-lg bg-muted animate-pulse" />
            ))}
          </div>
        ) : reports.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-14 text-center">
            <FileText className="mb-3 h-10 w-10 opacity-25" />
            <p className="text-sm font-medium text-foreground">No reports yet</p>
            <p className="mt-1 max-w-sm text-sm text-muted-foreground">
              Generate your first report from the header button — today's report or a day comparison.
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto rounded-xl border border-border">
            <table className="w-full text-sm">
              <thead className="bg-secondary/50 text-sm uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="text-left px-4 py-3 font-semibold">Type</th>
                  <th className="text-left px-3 py-3 font-semibold">Dates</th>
                  <th className="text-left px-3 py-3 font-semibold">Status</th>
                  <th className="text-left px-3 py-3 font-semibold">Created</th>
                  <th className="text-right px-3 py-3 font-semibold">Actions</th>
                </tr>
              </thead>
              <tbody>
                {reports.map((r: ReportDetail) => (
                  <tr key={r.id} className="border-t border-border hover:bg-surface-hover transition-colors">
                    <td className="px-4 py-3">
                      <TypeBadge type={r.type} />
                    </td>
                    <td className="px-3 py-3 font-mono text-muted-foreground">
                      {dateLabel(r.date_a, r.date_b)}
                    </td>
                    <td className="px-3 py-3">
                      <StatusBadge status={r.status} />
                      {r.status === "error" && r.error && (
                        <div className="mt-1 max-w-[220px] text-xs text-destructive" title={r.error}>
                          {r.error}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-3 text-muted-foreground whitespace-nowrap">
                      <div>{formatDateTime(r.created_at)}</div>
                      {r.finished_at && r.status !== "generating" && (
                        <div className="text-xs text-muted-foreground/70">
                          finished {formatDateTime(r.finished_at)}
                        </div>
                      )}
                    </td>
                    <td className="px-3 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => navigate(`/reports/${r.id}`)}
                          disabled={r.status === "generating"}
                        >
                          <Eye className="h-3.5 w-3.5 mr-1" />
                          View
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => regenerateMutation.mutate(r.id)}
                          disabled={regenerateMutation.isPending}
                        >
                          <RefreshCw className={cn("h-3.5 w-3.5 mr-1", regenerateMutation.isPending && "animate-spin")} />
                          Regenerate
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => deleteMutation.mutate(r.id)}
                          className="text-muted-foreground hover:text-red-600 dark:hover:text-red-400"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </SectionCard>
    </Page>
  )
}
