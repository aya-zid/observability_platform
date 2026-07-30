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
      <div className="rounded-lg border border-dashed border-border bg-card/30 p-6 text-center text-xs text-muted-foreground">
        No correlation findings — all dependency chains look healthy
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {findings.map((f) => {
        const cfg = severityConfig[f.severity] ?? severityConfig.info
        return (
          <div
            key={f.id}
            className={cn("rounded-lg border bg-card/70 p-3", cfg.border)}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className={cn("inline-block h-1.5 w-1.5 rounded-full shrink-0", statusDot[f.severity])} />
                  <span className={cn("rounded px-1 py-0.5 text-[11px] font-medium leading-none", cfg.badge)}>
                    {cfg.label}
                  </span>
                  <span className="text-[11px] text-muted-foreground">
                    {f.hostname} · {f.rule_id}
                  </span>
                  <span className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                    {f.duration}
                  </span>
                </div>
                <h4 className="mt-1 text-xs font-semibold">{f.title}</h4>
                <p className="mt-0.5 text-xs text-muted-foreground leading-relaxed">{f.message}</p>
              </div>
            </div>

            {f.evidence.length > 0 ? (
              <div className="mt-2 flex flex-wrap gap-1.5">
                {f.evidence.map((e, i) => (
                  <span
                    key={i}
                    className={cn(
                      "inline-flex items-center gap-1 rounded-md border border-border px-1.5 py-0.5 text-[11px]",
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

            {f.recommended_action ? (
              <div className="mt-2 flex items-start gap-1.5 rounded-md border border-border bg-background/50 p-1.5 text-[11px]">
                <Lightbulb className="mt-0.5 h-3 w-3 shrink-0 text-amber-500" />
                <span className="text-muted-foreground leading-relaxed">{f.recommended_action}</span>
              </div>
            ) : null}

            {f.dashboard_link ? (
              <div className="mt-1.5">
                <a
                  href={f.dashboard_link}
                  className="inline-flex items-center gap-1 text-[11px] text-blue-500 hover:text-blue-600 dark:text-blue-400 dark:hover:text-blue-300"
                >
                  <ExternalLink className="h-2.5 w-2.5" />
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
