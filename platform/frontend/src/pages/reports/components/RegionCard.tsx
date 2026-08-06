import { Moon, Sunrise, Sun, Sunset, AlertTriangle, CheckCircle2, Info } from "lucide-react"

import type { ReportRegion, ReportServiceMetrics, RegionAI } from "@/api/reports"
import { cn } from "@/lib/utils"
import { REGION_META, SERVICE_LABELS, METRIC_DEFS, serviceMetricEntries, formatNumber, formatPct, formatRate, formatMs, formatBytes } from "../utils"

const REGION_ICONS: Record<string, typeof Sun> = {
  night: Moon,
  morning: Sunrise,
  afternoon: Sun,
  evening: Sunset,
}

function humanizeKey(key: string): string {
  return key.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatNotableValue(key: string, raw: string): string {
  const num = Number(raw)
  if (!Number.isFinite(num)) return raw
  if (key.endsWith("_pct") || key.includes("ratio_pct") || key.includes("hit_rate") || key.includes("saturation_pct")) {
    const pct = num > 0 && num < 1 ? num * 100 : num
    return `${pct.toFixed(1)}%`
  }
  if (key.includes("_per_sec") || key === "commands_per_sec") return `${num.toFixed(2)}/s`
  if (key.includes("_per_min") || key.includes("_per_hr")) return `${num.toFixed(2)}/min`
  if (key.includes("_ms")) return `${num.toFixed(0)} ms`
  if (key.includes("_bytes")) return formatBytes(num)
  return num >= 1000 ? num.toLocaleString(undefined, { maximumFractionDigits: 0 }) : `${Number.isInteger(num) ? num : num.toFixed(2)}`
}

export function NotableMetricsList({
  metrics,
}: {
  metrics: { service: string; metric: string; value: string }[]
}) {
  if (!metrics || metrics.length === 0) return null
  return (
    <div className="@container">
      <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Notable metrics</p>
      <div className="grid grid-cols-1 gap-2 @md:grid-cols-2">
        {metrics.map((m, i) => {
          const def = METRIC_DEFS[m.metric]
          return (
            <div key={i} className="flex min-w-0 items-center justify-between gap-3 rounded-md border border-border bg-muted/30 px-3 py-2">
              <span className="min-w-0 truncate text-[13px] font-medium text-foreground">
                {SERVICE_LABELS[m.service] ?? m.service}
                <span className="ml-1.5 font-normal text-muted-foreground">{def?.label ?? humanizeKey(m.metric)}</span>
              </span>
              <span className="shrink-0 font-mono text-[13px] font-medium tabular-nums text-primary">
                {formatNotableValue(m.metric, m.value)}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function formatMetricValue(key: string, value: number): string {
  if (key.endsWith("_pct") || key.includes("ratio_pct") || key.includes("hit_rate") || key.includes("saturation_pct")) return formatPct(value)
  if (key.includes("_ms") || key === "p95_rt_ms" || key === "latency_p95_ms") return formatMs(value)
  if (key.includes("_bytes") || key === "bandwidth_bytes" || key === "db_size_bytes") return formatBytes(value)
  if (key.includes("rate") || key.includes("_per_sec") || key.includes("_per_min")) return formatRate(value)
  return formatNumber(value)
}

export function ServiceSnapshot({ services }: { services?: Record<string, ReportServiceMetrics> }) {
  const entries = Object.entries(services ?? {})
  if (entries.length === 0) {
    return <p className="text-xs text-muted-foreground">No service data for this region.</p>
  }
  return (
    <div className="@container mt-3">
      <div className="grid grid-cols-1 gap-2.5 @sm:grid-cols-2 @2xl:grid-cols-3">
        {entries.map(([kind, m]) => {
          const metrics = serviceMetricEntries(m).slice(0, 6)
          if (metrics.length === 0) return null
          return (
            <div key={kind} className="rounded-lg border border-border bg-card p-3">
              <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                {SERVICE_LABELS[kind] ?? kind}
              </p>
              <div className="divide-y divide-border/60">
                {metrics.map(({ key, def }) => (
                  <div key={key} className="flex min-w-0 items-center justify-between gap-3 py-1.5">
                    <span className="min-w-0 truncate text-[13px] text-muted-foreground" title={def.label}>{def.label}</span>
                    <span className="shrink-0 font-mono text-sm font-medium tabular-nums text-foreground">
                      {formatMetricValue(key, (m as unknown as Record<string, number>)[key])}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function RegionCard({
  region,
  ai,
}: {
  region: ReportRegion
  ai?: RegionAI
}) {
  const meta = REGION_META[region.region] ?? { label: region.region, hours: "", icon: "Sun" }
  const Icon = REGION_ICONS[region.region] ?? Sun
  const hasConcerns = (ai?.concerns?.length ?? 0) > 0

  return (
    <div className="rounded-lg border border-border bg-card shadow-card flex flex-col">
      <div className="flex items-center gap-2.5 border-b border-border px-3.5 py-2.5">
        <span className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md border border-border bg-muted/35 text-primary">
          <Icon className="h-3.5 w-3.5" />
        </span>
        <div className="min-w-0">
          <p className="text-sm font-semibold text-foreground">{meta.label}</p>
          <p className="text-xs text-muted-foreground">{meta.hours} UTC</p>
        </div>
      </div>

      <div className="flex-1 space-y-3 p-3.5">
        {ai ? (
          <>
            {ai.summary && (
              <p className="text-sm leading-relaxed text-foreground/90">{ai.summary}</p>
            )}
            {(ai.highlights?.length ?? 0) > 0 && (
              <div className="space-y-1">
                {ai.highlights!.map((h, i) => (
                  <p key={i} className="flex items-start gap-1.5 text-xs text-green-600 dark:text-green-400">
                    <CheckCircle2 className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <span>{h}</span>
                  </p>
                ))}
              </div>
            )}
            {(ai.concerns?.length ?? 0) > 0 && (
              <div className={cn("space-y-1 rounded-md border border-destructive/20 bg-destructive/5 p-2", !hasConcerns && "border-transparent bg-transparent p-0")}>
                {ai.concerns!.map((c, i) => (
                  <p key={i} className="flex items-start gap-1.5 text-xs text-red-600 dark:text-red-400">
                    <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                    <span>{c}</span>
                  </p>
                ))}
              </div>
            )}
            <NotableMetricsList metrics={ai.notable_metrics ?? []} />
          </>
        ) : (
          <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
            <Info className="h-4 w-4" />
            No AI narrative for this region.
          </p>
        )}

        <ServiceSnapshot services={region.day_a.services} />
      </div>
    </div>
  )
}
