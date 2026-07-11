import { useMemo } from "react"
import { useHostDisk } from "@/pages/hosts/hooks/useHostMetric"
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

const IO_DIR_AREAS = [
  { dataKey: "read", color: "#3b82f6", name: "Read" },
  { dataKey: "write", color: "#f59e0b", name: "Write" },
]

export function DiskSection({ machineId, params }: Props) {
  const query = useHostDisk(machineId, params)
  const d = query.data as Record<string, any> | undefined

  const diskUtil = useMemo(() => formatTimePoints(d?.utilization), [d])
  const diskIO = useMemo(() => formatTimePoints(d?.io), [d])
  const diskOps = useMemo(() => formatTimePoints(d?.ops), [d])
  const diskLatency = useMemo(() => formatTimePoints(d?.latency), [d])
  const diskQueue = useMemo(() => formatTimePoints(d?.queue_depth), [d])
  const inode = useMemo(() => formatTimePoints(d?.inode_usage), [d])
  const diskSpace = useMemo(() => formatTimePoints(d?.space), [d])

  const diskUtilLines = useMemo(() => {
    if (!diskUtil.length) return []
    return Object.keys(diskUtil[0]).filter((k) => k !== "time").map((key, i) => ({ dataKey: key, color: PALETTE[i % PALETTE.length], name: key }))
  }, [diskUtil])

  const diskSpaceAreas = useMemo(() => {
    if (!diskSpace.length) return []
    const keys = new Set<string>()
    for (const row of diskSpace) for (const k of Object.keys(row)) if (k !== "time") keys.add(k)
    const stateColor: Record<string, string> = { used: "#ef4444", free: "#22c55e", reserved: "#f59e0b" }
    const assigned = new Map<string, string>()
    let idx = 0
    for (const k of keys) {
      const suffix = k.split("_").pop() ?? ""
      assigned.set(k, stateColor[suffix] ?? PALETTE[idx++ % PALETTE.length])
    }
    return Array.from(keys).map((k) => ({ dataKey: k, color: assigned.get(k) ?? PALETTE[0], name: k }))
  }, [diskSpace])

  return (
    <div className="grid gap-6">
      <div className="grid gap-4 lg:grid-cols-2">
        <LineChartCard title="Disk Usage by Mountpoint" data={diskUtil} lines={diskUtilLines} loading={query.isLoading} />
        <LineChartCard title="Disk Queue Depth" data={diskQueue} lines={[{ dataKey: "value", color: "#f59e0b", name: "Pending Ops" }]} loading={query.isLoading} />
      </div>
      <div className="grid gap-4 lg:grid-cols-2">
        <StackedAreaCard title="Disk I/O Throughput" data={diskIO} areas={IO_DIR_AREAS} loading={query.isLoading} />
        <StackedAreaCard title="Disk I/O Operations" data={diskOps} areas={IO_DIR_AREAS} loading={query.isLoading} />
      </div>
      <div className="grid gap-4 lg:grid-cols-3">
        <LineChartCard title="Avg I/O Latency" data={diskLatency} lines={[
          { dataKey: "read", color: "#3b82f6", name: "Read (ms)" },
          { dataKey: "write", color: "#f59e0b", name: "Write (ms)" },
        ]} loading={query.isLoading} />
        <LineChartCard title="Inode Usage (/)" data={inode} lines={[{ dataKey: "value", color: "#06b6d4", name: "%" }]} loading={query.isLoading} />
      </div>
      <div>
        <StackedAreaCard title="Disk Space per Mountpoint" data={diskSpace} areas={diskSpaceAreas} loading={query.isLoading} />
      </div>
    </div>
  )
}
