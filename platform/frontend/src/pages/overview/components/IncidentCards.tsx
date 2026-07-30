import { useState } from "react"
import { Link } from "react-router-dom"
import {
  Activity,
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  ShieldAlert,
  X,
} from "lucide-react"

import type { IncidentCard, IncidentMember } from "@/api/overview"
import { dismissIncident } from "@/api/overview"
import { cn } from "@/lib/utils"

function formatRelativeTime(dateStr: string): string {
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = Math.max(0, now.getTime() - date.getTime())
  const diffMins = Math.floor(diffMs / 60000)
  if (diffMins < 1) return "just now"
  if (diffMins < 60) return `${diffMins}m`
  const diffHours = Math.floor(diffMins / 60)
  if (diffHours < 24) return `${diffHours}h ${diffMins % 60}m`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ${diffHours % 24}h`
}

function RoleBadge({ role }: { role: IncidentMember["role"] }) {
  switch (role) {
    case "root":
      return (
        <span className="inline-flex items-center rounded border border-red-500/30 bg-red-500/10 px-1 py-0.5 text-[11px] font-bold uppercase tracking-wider text-red-600 dark:text-red-400">
          Root
        </span>
      )
    case "symptom":
      return (
        <span className="inline-flex items-center rounded border border-amber-500/30 bg-amber-500/10 px-1 py-0.5 text-[11px] font-bold uppercase tracking-wider text-amber-600 dark:text-amber-400">
          Symptom
        </span>
      )
    default:
      return (
        <span className="inline-flex items-center rounded border border-border bg-muted/40 px-1 py-0.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
          Signal
        </span>
      )
  }
}

function CardItem({ card, onDismiss }: { card: IncidentCard; onDismiss: (id: string) => void }) {
  const [expanded, setExpanded] = useState(false)
  const [dismissing] = useState(false)
  const isCritical = card.severity === "critical"
  const isSecurity = card.category === "security"
  const isResolved = card.status === "resolved"

  return (
      <div
        className={cn(
          "rounded-lg border bg-card p-2.5 transition-all shadow-sm",
          isResolved && "opacity-60 hover:opacity-100",
          isSecurity && !isResolved
            ? "border-red-500/30 bg-red-500/[0.02]"
            : isCritical && !isResolved
            ? "border-l-[3px] border-l-red-500 border-border"
            : "border-l-[3px] border-l-amber-500 border-border"
        )}
      >
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-1 text-xs">
              <span className="rounded-md border border-border bg-muted/40 px-1.5 py-0.5 font-medium text-foreground text-[11px]">
                {card.hostname}
              </span>
              <span
                className={cn(
                  "inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide",
                  isCritical
                    ? "bg-red-500/15 text-red-600 dark:text-red-400"
                    : "bg-amber-500/15 text-amber-600 dark:text-amber-400"
                )}
              >
                {isCritical ? <ShieldAlert className="h-2.5 w-2.5" /> : <AlertTriangle className="h-2.5 w-2.5" />}
                {card.severity}
              </span>
              {isResolved ? (
                <span className="text-emerald-500 font-medium text-[11px]">
                  resolved {card.resolved_at ? formatRelativeTime(card.resolved_at) + " ago" : ""}
                </span>
              ) : (
                <span className="text-muted-foreground text-[11px]">
                  firing for {formatRelativeTime(card.firing_since)}
                </span>
              )}
            </div>

            <h3 className="mt-1 text-sm font-semibold tracking-tight text-foreground">
              {card.title}
            </h3>

            <p className="mt-0.5 text-xs text-muted-foreground leading-relaxed">
              {card.narrative}
            </p>

            {card.enrichments && card.enrichments.length > 0 ? (
              <div className="mt-1.5 space-y-0.5 rounded-md border border-amber-500/20 bg-amber-500/[0.04] p-1.5 text-xs text-amber-700 dark:text-amber-300">
                {card.enrichments.map((note, idx) => (
                  <div key={idx} className="flex items-center gap-1">
                    <AlertTriangle className="h-3 w-3 shrink-0" />
                    <span>{note}</span>
                  </div>
                ))}
              </div>
            ) : null}
          </div>

          <button
            type="button"
            onClick={() => onDismiss(card.id)}
            disabled={dismissing}
            className={cn(
              "shrink-0 inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors",
              isResolved
                ? "bg-muted/60 text-muted-foreground hover:bg-muted hover:text-foreground border border-border"
                : "text-muted-foreground/60 hover:text-muted-foreground hover:bg-muted/40"
            )}
            title="Dismiss"
          >
            <X className="h-3 w-3" />
            <span>Dismiss</span>
          </button>
        </div>

        <div className="mt-2 border-t border-border/60 pt-1.5">
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
          >
            {expanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
            <span>
              {card.members.length} contributing rule{card.members.length === 1 ? "" : "s"}
            </span>
          </button>

          {expanded ? (
            <div className="mt-1.5 space-y-1 pl-1">
              {card.members.map((m, idx) => (
                <div
                  key={`${m.rule_id}-${idx}`}
                  className="flex flex-wrap items-center justify-between gap-1.5 rounded-md border border-border/50 bg-muted/20 px-2 py-1 text-xs"
                >
                <div className="flex flex-wrap items-center gap-1.5 min-w-0">
                  <RoleBadge role={m.role} />
                  <span className="font-medium text-foreground truncate text-[11px]">{m.rule_name}</span>
                  {m.service_name ? (
                    <span className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                      {m.service_name}
                    </span>
                  ) : null}
                  {m.domain ? (
                    <span className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                      {m.domain}
                    </span>
                  ) : null}
                  {m.pool ? (
                    <span className="rounded bg-muted px-1 py-0.5 text-[11px] text-muted-foreground">
                      {m.pool}
                    </span>
                  ) : null}
                  <span className="text-[11px] text-muted-foreground">
                    {m.status === "resolved" && m.resolved_at
                      ? `resolved ${formatRelativeTime(m.resolved_at)} ago`
                      : `since ${formatRelativeTime(m.since)}`}
                  </span>
                </div>

                {m.investigate_url ? (
                  <Link
                    to={m.investigate_url}
                    className="inline-flex items-center gap-1 text-[11px] font-medium text-primary hover:underline shrink-0"
                  >
                    <span>Investigate</span>
                    <ArrowRight className="h-2.5 w-2.5" />
                  </Link>
                ) : null}
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  )
}

export function IncidentCards({ incidents }: { incidents?: IncidentCard[] }) {
  const [dismissed, setDismissed] = useState<Set<string>>(new Set())

  const handleDismiss = (id: string) => {
    setDismissed((prev) => new Set(prev).add(id))
    dismissIncident(id).catch(() => {
      setDismissed((prev) => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
    })
  }

  const safe = (incidents ?? []).filter((c) => !dismissed.has(c.id))
  const securityIncidents = safe.filter((c) => c.category === "security")
  const performanceIncidents = safe.filter((c) => c.category === "performance")

  if (safe.length === 0) {
    return (
      <div className="rounded-lg border border-emerald-500/25 bg-emerald-500/[0.03] p-5 text-center shadow-card">
        <div className="mx-auto flex h-7 w-7 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500">
          <CheckCircle2 className="h-3.5 w-3.5" />
        </div>
        <h3 className="mt-2 text-sm font-semibold text-foreground">No active incidents right now.</h3>
        <p className="mt-1 text-xs text-muted-foreground max-w-md mx-auto">
          All performance metrics and security checks across your fleet are within normal operating bounds.
        </p>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {securityIncidents.length > 0 ? (
        <div className="space-y-2">
          <div className="flex items-center gap-2 border-b border-red-500/20 pb-1.5">
            <ShieldAlert className="h-3.5 w-3.5 text-red-500" />
            <h3 className="text-xs font-semibold text-foreground">
              Security Incidents ({securityIncidents.length})
            </h3>
          </div>
          <div className="space-y-2">
            {securityIncidents.map((card) => (
              <CardItem key={card.id} card={card} onDismiss={handleDismiss} />
            ))}
          </div>
        </div>
      ) : null}

      {performanceIncidents.length > 0 ? (
        <div className="space-y-2">
          <div className="flex items-center gap-2 border-b border-border pb-1.5">
            <Activity className="h-3.5 w-3.5 text-primary" />
            <h3 className="text-xs font-semibold text-foreground">
              Performance Incidents ({performanceIncidents.length})
            </h3>
          </div>
          <div className="space-y-2">
            {performanceIncidents.map((card) => (
              <CardItem key={card.id} card={card} onDismiss={handleDismiss} />
            ))}
          </div>
        </div>
      ) : null}
    </div>
  )
}
