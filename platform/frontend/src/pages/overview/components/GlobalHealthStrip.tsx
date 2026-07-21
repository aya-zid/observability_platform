import { Server, Activity, AlertTriangle, AlertCircle, Zap, Link2 } from "lucide-react"

import type { GlobalHealth } from "@/api/overview"
import { cn } from "@/lib/utils"

type KpiCard = {
  key: string
  label: string
  value: string
  status: "green" | "yellow" | "red" | "neutral"
  icon: typeof Server
}

const statusStyles: Record<string, string> = {
  green: "text-green-600 dark:text-green-400",
  yellow: "text-amber-600 dark:text-amber-400",
  red: "text-red-600 dark:text-red-400",
  neutral: "text-muted-foreground",
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
      status: hostsPct === 100 ? "green" : hostsPct >= 50 ? "yellow" : "red",
      icon: Server,
    },
    {
      key: "healthy",
      label: "Services Healthy",
      value: String(data.services_healthy),
      status: healthyPct >= 90 ? "green" : healthyPct >= 50 ? "yellow" : "red",
      icon: Activity,
    },
    {
      key: "degraded",
      label: "Degraded",
      value: String(data.services_degraded),
      status: data.services_degraded === 0 ? "green" : data.services_degraded < 3 ? "yellow" : "red",
      icon: AlertTriangle,
    },
    {
      key: "down",
      label: "Down",
      value: String(data.services_down),
      status: data.services_down === 0 ? "green" : data.services_down < 2 ? "yellow" : "red",
      icon: AlertCircle,
    },
    {
      key: "signals",
      label: "Active Signals",
      value: String(data.active_signals),
      status: data.active_signals === 0 ? "green" : data.active_signals < 10 ? "yellow" : "red",
      icon: Zap,
    },
    {
      key: "findings",
      label: "Open Findings",
      value: String(data.open_findings),
      status: data.open_findings === 0 ? "green" : data.open_findings < 3 ? "yellow" : "red",
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
            className="rounded-lg border border-border bg-card p-3"
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-xs text-muted-foreground">{card.label}</p>
                <p className={cn("mt-1 text-xl font-bold", statusStyles[card.status])}>
                  {card.value}
                </p>
              </div>
              <Icon className="h-5 w-5 shrink-0 text-muted-foreground/30" />
            </div>
          </div>
        )
      })}
    </div>
  )
}
