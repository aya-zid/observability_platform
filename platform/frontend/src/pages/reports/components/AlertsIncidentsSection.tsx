import { AlertTriangle, BellRing, CheckCircle2, Link2, Clock } from "lucide-react"

import type { ReportAlertPayload, ReportIncidentPayload } from "@/api/reports"
import { cn } from "@/lib/utils"
import { SEVERITY_STYLES } from "../utils"

export function AlertsIncidentsSection({
  alertsA,
  alertsB,
  incidents,
}: {
  alertsA?: ReportAlertPayload[]
  alertsB?: ReportAlertPayload[]
  incidents?: ReportIncidentPayload[]
}) {
  const alerts = alertsA ?? []
  const alertsCount = alerts.length
  const resolvedCount = alerts.filter((a) => a.status === "resolved").length
  const dayBAlerts = alertsB?.length ?? 0

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      {/* Alerts */}
      <div className="rounded-lg border border-border bg-card p-3.5">
        <div className="mb-2.5 flex items-center justify-between">
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-foreground">
            <BellRing className="h-4 w-4 text-primary" />
            Alerts
          </h3>
          <span className="text-xs text-muted-foreground">
            {alertsCount} total · {resolvedCount} resolved
            {dayBAlerts ? ` · Day B: ${dayBAlerts}` : ""}
          </span>
        </div>
        {alertsCount === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">No alert events this day.</p>
        ) : (
          <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
            {alerts.map((a, i) => {
              const sev = a.severity || "info"
              const resolved = a.status === "resolved"
              return (
                <div
                  key={i}
                  className={cn(
                    "rounded-lg border p-2.5",
                    resolved ? "border-border bg-muted/20" : "border-destructive/30 bg-destructive/[0.04]"
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className={cn("inline-flex items-center gap-1 rounded border px-1.5 py-0.5 text-[10px] font-semibold uppercase", SEVERITY_STYLES[sev])}>
                      {resolved ? <CheckCircle2 className="h-3 w-3" /> : <AlertTriangle className="h-3 w-3" />}
                      {sev}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {resolved ? "resolved" : a.status}
                    </span>
                  </div>
                  <p className="mt-1.5 text-sm font-medium text-foreground">
                    {a.rule_name || "Alert"}
                    {a.service ? <span className="font-mono text-xs text-muted-foreground"> · {a.service}</span> : null}
                  </p>
                  <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                    {a.host_machine_id && <span className="font-mono">host: {a.host_machine_id}</span>}
                    {a.actual_value !== null && a.actual_value !== undefined && (
                      <span className="font-mono">value: {Number(a.actual_value).toFixed(2)}</span>
                    )}
                    <span className="inline-flex items-center gap-1">
                      <Clock className="h-3 w-3" />
                      {new Date(a.triggered_at).toLocaleString()}
                    </span>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Incidents */}
      <div className="rounded-lg border border-border bg-card p-3.5">
        <div className="mb-2.5 flex items-center justify-between">
          <h3 className="flex items-center gap-1.5 text-sm font-semibold text-foreground">
            <Link2 className="h-4 w-4 text-primary" />
            Incidents
          </h3>
          <span className="text-xs text-muted-foreground">{(incidents ?? []).length} overlapping</span>
        </div>
        {(incidents ?? []).length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">No incidents overlapped this day.</p>
        ) : (
          <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
            {(incidents ?? []).map((inc) => (
              <div key={inc.id} className="rounded-lg border border-border bg-card p-2.5">
                <div className="flex items-center justify-between gap-2">
                  <span className={cn("inline-flex items-center rounded border px-1.5 py-0.5 text-[10px] font-semibold uppercase", SEVERITY_STYLES[inc.severity])}>
                    {inc.severity}
                  </span>
                  <span className="text-xs text-muted-foreground">{inc.category} · {inc.status}</span>
                </div>
                <p className="mt-1.5 text-sm font-medium text-foreground">{inc.title}</p>
                {inc.hostname && <p className="font-mono text-xs text-muted-foreground">{inc.hostname}</p>}
                <div className="mt-1 text-xs text-muted-foreground">
                  Since {new Date(inc.firing_since).toLocaleString()} · {inc.members} member(s)
                  {inc.resolved_at ? ` · resolved ${new Date(inc.resolved_at).toLocaleString()}` : ""}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
