import type { GlobalHealth } from "@/api/overview"
import { cn } from "@/lib/utils"

const cards = [
  {
    key: "hosts" as const,
    label: "Hosts Online",
    valueKey: "hosts_online" as const,
    totalKey: "hosts_total" as const,
    icon: "🖥",
  },
  {
    key: "services" as const,
    label: "Services Healthy",
    valueKey: "services_healthy" as const,
    degradeKey: "services_degraded" as const,
    downKey: "services_down" as const,
    icon: "⚙",
  },
  {
    key: "signals" as const,
    label: "Active Signals",
    valueKey: "active_signals" as const,
    icon: "⚡",
  },
  {
    key: "findings" as const,
    label: "Correlation Findings",
    valueKey: "open_findings" as const,
    icon: "🔗",
  },
]

export function GlobalHealthStrip({ data }: { data: GlobalHealth }) {
  const hostsPct = data.hosts_total > 0 ? Math.round((data.hosts_online / data.hosts_total) * 100) : 0
  const totalSvcs = data.services_healthy + data.services_degraded + data.services_down
  const svcsPct = totalSvcs > 0 ? Math.round((data.services_healthy / totalSvcs) * 100) : 0

  const values: Record<string, { value: string; status: "green" | "yellow" | "red" }> = {
    hosts: {
      value: `${data.hosts_online}/${data.hosts_total}`,
      status: hostsPct === 100 ? "green" : hostsPct >= 50 ? "yellow" : "red",
    },
    services: {
      value: `${data.services_healthy}/${totalSvcs}`,
      status: svcsPct >= 90 ? "green" : svcsPct >= 50 ? "yellow" : "red",
    },
    signals: {
      value: String(data.active_signals),
      status: data.active_signals === 0 ? "green" : data.active_signals < 10 ? "yellow" : "red",
    },
    findings: {
      value: String(data.open_findings),
      status: data.open_findings === 0 ? "green" : data.open_findings < 3 ? "yellow" : "red",
    },
  }

  const colorMap = {
    green: "text-green-600 dark:text-green-400",
    yellow: "text-amber-600 dark:text-amber-400",
    red: "text-red-600 dark:text-red-400",
  }

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      {cards.map((card) => {
        const v = values[card.key]
        return (
          <div
            key={card.key}
            className="rounded-lg border border-border bg-card p-4"
          >
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-xs text-muted-foreground">{card.label}</p>
                <p className={cn("mt-1 text-2xl font-bold", colorMap[v.status])}>
                  {v.value}
                </p>
              </div>
              <span className="text-2xl opacity-30">{card.icon}</span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
