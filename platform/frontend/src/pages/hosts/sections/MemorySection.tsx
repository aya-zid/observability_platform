import { useMemo } from "react"
import { useHostMemory } from "@/pages/hosts/hooks/useHostMetric"
import { LineChartCard } from "@/pages/hosts/components/charts/LineChartCard"
import { StackedAreaCard } from "@/pages/hosts/components/charts/StackedAreaCard"
import { formatTimePoints } from "@/pages/hosts/utils/formatTime"

interface Props {
  machineId: string
  params: { start: string; end: string; step: string }
}

const MEM_AREAS = [
  { dataKey: "used", color: "#8b5cf6", name: "Used" },
  { dataKey: "cached", color: "#22c55e", name: "Cached" },
  { dataKey: "buffered", color: "#06b6d4", name: "Buffered" },
  { dataKey: "free", color: "#94a3b8", name: "Free" },
]

const SWAP_AREAS = [
  { dataKey: "used", color: "#ef4444", name: "Used" },
  { dataKey: "free", color: "#94a3b8", name: "Free" },
]

const FAULT_AREAS = [
  { dataKey: "major", color: "#ef4444", name: "Major" },
  { dataKey: "minor", color: "#f59e0b", name: "Minor" },
]

export function MemorySection({ machineId, params }: Props) {
  const query = useHostMemory(machineId, params)
  const d = query.data as Record<string, any> | undefined

  const memUsage = useMemo(() => formatTimePoints(d?.usage), [d])
  const memPct = useMemo(() => formatTimePoints(d?.percent), [d])
  const memAvail = useMemo(() => formatTimePoints(d?.avail), [d])
  const swapUsage = useMemo(() => formatTimePoints(d?.swap_usage), [d])
  const pageFaults = useMemo(() => formatTimePoints(d?.faults), [d])
  const swapIO = useMemo(() => formatTimePoints(d?.swap_io), [d])

  return (
    <div className="grid gap-4">
      <div className="grid gap-3 lg:grid-cols-2">
        <StackedAreaCard title="Memory Usage" data={memUsage} areas={MEM_AREAS} loading={query.isLoading} />
        <LineChartCard title="Memory Used %" data={memPct} lines={[{ dataKey: "value", color: "#8b5cf6", name: "%" }]} loading={query.isLoading} />
      </div>
      <div className="grid gap-3 lg:grid-cols-2">
        <LineChartCard title="Available Memory" data={memAvail} lines={[{ dataKey: "value", color: "#22c55e", name: "Bytes" }]} loading={query.isLoading} />
        <StackedAreaCard title="Swap Usage" data={swapUsage} areas={SWAP_AREAS} loading={query.isLoading} />
      </div>
      <div className="grid gap-3 lg:grid-cols-2">
        <StackedAreaCard title="Page Faults" data={pageFaults} areas={FAULT_AREAS} loading={query.isLoading} />
        <LineChartCard title="Swap I/O (Major)" data={swapIO} lines={[
          { dataKey: "page_in", color: "#3b82f6", name: "Page In" },
          { dataKey: "page_out", color: "#f59e0b", name: "Page Out" },
        ]} loading={query.isLoading} />
      </div>
    </div>
  )
}
