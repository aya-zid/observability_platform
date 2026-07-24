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
    <div className="max-w-6xl mx-auto w-full py-6 px-6 space-y-8">
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-4">
          <Link
            to="/hosts"
            className="p-2 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground">
              {host?.hostname ?? machineId}
            </h1>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {host ? `${host.os_name} ${host.arch || ""}` : machineId}
            </p>
          </div>
        </div>
        <Link
          to={`/hosts/${machineId}/logs`}
          className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-sm font-semibold text-primary-foreground hover:bg-primary/90 transition-colors shadow-accent-glow"
        >
          <FileText className="w-4 h-4" /> View Logs
        </Link>
      </div>

      <MetricsTab machineId={machineId} />
    </div>
  )
}
