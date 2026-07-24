import { useState, useMemo } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"

import { getDiscoveryHosts } from "@/api/discovery"
import { Page, PageHeader } from "@/components/shared/Page"
import { cn } from "@/lib/utils"
import { Server, ChevronDown, ChevronUp, ExternalLink } from "lucide-react"

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
  const onlineCount = hosts.filter(h => h.status === "online").length
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  return (
    <Page>
      <PageHeader
        title="Infrastructure"
        description={`${onlineCount}/${hosts.length} hosts online`}
        icon={Server}
      />
      <div className="space-y-3">
        {hostsQuery.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading hosts…</div>
        ) : hosts.length === 0 ? (
          <div className="text-sm text-muted-foreground">No hosts discovered yet.</div>
        ) : (
          hosts.map((host) => {
            const isExpanded = expanded[host.machine_id] ?? false
            return (
              <div
                key={host.machine_id}
                className="bg-card border border-border rounded-xl overflow-hidden transition-all"
              >
                <div className="flex items-center gap-3 px-4 py-3.5">
                  <div className="w-10 h-10 rounded-lg bg-secondary flex items-center justify-center shrink-0">
                    <Server className="w-5 h-5 text-muted-foreground" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <h3 className="text-sm font-semibold text-foreground">{host.hostname}</h3>
                      <span className={cn(
                        "w-2 h-2 rounded-full shrink-0",
                        host.status === "online" ? "bg-green-500" : "bg-red-500"
                      )} />
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {(host as any).private_ip_v4 || host.machine_id.slice(0, 12)} · {host.os_name} · Last seen {formatLastSeen(host.last_seen_at)}
                    </p>
                  </div>
                  <div className="flex items-center gap-1">
                    <Link
                      to={`/hosts/${host.machine_id}`}
                      className="px-3 py-1.5 rounded-md bg-primary/10 hover:bg-primary/20 text-primary border border-primary/25 text-xs font-semibold flex items-center gap-1.5 transition-colors"
                    >
                      Dashboard <ExternalLink className="w-3 h-3" />
                    </Link>
                    <button
                      onClick={() => setExpanded(p => ({ ...p, [host.machine_id]: !isExpanded }))}
                      className="w-8 h-8 rounded-md flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    >
                      {isExpanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
                    </button>
                  </div>
                </div>
                {isExpanded && (
                  <div className="border-t border-border px-4 py-3.5 bg-muted/30">
                    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                      <div>
                        <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">Machine ID</p>
                        <p className="mt-0.5 text-xs font-mono text-foreground">{host.machine_id}</p>
                      </div>
                      <div>
                        <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">Operating System</p>
                        <p className="mt-0.5 text-xs text-foreground">{host.os_name}</p>
                      </div>
                      <div>
                        <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">Architecture</p>
                        <p className="mt-0.5 text-xs text-foreground">{host.arch}</p>
                      </div>
                      <div>
                        <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">Agent Version</p>
                        <p className="mt-0.5 text-xs text-foreground">{host.agent_version}</p>
                      </div>
                      <div>
                        <p className="text-[10px] font-semibold text-muted-foreground uppercase tracking-wider">Status</p>
                        <p className="mt-0.5 text-xs text-foreground flex items-center gap-1.5">
                          <span className={cn("w-1.5 h-1.5 rounded-full", host.status === "online" ? "bg-green-500" : "bg-red-500")} />
                          <span className="capitalize">{host.status}</span>
                          <span className="text-muted-foreground">·</span>
                          <span className="text-muted-foreground">{formatLastSeen(host.last_seen_at)}</span>
                        </p>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            )
          })
        )}
      </div>
    </Page>
  )
}
