import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"
import { getDiscoveryServices } from "@/api/discovery"
import type { DiscoveryHost } from "@/api/discovery"
import { cn } from "@/lib/utils"
import { Server, Activity } from "lucide-react"

function statusColor(status: DiscoveryHost["status"]) {
  switch (status) {
    case "online":  return "bg-emerald-500"
    case "warning": return "bg-amber-500"
    case "offline": return "bg-red-500"
  }
}

function formatLastSeen(dateStr: string) {
  const d = new Date(dateStr)
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60_000) return "just now"
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}m ago`
  if (diff < 86400_000) return `${Math.floor(diff / 3600_000)}h ago`
  return `${Math.floor(diff / 86400_000)}d ago`
}

export function OverviewTab({ host, machineId }: { host: DiscoveryHost | undefined; machineId: string }) {
  const servicesQuery = useQuery({
    queryKey: ["discovery-services", machineId],
    queryFn: () => getDiscoveryServices(machineId),
    enabled: !!machineId,
  })

  const services = useMemo(() => servicesQuery.data ?? [], [servicesQuery.data])
  const hostServices = services.filter((s) => s.category !== "container" || !s.category.startsWith("container"))

  return (
    <div className="grid gap-6">
      <div className="rounded-2xl border border-border bg-card/70 backdrop-blur p-6">
        <h2 className="text-lg font-semibold flex items-center gap-2 mb-4">
          <Server className="w-5 h-5 text-primary" />
          Host Information
        </h2>
        {host ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Hostname</div>
              <div className="mt-1 font-medium">{host.hostname}</div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Machine ID</div>
              <div className="mt-1 font-mono text-sm">{host.machine_id}</div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Operating System</div>
              <div className="mt-1">{host.os_name}</div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Architecture</div>
              <div className="mt-1">{host.arch}</div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Agent Version</div>
              <div className="mt-1">{host.agent_version}</div>
            </div>
            <div>
              <div className="text-sm text-muted-foreground uppercase tracking-wide">Status</div>
              <div className="mt-1 flex items-center gap-2">
                <span className={cn("size-2 rounded-full", statusColor(host.status))} />
                <span className="capitalize">{host.status}</span>
                <span className="text-muted-foreground">&middot;</span>
                <span className="text-muted-foreground">{formatLastSeen(host.last_seen_at)}</span>
              </div>
            </div>
          </div>
        ) : (
          <div className="text-sm text-muted-foreground">Loading host info...</div>
        )}
      </div>

      <div className="rounded-2xl border border-border bg-card/70 backdrop-blur p-6">
        <h2 className="text-lg font-semibold flex items-center gap-2 mb-4">
          <Activity className="w-5 h-5 text-primary" />
          Services ({hostServices.length})
        </h2>
        {servicesQuery.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading services...</div>
        ) : hostServices.length === 0 ? (
          <div className="text-sm text-muted-foreground">No services discovered on this host.</div>
        ) : (
          <div className="grid gap-3">
            {hostServices.map((svc) => (
              <Link
                key={svc.id}
                to={`/services/${svc.id}`}
                className="flex items-center justify-between rounded-xl border border-border bg-card/50 p-3 hover:border-primary/40 transition-colors"
              >
                <div>
                  <div className="font-medium">{svc.service_name}</div>
                  <div className="text-sm text-muted-foreground mt-0.5">
                    {svc.category} {svc.port ? `\u2022 Port ${svc.port}` : null} {svc.version ? `\u2022 v${svc.version}` : null}
                  </div>
                </div>
                <div className="flex items-center gap-3 text-sm">
                  <span className={cn(svc.log_collection_on ? "text-emerald-500" : "text-muted-foreground")}>
                    Logs {svc.log_collection_on ? "ON" : "OFF"}
                  </span>
                  <span className={cn(svc.metric_collection_on ? "text-emerald-500" : "text-muted-foreground")}>
                    Metrics {svc.metric_collection_on ? "ON" : "OFF"}
                  </span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
