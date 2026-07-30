import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useParams } from "react-router-dom"

import { getDiscoveryHosts } from "@/api/discovery"
import { ArrowLeft, FileText } from "lucide-react"
import { MetricsTab } from "./MetricsTab"

export default function HostDetailPage() {
  const { machineId = "" } = useParams<{ machineId?: string }>()

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const host = useMemo(
    () => (hostsQuery.data ?? []).find((h) => h.machine_id === machineId),
    [hostsQuery.data, machineId],
  )

  if (!machineId) {
    return <div className="p-6 text-sm text-red-600">Missing machine ID.</div>
  }

  return (
    <div className="max-w-6xl mx-auto w-full py-4 px-4 md:px-6 space-y-5">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Link
            to="/hosts"
            className="p-1.5 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
          >
            <ArrowLeft className="w-4 h-4" />
          </Link>
          <div>
            <h1 className="text-lg font-bold tracking-tight text-foreground">
              {host?.hostname ?? machineId}
            </h1>
            <p className="text-xs text-muted-foreground">
              {host ? `${host.os_name} ${host.arch || ""}` : machineId}
            </p>
          </div>
        </div>
        <Link
          to={`/hosts/${machineId}/logs`}
          className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3 py-1.5 text-xs font-semibold text-primary-foreground hover:bg-primary/90 transition-colors shadow-accent-glow"
        >
          <FileText className="w-3.5 h-3.5" /> View Logs
        </Link>
      </div>

      <MetricsTab machineId={machineId} />
    </div>
  )
}
