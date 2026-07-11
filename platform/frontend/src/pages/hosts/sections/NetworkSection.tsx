import { useMemo } from "react"
import { useHostNetwork } from "@/pages/hosts/hooks/useHostMetric"
import { LineChartCard } from "@/pages/hosts/components/charts/LineChartCard"
import { StackedAreaCard } from "@/pages/hosts/components/charts/StackedAreaCard"
import { formatTimePoints } from "@/pages/hosts/utils/formatTime"

interface Props {
  machineId: string
  params: { start: string; end: string; step: string }
}

const DIR_AREAS = [
  { dataKey: "receive", color: "#3b82f6", name: "Receive" },
  { dataKey: "transmit", color: "#f59e0b", name: "Transmit" },
]

const TCP_AREAS = [
  { dataKey: "ESTABLISHED", color: "#22c55e", name: "Established" },
  { dataKey: "TIME_WAIT", color: "#f59e0b", name: "Time Wait" },
  { dataKey: "CLOSE_WAIT", color: "#ef4444", name: "Close Wait" },
  { dataKey: "LISTEN", color: "#8b5cf6", name: "Listen" },
]

export function NetworkSection({ machineId, params }: Props) {
  const query = useHostNetwork(machineId, params)
  const d = query.data as Record<string, any> | undefined

  const throughput = useMemo(() => formatTimePoints(d?.throughput), [d])
  const packets = useMemo(() => formatTimePoints(d?.packets), [d])
  const errors = useMemo(() => formatTimePoints(d?.errors), [d])
  const dropped = useMemo(() => formatTimePoints(d?.dropped), [d])
  const tcp = useMemo(() => formatTimePoints(d?.tcp), [d])
  const conntrack = useMemo(() => formatTimePoints(d?.conntrack), [d])

  return (
    <div className="grid gap-6">
      <div className="grid gap-4 lg:grid-cols-2">
        <StackedAreaCard title="Network Throughput" data={throughput} areas={DIR_AREAS} loading={query.isLoading} />
        <StackedAreaCard title="Network Packets" data={packets} areas={DIR_AREAS} loading={query.isLoading} />
      </div>
      <div className="grid gap-4 lg:grid-cols-3">
        <LineChartCard title="Network Errors" data={errors} lines={[{ dataKey: "value", color: "#ef4444", name: "Errors/s" }]} loading={query.isLoading} />
        <LineChartCard title="Dropped Packets" data={dropped} lines={[{ dataKey: "value", color: "#f59e0b", name: "Drops/s" }]} loading={query.isLoading} />
        <LineChartCard title="Conntrack Usage" data={conntrack} lines={[{ dataKey: "value", color: "#8b5cf6", name: "%" }]} loading={query.isLoading} />
      </div>
      <StackedAreaCard title="TCP Connections by State" data={tcp} areas={TCP_AREAS} loading={query.isLoading} />
    </div>
  )
}
