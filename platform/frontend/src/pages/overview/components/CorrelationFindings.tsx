import { ExternalLink, Lightbulb } from "lucide-react"

import type { Finding } from "@/api/overview"
import { cn } from "@/lib/utils"

const severityConfig: Record<string, { border: string; badge: string; label: string }> = {
  critical: {
    border: "border-red-200 dark:border-red-900/40",
    badge: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    label: "Critical",
  },
  warning: {
    border: "border-amber-200 dark:border-amber-900/40",
    badge: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    label: "Warning",
  },
  info: {
    border: "border-blue-200 dark:border-blue-900/40",
    badge: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    label: "Info",
  },
}

const statusDot: Record<string, string> = {
  critical: "bg-red-500",
  warning: "bg-amber-500",
  info: "bg-blue-500",
}

export function CorrelationFindings({ findings }: { findings: Finding[] | null | undefined }) {
  if (!findings || findings.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
        No correlation findings — all dependency chains look healthy
      </div>
    )
  }

  return (
    <div className="space-y-3">
      {findings.map((f) => {
        const cfg = severityConfig[f.severity] ?? severityConfig.info
        return (
          <div
            key={f.id}
            className={cn("rounded-lg border bg-card/70 p-4", cfg.border)}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className={cn("inline-block h-2 w-2 rounded-full", statusDot[f.severity])} />
                  <span className={cn("rounded px-1.5 py-0.5 text-[11px] font-medium", cfg.badge)}>
                    {cfg.label}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {f.hostname} · {f.rule_id}
                  </span>
                  <span className="rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground">
                    {f.duration}
                  </span>
                </div>
                <h4 className="mt-1.5 text-sm font-medium">{f.title}</h4>
                <p className="mt-1 text-xs text-muted-foreground">{f.message}</p>
              </div>
            </div>

            {f.evidence.length > 0 ? (
              <div className="mt-3 flex flex-wrap gap-2">
                {f.evidence.map((e, i) => (
                  <span
                    key={i}
                    className={cn(
                      "inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs",
                      e.status === "critical"
                        ? "border-red-200 bg-red-50/50 dark:border-red-900/40 dark:bg-red-950/20"
                        : e.status === "warning"
                          ? "border-amber-200 bg-amber-50/50 dark:border-amber-900/40 dark:bg-amber-950/20"
                          : "bg-card"
                    )}
                  >
                    <span className="text-muted-foreground">{e.metric}:</span>
                    <span className="font-medium">{e.value}</span>
                  </span>
                ))}
              </div>
            ) : null}

            {/* Recommended action */}
            {f.recommended_action ? (
              <div className="mt-3 flex items-start gap-2 rounded-md border border-border bg-background/50 p-2 text-xs">
                <Lightbulb className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-500" />
                <span className="text-muted-foreground">{f.recommended_action}</span>
              </div>
            ) : null}

            {/* Investigation link */}
            {f.dashboard_link ? (
              <div className="mt-2">
                <a
                  href={f.dashboard_link}
                  className="inline-flex items-center gap-1 text-xs text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  <ExternalLink className="h-3 w-3" />
                  Investigate in dashboard
                </a>
              </div>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
