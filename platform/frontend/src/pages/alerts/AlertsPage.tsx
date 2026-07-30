import React from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, useMemo } from "react"
import { Bell, AlertTriangle, ListChecks, CheckCircle2, Eye, Trash2, Plus, ChevronDown, ChevronRight, Search, ShieldAlert, ShieldCheck, Server } from "lucide-react"

import { getVulnerabilityAlerts, getAlertRules, getAlertEvents, updateAlertRule, deleteAlertRule, acknowledgeAlertEvent } from "@/api/alerts"
import type { AlertRule, AlertEvent, VulnerabilityAlert } from "@/api/alerts"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useAuthStore } from "@/store/authStore"
import CreateAlertRuleModal from "@/pages/alerts/CreateAlertRuleModal"

type Tab = "events" | "rules" | "vulnerabilities"

function timeAgo(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  const days = Math.floor(hr / 24)
  return `${days}d ago`
}

function formatScope(scopes: { scope_type: string; scope_id: number | null }[]): string {
  if (!scopes || scopes.length === 0) return "—"
  return scopes
    .map((s) => {
      if (s.scope_type === "all_hosts") return "All Hosts"
      if (s.scope_type === "all_services_of_type") return "All matching services"
      if (s.scope_type === "host") return `Host #${s.scope_id}`
      if (s.scope_type === "service") return `Service #${s.scope_id}`
      return s.scope_type
    })
    .join(", ")
}

function scopeLabel(rule: AlertRule): string {
  if (rule.domain) return rule.domain
  return formatScope(rule.scopes)
}

/** Extract entity name (service / container / domain / pool) from a metric_string like "host=kali service=nginx value=92.50%". */
function entityName(ev: AlertEvent): string {
  const ms = ev.metric_string || ""
  // Order matters: prefer container, then service, then domain+pool, then host
  const m = ms.match(/\b(?:container|service)=(\S+)/)
  if (m) return m[1]
  const d = ms.match(/\bdomain=(\S+)/)
  const p = ms.match(/\bpool=(\S+)/)
  if (d && p) return `${d[1]} (${p[1]})`
  if (d) return d[1]
  if (p) return p[1]
  const h = ms.match(/\bhost=(\S+)/)
  if (h) return h[1]
  return ""
}

const severityColors: Record<string, string> = {
  critical: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30",
  warning: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30",
  info: "text-blue-600 dark:text-blue-400 bg-blue-500/10 border-blue-500/30",
}

const statusConfig: Record<string, { icon: typeof Bell; label: string; className: string; dot: string }> = {
  firing: { icon: AlertTriangle, label: "Firing", className: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30", dot: "bg-red-500" },
  resolved: { icon: CheckCircle2, label: "Resolved", className: "text-green-600 dark:text-green-400 bg-green-500/10 border-green-500/30", dot: "bg-green-500" },
  acknowledged: { icon: Eye, label: "Acknowledged", className: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30", dot: "bg-amber-500" },
}

function StatCard({ label, value, tone }: { label: string; value: number; tone: "red" | "amber" | "accent" | "muted" }) {
  const tones = {
    red: "text-red-600 dark:text-red-400 border-red-500/20 bg-red-500/5",
    amber: "text-amber-600 dark:text-amber-400 border-amber-500/20 bg-amber-500/5",
    accent: "text-primary border-primary/20 bg-primary/5",
    muted: "text-foreground border-border bg-card",
  }
  return (
    <div className={cn("rounded-lg border p-3", tones[tone])}>
      <div className="text-xs uppercase tracking-wider text-muted-foreground font-semibold">{label}</div>
      <div className="text-xl font-bold mt-1 tabular-nums">{value}</div>
    </div>
  )
}

function Badge({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-sm font-medium", className)}>
      {children}
    </span>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const color = severityColors[severity] || "text-gray-600 dark:text-gray-400 bg-gray-500/10 border-gray-500/30"
  return <Badge className={color}>{severity}</Badge>
}

export default function AlertsPage() {
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === "admin"
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>("events")
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [expandedEvent, setExpandedEvent] = useState<number | null>(null)
  const [filterStatus, setFilterStatus] = useState("")
  const [filterSeverity, setFilterSeverity] = useState("")
  const [filterRule, setFilterRule] = useState("")
  const [expandedVulnService, setExpandedVulnService] = useState<string | null>(null)

  const alertsQuery = useQuery({
    queryKey: ["vulnerability-alerts"],
    queryFn: getVulnerabilityAlerts,
  })

  const rulesQuery = useQuery({
    queryKey: ["alert-rules"],
    queryFn: getAlertRules,
  })

  const eventsQuery = useQuery({
    queryKey: ["alert-events"],
    queryFn: () => getAlertEvents(50),
    refetchInterval: 30_000,
  })

  const toggleRuleMutation = useMutation({
    mutationFn: ({ id, isEnabled }: { id: number; isEnabled: boolean }) =>
      updateAlertRule(id, { is_enabled: !isEnabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alert-rules"] }),
  })

  const deleteRuleMutation = useMutation({
    mutationFn: (id: number) => deleteAlertRule(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alert-rules"] }),
  })

  const acknowledgeMutation = useMutation({
    mutationFn: (id: number) => acknowledgeAlertEvent(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["alert-events"] }),
  })

  const events = eventsQuery.data ?? []
  const rules = rulesQuery.data ?? []
  const vulnerabilities = alertsQuery.data ?? []

  const counts = useMemo(() => ({
    firing: events.filter((e) => e.status === "firing").length,
    unack: events.filter((e) => e.status === "firing" && !e.acknowledged_by).length,
    rules: rules.length,
    today: events.filter((e) => {
      const d = new Date(e.triggered_at)
      const now = new Date()
      return d.toDateString() === now.toDateString()
    }).length,
  }), [events, rules])

  const filteredEvents = events.filter((e) => {
    if (filterStatus && e.status !== filterStatus) return false
    const sev = e.threshold?.severity || e.rule?.default_severity || ""
    if (filterSeverity && sev !== filterSeverity) return false
    if (filterRule) {
      const name = e.rule?.template_name || ""
      if (!name.toLowerCase().includes(filterRule.toLowerCase())) return false
    }
    return true
  })

  const tabs: { key: Tab; label: string; count: number }[] = [
    { key: "events", label: "Alert Events", count: events.length },
    { key: "rules", label: "Alert Rules", count: rules.length },
    { key: "vulnerabilities", label: "Vulnerability Alerts", count: vulnerabilities.length },
  ]

  // Group CVEs by host → service → severity
  const groupedVulns = useMemo(() => {
    type ServiceGroup = { serviceKey: string; serviceName: string; version: string; alerts: VulnerabilityAlert[] }
    type HostGroup = { hostname: string; machineId: string; services: ServiceGroup[]; totalCves: number; critCount: number; warnCount: number }
    const hostMap: Record<string, HostGroup> = {}
    for (const v of vulnerabilities) {
      if (!hostMap[v.machine_id]) {
        hostMap[v.machine_id] = { hostname: v.hostname || "-", machineId: v.machine_id, services: [], totalCves: 0, critCount: 0, warnCount: 0 }
      }
      const host = hostMap[v.machine_id]
      host.totalCves++
      if (v.alert_level === "alert") host.critCount++
      else host.warnCount++
      const serviceKey = `${v.service_name} ${v.service_version}`
      let svc = host.services.find(s => s.serviceKey === serviceKey)
      if (!svc) {
        svc = { serviceKey, serviceName: v.service_name, version: v.service_version, alerts: [] }
        host.services.push(svc)
      }
      svc.alerts.push(v)
    }
    for (const host of Object.values(hostMap)) {
      for (const svc of host.services) {
        svc.alerts.sort((a, b) => {
          const order = ["critical", "high", "medium", "low"]
          return order.indexOf(a.severity) - order.indexOf(b.severity)
        })
      }
    }
    return Object.values(hostMap)
  }, [vulnerabilities])

  return (
    <Page>
      <PageHeader
        title="Alerts"
        description="Monitor and manage alert rules, events, and vulnerability findings."
        icon={Bell}
      />

      {/* Stats bar */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard label="Firing" value={counts.firing} tone="red" />
        <StatCard label="Unacknowledged" value={counts.unack} tone="amber" />
        <StatCard label="Rules" value={counts.rules} tone="accent" />
        <StatCard label="Triggered Today" value={counts.today} tone="muted" />
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={cn(
              "relative px-4 py-2.5 text-sm font-medium transition-colors flex items-center gap-2",
              tab === t.key ? "text-foreground" : "text-muted-foreground hover:text-foreground"
            )}
          >
            {t.label}
            <span className={cn(
              "text-sm px-1.5 py-0.5 rounded-md",
              tab === t.key ? "bg-primary/15 text-primary" : "bg-secondary text-muted-foreground"
            )}>
              {t.count}
            </span>
            {tab === t.key && <span className="absolute -bottom-px left-0 right-0 h-0.5 bg-primary rounded-full" />}
          </button>
        ))}
      </div>

      {/* ───────────── Alert Events Tab ───────────── */}
      {tab === "events" && (
        <SectionCard>
          {eventsQuery.isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-12 bg-muted rounded-lg animate-pulse" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-10 text-muted-foreground">
              <Bell className="w-8 h-8 mb-3 opacity-30" />
              <p className="text-sm font-medium text-foreground">No alert events yet.</p>
              <p className="text-sm mt-1">Events appear when a metric exceeds a configured threshold.</p>
            </div>
          ) : (
            <div className="space-y-4">
              {/* Filter bar */}
              <div className="flex flex-wrap items-center gap-3">
                <select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}
                  className="bg-secondary border border-border rounded-lg px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary/50">
                  <option value="">All statuses</option>
                  <option value="firing">Firing</option>
                  <option value="resolved">Resolved</option>
                  <option value="acknowledged">Acknowledged</option>
                </select>
                <select value={filterSeverity} onChange={(e) => setFilterSeverity(e.target.value)}
                  className="bg-secondary border border-border rounded-lg px-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary/50">
                  <option value="">All severities</option>
                  <option value="critical">Critical</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
                <div className="relative flex-1 min-w-[200px] max-w-md">
                  <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground" />
                  <input type="text" placeholder="Search by rule name…" value={filterRule} onChange={(e) => setFilterRule(e.target.value)}
                    className="w-full bg-secondary border border-border rounded-lg pl-9 pr-3 py-2 text-sm text-foreground focus:outline-none focus:border-primary/50" />
                </div>
                {(filterStatus || filterSeverity || filterRule) && (
                  <button onClick={() => { setFilterStatus(""); setFilterSeverity(""); setFilterRule("") }}
                    className="text-sm text-primary hover:underline">Clear</button>
                )}
              </div>

              {/* Events table */}
              <div className="rounded-xl border border-border bg-card overflow-hidden">
                {filteredEvents.length === 0 ? (
                  <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <Bell className="w-8 h-8 mb-3 opacity-30" />
                    <p className="text-sm">No events match the selected filters.</p>
                  </div>
                ) : (
                  <table className="w-full text-sm">
                    <thead className="bg-secondary/50 text-sm uppercase tracking-wider text-muted-foreground">
                      <tr>
                        <th className="w-8 px-0" />
                        <th className="text-left px-3 py-3 font-semibold">Status</th>
                        <th className="text-left px-3 py-3 font-semibold">Rule</th>
                        <th className="text-left px-3 py-3 font-semibold">Severity</th>
                        <th className="text-left px-3 py-3 font-semibold">Scope</th>
                        <th className="text-right px-3 py-3 font-semibold">Value</th>
                        <th className="text-left px-3 py-3 font-semibold">Triggered</th>
                        <th className="w-32 px-0" />
                      </tr>
                    </thead>
                    <tbody>
                      {filteredEvents.map((event) => {
                        const expanded = expandedEvent === event.id
                        const severity = event.threshold?.severity || event.rule?.default_severity || "-"
                        const isFiring = event.status === "firing"
                        const S = statusConfig[event.status]
                        return (
                          <React.Fragment key={event.id}>
                            <tr
                              className={cn(
                                "border-t border-border hover:bg-surface-hover cursor-pointer transition-colors",
                                isFiring && "bg-red-500/[0.02]"
                              )}
                              onClick={() => setExpandedEvent(expanded ? null : event.id)}
                            >
                              <td className="pl-4 py-3">
                                {expanded ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />}
                              </td>
                              <td className="px-3 py-3">
                                {S ? (
                                  <Badge className={S.className}>
                                    <span className={cn("w-1.5 h-1.5 rounded-full", S.dot)} />
                                    <S.icon className="w-3.5 h-3.5" />
                                    {S.label}
                                  </Badge>
                                ) : (
                                  <span className="text-sm text-muted-foreground">{event.status}</span>
                                )}
                              </td>
                              <td className="px-3 py-3">
                                <div className="font-medium text-foreground leading-tight">{event.rule?.template_name || `Rule #${event.rule_id}`}</div>
                                {entityName(event) && (
                                  <div className="text-sm text-muted-foreground/70 font-mono mt-0.5 truncate max-w-[220px]" title={entityName(event)}>{entityName(event)}</div>
                                )}
                              </td>
                              <td className="px-3 py-3">
                                <SeverityBadge severity={severity} />
                              </td>
                              <td className="px-3 py-3 text-sm text-muted-foreground max-w-[200px] truncate" title={event.rule ? scopeLabel(event.rule) : "—"}>
                                {event.rule ? scopeLabel(event.rule) : "—"}
                              </td>
                              <td className="px-3 py-3 text-right font-mono tabular-nums text-foreground">
                                {event.actual_value !== null && event.actual_value !== undefined
                                  ? `${Number(event.actual_value.toFixed(1))}${event.rule?.unit || ""}`
                                  : "—"}
                              </td>
                              <td className="px-3 py-3 text-sm text-muted-foreground whitespace-nowrap">
                                <span title={new Date(event.triggered_at).toLocaleString()}>{timeAgo(event.triggered_at)}</span>
                              </td>
                              <td className="px-3 py-3">
                                {isAdmin && isFiring && (
                                  <button onClick={(ev) => { ev.stopPropagation(); acknowledgeMutation.mutate(event.id) }}
                                    className="text-sm px-2.5 py-1.5 rounded-md bg-primary/10 text-primary border border-primary/30 hover:bg-primary/20 transition-colors font-medium">
                                    <Eye className="w-3 h-3 inline mr-1" /> Acknowledge
                                  </button>
                                )}
                              </td>
                            </tr>
                            {expanded && (
                              <tr key={`detail-${event.id}`} className="bg-secondary/30 border-t border-border">
                                <td />
                                <td colSpan={7} className="px-3 py-4">
                                  <div className="space-y-3">
                                    {event.rule?.explanation && (
                                      <div className="text-sm text-muted-foreground/80 italic border-l-2 border-border pl-2.5">
                                        {event.rule.explanation}
                                      </div>
                                    )}
                                    <div className="grid grid-cols-2 md:grid-cols-3 gap-x-8 gap-y-2 text-sm">
                                      {event.metric_string && (
                                        <div><span className="text-muted-foreground">Metric:</span> <span className="font-mono text-foreground">{event.metric_string}</span></div>
                                      )}
                                      {event.actual_value !== null && event.actual_value !== undefined && (
                                        <div><span className="text-muted-foreground">Actual:</span> <span className={cn("font-mono", isFiring ? "text-red-600 dark:text-red-400" : "text-foreground")}>{Number(event.actual_value.toFixed(2))}{event.rule?.unit ? ` ${event.rule.unit}` : ""}</span></div>
                                      )}
                                      {event.threshold && (
                                        <div><span className="text-muted-foreground">Threshold:</span> <span className="font-mono text-foreground">{event.threshold.threshold}{event.rule?.unit ? ` ${event.rule.unit}` : ""}</span></div>
                                      )}
                                      <div><span className="text-muted-foreground">Host:</span> <span className="font-mono text-foreground">{event.host_machine_id}</span></div>
                                      {event.resolved_at && (
                                        <div><span className="text-muted-foreground">Resolved:</span> <span className="text-green-600 dark:text-green-400">{new Date(event.resolved_at).toLocaleString()}</span></div>
                                      )}
                                      {event.acknowledged_by && (
                                        <div><span className="text-muted-foreground">Acknowledged by:</span> <span className="text-foreground">User #{event.acknowledged_by}</span></div>
                                      )}
                                    </div>
                                  </div>
                                </td>
                              </tr>
                            )}
                          </React.Fragment>
                        )
                      })}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          )}
        </SectionCard>
      )}

      {/* ───────────── Alert Rules Tab ───────────── */}
      {tab === "rules" && (
        <div className="space-y-4">
          <SectionCard>
            {rulesQuery.isLoading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="h-12 bg-muted rounded-lg animate-pulse" />
                ))}
              </div>
            ) : rules.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-10 text-muted-foreground gap-3">
                <ListChecks className="w-8 h-8 opacity-30" />
                <div className="text-center">
                  <p className="text-sm font-medium text-foreground">No alert rules configured.</p>
                  <p className="text-sm mt-1">Create your first rule to start receiving alerts.</p>
                </div>
                {isAdmin && (
                  <Button size="sm" onClick={() => setShowCreateModal(true)}>
                    <Plus className="w-4 h-4 mr-1" /> Add Alert Rule
                  </Button>
                )}
              </div>
            ) : (
              <div className="rounded-xl border border-border bg-card overflow-hidden">
                <table className="w-full text-sm">
                  <thead className="bg-secondary/50 text-sm uppercase tracking-wider text-muted-foreground">
                    <tr>
                      <th className="text-left px-4 py-3 font-semibold">Rule</th>
                      <th className="text-left px-3 py-3 font-semibold">Scope / Domain</th>
                      <th className="text-left px-3 py-3 font-semibold">Thresholds</th>
                      <th className="text-left px-3 py-3 font-semibold">Duration</th>
                      <th className="text-center px-3 py-3 font-semibold">Enabled</th>
                      <th className="w-16 px-0" />
                    </tr>
                  </thead>
                  <tbody>
                    {rules.map((rule) => (
                      <tr key={rule.id} className="border-t border-border hover:bg-surface-hover transition-colors">
                        <td className="px-4 py-3">
                          <div className="font-medium text-foreground">{rule.template_name}</div>
                          <div className="text-sm text-muted-foreground mt-0.5">{rule.description}</div>
                          {rule.explanation && (
                            <div className="text-sm text-muted-foreground/60 mt-1 italic border-l-2 border-border/50 pl-2 leading-relaxed">
                              {rule.explanation}
                            </div>
                          )}
                        </td>
                        <td className="px-3 py-3 text-sm text-muted-foreground">
                          {rule.domain ? <span><span className="text-muted-foreground">Domain:</span> <span className="font-mono">{rule.domain}</span></span> : <span>{formatScope(rule.scopes)}</span>}
                        </td>
                        <td className="px-3 py-3">
                          <div className="flex flex-wrap gap-1.5">
                            {rule.thresholds.map((t, i) => (
                              <span key={i} className={cn("text-sm px-2 py-0.5 rounded-md border font-mono", severityColors[t.severity] || "")}>
                                {t.label || t.severity}: {t.threshold}{rule.unit}
                              </span>
                            ))}
                          </div>
                        </td>
                        <td className="px-3 py-3 font-mono text-sm text-muted-foreground">{rule.for_duration || "5m"}</td>
                        <td className="px-3 py-3 text-center">
                          {isAdmin ? (
                            <button onClick={() => toggleRuleMutation.mutate({ id: rule.id, isEnabled: rule.is_enabled })}
                              className={cn("w-10 h-5 rounded-full relative transition-colors", rule.is_enabled ? "bg-primary" : "bg-secondary border border-border")}>
                              <span className={cn("absolute top-0.5 left-0.5 w-4 h-4 rounded-full transition-transform bg-white", rule.is_enabled ? "translate-x-5" : "")} />
                            </button>
                          ) : (
                            <span className={cn("text-sm font-medium", rule.is_enabled ? "text-primary" : "text-muted-foreground")}>
                              {rule.is_enabled ? "Enabled" : "Disabled"}
                            </span>
                          )}
                        </td>
                        <td className="px-3 py-3">
                          {isAdmin && (
                            <button onClick={() => deleteRuleMutation.mutate(rule.id)}
                              className="p-1.5 rounded-md text-muted-foreground hover:text-red-600 dark:hover:text-red-400 hover:bg-red-500/10 transition-colors">
                              <Trash2 className="w-4 h-4" />
                            </button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {isAdmin && rules.length > 0 && (
              <div className="flex justify-end">
                <Button size="sm" onClick={() => setShowCreateModal(true)}>
                  <Plus className="w-4 h-4 mr-1" /> Add Alert Rule
                </Button>
              </div>
            )}
          </SectionCard>
          {isAdmin && showCreateModal && <CreateAlertRuleModal onClose={() => setShowCreateModal(false)} />}
        </div>
      )}

      {/* ───────────── Vulnerability Alerts Tab ───────────── */}
      {tab === "vulnerabilities" && (
        <div className="space-y-4">
          <div className="rounded-lg border border-amber-500/30 bg-amber-500/5 px-4 py-3 text-sm text-amber-600 dark:text-amber-400 flex items-start gap-2">
            <ShieldAlert className="w-4 h-4 shrink-0 mt-0.5" />
            <span>Medium and below show as <strong>warning</strong> (amber). High and critical show as <strong>alert</strong> (red).</span>
          </div>

          {alertsQuery.isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-24 bg-muted rounded-lg animate-pulse" />
              ))}
            </div>
          ) : vulnerabilities.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-10 text-muted-foreground rounded-xl border border-border bg-card">
              <ShieldCheck className="w-8 h-8 mb-3 opacity-30 text-green-500" />
              <p className="text-sm font-medium text-foreground">No active vulnerability alerts right now.</p>
            </div>
          ) : (
            <div className="space-y-3">
              {groupedVulns.map((host) => (
                <div key={host.machineId} className="rounded-xl border border-border bg-card overflow-hidden">
                  <div className="flex items-center gap-3 px-5 py-3.5 bg-card">
                    <Server className="w-4 h-4 text-muted-foreground shrink-0" />
                    <div className="flex items-center gap-2 flex-1 min-w-0">
                      <span className="text-sm font-semibold text-foreground">{host.hostname}</span>
                      <span className="text-sm font-mono text-muted-foreground">· {host.machineId.slice(0, 12)}…</span>
                    </div>
                    <div className="flex items-center gap-3 text-sm shrink-0">
                      {host.critCount > 0 && (
                        <span className="text-red-600 dark:text-red-400 font-bold flex items-center gap-1">
                          <span className="w-2 h-2 rounded-full bg-red-500" />
                          {host.critCount} critical
                        </span>
                      )}
                      {host.warnCount > 0 && (
                        <span className="text-amber-600 dark:text-amber-400 font-bold flex items-center gap-1">
                          <span className="w-2 h-2 rounded-full bg-amber-500" />
                          {host.warnCount} warnings
                        </span>
                      )}
                      <span className="text-muted-foreground">({host.totalCves})</span>
                    </div>
                  </div>
                  <div className="border-t border-border divide-y divide-border">
                    {host.services.map((svc) => {
                      const svcCrit = svc.alerts.filter(a => a.alert_level === "alert").length
                      const svcWarn = svc.alerts.filter(a => a.alert_level !== "alert").length
                      const svcKey = `${host.machineId}-${svc.serviceKey}`
                      const svcOpen = expandedVulnService === svcKey
                      return (
                        <div key={svc.serviceKey}>
                          <button
                            onClick={() => setExpandedVulnService(svcOpen ? null : svcKey)}
                            className="w-full flex items-center gap-3 px-5 py-2.5 bg-secondary/20 hover:bg-muted/30 transition-colors text-left"
                          >
                            <div className="flex-1 min-w-0">
                              <span className="text-sm font-semibold text-foreground">{svc.serviceName}</span>
                              <span className="text-sm font-mono text-muted-foreground ml-1.5">v{svc.version}</span>
                            </div>
                            <div className="flex items-center gap-2 text-sm">
                              {svcCrit > 0 && <span className="text-red-600 dark:text-red-400 font-semibold">{svcCrit} critical</span>}
                              {svcWarn > 0 && <span className="text-amber-600 dark:text-amber-400 font-semibold">{svcWarn} warning</span>}
                            </div>
                            {svcOpen ? <ChevronDown className="w-3.5 h-3.5 text-muted-foreground" /> : <ChevronRight className="w-3.5 h-3.5 text-muted-foreground" />}
                          </button>
                          {svcOpen && (
                            <div className="px-5 py-3 space-y-2 bg-card">
                              {svc.alerts.map((alert) => {
                                const isAlert = alert.alert_level === "alert"
                                return (
                                  <div key={alert.id}
                                    className={cn(
                                      "rounded-lg p-3.5 border-l-4 border border-border/60",
                                      isAlert ? "border-l-red-500 bg-red-500/[0.03]" : "border-l-amber-500 bg-amber-500/[0.03]"
                                    )}
                                  >
                                    <div className="flex items-start justify-between gap-3">
                                      <div className="min-w-0 flex-1">
                                        <div className="flex items-center gap-2 flex-wrap">
                                          <span className="font-mono font-bold text-foreground text-sm">{alert.cve_id}</span>
                                          <span className={cn(
                                            "px-2 py-0.5 rounded-md border text-sm font-bold uppercase tracking-wider",
                                            isAlert
                                              ? "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30"
                                              : "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30"
                                          )}>
                                            {alert.severity}
                                          </span>
                                          <span className={cn(
                                            "text-sm font-semibold",
                                            isAlert ? "text-red-600 dark:text-red-400" : "text-amber-600 dark:text-amber-400"
                                          )}>
                                            CVSS {Number(alert.cvss_score.toFixed(1))}
                                          </span>
                                        </div>
                                        <p className="text-sm text-muted-foreground mt-2 leading-relaxed">{alert.description || "No description available."}</p>
                                      </div>
                                      <span className={cn(
                                        "shrink-0 inline-flex items-center gap-1 px-2 py-1 rounded-md border text-sm font-bold uppercase tracking-wider",
                                        isAlert
                                          ? "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30"
                                          : "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30"
                                      )}>
                                        {isAlert ? <AlertTriangle className="w-3 h-3" /> : <Bell className="w-3 h-3" />}
                                        {alert.alert_level}
                                      </span>
                                    </div>
                                    <div className="mt-2 text-sm text-muted-foreground">
                                      Fix ≥ <span className="font-mono text-emerald-600 dark:text-emerald-400 font-semibold">{alert.fixed_in_version || "-"}</span>
                                    </div>
                                  </div>
                                )
                              })}
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Page>
  )
}
