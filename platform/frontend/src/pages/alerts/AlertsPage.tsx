import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { Bell, AlertTriangle, ListChecks, Clock, CheckCircle2, Eye, Trash2, Plus, ChevronDown, ChevronRight } from "lucide-react"

import { getVulnerabilityAlerts, getAlertRules, getAlertEvents, updateAlertRule, deleteAlertRule, acknowledgeAlertEvent } from "@/api/alerts"
import type { AlertRule, AlertEvent } from "@/api/alerts"
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

const severityColors: Record<string, string> = {
  critical: "bg-red-500/15 text-red-600 dark:text-red-400 border-red-500/30",
  warning: "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30",
  info: "bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30",
}

const statusConfig: Record<string, { icon: typeof Bell; label: string; bg: string; dot: string }> = {
  firing: { icon: AlertTriangle, label: "Firing", bg: "bg-red-500/10 text-red-600 dark:text-red-400", dot: "bg-red-500" },
  resolved: { icon: CheckCircle2, label: "Resolved", bg: "bg-green-500/10 text-green-600 dark:text-green-400", dot: "bg-green-500" },
  acknowledged: { icon: Eye, label: "Acknowledged", bg: "bg-amber-500/10 text-amber-600 dark:text-amber-400", dot: "bg-amber-500" },
}

function StatCard({ icon: Icon, label, value, accent }: { icon: typeof Bell; label: string; value: string | number; accent: string }) {
  return (
    <div className="rounded-xl border border-border bg-card/70 backdrop-blur p-4 flex items-center gap-4">
      <div className={cn("rounded-lg p-2.5", accent)}>
        <Icon className="h-5 w-5" />
      </div>
      <div>
        <p className="text-xs text-muted-foreground">{label}</p>
        <p className="text-2xl font-bold mt-0.5">{value}</p>
      </div>
    </div>
  )
}

function Badge({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium border", className)}>
      {children}
    </span>
  )
}

function StatusBadge({ status }: { status: string }) {
  const cfg = statusConfig[status]
  if (!cfg) return <span className="text-xs text-muted-foreground">{status}</span>
  const Icon = cfg.icon
  return (
    <Badge className={cfg.bg}>
      <span className={cn("w-1.5 h-1.5 rounded-full", cfg.dot)} />
      <Icon className="h-3 w-3" />
      {cfg.label}
    </Badge>
  )
}

function SeverityBadge({ severity }: { severity: string }) {
  const color = severityColors[severity] || "bg-gray-500/15 text-gray-600 dark:text-gray-400 border-gray-500/30"
  return <Badge className={color}>{severity}</Badge>
}

function ExpandedEventDetail({ event }: { event: AlertEvent }) {
  return (
    <div className="px-2 pb-3 pt-1">
      <div className="bg-muted/30 rounded-lg p-3 text-xs space-y-2 border border-border/50">
        {event.metric_string && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Metric</span>
            <span className="font-medium">{event.metric_string}</span>
          </div>
        )}
        {event.actual_value !== null && event.actual_value !== undefined && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Value</span>
            <span className="font-medium">{event.actual_value.toFixed(2)}{event.rule?.unit ? ` ${event.rule.unit}` : ""}</span>
          </div>
        )}
        {event.threshold && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Threshold</span>
            <span className="font-medium">{event.threshold.threshold}{event.rule?.unit ? ` ${event.rule.unit}` : ""}</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-muted-foreground">Host</span>
          <span className="font-medium font-mono">{event.host_machine_id}</span>
        </div>
        {event.resolved_at && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Resolved</span>
            <span className="font-medium">{timeAgo(event.resolved_at)}</span>
          </div>
        )}
        {event.acknowledged_by && (
          <div className="flex justify-between">
            <span className="text-muted-foreground">Acknowledged by</span>
            <span className="font-medium">User #{event.acknowledged_by}</span>
          </div>
        )}
      </div>
    </div>
  )
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

  const firingCount = events.filter((e) => e.status === "firing").length
  const acknowledgedCount = events.filter((e) => e.status === "acknowledged").length
  const todayCount = events.filter((e) => {
    const d = new Date(e.triggered_at)
    const now = new Date()
    return d.toDateString() === now.toDateString()
  }).length

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

  const tabs: { key: Tab; label: string; count?: number }[] = [
    { key: "events", label: "Alert Events", count: filteredEvents.length },
    { key: "rules", label: "Alert Rules", count: rules.length },
    { key: "vulnerabilities", label: "Vulnerability Alerts", count: vulnerabilities.length },
  ]

  return (
    <Page>
      <PageHeader
        title="Alerts"
        description="Monitor and manage alert rules, events, and vulnerability findings."
      />

      {/* Stats bar */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <StatCard icon={AlertTriangle} label="Firing" value={firingCount} accent="bg-red-500/15 text-red-600 dark:text-red-400" />
        <StatCard icon={Bell} label="Unacknowledged" value={events.length - acknowledgedCount} accent="bg-amber-500/15 text-amber-600 dark:text-amber-400" />
        <StatCard icon={ListChecks} label="Rules" value={rules.length} accent="bg-blue-500/15 text-blue-600 dark:text-blue-400" />
        <StatCard icon={Clock} label="Triggered Today" value={todayCount} accent="bg-purple-500/15 text-purple-600 dark:text-purple-400" />
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={cn(
              "px-4 py-2.5 text-sm font-medium border-b-2 -mb-[1px] transition-colors flex items-center gap-2",
              tab === t.key
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {t.label}
            {t.count !== undefined && t.count > 0 && (
              <span className={cn(
                "text-xs rounded-full px-1.5 py-0.5 min-w-[1.25rem] text-center",
                tab === t.key
                  ? "bg-primary/15 text-primary"
                  : "bg-muted text-muted-foreground"
              )}>
                {t.count}
              </span>
            )}
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
            <div className="py-12 text-center">
              <Bell className="h-10 w-10 mx-auto text-muted-foreground/40" />
              <p className="mt-3 text-sm text-muted-foreground">No alert events yet.</p>
              <p className="text-xs text-muted-foreground/60 mt-1">Events appear when a metric exceeds a configured threshold.</p>
            </div>
          ) : (
            <>
              {/* Filter bar */}
              <div className="flex flex-wrap gap-2 mb-4 pb-4 border-b border-border">
                <select
                  value={filterStatus}
                  onChange={(e) => setFilterStatus(e.target.value)}
                  className="h-8 rounded-md border border-border bg-background px-2.5 text-xs text-foreground outline-none focus:border-primary"
                >
                  <option value="">All statuses</option>
                  <option value="firing">Firing</option>
                  <option value="resolved">Resolved</option>
                  <option value="acknowledged">Acknowledged</option>
                </select>
                <select
                  value={filterSeverity}
                  onChange={(e) => setFilterSeverity(e.target.value)}
                  className="h-8 rounded-md border border-border bg-background px-2.5 text-xs text-foreground outline-none focus:border-primary"
                >
                  <option value="">All severities</option>
                  <option value="critical">Critical</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
                <input
                  type="text"
                  placeholder="Search rule name..."
                  value={filterRule}
                  onChange={(e) => setFilterRule(e.target.value)}
                  className="h-8 rounded-md border border-border bg-background px-2.5 text-xs text-foreground outline-none focus:border-primary w-44"
                />
                {(filterStatus || filterSeverity || filterRule) && (
                  <button
                    onClick={() => { setFilterStatus(""); setFilterSeverity(""); setFilterRule("") }}
                    className="h-8 px-2.5 text-xs text-muted-foreground hover:text-foreground transition-colors"
                  >
                    Clear filters
                  </button>
                )}
              </div>
              {filteredEvents.length === 0 ? (
                <div className="py-8 text-center text-sm text-muted-foreground">No events match the selected filters.</div>
              ) : (
              <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="pb-2.5 font-medium w-5"></th>
                    <th className="pb-2.5 font-medium">Status</th>
                    <th className="pb-2.5 font-medium">Rule</th>
                    <th className="pb-2.5 font-medium">Severity</th>
                    <th className="pb-2.5 font-medium">Scope</th>
                    <th className="pb-2.5 font-medium">Value</th>
                    <th className="pb-2.5 font-medium">Triggered</th>
                    <th className="pb-2.5 font-medium"></th>
                  </tr>
                </thead>
                <tbody>
                  {filteredEvents.flatMap((event) => {
                    const expanded = expandedEvent === event.id
                    const severity = event.threshold?.severity || event.rule?.default_severity || "-"
                    const isFiring = event.status === "firing"
                    const row = (
                      <tr key={event.id} className={cn("border-b border-border/40 transition-colors", isFiring && "bg-red-500/[0.02]")}>
                        <td className="py-2.5">
                          <button
                            onClick={() => setExpandedEvent(expanded ? null : event.id)}
                            className="text-muted-foreground hover:text-foreground transition-colors"
                          >
                            {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                          </button>
                        </td>
                        <td className="py-2.5">
                          <StatusBadge status={event.status} />
                        </td>
                        <td className="py-2.5 font-medium">
                          {event.rule?.template_name || `Rule #${event.rule_id}`}
                        </td>
                        <td className="py-2.5">
                          <SeverityBadge severity={severity} />
                        </td>
                        <td className="py-2.5 text-muted-foreground text-xs max-w-[180px] truncate" title={event.rule ? scopeLabel(event.rule) : "—"}>
                          {event.rule ? scopeLabel(event.rule) : "—"}
                        </td>
                        <td className="py-2.5 font-mono text-xs">
                          {event.actual_value !== null && event.actual_value !== undefined
                            ? `${event.actual_value.toFixed(1)}${event.rule?.unit ? event.rule.unit : ""}`
                            : "—"}
                        </td>
                        <td className="py-2.5 text-xs text-muted-foreground whitespace-nowrap">
                          <span title={new Date(event.triggered_at).toLocaleString()}>
                            {timeAgo(event.triggered_at)}
                          </span>
                        </td>
                        <td className="py-2.5">
                          {isAdmin && isFiring && (
                            <Button
                              variant="outline"
                              size="sm"
                              className="h-7 text-xs"
                              onClick={() => acknowledgeMutation.mutate(event.id)}
                            >
                              <Eye className="h-3 w-3 mr-1" />
                              Acknowledge
                            </Button>
                          )}
                        </td>
                      </tr>
                    )
                    if (!expanded) return [row]
                    return [row, (
                      <tr key={`detail-${event.id}`}>
                        <td colSpan={8} className="p-0">
                          <ExpandedEventDetail event={event} />
                        </td>
                      </tr>
                    )]
                  })}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
        </SectionCard>
      )}

      {/* ───────────── Alert Rules Tab ───────────── */}
      {tab === "rules" && (
        <>
          <SectionCard>
            {rulesQuery.isLoading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <div key={i} className="h-12 bg-muted rounded-lg animate-pulse" />
                ))}
              </div>
            ) : rules.length === 0 ? (
              <div className="py-12 text-center">
                <ListChecks className="h-10 w-10 mx-auto text-muted-foreground/40" />
                <p className="mt-3 text-sm text-muted-foreground">No alert rules configured.</p>
                <p className="text-xs text-muted-foreground/60 mt-1">Create a rule to start monitoring your infrastructure.</p>
                {isAdmin && (
                  <Button className="mt-4" size="sm" onClick={() => setShowCreateModal(true)}>
                    <Plus className="h-4 w-4 mr-1" /> Add Alert Rule
                  </Button>
                )}
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-muted-foreground">
                      <th className="pb-2.5 font-medium">Name</th>
                      <th className="pb-2.5 font-medium">Scope / Domain</th>
                      <th className="pb-2.5 font-medium">Thresholds</th>
                      <th className="pb-2.5 font-medium">Duration</th>
                      <th className="pb-2.5 font-medium">Status</th>
                      <th className="pb-2.5 font-medium"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {rules.map((rule) => (
                      <tr key={rule.id} className="border-b border-border/40">
                        <td className="py-2.5">
                          <div className="font-medium">{rule.template_name}</div>
                          <div className="text-xs text-muted-foreground mt-0.5">{rule.description}</div>
                        </td>
                        <td className="py-2.5">
                          <div className="text-xs">
                            {rule.domain ? (
                              <span className="flex items-center gap-1">
                                <span className="text-muted-foreground">Domain:</span>
                                <span className="font-mono">{rule.domain}</span>
                              </span>
                            ) : (
                              <span className="text-muted-foreground">{formatScope(rule.scopes)}</span>
                            )}
                          </div>
                        </td>
                        <td className="py-2.5">
                          <div className="flex flex-col gap-1">
                            {rule.thresholds.map((t, i) => (
                              <span key={i} className="inline-flex items-center gap-1.5 text-xs">
                                <SeverityBadge severity={t.severity} />
                                <span className="font-mono">{t.threshold}{rule.unit}</span>
                                {t.label && <span className="text-muted-foreground">· {t.label}</span>}
                              </span>
                            ))}
                          </div>
                        </td>
                        <td className="py-2.5 text-xs text-muted-foreground">
                          {rule.for_duration || "5m"}
                        </td>
                        <td className="py-2.5">
                          {isAdmin ? (
                            <button
                              onClick={() => toggleRuleMutation.mutate({ id: rule.id, isEnabled: rule.is_enabled })}
                              className={cn(
                                "relative inline-flex h-5 w-9 items-center rounded-full transition-colors",
                                rule.is_enabled ? "bg-green-500" : "bg-muted-foreground/30"
                              )}
                            >
                              <span className={cn(
                                "inline-block h-3.5 w-3.5 rounded-full bg-white shadow-sm transition-transform",
                                rule.is_enabled ? "translate-x-[18px]" : "translate-x-[3px]"
                              )} />
                            </button>
                          ) : (
                            <span className={cn(
                              "text-xs font-medium",
                              rule.is_enabled ? "text-green-500" : "text-muted-foreground"
                            )}>
                              {rule.is_enabled ? "Enabled" : "Disabled"}
                            </span>
                          )}
                        </td>
                        <td className="py-2.5">
                          {isAdmin && (
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-muted-foreground hover:text-destructive h-7 w-7 p-0"
                              onClick={() => deleteRuleMutation.mutate(rule.id)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {isAdmin && rules.length > 0 && (
              <div className="mt-4">
                <Button size="sm" onClick={() => setShowCreateModal(true)}>
                  <Plus className="h-4 w-4 mr-1" /> Add Alert Rule
                </Button>
              </div>
            )}
          </SectionCard>
          {isAdmin && showCreateModal && <CreateAlertRuleModal onClose={() => setShowCreateModal(false)} />}
        </>
      )}

      {/* ───────────── Vulnerability Alerts Tab ───────────── */}
      {tab === "vulnerabilities" && (
        <SectionCard>
          <div className="mb-4">
            <p className="text-sm text-muted-foreground">Medium findings show as warnings. High and critical findings show as alerts.</p>
          </div>
          {alertsQuery.isLoading ? (
            <div className="space-y-3">
              {Array.from({ length: 3 }).map((_, i) => (
                <div key={i} className="h-24 bg-muted rounded-lg animate-pulse" />
              ))}
            </div>
          ) : vulnerabilities.length === 0 ? (
            <div className="py-12 text-center">
              <ShieldCheck className="h-10 w-10 mx-auto text-muted-foreground/40" />
              <p className="mt-3 text-sm text-muted-foreground">No active vulnerability alerts right now.</p>
            </div>
          ) : (
            <div className="grid gap-3">
              {vulnerabilities.map((alert) => (
                <div key={alert.id} className="rounded-xl border border-border bg-card/40 p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="font-medium">
                      {alert.cve_id} · {alert.service_name} {alert.service_version || ""}
                    </div>
                    <Badge className={cn(
                      alert.alert_level === "alert"
                        ? "bg-red-500/15 text-red-600 dark:text-red-400 border-red-500/30"
                        : "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30"
                    )}>
                      {alert.alert_level === "alert" ? <AlertTriangle className="h-3 w-3" /> : <Bell className="h-3 w-3" />}
                      {alert.alert_level}
                    </Badge>
                  </div>
                  <div className="mt-2 text-sm text-muted-foreground">
                    Host: {alert.hostname || "-"} ({alert.machine_id})
                  </div>
                  <div className="mt-2 flex flex-wrap gap-4 text-xs">
                    <span>Severity {alert.severity}</span>
                    <span>CVSS {alert.cvss_score.toFixed(1)}</span>
                    <span>Fix {alert.fixed_in_version || "-"}</span>
                  </div>
                  <div className="mt-3 text-sm text-muted-foreground">
                    {alert.description || "No description available."}
                  </div>
                </div>
              ))}
            </div>
          )}
        </SectionCard>
      )}
    </Page>
  )
}

function ShieldCheck(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg
      {...props}
      xmlns="http://www.w3.org/2000/svg"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  )
}
