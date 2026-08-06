import { ShieldAlert, ShieldCheck, Shield } from "lucide-react"

import { cn } from "@/lib/utils"
import { STATUS_STYLES } from "../utils"

export function ReportStatusBanner({
  overallStatus,
  executiveSummary,
}: {
  overallStatus?: string
  executiveSummary?: string
}) {
  const status = overallStatus || "unknown"
  const Icon =
    status === "critical" ? ShieldAlert : status === "degraded" ? Shield : ShieldCheck

  return (
    <div
      className={cn(
        "rounded-lg border p-4 flex items-start gap-3",
        status === "healthy" && "border-green-500/40 bg-green-500/[0.05]",
        status === "degraded" && "border-amber-500/40 bg-amber-500/[0.05]",
        status === "critical" && "border-red-500/40 bg-red-500/[0.05]",
        !["healthy", "degraded", "critical"].includes(status) && "border-border bg-card"
      )}
    >
      <span
        className={cn(
          "mt-0.5 inline-flex shrink-0 items-center gap-1 rounded-md border px-2.5 py-1 text-xs font-semibold uppercase tracking-wider",
          STATUS_STYLES[status] ?? "text-muted-foreground bg-muted border-border"
        )}
      >
        <Icon className="h-3.5 w-3.5" />
        {status}
      </span>
      {executiveSummary && (
        <p className="text-sm leading-relaxed text-foreground/90">{executiveSummary}</p>
      )}
    </div>
  )
}
