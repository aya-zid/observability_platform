import { useNavigate } from "react-router-dom"
import { ArrowLeft, CalendarRange, FileText, Loader2, Printer, RefreshCw, XCircle, User } from "lucide-react"

import type { ReportDetail } from "@/api/reports"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { dateLabel } from "../utils"

export function ReportHeader({
  report,
  onRegenerate,
  regenerating,
  onPrint,
}: {
  report: ReportDetail
  onRegenerate?: () => void
  regenerating?: boolean
  onPrint?: () => void
}) {
  const navigate = useNavigate()

  const isCompare = report.type === "compare"
  const isGenerating = report.status === "generating"
  const isError = report.status === "error"

  return (
    <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-card/50 px-4 py-3">
      <div className="flex items-center gap-3 min-w-0">
        <Button variant="ghost" size="sm" onClick={() => navigate("/reports")}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h1 className="truncate text-lg font-bold tracking-tight text-foreground">
              {isCompare ? "Day Comparison Report" : "Today's Report"}
            </h1>
            <span
              className={cn(
                "inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-semibold uppercase tracking-wider",
                isCompare
                  ? "text-purple-600 dark:text-purple-400 bg-purple-500/10 border-purple-500/30"
                  : "text-blue-600 dark:text-blue-400 bg-blue-500/10 border-blue-500/30"
              )}
            >
              {isCompare ? <CalendarRange className="h-3 w-3" /> : <FileText className="h-3 w-3" />}
              {isCompare ? "Comparison" : "Today"}
            </span>
            <span
              className={cn(
                "inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-semibold",
                isGenerating && "text-blue-600 dark:text-blue-400 bg-blue-500/10 border-blue-500/30",
                isError && "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30",
                report.status === "ready" && "text-green-600 dark:text-green-400 bg-green-500/10 border-green-500/30"
              )}
            >
              {isGenerating && <Loader2 className="h-3 w-3 animate-spin" />}
              {isError && <XCircle className="h-3 w-3" />}
              {report.status === "ready" && <RefreshCw className="h-3 w-3" />}
              {report.status}
            </span>
          </div>
          <p className="mt-0.5 text-xs text-muted-foreground">
            <span className="font-mono">{dateLabel(report.date_a, report.date_b)}</span> · UTC
          </p>
        </div>
      </div>

      <div className="flex items-center gap-2">
        {report.generated_by ? (
          <span className="hidden items-center gap-1 text-xs text-muted-foreground sm:inline-flex">
            <User className="h-3 w-3" />
            by user #{report.generated_by}
          </span>
        ) : null}
        <Button variant="outline" size="sm" onClick={onPrint} disabled={!report || report.status !== "ready"}>
          <Printer className="h-3.5 w-3.5" />
          Print
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={onRegenerate}
          disabled={regenerating || isGenerating}
        >
          <RefreshCw className={cn("h-3.5 w-3.5", regenerating && "animate-spin")} />
          Regenerate
        </Button>
      </div>
    </header>
  )
}
