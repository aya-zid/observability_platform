import { useQuery } from "@tanstack/react-query"

import { getVulnerabilityAlerts } from "@/api/alerts"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { cn } from "@/lib/utils"

export default function AlertsPage() {
  const alertsQuery = useQuery({
    queryKey: ["vulnerability-alerts"],
    queryFn: getVulnerabilityAlerts,
  })

  const alerts = alertsQuery.data ?? []

  return (
    <Page>
      <PageHeader
        title="Alerts"
        description="Security findings generated from daily vulnerability scans of discovered service versions."
      />

      <SectionCard
        title="Vulnerability Alerts"
        description="Medium findings show as warnings. High and critical findings show as alerts."
      >
        {alertsQuery.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading alerts...</div>
        ) : null}

        {!alertsQuery.isLoading && !alerts.length ? (
          <div className="text-sm text-muted-foreground">
            No active vulnerability alerts right now.
          </div>
        ) : null}

        <div className="grid gap-3">
          {alerts.map((alert) => (
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
      </SectionCard>
    </Page>
  )
}
