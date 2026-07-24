import { useEffect, useMemo, useState } from "react"
import { useQuery } from "@tanstack/react-query"

import { getDiscoveryHosts } from "@/api/discovery"
import {
  getContainerServices, getContainerService, getContainerDashboardSummary,
  getContainerLogs, type ContainerDashboardSummary,
} from "@/api/container"
import { cn } from "@/lib/utils"
import {
  Box, Search, Server, ChevronDown, ChevronUp, Wifi, WifiOff, Activity,
  FileText, X, RefreshCw,
} from "lucide-react"

import { DashboardStatCard, DashboardLinePanel, DASHBOARD_COLORS } from "@/components/shared/DashboardComponents"
import { getStep, mapTimeseries } from "@/lib/dashboardHelpers"
import { formatBytes, formatNumber, formatPercent, getNowSeconds, getStartSeconds, secondsToNanos } from "@/lib/queryHelpers"
import { LogPagination } from "@/components/shared/LogPagination"

type DrawerTab = "dashboard" | "logs"

export default function ContainersPage() {
  const [search, setSearch] = useState("")
  const [expandedHosts, setExpandedHosts] = useState<Record<string, boolean>>({})
  const [drawer, setDrawer] = useState<{ containerId: string; tab: DrawerTab } | null>(null)

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const containersQuery = useQuery({
    queryKey: ["discovery-containers"],
    queryFn: () => getContainerServices(),
  })

  const hosts = useMemo(() => hostsQuery.data ?? [], [hostsQuery.data])
  const containers = useMemo(() => containersQuery.data ?? [], [containersQuery.data])
  const activeContainer = useMemo(
    () => drawer ? containers.find(c => String(c.id) === drawer.containerId) : null,
    [drawer, containers],
  )

  const filtered = useMemo(() =>
    containers.filter(c => !search ||
      c.container_name.toLowerCase().includes(search.toLowerCase()) ||
      (c.container_image || "").toLowerCase().includes(search.toLowerCase())
    ), [containers, search])

  const grouped = useMemo(() =>
    hosts.map(h => ({
      host: h,
      items: filtered.filter(c => c.machine_id === h.machine_id),
    })).filter(g => g.items.length > 0),
    [hosts, filtered]
  )

  useEffect(() => {
    if (Object.keys(expandedHosts).length === 0 && hosts.length > 0) {
      const initial: Record<string, boolean> = {}
      for (const h of hosts) initial[h.machine_id] = true
      setExpandedHosts(initial)
    }
  }, [hosts])

  const toggleHost = (id: string) => setExpandedHosts(p => ({ ...p, [id]: !p[id] }))

  const runningCount = containers.length

  return (
    <div className="flex-1 overflow-y-auto">
      <header className="border-b border-border bg-card/50 backdrop-blur-sm px-6 py-4 shrink-0">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
              <Box className="w-5 h-5 text-primary" /> Containers
            </h1>
            <p className="text-sm text-muted-foreground mt-0.5">
              {runningCount} running · {containers.length} total across {hosts.length} hosts
            </p>
          </div>
          <div className="relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text" placeholder="Search containers..." value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-9 pr-4 py-2 bg-secondary border border-border rounded-lg text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25 w-72 transition-colors"
            />
          </div>
        </div>
      </header>

      <main className="max-w-6xl mx-auto p-6 space-y-4">
        {containersQuery.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading containers…</div>
        ) : grouped.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-muted-foreground">
            <Box className="w-10 h-10 mb-3 opacity-30" />
            <p className="text-sm">No containers found</p>
          </div>
        ) : (
          grouped.map(({ host, items }) => {
            const open = expandedHosts[host.machine_id]
            return (
              <section key={host.machine_id} className="bg-card border border-border rounded-lg overflow-hidden">
                <button onClick={() => toggleHost(host.machine_id)}
                  className="w-full flex items-center gap-3 px-4 py-3 hover:bg-surface-hover/30 transition-colors">
                  <div className="w-9 h-9 rounded-md bg-primary/15 flex items-center justify-center shrink-0 ring-1 ring-primary/25">
                    <Server className="w-4 h-4 text-primary" />
                  </div>
                  <div className="flex-1 text-left">
                    <div className="flex items-center gap-2">
                      <h2 className="text-sm font-semibold text-foreground">{host.hostname}</h2>
                      {host.status === "online"
                        ? <span className="inline-flex items-center gap-1 text-[10px] font-semibold text-green-500"><Wifi className="w-3 h-3" /> ONLINE</span>
                        : <span className="inline-flex items-center gap-1 text-[10px] font-semibold text-red-500"><WifiOff className="w-3 h-3" /> OFFLINE</span>}
                    </div>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {(host as any).private_ip_v4 || host.machine_id.slice(0, 12)} · {host.os_name} · {items.length} container{items.length !== 1 ? "s" : ""}
                    </p>
                  </div>
                  {open ? <ChevronUp className="w-4 h-4 text-muted-foreground" /> : <ChevronDown className="w-4 h-4 text-muted-foreground" />}
                </button>

                {open && (
                  <div className="border-t border-border divide-y divide-border">
                    {items.map(c => (
                      <div key={c.id} className="p-4 hover:bg-surface-hover/20 transition-colors">
                        <div className="flex items-start gap-3">
                          <div className="w-9 h-9 rounded-md bg-secondary flex items-center justify-center shrink-0">
                            <Box className="w-4 h-4 text-muted-foreground" />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <h3 className="text-sm font-semibold text-foreground">{c.container_name}</h3>
                              <span className="text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded border text-green-600 dark:text-green-400 bg-green-500/10 border-green-500/30">
                                running
                              </span>
                              <code className="text-[10px] font-mono text-muted-foreground">{c.service_name}</code>
                            </div>
                            {c.container_image && (
                              <p className="text-xs text-muted-foreground mt-0.5 truncate">{c.container_image}</p>
                            )}
                          </div>
                          <div className="flex flex-col gap-2 shrink-0">
                            <button
                              onClick={() => setDrawer({ containerId: String(c.id), tab: "dashboard" })}
                              className="px-3 py-1.5 bg-primary/10 hover:bg-primary/20 text-primary border border-primary/25 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors">
                              <Activity className="w-3.5 h-3.5" /> Dashboard
                            </button>
                            <button
                              onClick={() => setDrawer({ containerId: String(c.id), tab: "logs" })}
                              className="px-3 py-1.5 bg-secondary hover:bg-secondary/80 text-secondary-foreground border border-border rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors">
                              <FileText className="w-3.5 h-3.5" /> Logs
                            </button>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </section>
            )
          })
        )}
      </main>

      {activeContainer && drawer && (
        <ContainerDrawer
          containerId={String(activeContainer.id)}
          containerName={activeContainer.container_name}
          containerImage={activeContainer.container_image}
          tab={drawer.tab}
          onTab={(t) => setDrawer({ containerId: String(activeContainer.id), tab: t })}
          onClose={() => setDrawer(null)}
        />
      )}
    </div>
  )
}

function ContainerDrawer({
  containerId, containerName, containerImage, tab, onTab, onClose,
}: {
  containerId: string
  containerName: string
  containerImage?: string
  tab: DrawerTab
  onTab: (t: DrawerTab) => void
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-background/70 backdrop-blur-sm" onClick={onClose} />
      <div className="relative w-full max-w-3xl h-full bg-card border-l border-border shadow-2xl flex flex-col">
        <div className="flex items-center justify-between px-5 py-4 border-b border-border shrink-0">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-foreground truncate">{containerName}</h2>
            {containerImage && (
              <p className="text-xs text-muted-foreground truncate mt-0.5">{containerImage}</p>
            )}
          </div>
          <button onClick={onClose} className="w-8 h-8 rounded-md flex items-center justify-center text-muted-foreground hover:text-foreground hover:bg-muted transition-colors shrink-0 ml-3">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="border-b border-border px-5 flex gap-1 shrink-0">
          {(["dashboard", "logs"] as DrawerTab[]).map((t) => (
            <button
              key={t}
              onClick={() => onTab(t)}
              className={cn(
                "px-4 py-2.5 text-xs font-semibold uppercase tracking-wider border-b-2 transition-colors",
                tab === t
                  ? "text-primary border-primary"
                  : "text-muted-foreground border-transparent hover:text-foreground"
              )}
            >
              {t === "dashboard" ? "Dashboard" : "Logs"}
            </button>
          ))}
        </div>

        <div className="flex-1 overflow-y-auto">
          {tab === "dashboard" ? (
            <ContainerDashboardPanel containerId={containerId} />
          ) : (
            <ContainerLogsPanel containerId={containerId} />
          )}
        </div>
      </div>
    </div>
  )
}

function ContainerDashboardPanel({ containerId }: { containerId: string }) {
  const serviceQuery = useQuery({
    queryKey: ["discovery-container", containerId],
    queryFn: () => getContainerService(containerId),
    enabled: !!containerId,
  })

  const dashQuery = useQuery<ContainerDashboardSummary>({
    queryKey: ["container-dashboard-summary", containerId, "1h"],
    enabled: !!containerId && !!serviceQuery.data,
    queryFn: () => getContainerDashboardSummary(containerId, {
      start: secondsToNanos(getStartSeconds(3600)),
      end: secondsToNanos(getNowSeconds()),
      step: getStep(3600),
    }),
    placeholderData: (previousData) => previousData,
    staleTime: 20_000,
    refetchInterval: 25_000,
  })

  const data = dashQuery.data
  const cpu = mapTimeseries(data?.cpu_timeseries ?? [])
  const mem = mapTimeseries(data?.mem_timeseries ?? [])
  const net = mapTimeseries(data?.net_timeseries ?? [])
  const blockIO = mapTimeseries(data?.block_timeseries ?? [])

  if (dashQuery.isLoading) {
    return <div className="flex items-center justify-center py-20 text-sm text-muted-foreground">Loading dashboard...</div>
  }

  return (
    <div className="p-5 space-y-6">
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label="CPU" value={data?.cpu_percent != null ? formatPercent(data.cpu_percent / 100) : "-"} />
        <DashboardStatCard label="Memory" value={data?.mem_usage_bytes != null ? formatBytes(data.mem_usage_bytes) : "-"} />
        <DashboardStatCard label="Network RX" value={data?.net_rx_bytes_total != null ? formatBytes(data.net_rx_bytes_total) : "-"} />
        <DashboardStatCard label="Network TX" value={data?.net_tx_bytes_total != null ? formatBytes(data.net_tx_bytes_total) : "-"} />
      </div>

      <div className="space-y-5">
        <DashboardLinePanel title="CPU % Over Time" data={cpu} formatter={(value) => formatNumber(value as number, 1)} lines={[{ dataKey: "cpu_percent", name: "CPU %", color: DASHBOARD_COLORS.blue }]} />
        <DashboardLinePanel title="Memory Usage" data={mem} formatter={(value) => formatBytes(value as number)} lines={[{ dataKey: "used", name: "Used", color: DASHBOARD_COLORS.blue }]} />
        <DashboardLinePanel title="Network I/O (rate)" data={net} formatter={(value) => `${formatBytes(value as number)}/s`} lines={[
          { dataKey: "rx", name: "RX", color: DASHBOARD_COLORS.green },
          { dataKey: "tx", name: "TX", color: DASHBOARD_COLORS.blue },
        ]} />
        <DashboardLinePanel title="Block I/O (rate)" data={blockIO} formatter={(value) => `${formatBytes(value as number)}/s`} lines={[
          { dataKey: "read", name: "Read", color: DASHBOARD_COLORS.blue },
          { dataKey: "write", name: "Write", color: DASHBOARD_COLORS.orange },
        ]} />
      </div>

      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <RefreshCw className="w-3 h-3" />
        Auto-refreshing every 25s
      </div>
    </div>
  )
}

const PAGE_SIZE = 50

function ContainerLogsPanel({ containerId }: { containerId: string }) {
  const [search, setSearch] = useState("")
  const [searchInput, setSearchInput] = useState("")
  const [page, setPage] = useState(1)

  const serviceQuery = useQuery({
    queryKey: ["discovery-container", containerId],
    queryFn: () => getContainerService(containerId),
    enabled: !!containerId,
  })

  function commitSearch() {
    setSearch(searchInput)
    setPage(1)
  }

  const logsQuery = useQuery({
    queryKey: ["container-logs", containerId, search, page],
    enabled: !!containerId && !!serviceQuery.data,
    queryFn: () => getContainerLogs(containerId, {
      start: secondsToNanos(getStartSeconds(3600)),
      end: secondsToNanos(getNowSeconds()),
      search,
      page,
      page_size: PAGE_SIZE,
    }),
    placeholderData: (prev) => prev,
    staleTime: 20_000,
    refetchInterval: 30_000,
  })

  const result = logsQuery.data
  const events = result?.events ?? []

  return (
    <div className="p-5 space-y-4">
      <div className="flex gap-2">
        <input
          className="flex-1 h-9 rounded-md border border-input bg-background/70 px-3 text-sm outline-none"
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && commitSearch()}
          onBlur={commitSearch}
          placeholder="Search log lines..."
        />
        <button
          type="button"
          onClick={commitSearch}
          className="h-9 rounded-md border border-border px-3 text-sm hover:bg-muted"
        >
          Search
        </button>
      </div>

      {logsQuery.isError ? (
        <div className="py-6 text-center text-sm text-red-500">Failed to load logs.</div>
      ) : (
        <>
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="text-xs uppercase text-muted-foreground">
                <tr className="border-b border-border">
                  <th className="py-2 pr-4 text-left w-44">Timestamp</th>
                  <th className="py-2 text-left">Message</th>
                </tr>
              </thead>
              <tbody>
                {events.map((event, index) => (
                  <tr key={`${event.timestamp}-${index}`} className="border-b border-border/60 hover:bg-muted/20">
                    <td className="whitespace-nowrap py-2 pr-4 font-mono text-xs text-muted-foreground align-top">
                      {formatTimestamp(event.timestamp)}
                    </td>
                    <td className="py-2">
                      <div className="font-mono text-xs whitespace-pre-wrap break-all">{event.line}</div>
                    </td>
                  </tr>
                ))}
                {!logsQuery.isFetching && events.length === 0 ? (
                  <tr><td colSpan={2} className="py-8 text-center text-sm text-muted-foreground">No log lines found.</td></tr>
                ) : null}
                {logsQuery.isFetching && events.length === 0 ? (
                  <tr><td colSpan={2} className="py-8 text-center text-sm text-muted-foreground">Loading...</td></tr>
                ) : null}
              </tbody>
            </table>
          </div>

          {result && result.total > 0 ? (
            <LogPagination
              page={result.page}
              total={result.total}
              pageSize={result.page_size}
              hasMore={result.has_more}
              onChange={(p) => setPage(p)}
            />
          ) : null}
        </>
      )}
    </div>
  )
}

function formatTimestamp(ts: string): string {
  try {
    const d = new Date(ts)
    if (isNaN(d.getTime())) return ts
    return d.toLocaleString()
  } catch {
    return ts
  }
}


