import { useMemo } from "react"
import { useHostSummary } from "@/pages/hosts/hooks/useHostMetric"
import { StatCard } from "@/components/shared/StatCard"
import { formatBytes } from "@/lib/queryHelpers"

export function SummaryCards({ machineId }: { machineId: string }) {
  const query = useHostSummary(machineId)

  const cards = useMemo(() => query.data as Record<string, number | undefined> | undefined, [query.data])

  const val = (k: string) => cards?.[k] as number | undefined

  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-7">
      <StatCard title="CPU Usage" value={val("cpu_usage_percent") != null ? `${Number(val("cpu_usage_percent")!.toFixed(1))}%` : "\u2014"} loading={query.isLoading} />
      <StatCard title="Memory Used" value={val("memory_used_percent") != null ? `${Number(val("memory_used_percent")!.toFixed(1))}%` : "\u2014"} loading={query.isLoading} />
      <StatCard title="Disk (/) Used" value={val("disk_used_percent") != null ? `${Number(val("disk_used_percent")!.toFixed(1))}%` : "\u2014"} loading={query.isLoading} />
      <StatCard title="Load Average" value={val("load_average_1m") != null ? Number(val("load_average_1m")!.toFixed(2)).toString() : "\u2014"} loading={query.isLoading} />
      <StatCard title="Net In" value={val("network_in_rate") != null ? formatBytes(val("network_in_rate")!) + "/s" : "\u2014"} loading={query.isLoading} />
      <StatCard title="Net Out" value={val("network_out_rate") != null ? formatBytes(val("network_out_rate")!) + "/s" : "\u2014"} loading={query.isLoading} />
      <StatCard title="Swap Used" value={val("swap_used_percent") != null ? `${Number(val("swap_used_percent")!.toFixed(1))}%` : "\u2014"} loading={query.isLoading} />
    </div>
  )
}
