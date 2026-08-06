import { TrendingDown, TrendingUp, Minus } from "lucide-react"

import type { ServiceComparison } from "@/api/reports"
import { cn } from "@/lib/utils"
import { SERVICE_LABELS } from "../utils"

function DeltaChip({ value }: { value?: number | null }) {
  if (value === null || value === undefined || Number.isNaN(value)) {
    return <span className="text-xs text-muted-foreground">—</span>
  }
  const up = value > 0
  const Icon = Math.abs(value) < 0.01 ? Minus : up ? TrendingUp : TrendingDown
  return (
    <span
      className={cn(
        "inline-flex items-center gap-0.5 rounded px-1.5 py-0.5 text-[10px] font-semibold tabular-nums",
        Math.abs(value) < 0.01
          ? "text-muted-foreground bg-muted"
          : up
            ? "text-red-600 dark:text-red-400 bg-red-500/10"
            : "text-green-600 dark:text-green-400 bg-green-500/10"
      )}
    >
      <Icon className="h-3 w-3" />
      {value > 0 ? "+" : ""}{Number(value).toFixed(1)}%
    </span>
  )
}

export function ServiceComparisonTable({ comparisons }: { comparisons?: ServiceComparison[] }) {
  const list = comparisons ?? []
  if (list.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">No service comparisons in this report.</p>
  }

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
          {list.map((svc) =>
            svc.metrics.map((m, i) => (
              <tr key={`${svc.service}-${m.metric}`} className="border-t border-border">
                <td className="px-3 py-2 font-medium text-foreground">
                  {i === 0 ? SERVICE_LABELS[svc.service] ?? svc.service : ""}
                </td>
                <td className="px-3 py-2 text-muted-foreground">{m.metric}</td>
                <td className="px-3 py-2 text-right tabular-nums text-muted-foreground">{m.day_a}</td>
                <td className="px-3 py-2 text-right font-mono tabular-nums text-foreground">{m.day_b}</td>
                <td className="px-3 py-2 text-right">
                  <DeltaChip value={m.delta_pct ?? null} />
                </td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}
