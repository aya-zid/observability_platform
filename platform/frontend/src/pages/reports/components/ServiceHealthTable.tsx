import { Activity, Circle, Server } from "lucide-react"

import type { ReportServiceHealth } from "@/api/reports"
import { cn } from "@/lib/utils"
import { SERVICE_LABELS } from "../utils"

export function ServiceHealthTable({ health }: { health?: ReportServiceHealth[] }) {
  const rows = health ?? []
  if (rows.length === 0) {
    return <p className="py-6 text-center text-sm text-muted-foreground">No service health data.</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead className="bg-secondary/50 text-xs uppercase tracking-wider text-muted-foreground">
          <tr>
            <th className="text-left px-3 py-2 font-semibold">Service</th>
            <th className="text-left px-3 py-2 font-semibold">Present</th>
            <th className="text-right px-3 py-2 font-semibold">Up Instances</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((s) => (
            <tr key={s.service} className="border-t border-border">
              <td className="px-3 py-2">
                <span className="flex items-center gap-1.5 font-medium text-foreground">
                  {s.service === "container" ? <Server className="h-3.5 w-3.5 text-muted-foreground" /> : <Activity className="h-3.5 w-3.5 text-muted-foreground" />}
                  {SERVICE_LABELS[s.service] ?? s.service}
                </span>
              </td>
              <td className="px-3 py-2">
                <span
                  className={cn(
                    "inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium",
                    s.present
                      ? "bg-green-500/10 text-green-600 dark:text-green-400"
                      : "bg-muted text-muted-foreground"
                  )}
                >
                  <Circle className={cn("h-2 w-2 fill-current", s.present ? "text-green-500" : "text-muted-foreground")} />
                  {s.present ? "Up" : "No data"}
                </span>
              </td>
              <td className="px-3 py-2 text-right tabular-nums text-foreground">
                {s.present ? s.up_instances : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
