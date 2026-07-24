import { Server, Activity, AlertTriangle, AlertCircle, Zap, Link2 } from "lucide-react"

import type { GlobalHealth } from "@/api/overview"
import { cn } from "@/lib/utils"

type KpiCard = {
  key: string
  label: string
  value: string
  attention: boolean
  icon: typeof Server
}

export function GlobalHealthStrip({ data }: { data: GlobalHealth }) {
  const totalSvcs = data.services_healthy + data.services_degraded + data.services_down
  const hostsPct = data.hosts_total > 0 ? Math.round((data.hosts_online / data.hosts_total) * 100) : 0
  const healthyPct = totalSvcs > 0 ? Math.round((data.services_healthy / totalSvcs) * 100) : 0

  const cards: KpiCard[] = [
    {
      key: "hosts",
      label: "Hosts Online",
      value: `${data.hosts_online}/${data.hosts_total}`,
      attention: hostsPct < 100,
      icon: Server,
    },
    {
      key: "healthy",
      label: "Services Healthy",
      value: String(data.services_healthy),
      attention: healthyPct < 90,
      icon: Activity,
    },
    {
      key: "degraded",
      label: "Degraded",
      value: String(data.services_degraded),
      attention: data.services_degraded > 0,
      icon: AlertTriangle,
    },
    {
      key: "down",
      label: "Down",
      value: String(data.services_down),
      attention: data.services_down > 0,
      icon: AlertCircle,
    },
    {
      key: "signals",
      label: "Active Signals",
      value: String(data.active_signals),
      attention: data.active_signals > 0,
      icon: Zap,
    },
    {
      key: "findings",
      label: "Open Findings",
      value: String(data.open_findings),
      attention: data.open_findings > 0,
      icon: Link2,
    },
  ]

  return (
    <div className="grid grid-cols-3 gap-3 md:grid-cols-6">
      {cards.map((card) => {
        const Icon = card.icon
        return (
          <div
            key={card.key}
            className={cn(
              "rounded-lg border p-3 transition-colors",
              card.attention
                ? "border-amber-500/30 bg-amber-500/[0.04]"
                : "border-border bg-card"
            )}
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-xs text-muted-foreground">{card.label}</p>
                <p className="mt-1 text-xl font-bold text-foreground tabular-nums">
                  {card.value}
                </p>
              </div>
              <Icon className={cn(
                "h-5 w-5 shrink-0",
                card.attention ? "text-amber-400" : "text-muted-foreground/30"
              )} />
            </div>
          </div>
        )
      })}
    </div>
  )
}
