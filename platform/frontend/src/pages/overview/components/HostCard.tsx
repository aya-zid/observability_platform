import { Link } from "react-router-dom"
import { ChevronRight } from "lucide-react"

import type { HostOverview } from "@/api/overview"
import { cn } from "@/lib/utils"

const serviceColors: Record<string, string> = {
  nginx: "bg-blue-500",
  apache: "bg-orange-500",
  phpfpm: "bg-purple-500",
  mariadb: "bg-rose-500",
  postgres: "bg-cyan-500",
  redis: "bg-amber-500",
}

const statusColors: Record<string, string> = {
  healthy: "bg-green-500",
  degraded: "bg-amber-500",
  down: "bg-red-500",
  unknown: "bg-gray-400",
  offline: "bg-red-500",
}

function formatUsage(val: number | undefined, unit: string): string {
  if (val === undefined || val === null) return "—"
  return `${val.toFixed(1)}${unit}`
}

export function HostCard({ host }: { host: HostOverview }) {
  const isOnline = host.status === "online"

  return (
    <Link
      to={`/hosts/${host.machine_id}`}
      className={cn(
        "block rounded-lg border border-border bg-card p-4 transition-colors hover:border-accent-foreground/30",
        !isOnline && "opacity-60"
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "inline-block h-2.5 w-2.5 rounded-full",
                isOnline ? "bg-green-500" : "bg-red-500"
              )}
            />
            <span className="font-medium truncate">{host.hostname}</span>
            <span className="text-xs text-muted-foreground">{host.os}</span>
          </div>
        </div>
        <ChevronRight className="h-4 w-4 shrink-0 text-muted-foreground" />
      </div>

      <div className="mt-3 flex flex-wrap gap-1.5">
        {(host.services ?? []).map((svc) => (
          <span
            key={svc.name}
            className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-xs"
          >
            <span
              className={cn(
                "inline-block h-2 w-2 rounded-full",
                statusColors[svc.status] ?? "bg-gray-400"
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

      <div className="mt-3 grid grid-cols-3 gap-2 text-xs">
        <div>
          <span className="text-muted-foreground">CPU </span>
          <span className={cn(host.cpu_pct != null && host.cpu_pct > 80 ? "text-red-500" : "")}>
            {formatUsage(host.cpu_pct, "%")}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground">Mem </span>
          <span className={cn(host.mem_pct != null && host.mem_pct > 80 ? "text-red-500" : "")}>
            {formatUsage(host.mem_pct, "%")}
          </span>
        </div>
        <div>
          <span className="text-muted-foreground">Disk </span>
          <span className={cn(host.disk_pct != null && host.disk_pct > 80 ? "text-red-500" : "")}>
            {formatUsage(host.disk_pct, "%")}
          </span>
        </div>
      </div>
    </Link>
  )
}
