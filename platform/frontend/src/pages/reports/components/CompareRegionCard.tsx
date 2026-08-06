import { Moon, Sunrise, Sun, Sunset, TrendingDown, TrendingUp, Minus } from "lucide-react"

import type { ReportRegion, RegionAI, ReportServiceMetrics } from "@/api/reports"
import { cn } from "@/lib/utils"
import {
  REGION_META, SERVICE_LABELS, serviceMetricEntries, METRIC_DEFS,
  formatNumber, formatPct, formatRate, formatMs, formatBytes,
} from "../utils"

const REGION_ICONS: Record<string, typeof Sun> = {
  night: Moon,
  morning: Sunrise,
  afternoon: Sun,
  evening: Sunset,
}

function formatMetricValue(key: string, value: number): string {
  if (key.endsWith("_pct") || key.includes("ratio_pct") || key.includes("hit_rate") || key.includes("saturation_pct")) return formatPct(value)
  if (key.includes("_ms") || key === "p95_rt_ms" || key === "latency_p95_ms") return formatMs(value)
  if (key.includes("_bytes") || key === "bandwidth_bytes" || key === "db_size_bytes") return formatBytes(value)
  if (key.includes("rate") || key.includes("_per_sec") || key.includes("_per_min")) return formatRate(value)
  return formatNumber(value)
}

function DeltaBadge({ value }: { value?: number | null }) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return (
      <span className="inline-flex items-center gap-0.5 rounded px-1 py-0.5 text-[11px] font-semibold text-muted-foreground bg-muted">
        <Minus className="h-3 w-3" /> n/a
      </span>
    )
  }
  const up = value > 0
  const Icon = Math.abs(value) < 0.01 ? Minus : up ? TrendingUp : TrendingDown
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded px-1 py-0.5 text-[11px] font-semibold tabular-nums",
        Math.abs(value) < 0.01
          ? "text-muted-foreground bg-muted"
          : up
            ? "text-green-600 dark:text-green-400 bg-green-500/10"
            : "text-red-600 dark:text-red-400 bg-red-500/10"
      )}
    >
      <Icon className="h-3 w-3" />
      {value > 0 ? "+" : ""}{Number(value).toFixed(1)}%
    </span>
  )
}

export function CompareRegionCard({
  region,
  ai,
}: {
  region: ReportRegion
  ai?: RegionAI
}) {
  const meta = REGION_META[region.region] ?? { label: region.region, hours: "", icon: "Sun" }
  const Icon = REGION_ICONS[region.region] ?? Sun
  const dayA = region.day_a
  const dayB = region.day_b
  const diffs = ai?.biggest_differences ?? []

  // Merge services from both days for a side-by-side table
  const kinds = Array.from(new Set([
    ...Object.keys(dayA.services ?? {}),
    ...Object.keys(dayB?.services ?? {}),
  ]))

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
        {dayA.alerts_fired !== dayB?.alerts_fired && dayB && (
          <span className="ml-auto text-xs text-muted-foreground">
            Alerts {dayA.alerts_fired ?? 0} → {dayB.alerts_fired ?? 0}
          </span>
        )}
      </div>

      <div className="flex-1 space-y-3 p-3.5">
        {ai?.summary && (
          <p className="text-sm leading-relaxed text-foreground/90">{ai.summary}</p>
        )}

        {diffs.length > 0 && (
          <div className="space-y-1">
            <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Biggest differences</p>
            {diffs.map((d, i) => (
              <div key={i} className="flex flex-wrap items-center gap-2 rounded-md border border-border bg-muted/30 px-2.5 py-1.5 text-xs">
                <span className="font-medium text-foreground">{d.metric}</span>
                <span className="text-muted-foreground">{d.day_a}</span>
                <span className="text-muted-foreground">→</span>
                <span className="font-mono text-foreground">{d.day_b}</span>
                <DeltaBadge value={d.delta_pct ?? null} />
              </div>
            ))}
          </div>
        )}

        {kinds.length > 0 ? (
          <div className="@container space-y-3">
            {kinds.map((kind) => {
              const ma = dayA.services?.[kind] as ReportServiceMetrics | undefined
              const mb = dayB?.services?.[kind] as ReportServiceMetrics | undefined
              const keys = Array.from(new Set([
                ...(ma ? serviceMetricEntries(ma).map((e) => e.key) : []),
                ...(mb ? serviceMetricEntries(mb).map((e) => e.key) : []),
              ])).slice(0, 8)
              if (keys.length === 0) return null
              return (
                <div key={kind} className="rounded-lg border border-border bg-card p-3">
                  <p className="mb-2 text-sm font-semibold uppercase tracking-wide text-muted-foreground">
                    {SERVICE_LABELS[kind] ?? kind}
                  </p>
                  <div className="grid grid-cols-1 gap-2 @md:grid-cols-2">
                    {keys.map((key) => {
                      const a = ma ? (ma as unknown as Record<string, number>)[key] : undefined
                      const b = mb ? (mb as unknown as Record<string, number>)[key] : undefined
                      const label = METRIC_DEFS[key]?.label ?? key
                      return (
                        <div key={key} className="flex min-w-0 items-center justify-between gap-2.5 rounded-md border border-border/60 px-2.5 py-1.5 text-[13px]">
                          <span className="min-w-0 truncate text-muted-foreground">{label}</span>
                          <span className="flex shrink-0 items-center gap-1.5 font-mono tabular-nums">
                            <span className={cn(a !== undefined && "text-muted-foreground")}>{a !== undefined ? formatMetricValue(key, a) : "—"}</span>
                            <span className="text-muted-foreground/50">→</span>
                            <span className={cn(b !== undefined && "text-foreground font-medium")}>{b !== undefined ? formatMetricValue(key, b) : "—"}</span>
                          </span>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        ) : (
          <p className="text-xs text-muted-foreground">No service data for this region.</p>
        )}
      </div>
    </div>
  )
}
