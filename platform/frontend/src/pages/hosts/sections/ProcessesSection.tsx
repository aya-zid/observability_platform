import { useMemo } from "react"
import { useHostProcesses } from "@/pages/hosts/hooks/useHostMetric"
import { StackedAreaCard } from "@/pages/hosts/components/charts/StackedAreaCard"
import { StatCard } from "@/components/shared/StatCard"
import { formatTimePoints } from "@/pages/hosts/utils/formatTime"

interface Props {
  machineId: string
  params: { start: string; end: string; step: string }
}

const PROC_AREAS = [
  { dataKey: "running", color: "#22c55e", name: "Running" },
  { dataKey: "sleeping", color: "#3b82f6", name: "Sleeping" },
  { dataKey: "blocked", color: "#ef4444", name: "Blocked" },
  { dataKey: "zombie", color: "#f59e0b", name: "Zombie" },
  { dataKey: "idle", color: "#94a3b8", name: "Idle" },
]

export function ProcessesSection({ machineId, params }: Props) {
  const query = useHostProcesses(machineId, params)
  const d = query.data as Record<string, any> | undefined

  const procByState = useMemo(() => formatTimePoints(d?.by_state), [d])
  const zombies = (d?.zombie_count as number) ?? null
  const createRate = (d?.creation_rate as number) ?? null

  return (
    <div className="grid gap-4">
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <StatCard
          title="Zombie Processes"
          value={zombies != null ? String(Math.round(zombies)) : "\u2014"}
          status={zombies != null && zombies > 0 ? "red" : "green"}
          loading={query.isLoading}
        />
        <StatCard
          title="Process Creation Rate"
          value={createRate != null ? createRate.toFixed(2) : "\u2014"}
          unit="/s"
          loading={query.isLoading}
        />
      </div>
      <StackedAreaCard title="Processes by State" data={procByState} areas={PROC_AREAS} loading={query.isLoading} />
    </div>
  )
}
