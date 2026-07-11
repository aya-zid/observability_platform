import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom"
import { getDiscoveryHosts } from "@/api/discovery"
import { cn } from "@/lib/utils"
import { ArrowLeft } from "lucide-react"
import { OverviewTab } from "./OverviewTab"
import { MetricsTab } from "./MetricsTab"

const TABS = [
  { key: "overview", label: "Overview" },
  { key: "metrics", label: "Metrics & Dashboards" },
  { key: "logs", label: "Logs" },
] as const

export default function HostDetailPage() {
  const { machineId = "" } = useParams<{ machineId?: string }>()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = searchParams.get("tab") || "overview"

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const host = useMemo(
    () => (hostsQuery.data ?? []).find((h) => h.machine_id === machineId),
    [hostsQuery.data, machineId],
  )

  const setTab = (tab: string) => {
    if (tab === "logs") {
      navigate(`/hosts/${machineId}/logs`)
      return
    }
    setSearchParams({ tab }, { replace: true })
  }

  if (!machineId) {
    return <div className="p-6 text-sm text-red-600">Missing machine ID.</div>
  }

  return (
    <div className="space-y-6 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3 pb-2">
        <div className="flex items-center gap-4">
          <Link
            to="/hosts"
            className="p-2 rounded-lg text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors"
          >
            <ArrowLeft className="w-5 h-5" />
          </Link>
          <div>
            <h1 className="text-2xl font-bold">{host?.hostname ?? machineId}</h1>
            <p className="mt-0.5 text-sm text-muted-foreground font-mono">{machineId}</p>
          </div>
        </div>
      </div>

      <div className="flex gap-1 rounded-lg border border-border bg-muted p-1 w-fit">
        {TABS.map((tab) => (
          <button
            key={tab.key}
            onClick={() => setTab(tab.key)}
            className={cn(
              "px-4 py-2 rounded-md text-sm font-medium transition-colors",
              activeTab === tab.key
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === "overview" ? <OverviewTab host={host} machineId={machineId} /> : null}
      {activeTab === "metrics" ? <MetricsTab machineId={machineId} /> : null}
    </div>
  )
}
