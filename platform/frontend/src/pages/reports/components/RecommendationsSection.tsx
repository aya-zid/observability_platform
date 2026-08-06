import { AlertTriangle, ArrowRight, Flame, Wrench, Zap } from "lucide-react"

import type { Recommendation } from "@/api/reports"
import { cn } from "@/lib/utils"
import { SEVERITY_STYLES } from "../utils"

export function RecommendationsSection({ recommendations }: { recommendations: Recommendation[] }) {
  if (!recommendations || recommendations.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        No AI recommendations for this report.
      </p>
    )
  }

  const priorityOrder = ["critical", "high", "medium", "low"]
  const sorted = [...recommendations].sort(
    (a, b) => priorityOrder.indexOf(a.priority) - priorityOrder.indexOf(b.priority)
  )

  return (
    <div className="space-y-3">
      {sorted.map((rec, i) => (
        <div
          key={i}
          className={cn(
            "rounded-lg border bg-card p-3.5",
            rec.priority === "critical" && "border-red-500/40 bg-red-500/[0.03]",
            rec.priority === "high" && "border-amber-500/40",
            (rec.priority === "medium" || rec.priority === "low") && "border-border"
          )}
        >
          <div className="flex items-start justify-between gap-3">
            <div className="flex items-center gap-2 min-w-0">
              <span
                className={cn(
                  "inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider",
                  SEVERITY_STYLES[rec.priority] ?? "text-muted-foreground bg-muted border-border"
                )}
              >
                {rec.priority === "critical" ? <Flame className="h-3 w-3" /> : rec.priority === "high" ? <AlertTriangle className="h-3 w-3" /> : <Zap className="h-3 w-3" />}
                {rec.priority}
              </span>
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
                <Wrench className="h-3 w-3" />
                effort: {rec.effort}
              </span>
            </div>
          </div>

          <h3 className="mt-2 text-sm font-semibold text-foreground">{rec.title}</h3>
          {rec.action && (
            <p className="mt-1.5 text-sm leading-relaxed text-foreground/85">{rec.action}</p>
          )}
          {rec.rationale && (
            <p className="mt-1.5 text-xs text-muted-foreground border-l-2 border-primary/30 pl-2.5 italic">
              {rec.rationale}
            </p>
          )}
          {rec.expected_impact && (
            <p className="mt-2 flex items-center gap-1.5 text-xs font-medium text-green-600 dark:text-green-400">
              <ArrowRight className="h-3 w-3" />
              {rec.expected_impact}
            </p>
          )}
        </div>
      ))}
    </div>
  )
}
