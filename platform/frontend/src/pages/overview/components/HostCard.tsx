import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"

import type { HostOverview } from "@/api/overview"
import { cn } from "@/lib/utils"

function UsageBar({ label, pct, unit }: { label: string; pct: number | undefined; unit: string }) {
  const safe = pct ?? 0
  return (
    <div className="space-y-0.5">
      <div className="flex items-center justify-between text-xs">
        <span className="text-muted-foreground">{label}</span>
        <span className="font-medium tabular-nums text-foreground">
          {pct !== undefined ? `${pct.toFixed(1)}${unit}` : "—"}
        </span>
      </div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div className="h-full rounded-full bg-foreground/15 transition-all" style={{ width: `${Math.min(safe, 100)}%` }} />
      </div>
    </div>
  )
}

export function HostCard({ host }: { host: HostOverview }) {
  const isOnline = host.status === "online"

  return (
    <Link
      to={`/hosts/${host.machine_id}`}
      className={cn(
        "block rounded-lg border border-border bg-card p-4 transition-all hover:shadow-elevated hover:border-primary/30",
        !isOnline && "opacity-60"
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "inline-block h-2 w-2 rounded-full",
                isOnline ? "bg-emerald-500" : "bg-muted-foreground/40"
              )}
            />
            <span className="font-medium text-foreground truncate">{host.hostname}</span>
            <span className="text-xs text-muted-foreground">{host.os}</span>
          </div>
        </div>
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {(host.services ?? []).map((svc) => (
          <span
            key={svc.name}
            className="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-border bg-muted/50 px-2 py-0.5 text-xs text-foreground transition-colors hover:bg-muted"
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              window.location.href = `/${svc.name}/${host.machine_id}/dashboard`
            }}
          >
            <span
              className={cn(
                "inline-block h-1.5 w-1.5 rounded-full",
                svc.status === "healthy" ? "bg-emerald-500" :
                svc.status === "degraded" ? "bg-amber-500" :
                svc.status === "down" ? "bg-red-500" : "bg-muted-foreground/40"
              )}
            />
            {svc.name}
            {svc.port ? <span className="text-muted-foreground">:{svc.port}</span> : null}
          </span>
        ))}
      </div>

      {host.summary ? (
        <p className="mt-2 text-xs text-amber-600 dark:text-amber-400 line-clamp-1">
          {host.summary}
        </p>
      ) : null}

      <div className="mt-3 space-y-2">
        <UsageBar label="CPU" pct={host.cpu_pct} unit="%" />
        <UsageBar label="Memory" pct={host.mem_pct} unit="%" />
        <UsageBar label="Disk" pct={host.disk_pct} unit="%" />
      </div>
    </Link>
  )
}
