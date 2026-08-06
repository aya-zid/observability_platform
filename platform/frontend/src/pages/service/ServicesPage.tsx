import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"
import { Link } from "react-router-dom"

import {
  getCatalog,
  getDiscoveryHosts,
  getDiscoveryServices,
  type DiscoveryService,
} from "@/api/discovery"
import { cn } from "@/lib/utils"
import { Search, Activity, Server, FileText, BarChart3, Settings, Wifi, WifiOff, CheckCircle, XCircle } from "lucide-react"

function HostSidebar({
  hosts,
  selectedHost,
  onSelectHost,
}: {
  hosts: { machine_id: string; hostname: string; os_name: string; arch?: string; status: string; private_ip_v4?: string; last_seen_at: string }[]
  selectedHost: string
  onSelectHost: (id: string) => void
}) {
  return (
    <div className="w-56 border-r border-border bg-card flex flex-col h-full shrink-0">
      <div className="p-3 border-b border-border">
        <h2 className="text-xs font-semibold text-foreground uppercase tracking-wider flex items-center gap-1.5">
          <Server className="w-3.5 h-3.5 text-primary" />
          Hosts
        </h2>
        <p className="text-xs text-muted-foreground mt-0.5">{hosts.length} discovered</p>
      </div>
      <div className="flex-1 overflow-y-auto p-1.5 space-y-0.5">
        <button onClick={() => onSelectHost("")}
          className={cn(
            "w-full text-left px-2.5 py-2 rounded-md text-xs transition-all",
            selectedHost === ""
              ? "bg-primary/15 text-primary border border-primary/30"
              : "text-secondary-foreground hover:bg-surface-hover"
          )}>
          All Hosts
        </button>
        {hosts.map((host) => (
          <button key={host.machine_id} onClick={() => onSelectHost(host.machine_id)}
            className={cn(
              "w-full text-left px-2.5 py-2 rounded-md transition-all group",
              selectedHost === host.machine_id ? "bg-primary/15 border border-primary/30" : "hover:bg-surface-hover"
            )}>
            <div className="flex items-center justify-between">
              <span className={cn("text-xs font-medium", selectedHost === host.machine_id ? "text-primary" : "text-foreground")}>
                {host.hostname}
              </span>
              {host.status === "online" ? (
                <Wifi className="w-3 h-3 text-green-500" />
              ) : (
                <WifiOff className="w-3 h-3 text-red-500" />
              )}
            </div>
            <div className="flex items-center justify-between mt-0.5">
              <span className="text-xs text-muted-foreground">{host.private_ip_v4 || host.machine_id.slice(0, 12)}</span>
              <span className="text-xs text-muted-foreground">{host.os_name}</span>
            </div>
          </button>
        ))}
      </div>
    </div>
  )
}

export default function ServicesPage() {
  const [selectedHost, setSelectedHost] = useState("")
  const [search, setSearch] = useState("")

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })
  const catalogQuery = useQuery({
    queryKey: ["catalog"],
    queryFn: getCatalog,
  })
  const servicesQuery = useQuery({
    queryKey: ["discovery-services", selectedHost],
    queryFn: () => getDiscoveryServices(selectedHost || undefined),
  })

  const hosts = useMemo(() => hostsQuery.data ?? [], [hostsQuery.data])
  const catalog = useMemo(() => catalogQuery.data ?? [], [catalogQuery.data])
  const allServices = useMemo(() => servicesQuery.data ?? [], [servicesQuery.data])

  const services = useMemo(() => {
    let filtered = selectedHost ? allServices.filter(s => s.machine_id === selectedHost) : allServices
    if (search) {
      const q = search.toLowerCase()
      filtered = filtered.filter(s => s.service_name.toLowerCase().includes(q) || (s.category && s.category.toLowerCase().includes(q)))
    }
    return filtered
  }, [allServices, selectedHost, search])

  const categoryLabelByKey: Record<string, string> = {}
  for (const item of catalog) categoryLabelByKey[item.category] = item.category

  const servicesByCategory: Record<string, DiscoveryService[]> = {}
  for (const service of services) {
    const cat = service.is_supported ? (service.category || "Other") : "Unknown"
    if (!servicesByCategory[cat]) servicesByCategory[cat] = []
    servicesByCategory[cat].push(service)
  }

  const categoryOrder = useMemo(() => {
    const cats = Object.keys(servicesByCategory)
    cats.sort((a, b) => {
      if (a === "Unknown") return 1
      if (b === "Unknown") return -1
      const aIdx = catalog.findIndex((c) => c.category === a)
      const bIdx = catalog.findIndex((c) => c.category === b)
      if (aIdx === -1 && bIdx === -1) return a.localeCompare(b)
      if (aIdx === -1) return 1
      if (bIdx === -1) return -1
      return aIdx - bIdx
    })
    return cats
  }, [servicesByCategory, catalog])

  useEffect(() => {
    if (!selectedHost && hosts.length > 0) {
      setSelectedHost(hosts[0].machine_id)
    }
  }, [hosts, selectedHost])

  const totalOnline = hosts.filter(h => h.status === "online").length

  return (
    <div className="flex h-full overflow-hidden">
      <HostSidebar hosts={hosts} selectedHost={selectedHost} onSelectHost={setSelectedHost} />
      <div className="flex-1 flex flex-col overflow-hidden">
        <header className="border-b border-border bg-card/50 backdrop-blur-sm px-4 py-2 shrink-0">
          <div className="flex items-center justify-between">
            <div>
              <h1 className="text-base font-bold text-foreground flex items-center gap-2">
                <Activity className="w-3.5 h-3.5 text-primary" />
                {selectedHost ? hosts.find(h => h.machine_id === selectedHost)?.hostname || "All Services" : "All Services"}
              </h1>
              <p className="text-xs text-muted-foreground mt-0.5">
                {selectedHost
                  ? `${hosts.find(h => h.machine_id === selectedHost)?.os_name || ""} · ${services.length} services`
                  : `${totalOnline}/${hosts.length} hosts online · ${services.length} services discovered`}
              </p>
            </div>
            <div className="relative">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3 h-3 text-muted-foreground" />
              <input
                type="text" placeholder="Search services…" value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-7 pr-2.5 py-1 bg-secondary border border-border rounded-lg text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25 w-56 transition-colors"
              />
            </div>
          </div>
        </header>
        <main className="flex-1 overflow-y-auto p-3 md:p-4 space-y-4">
          {!selectedHost && !servicesQuery.isLoading && (
            <div className="text-sm text-muted-foreground">Select a host from the sidebar.</div>
          )}
          {servicesQuery.isLoading && <div className="text-sm text-muted-foreground">Loading services…</div>}
          {categoryOrder.map((category) => (
            <section key={category} className="animate-fade-in">
              <h2 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-3 flex items-center gap-1.5">
                {categoryLabelByKey[category] || category}
                <span className="text-xs bg-secondary text-secondary-foreground px-1.5 py-0.5 rounded-full ml-1">{servicesByCategory[category].length}</span>
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3">
                {servicesByCategory[category].map((service) => {
                  const bothOn = service.log_collection_on && service.metric_collection_on
                  return (
                    <div key={service.id} className="bg-card border border-border rounded-lg p-3 hover:border-primary/30 transition-all group">
                      <div className="flex items-start justify-between mb-2">
                        <div className="flex-1 min-w-0">
                          <h3 className="text-xs font-semibold text-foreground">{service.service_name}</h3>
                          <p className="text-xs text-muted-foreground mt-0.5">
                            {service.version ? `v${service.version}` : ""}{service.port ? ` · port ${service.port}` : ""}
                          </p>
                        </div>
                        <div className="flex items-center gap-1 shrink-0 ml-2">
                          {service.status === "running" ? (
                            <span className="inline-flex items-center gap-1 text-xs font-semibold text-green-500"><Wifi className="w-2.5 h-2.5" /> running</span>
                          ) : service.status === "warning" ? (
                            <span className="inline-flex items-center gap-1 text-xs font-semibold text-yellow-500"><Wifi className="w-2.5 h-2.5" /> warning</span>
                          ) : (
                            <span className="inline-flex items-center gap-1 text-xs font-semibold text-red-500"><WifiOff className="w-2.5 h-2.5" /> stopped</span>
                          )}
                        </div>
                        {service.is_supported && (
                          <div className="flex items-center gap-1 shrink-0 ml-2">
                            <Link to={`/services/${service.id}`}
                              className="p-1 rounded-md text-muted-foreground hover:text-primary hover:bg-primary/10 transition-colors" title="Configure">
                              <Settings className="w-3 h-3" />
                            </Link>
                          </div>
                        )}
                      </div>

                      {service.is_supported && (
                        <div className={cn("inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-xs font-medium mb-2",
                          bothOn ? "text-green-600 dark:text-green-400 bg-green-500/10" : "text-muted-foreground bg-secondary"
                        )}>
                          {bothOn ? <CheckCircle className="w-2.5 h-2.5" /> : <XCircle className="w-2.5 h-2.5" />}
                          {bothOn ? "Active" : "Not configured"}
                        </div>
                      )}

                      {!service.is_supported && (
                        <div className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full text-xs font-medium mb-2 text-muted-foreground bg-secondary">
                          <XCircle className="w-2.5 h-2.5" />
                          Unknown service
                        </div>
                      )}

                      <div className="space-y-1.5">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-1.5">
                            <FileText className="w-2.5 h-2.5 text-muted-foreground" />
                            <span className="text-xs text-muted-foreground">Log Collection</span>
                          </div>
                          <span className={cn("text-xs font-semibold", service.log_collection_on ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400")}>
                            {service.log_collection_on ? "ON" : "OFF"}
                          </span>
                        </div>
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-1.5">
                            <BarChart3 className="w-2.5 h-2.5 text-muted-foreground" />
                            <span className="text-xs text-muted-foreground">Metrics Collection</span>
                          </div>
                          <span className={cn("text-xs font-semibold", service.metric_collection_on ? "text-green-600 dark:text-green-400" : "text-red-600 dark:text-red-400")}>
                            {service.metric_collection_on ? "ON" : "OFF"}
                          </span>
                        </div>
                        {bothOn && service.is_supported && (
                          <div className="flex gap-1.5 pt-1.5">
                            <ServiceQuickLinks serviceId={service.id} serviceName={service.service_name} />
                          </div>
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            </section>
          ))}
          {!servicesQuery.isLoading && services.length === 0 && selectedHost && (
            <div className="flex flex-col items-center justify-center h-64 text-muted-foreground">
              <Search className="w-8 h-8 mb-3 opacity-30" />
              <p className="text-sm">No services found</p>
            </div>
          )}
        </main>
      </div>
    </div>
  )
}

const dashboardRoutes: Record<string, string> = {
  nginx: "/services/nginx-dashboard",
  apache: "/services/apache-dashboard",
  postgresql: "/services/postgres-dashboard",
  postgres: "/services/postgres-dashboard",
  phpfpm: "/services/phpfpm-dashboard",
  redis: "/services/redis-dashboard",
  mariadb: "/services/mariadb-dashboard",
  mysql: "/services/mariadb-dashboard",
}

const logsRoutes: Record<string, string> = {
  nginx: "/services/nginx-logs",
  apache: "/services/apache-logs",
  postgresql: "/services/postgres-logs",
  postgres: "/services/postgres-logs",
  phpfpm: "/services/phpfpm-logs",
  redis: "/services/redis-logs",
  mariadb: "/services/mariadb-logs",
  mysql: "/services/mariadb-logs",
}

function ServiceQuickLinks({ serviceId, serviceName }: { serviceId: number; serviceName: string }) {
  const name = serviceName.toLowerCase()
  const matchRoute = (routes: Record<string, string>) =>
    Object.entries(routes).find(([key]) => {
      if (name === key) return true
      if (name.startsWith(key + "-")) return true
      if (name.startsWith(key + "_")) return true
      if (name.startsWith(key) && name.length > key.length && /^\d+$/.test(name.slice(key.length))) return true
      return false
    })?.[1]
  const dashPath = matchRoute(dashboardRoutes)
  const logsPath = matchRoute(logsRoutes)
  return (
    <>
      {dashPath && (
        <Link
          to={`${dashPath}/${serviceId}`}
          className="flex-1 text-center px-1.5 py-1 bg-primary/10 hover:bg-primary/20 text-primary border border-primary/25 rounded-md text-xs font-semibold transition-colors"
        >
          Dashboard
        </Link>
      )}
      {logsPath && (
        <Link
          to={`${logsPath}/${serviceId}`}
          className="flex-1 text-center px-1.5 py-1 bg-secondary hover:bg-secondary/80 text-secondary-foreground border border-border rounded-md text-xs font-semibold transition-colors"
        >
          Logs
        </Link>
      )}
    </>
  )
}
