import { useMemo } from "react"
import { useHostCPU } from "@/pages/hosts/hooks/useHostMetric"
import { LineChartCard } from "@/pages/hosts/components/charts/LineChartCard"
import { StackedAreaCard } from "@/pages/hosts/components/charts/StackedAreaCard"
import { formatTimePoints } from "@/pages/hosts/utils/formatTime"

interface Props {
  machineId: string
  params: { start: string; end: string; step: string }
}

const PALETTE = [
  "#3b82f6", "#ef4444", "#22c55e", "#f59e0b", "#8b5cf6", "#ec4899",
  "#06b6d4", "#84cc16", "#f97316", "#14b8a6", "#d946ef", "#0ea5e9",
]

const CPU_STATE_AREAS = [
  { dataKey: "user", color: "#22c55e", name: "User" },
  { dataKey: "system", color: "#3b82f6", name: "System" },
  { dataKey: "wait", color: "#f59e0b", name: "I/O Wait" },
  { dataKey: "steal", color: "#ef4444", name: "Steal" },
  { dataKey: "softirq", color: "#8b5cf6", name: "SoftIRQ" },
  { dataKey: "nice", color: "#06b6d4", name: "Nice" },
  { dataKey: "interrupt", color: "#ec4899", name: "Interrupt" },
]

export function CPUSection({ machineId, params }: Props) {
  const query = useHostCPU(machineId, params)
  const d = query.data as Record<string, any> | undefined

  const overall = useMemo(() => formatTimePoints(d?.overall), [d])
  const breakdown = useMemo(() => formatTimePoints(d?.breakdown), [d])
  const perCPU = useMemo(() => formatTimePoints(d?.per_cpu), [d])
  const loadAvg = useMemo(() => formatTimePoints(d?.load_avg), [d])
  const freq = useMemo(() => formatTimePoints(d?.frequency), [d])

  const perCPULines = useMemo(() => {
    if (!perCPU.length) return []
    return Object.keys(perCPU[0])
      .filter((k) => k !== "time")
      .map((key, i) => ({ dataKey: key, color: PALETTE[i % PALETTE.length], name: key }))
  }, [perCPU])

  return (
    <div className="grid gap-6">
      <div className="grid gap-4 lg:grid-cols-2">
        <LineChartCard title="Overall CPU Usage" data={overall} lines={[{ dataKey: "value", color: "#22c55e", name: "CPU %" }]} loading={query.isLoading} />
        <LineChartCard title="CPU Frequency" data={freq} lines={[{ dataKey: "value", color: "#8b5cf6", name: "Hz" }]} loading={query.isLoading} />
      </div>
      <StackedAreaCard title="CPU Breakdown by State" data={breakdown} areas={CPU_STATE_AREAS} loading={query.isLoading} />
      <LineChartCard title="Per-CPU Utilization" data={perCPU} lines={perCPULines} loading={query.isLoading} />
      <LineChartCard title="Load Average" data={loadAvg} lines={[
        { dataKey: "1m", color: "#ef4444", name: "1m" },
        { dataKey: "5m", color: "#f59e0b", name: "5m" },
        { dataKey: "15m", color: "#22c55e", name: "15m" },
      ]} loading={query.isLoading} />
    </div>
  )
}
