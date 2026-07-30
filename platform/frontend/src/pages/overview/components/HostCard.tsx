import { Link } from "react-router-dom"
import { ChevronRight, Cpu, HardDrive, MemoryStick, Server } from "lucide-react"

import type { HostOverview, ServiceChip } from "@/api/overview"
import { cn } from "@/lib/utils"

function formatRelative(value: string) {
  const ts = new Date(value).getTime()
  if (!Number.isFinite(ts)) return "unknown"
  const seconds = Math.max(Math.floor((Date.now() - ts) / 1000), 0)
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function usageTone(pct: number | undefined) {
  if (pct === undefined) return "bg-muted-foreground/20"
  if (pct >= 90) return "bg-red-500"
  if (pct >= 75) return "bg-amber-500"
  return "bg-emerald-500"
}

function UsageBar({ icon: Icon, label, pct }: { icon: typeof Cpu; label: string; pct: number | undefined }) {
  const safe = Math.max(0, Math.min(pct ?? 0, 100))
  return (
    <div className="space-y-0.5">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="inline-flex min-w-0 items-center gap-1 text-muted-foreground">
          <Icon className="h-3 w-3 shrink-0" />
          {label}
        </span>
        <span className="font-medium tabular-nums text-foreground">
          {pct !== undefined ? `${Number(pct.toFixed(1))}%` : "n/a"}
        </span>
      </div>
      <div className="h-1 overflow-hidden rounded-full bg-muted">
        <div className={cn("h-full rounded-full transition-all", usageTone(pct))} style={{ width: `${safe}%` }} />
      </div>
    </div>
  )
}

function serviceTone(status: string) {
  switch (status) {
    case "healthy":
      return "border-emerald-500/30 bg-emerald-500/[0.05] text-emerald-600 dark:text-emerald-400"
    case "degraded":
      return "border-amber-500/30 bg-amber-500/[0.06] text-amber-600 dark:text-amber-400"
    case "down":
      return "border-red-500/30 bg-red-500/[0.06] text-red-600 dark:text-red-400"
    default:
      return "border-border bg-muted/40 text-muted-foreground"
  }
}

function serviceCounts(services: ServiceChip[]) {
  return services.reduce(
    (acc, svc) => {
      if (svc.status === "healthy") acc.healthy += 1
      else if (svc.status === "degraded") acc.degraded += 1
      else if (svc.status === "down") acc.down += 1
      else acc.unknown += 1
      return acc
    },
    { healthy: 0, degraded: 0, down: 0, unknown: 0 }
  )
}

export function HostCard({ host }: { host: HostOverview }) {
  const isOnline = host.status === "online"
  const services = host.services ?? []
  const counts = serviceCounts(services)
  const hasProblems = !isOnline || counts.down > 0 || counts.degraded > 0
  const statusLabel = !isOnline ? "Offline" : hasProblems ? "Needs attention" : "Healthy"

  return (
    <Link
      to={`/hosts/${host.machine_id}`}
      className={cn(
        "group block rounded-lg border bg-card p-3 transition-all hover:border-primary/35 hover:shadow-elevated",
        hasProblems ? "border-amber-500/25" : "border-border"
      )}
    >
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span
              className={cn(
                "inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border",
                isOnline ? "border-emerald-500/25 bg-emerald-500/[0.06] text-emerald-500" : "border-border bg-muted text-muted-foreground"
              )}
            >
              <Server className="h-3.5 w-3.5" />
            </span>
            <div className="min-w-0">
              <div className="truncate text-sm font-semibold text-foreground">{host.hostname}</div>
              <div className="truncate text-xs text-muted-foreground">{host.os || host.machine_id}</div>
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          <span
            className={cn(
              "rounded-md border px-1.5 py-0.5 text-xs font-semibold uppercase tracking-wide",
              !isOnline
                ? "border-red-500/30 bg-red-500/[0.06] text-red-500"
                : hasProblems
                  ? "border-amber-500/30 bg-amber-500/[0.06] text-amber-500"
                  : "border-emerald-500/30 bg-emerald-500/[0.06] text-emerald-500"
            )}
          >
            {statusLabel}
          </span>
          <ChevronRight className="h-3.5 w-3.5 text-muted-foreground transition-transform group-hover:translate-x-0.5" />
        </div>
      </div>

      <div className="mt-3 grid grid-cols-4 gap-1.5 rounded-lg border border-border bg-muted/20 p-1.5 text-center">
        <div>
          <div className="text-xs font-semibold tabular-nums text-foreground">{services.length}</div>
          <div className="text-[11px] text-muted-foreground">services</div>
        </div>
        <div>
          <div className="text-xs font-semibold tabular-nums text-emerald-500">{counts.healthy}</div>
          <div className="text-[11px] text-muted-foreground">healthy</div>
        </div>
        <div>
          <div className="text-xs font-semibold tabular-nums text-amber-500">{counts.degraded}</div>
          <div className="text-[11px] text-muted-foreground">degraded</div>
        </div>
        <div>
          <div className="text-xs font-semibold tabular-nums text-red-500">{counts.down}</div>
          <div className="text-[11px] text-muted-foreground">down</div>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap gap-1">
        {services.length > 0 ? services.map((svc) => (
          <span
            key={`${svc.name}-${svc.port ?? "none"}`}
            className={cn("inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-xs", serviceTone(svc.status))}
          >
            <span className="max-w-[8rem] truncate">{svc.name}</span>
            {svc.port ? <span className="text-muted-foreground">:{svc.port}</span> : null}
          </span>
        )) : (
          <span className="text-xs text-muted-foreground">No supported services discovered on this host.</span>
        )}
      </div>

      {host.summary ? (
        <p className="mt-2 line-clamp-2 rounded-md border border-amber-500/20 bg-amber-500/[0.04] px-2 py-1 text-xs text-amber-700 dark:text-amber-300">
          {host.summary}
        </p>
      ) : null}

      <div className="mt-3 space-y-2">
        <UsageBar icon={Cpu} label="CPU" pct={host.cpu_pct} />
        <UsageBar icon={MemoryStick} label="Memory" pct={host.mem_pct} />
        <UsageBar icon={HardDrive} label="Disk" pct={host.disk_pct} />
      </div>

      <div className="mt-3 flex items-center justify-between border-t border-border pt-2 text-xs text-muted-foreground">
        <span>Last heartbeat</span>
        <span className="font-medium tabular-nums">{formatRelative(host.last_seen)}</span>
      </div>
    </Link>
  )
}
