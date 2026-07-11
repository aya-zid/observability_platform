import { useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"

import { getDiscoveryHosts } from "@/api/discovery"
import type { DiscoveryHost } from "@/api/discovery"
import { cn } from "@/lib/utils"
import { Page, PageHeader } from "@/components/shared/Page"

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

export default function HostsPage() {
  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const hosts = useMemo(() => hostsQuery.data ?? [], [hostsQuery.data])

  return (
    <Page>
      <PageHeader
        title="Hosts"
        description="View all discovered hosts, their services, system metrics, and logs."
      />
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {hostsQuery.isLoading ? (
          <div className="col-span-full text-sm text-muted-foreground">Loading hosts...</div>
        ) : null}
        {!hostsQuery.isLoading && hosts.length === 0 ? (
          <div className="col-span-full text-sm text-muted-foreground">No hosts discovered yet.</div>
        ) : null}
        {hosts.map((host) => (
          <Link
            key={host.machine_id}
            to={`/hosts/${host.machine_id}`}
            className="block rounded-2xl border border-border bg-card/70 backdrop-blur p-5 hover:border-primary/50 hover:shadow-sm transition-all"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="font-semibold text-foreground truncate">{host.hostname}</div>
                <div className="mt-1 text-xs text-muted-foreground">
                  {host.os_name} {host.arch ? `\u2022 ${host.arch}` : null}
                </div>
              </div>
              <div className={cn("shrink-0 size-2 rounded-full mt-2", statusColor(host.status))} />
            </div>
            <div className="mt-3 flex items-center gap-3 text-xs text-muted-foreground">
              <span className="font-mono">{host.machine_id.slice(0, 12)}...</span>
              <span>{host.agent_version ? `v${host.agent_version}` : null}</span>
            </div>
            <div className="mt-2 text-xs text-muted-foreground">
              Last seen: {formatLastSeen(host.last_seen_at)}
            </div>
          </Link>
        ))}
      </div>
    </Page>
  )
}
