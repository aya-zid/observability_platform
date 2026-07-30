import { useState } from "react"
import { ChevronDown, ChevronRight, AlertTriangle, Info, AlertCircle } from "lucide-react"

import type { DiagnosedIssue } from "@/api/overview"
import { cn } from "@/lib/utils"

const severityConfig: Record<string, { border: string; badge: string; dot: string; icon: typeof AlertTriangle }> = {
  critical: {
    border: "border-red-200 dark:border-red-900/40",
    badge: "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400",
    dot: "bg-red-500",
    icon: AlertCircle,
  },
  warning: {
    border: "border-amber-200 dark:border-amber-900/40",
    badge: "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400",
    dot: "bg-amber-500",
    icon: AlertTriangle,
  },
  info: {
    border: "border-blue-200 dark:border-blue-900/40",
    badge: "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400",
    dot: "bg-blue-500",
    icon: Info,
  },
}

export function DiagnosedIssues({ issues }: { issues: DiagnosedIssue[] | null | undefined }) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  const toggle = (id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  if (!issues || issues.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
        No active issues — everything looks healthy
      </div>
    )
  }

  return (
    <div className="space-y-2">
      {issues.map((issue) => {
        const cfg = severityConfig[issue.severity] ?? severityConfig.info
        const SeverityIcon = cfg.icon
        const isOpen = expanded.has(issue.id)

        return (
          <div
            key={issue.id}
            className={cn("rounded-lg border bg-card/70", cfg.border)}
          >
            {/* Header / collapsed view */}
            <button
              type="button"
              onClick={() => toggle(issue.id)}
              className="flex w-full items-start gap-3 p-3 text-left"
            >
              <SeverityIcon className={cn("mt-0.5 h-4 w-4 shrink-0", issue.severity === "critical" ? "text-red-500" : issue.severity === "warning" ? "text-amber-500" : "text-blue-500")} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className={cn("rounded px-1.5 py-0.5 text-sm font-medium", cfg.badge)}>
                    {issue.severity}
                  </span>
                  <span className="text-sm text-muted-foreground">
                    {issue.hostname} · {issue.service}
                  </span>
                  <span className="text-sm text-muted-foreground">· {issue.duration}</span>
                </div>
                <p className="mt-0.5 text-sm font-medium">{issue.diagnosis}</p>
                <p className="mt-0.5 text-sm text-muted-foreground line-clamp-1">{issue.what}</p>
              </div>
              {isOpen ? <ChevronDown className="mt-1 h-4 w-4 shrink-0 text-muted-foreground" /> : <ChevronRight className="mt-1 h-4 w-4 shrink-0 text-muted-foreground" />}
            </button>

            {/* Expanded detail */}
            {isOpen ? (
              <div className="border-t border-border px-3 pb-3 pt-2">
                <div className="grid gap-3 text-sm sm:grid-cols-2">
                  <div className="space-y-1">
                    <span className="font-medium text-muted-foreground">What happened</span>
                    <p className="text-foreground/80">{issue.what}</p>
                  </div>
                  <div className="space-y-1">
                    <span className="font-medium text-muted-foreground">Why</span>
                    <p className="text-foreground/80">{issue.why}</p>
                  </div>
                  <div className="space-y-1">
                    <span className="font-medium text-muted-foreground">Impact</span>
                    <p className="text-foreground/80">{issue.impact}</p>
                  </div>
                  <div className="space-y-1">
                    <span className="font-medium text-muted-foreground">Recommended action</span>
                    <p className="text-foreground/80">{issue.action}</p>
                  </div>
                </div>

                {issue.evidence.length > 0 ? (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {issue.evidence.map((e, i) => (
                      <span
                        key={i}
                        className={cn(
                          "inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 text-sm",
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

                {issue.log_snippet ? (
                  <div className="mt-3 rounded-md border border-border bg-background/50 p-2 font-mono text-sm text-muted-foreground">
                    {issue.log_snippet}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        )
      })}
    </div>
  )
}
