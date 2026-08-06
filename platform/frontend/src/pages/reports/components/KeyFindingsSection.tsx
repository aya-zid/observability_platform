import { AlertCircle, AlertTriangle, Info } from "lucide-react"

import type { KeyFinding } from "@/api/reports"
import { cn } from "@/lib/utils"
import { SEVERITY_STYLES } from "../utils"

const SEVERITY_ORDER = ["critical", "warning", "info"] as const

function SeverityIcon({ severity }: { severity: string }) {
  if (severity === "critical") return <AlertTriangle className="h-3 w-3" />
  if (severity === "warning") return <AlertCircle className="h-3 w-3" />
  return <Info className="h-3 w-3" />
}

export function KeyFindingsSection({ findings }: { findings: KeyFinding[] }) {
  if (!findings || findings.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        No key findings identified for this report.
      </p>
    )
  }

  const sorted = [...findings].sort(
    (a, b) => SEVERITY_ORDER.indexOf(a.severity as (typeof SEVERITY_ORDER)[number]) - SEVERITY_ORDER.indexOf(b.severity as (typeof SEVERITY_ORDER)[number])
  )

  return (
    <div className="space-y-3">
      {sorted.map((f, i) => (
        <div
          key={i}
          className={cn(
            "rounded-lg border bg-card p-3.5",
            f.severity === "critical" && "border-red-500/40 bg-red-500/[0.03]",
            f.severity === "warning" && "border-amber-500/40",
            (f.severity === "info" || !f.severity) && "border-border"
          )}
        >
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "inline-flex shrink-0 items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wider",
                SEVERITY_STYLES[f.severity] ?? "text-muted-foreground bg-muted border-border"
              )}
            >
              <SeverityIcon severity={f.severity} />
              {f.severity}
            </span>
            <h3 className="min-w-0 text-sm font-semibold text-foreground">{f.title}</h3>
          </div>

          {f.detail && (
            <p className="mt-1.5 text-sm leading-relaxed text-foreground/85">{f.detail}</p>
          )}

          {f.day_a_value && f.day_b_value && (
            <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
              <span>
                Day A: <span className="font-medium text-foreground">{f.day_a_value}</span>
              </span>
              <span>
                Day B: <span className="font-medium text-foreground">{f.day_b_value}</span>
              </span>
              {f.delta_pct !== null && f.delta_pct !== undefined && (
                <span
                  className={cn(
                    "font-medium",
                    f.delta_pct > 0 ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"
                  )}
                >
                  Δ {f.delta_pct > 0 ? "+" : ""}
                  {f.delta_pct.toFixed(1)}%
                </span>
              )}
            </div>
          )}
        </div>
      ))}
    </div>
  )
}
