import { Link } from "react-router-dom"
import { Activity, Boxes, Bug, CheckCircle2, Server, ShieldAlert } from "lucide-react"

import type { GlobalHealth } from "@/api/overview"
import { cn } from "@/lib/utils"

type TileTone = "good" | "watch" | "bad" | "neutral"

type KpiTile = {
  key: string
  label: string
  value: string
  detail: string
  tone: TileTone
  icon: typeof Server
  href?: string
}

const toneClasses: Record<TileTone, string> = {
  good: "border-emerald-500/25 bg-emerald-500/[0.04]",
  watch: "border-amber-500/30 bg-amber-500/[0.05]",
  bad: "border-red-500/30 bg-red-500/[0.05]",
  neutral: "border-border bg-card",
}

const iconClasses: Record<TileTone, string> = {
  good: "text-emerald-500",
  watch: "text-amber-500",
  bad: "text-red-500",
  neutral: "text-muted-foreground",
}

function pct(part: number, total: number) {
  return total > 0 ? Math.round((part / total) * 100) : 100
}

function Tile({ tile }: { tile: KpiTile }) {
  const Icon = tile.icon
  const content = (
      <div
        className={cn(
          "h-full rounded-lg border p-2.5 transition-colors",
          toneClasses[tile.tone],
          tile.href && "hover:border-primary/40 hover:bg-accent/20"
        )}
      >
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="truncate text-xs font-medium text-muted-foreground">{tile.label}</p>
            <p className="mt-0.5 text-lg font-semibold tabular-nums text-foreground">{tile.value}</p>
          </div>
          <Icon className={cn("mt-0.5 h-4 w-4 shrink-0", iconClasses[tile.tone])} />
        </div>
        <p className="mt-1.5 line-clamp-1 text-xs text-muted-foreground">{tile.detail}</p>
      </div>
  )

  if (!tile.href) return content
  return (
    <Link to={tile.href} className="block h-full">
      {content}
    </Link>
  )
}

export function GlobalHealthStrip({ data }: { data: GlobalHealth }) {
  const totalServices = data.services_healthy + data.services_degraded + data.services_down
  const serviceProblemCount = data.services_degraded + data.services_down
  const hostsPct = pct(data.hosts_online, data.hosts_total)
  const containersTotal = data.containers_total ?? 0
  const containersHealthy = data.containers_healthy ?? 0
  const containersDown = Math.max(containersTotal - containersHealthy, 0)
  const activeCritical = data.active_critical ?? 0
  const activeWarning = data.active_warning ?? Math.max(data.active_signals - activeCritical, 0)
  const cveExposure = data.cve_exposure ?? data.open_findings ?? 0

  const tiles: KpiTile[] = [
    {
      key: "hosts",
      label: "Hosts Online",
      value: `${data.hosts_online}/${data.hosts_total}`,
      detail: data.hosts_total === 0 ? "No hosts enrolled yet" : `${hostsPct}% of fleet reporting`,
      tone: data.hosts_total === 0 ? "neutral" : hostsPct === 100 ? "good" : "bad",
      icon: Server,
      href: "/hosts",
    },
    {
      key: "services",
      label: "Services",
      value: `${data.services_healthy}/${totalServices}`,
      detail: serviceProblemCount > 0 ? `${data.services_degraded} degraded, ${data.services_down} down` : "All tracked services healthy",
      tone: serviceProblemCount === 0 ? "good" : data.services_down > 0 ? "bad" : "watch",
      icon: Activity,
      href: "/services",
    },
    {
      key: "containers",
      label: "Containers",
      value: containersTotal > 0 ? `${containersHealthy}/${containersTotal}` : "0",
      detail: containersTotal > 0 ? (containersDown > 0 ? `${containersDown} stopped or stale` : "All discovered containers running") : "No containers discovered",
      tone: containersTotal === 0 ? "neutral" : containersDown > 0 ? "bad" : "good",
      icon: Boxes,
      href: "/containers",
    },
    {
      key: "alerts",
      label: "Active Alerts",
      value: String(data.active_signals),
      detail: data.active_signals > 0 ? `${activeCritical} critical, ${activeWarning} warning` : "No firing alert rules",
      tone: activeCritical > 0 ? "bad" : data.active_signals > 0 ? "watch" : "good",
      icon: data.active_signals > 0 ? ShieldAlert : CheckCircle2,
      href: "/alerts",
    },
    {
      key: "cve",
      label: "CVE Exposure",
      value: String(cveExposure),
      detail: cveExposure > 0 ? "Services with high or critical CVEs" : "No high-risk exposure detected",
      tone: cveExposure > 0 ? "bad" : "good",
      icon: Bug,
      href: "/alerts",
    },
  ]

  return (
    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        {tiles.map((tile) => (
          <Tile key={tile.key} tile={tile} />
        ))}
      </div>
    </div>
  )
}
