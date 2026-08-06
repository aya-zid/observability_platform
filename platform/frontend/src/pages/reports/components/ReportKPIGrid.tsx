import { TrendingDown, TrendingUp, Minus } from "lucide-react"

import type { ReportNumbers, ReportDelta } from "@/api/reports"
import { cn } from "@/lib/utils"
import { formatNumber, formatPct, formatRate, formatMs, SERVICE_LABELS } from "../utils"

type KPI = {
  label: string
  value: string
  sub?: string
  delta?: number | null
  invert?: boolean
  tone?: "good" | "bad" | "neutral"
}

function fmtDelta(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return ""
  const sign = v > 0 ? "+" : ""
  return `${sign}${Number(v).toFixed(1)}%`
}

function DeltaChip({ delta, invert }: { delta: number | null; invert?: boolean }) {
  if (delta === null || delta === undefined || Number.isNaN(delta)) return null
  const up = delta > 0
  const bad = invert ? !up : up
  const Icon = Math.abs(delta) < 0.01 ? Minus : up ? TrendingUp : TrendingDown
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-semibold",
        Math.abs(delta) < 0.01
          ? "text-muted-foreground bg-muted"
          : bad
            ? "text-red-600 dark:text-red-400 bg-red-500/10"
            : "text-green-600 dark:text-green-400 bg-green-500/10"
      )}
    >
      <Icon className="h-3 w-3" />
      {fmtDelta(delta)}
    </span>
  )
}

// Aggregate a metric across the 4 regions of a single day's windows.
function daySum(numbers: ReportNumbers, metric: string): number | null {
  let sum = 0
  let found = false
  for (const r of numbers.regions ?? []) {
    const win = r.day_a
    for (const svc of Object.values(win.services ?? {})) {
      const v = (svc as Record<string, unknown>)[metric]
      if (typeof v === "number" && Number.isFinite(v)) {
        sum += v
        found = true
      }
    }
  }
  return found ? sum : null
}

function dayMax(numbers: ReportNumbers, metric: string): number | null {
  let max = -Infinity
  let found = false
  for (const r of numbers.regions ?? []) {
    const win = r.day_a
    for (const svc of Object.values(win.services ?? {})) {
      const v = (svc as Record<string, unknown>)[metric]
      if (typeof v === "number" && Number.isFinite(v)) {
        if (v > max) max = v
        found = true
      }
    }
  }
  return found ? max : null
}

function deltaFor(deltas: ReportDelta[] | undefined, metric: string): number | null {
  if (!deltas) return null
  // whole-day row has region === "day"
  const row = deltas.find((d) => d.region === "day" && d.metric === metric)
  return row?.delta_pct ?? null
}

export function ReportKPIGrid({ numbers }: { numbers: ReportNumbers }) {
  const isCompare = numbers.type === "compare"
  const deltas = numbers.deltas

  const requests = daySum(numbers, "total_requests")
  const fiveXx = daySum(numbers, "total_5xx")
  const p95 = dayMax(numbers, "p95_rt_ms")
  const reqRate = daySum(numbers, "avg_req_rate")

  const errorRate =
    requests && requests > 0 && fiveXx !== null && fiveXx !== undefined
      ? (fiveXx / requests) * 100
      : null

  const alerts = (numbers.alerts_a ?? []).length
  const alertsB = isCompare ? (numbers.alerts_b ?? []).length : null
  const incidents = (numbers.incidents ?? []).length

  // For compare, prefer the "day" delta rows to compute the change chips.
  const kpis: KPI[] = [
    {
      label: "Requests",
      value: requests !== null ? formatNumber(requests) : "—",
      delta: deltaFor(deltas, "total_requests"),
      tone: "neutral",
    },
    {
      label: "Avg Request Rate",
      value: reqRate !== null ? formatRate(reqRate) : "—",
      delta: deltaFor(deltas, "avg_req_rate"),
    },
    {
      label: "Error Rate",
      value: errorRate !== null ? formatPct(errorRate) : "—",
      sub: fiveXx !== null ? `${formatNumber(fiveXx)} 5xx` : undefined,
      delta: fiveXx !== null ? deltaFor(deltas, "total_5xx") : null,
      tone: errorRate !== null && errorRate > 5 ? "bad" : errorRate !== null && errorRate > 1 ? "neutral" : "good",
    },
    {
      label: "P95 Latency",
      value: p95 !== null ? formatMs(p95) : "—",
      delta: deltaFor(deltas, "p95_rt_ms"),
      tone: p95 !== null && p95 > 500 ? "bad" : "neutral",
    },
    {
      label: "Alerts",
      value: isCompare ? `${alertsB ?? 0} / ${alerts}` : String(alerts ?? 0),
      delta: isCompare && alerts && alertsB !== null ? ((alertsB - alerts) / alerts) * 100 : null,
      invert: true,
    },
    {
      label: "Incidents",
      value: String(incidents),
    },
  ]

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
      {kpis.map((k) => (
        <div key={k.label} className="rounded-lg border border-border bg-card p-3 shadow-sm">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">{k.label}</p>
          <div className="mt-1 flex items-baseline gap-1.5">
            <p
              className={cn(
                "text-lg font-bold tracking-tight tabular-nums",
                k.tone === "bad" && "text-red-600 dark:text-red-400",
                k.tone === "good" && "text-green-600 dark:text-green-400",
                !k.tone && "text-foreground"
              )}
            >
              {k.value}
            </p>
            {k.delta !== undefined && <DeltaChip delta={k.delta} invert={k.invert} />}
          </div>
          {k.sub ? <p className="mt-0.5 text-xs text-muted-foreground">{k.sub}</p> : null}
        </div>
      ))}
    </div>
  )
}

export function ServiceDeltaTable({ numbers }: { numbers: ReportNumbers }) {
  const deltas = (numbers.deltas ?? []).filter((d) => d.region === "day")
  if (deltas.length === 0) {
    return (
      <p className="py-6 text-center text-sm text-muted-foreground">
        No whole-day comparison metrics available.
      </p>
    )
  }
  // Group by service
  const byService: Record<string, ReportDelta[]> = {}
  for (const d of deltas) {
    ;(byService[d.service] ??= []).push(d)
  }
  const order = Object.keys(byService).sort((a, b) =>
    (SERVICE_LABELS[a] ?? a).localeCompare(SERVICE_LABELS[b] ?? b)
  )

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="bg-secondary/50 text-xs uppercase tracking-wider text-muted-foreground">
          <tr>
            <th className="text-left px-3 py-2 font-semibold">Service</th>
            <th className="text-left px-3 py-2 font-semibold">Metric</th>
            <th className="text-right px-3 py-2 font-semibold">Day A</th>
            <th className="text-right px-3 py-2 font-semibold">Day B</th>
            <th className="text-right px-3 py-2 font-semibold">Δ</th>
          </tr>
        </thead>
        <tbody>
          {order.map((service) =>
            byService[service].map((d, i) => (
              <tr key={`${service}-${d.metric}`} className="border-t border-border">
                <td className="px-3 py-2 font-medium text-foreground">
                  {i === 0 ? (SERVICE_LABELS[service] ?? service) : ""}
                </td>
                <td className="px-3 py-2 text-muted-foreground">{d.metric}</td>
                <td className="px-3 py-2 text-right tabular-nums">{d.day_a_value !== null && d.day_a_value !== undefined ? formatNumber(d.day_a_value, 2) : "—"}</td>
                <td className="px-3 py-2 text-right tabular-nums">{d.day_b_value !== null && d.day_b_value !== undefined ? formatNumber(d.day_b_value, 2) : "—"}</td>
                <td className="px-3 py-2 text-right">
                  <DeltaChip delta={d.delta_pct ?? null} />
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
