import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { getVulnerabilityAlerts, getAlertRules, getAlertEvents, updateAlertRule, deleteAlertRule, acknowledgeAlertEvent } from "@/api/alerts"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { useAuthStore } from "@/store/authStore"
import CreateAlertRuleModal from "@/pages/alerts/CreateAlertRuleModal"

type Tab = "events" | "rules" | "vulnerabilities"

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

function formatThresholds(thresholds: { threshold: number; severity: string; unit?: string }[], unit?: string): string {
  if (!thresholds || thresholds.length === 0) return "—"
  return thresholds
    .map((t) => `${t.threshold}${unit ?? ""} (${t.severity})`)
    .join(", ")
}

export default function AlertsPage() {
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === "admin"
  const queryClient = useQueryClient()
  const [tab, setTab] = useState<Tab>("events")
  const [showCreateModal, setShowCreateModal] = useState(false)

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

  const tabs: { key: Tab; label: string }[] = [
    { key: "events", label: "Alert Events" },
    { key: "rules", label: "Alert Rules" },
    { key: "vulnerabilities", label: "Vulnerability Alerts" },
  ]

  return (
    <Page>
      <PageHeader
        title="Alerts"
        description="Monitor and manage alert rules, events, and vulnerability findings."
      />

      <div className="flex gap-1 border-b border-border">
        {tabs.map((t) => (
          <button
            key={t.key}
            onClick={() => setTab(t.key)}
            className={cn(
              "px-4 py-2 text-sm font-medium border-b-2 -mb-[1px] transition-colors",
              tab === t.key
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "events" && (
        <SectionCard title="Alert Events" description="Recent alert events from your configured rules.">
          {eventsQuery.isLoading ? (
            <div className="text-sm text-muted-foreground">Loading events...</div>
          ) : !eventsQuery.data?.length ? (
            <div className="text-sm text-muted-foreground">No alert events yet.</div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-muted-foreground">
                    <th className="pb-2 font-medium">Rule</th>
                    <th className="pb-2 font-medium">Status</th>
                    <th className="pb-2 font-medium">Severity</th>
                    <th className="pb-2 font-medium">Scope</th>
                    <th className="pb-2 font-medium">Triggered</th>
                    <th className="pb-2 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {eventsQuery.data.map((event) => (
                    <tr key={event.id} className="border-b border-border/50">
                      <td className="py-2">{event.rule?.template_name || `Rule #${event.rule_id}`}</td>
                      <td className="py-2">
                        <span className={cn(
                          "text-xs uppercase tracking-wide",
                          event.status === "firing" && "text-red-500",
                          event.status === "resolved" && "text-green-500",
                          event.status === "acknowledged" && "text-amber-500",
                        )}>
                          {event.status}
                        </span>
                      </td>
                      <td className="py-2 capitalize">
                        {event.threshold?.severity || event.rule?.default_severity || "-"}
                      </td>
                      <td className="py-2 text-muted-foreground">
                        {formatScope(event.rule?.scopes ?? [])}
                      </td>
                      <td className="py-2 text-muted-foreground">
                        {new Date(event.triggered_at).toLocaleString()}
                      </td>
                      <td className="py-2">
                        {isAdmin && event.status === "firing" && (
                          <Button variant="outline" size="sm" onClick={() => acknowledgeMutation.mutate(event.id)}>
                            Acknowledge
                          </Button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </SectionCard>
      )}

      {tab === "rules" && (
        <>
          <SectionCard title="Alert Rules" description="Configured alert rules for your infrastructure.">
            {rulesQuery.isLoading ? (
              <div className="text-sm text-muted-foreground">Loading rules...</div>
            ) : !rulesQuery.data?.length ? (
              <div className="text-sm text-muted-foreground">No alert rules configured.</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-muted-foreground">
                      <th className="pb-2 font-medium">Name</th>
                      <th className="pb-2 font-medium">Scope</th>
                      <th className="pb-2 font-medium">Thresholds</th>
                      <th className="pb-2 font-medium">Enabled</th>
                      <th className="pb-2 font-medium">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rulesQuery.data.map((rule) => (
                      <tr key={rule.id} className="border-b border-border/50">
                        <td className="py-2">{rule.template_name}</td>
                        <td className="py-2 text-muted-foreground text-xs">
                          {formatScope(rule.scopes)}
                        </td>
                        <td className="py-2 text-xs">
                          {formatThresholds(rule.thresholds, rule.unit)}
                        </td>
                        <td className="py-2">
                          {isAdmin ? (
                            <button
                              onClick={() => toggleRuleMutation.mutate({ id: rule.id, isEnabled: rule.is_enabled })}
                              className={cn(
                                "px-2 py-1 rounded text-xs font-medium",
                                rule.is_enabled ? "bg-green-500/20 text-green-500" : "bg-muted text-muted-foreground"
                              )}
                            >
                              {rule.is_enabled ? "ON" : "OFF"}
                            </button>
                          ) : (
                            <span className={cn(
                              "px-2 py-1 rounded text-xs font-medium",
                              rule.is_enabled ? "bg-green-500/20 text-green-500" : "bg-muted text-muted-foreground"
                            )}>
                              {rule.is_enabled ? "ON" : "OFF"}
                            </span>
                          )}
                        </td>
                        <td className="py-2">
                          {isAdmin && (
                            <Button variant="ghost" size="sm" className="text-destructive" onClick={() => deleteRuleMutation.mutate(rule.id)}>
                              Delete
                            </Button>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {isAdmin && (
              <div className="mt-3">
                <Button onClick={() => setShowCreateModal(true)}>Add Alert Rule</Button>
              </div>
            )}
          </SectionCard>
          {isAdmin && showCreateModal && <CreateAlertRuleModal onClose={() => setShowCreateModal(false)} />}
        </>
      )}

      {tab === "vulnerabilities" && (
        <SectionCard
          title="Vulnerability Alerts"
          description="Medium findings show as warnings. High and critical findings show as alerts."
        >
          {alertsQuery.isLoading ? (
            <div className="text-sm text-muted-foreground">Loading alerts...</div>
          ) : !alertsQuery.data?.length ? (
            <div className="text-sm text-muted-foreground">No active vulnerability alerts right now.</div>
          ) : (
            <div className="grid gap-3">
              {alertsQuery.data.map((alert) => (
                <div key={alert.id} className="rounded-xl border border-border bg-card/40 p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="font-medium">
                      {alert.cve_id} · {alert.service_name} {alert.service_version || ""}
                    </div>
                    <div
                      className={cn(
                        "text-xs uppercase tracking-wide",
                        alert.alert_level === "alert" ? "text-red-500" : "text-amber-500"
                      )}
                    >
                      {alert.alert_level}
                    </div>
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
