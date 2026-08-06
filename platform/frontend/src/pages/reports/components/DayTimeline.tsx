import { CalendarClock, BellRing, AlertTriangle, CheckCircle2, type LucideIcon } from "lucide-react"

import type { ReportRegion, RegionAI, ReportAlertPayload } from "@/api/reports"
import { cn } from "@/lib/utils"
import { REGION_META } from "../utils"
/**
 * A vertical timeline of significant events across the day's regions. Events are
 * derived from AI region narratives (highlights/concerns) plus alerts fired per
 * region — matching the "Day Timeline" section of the report layout.
 */
export function DayTimeline({
  regions,
  aiRegions,
  alerts,
}: {
  regions: ReportRegion[]
  aiRegions: RegionAI[]
  alerts?: ReportAlertPayload[]
}) {
  const aiByRegion = new Map(aiRegions.map((r) => [r.region, r]))

  const events: { region: string; regionLabel: string; icon: LucideIcon; type: string; text: string }[] = []

  for (const r of regions) {
    const meta = REGION_META[r.region] ?? { label: r.region, hours: "" }
    const ai = aiByRegion.get(r.region)
    for (const h of ai?.highlights ?? []) {
      events.push({ region: r.region, regionLabel: `${meta.label} ${meta.hours}`, icon: CheckCircle2, type: "highlight", text: h })
    }
    for (const c of ai?.concerns ?? []) {
      events.push({ region: r.region, regionLabel: `${meta.label} ${meta.hours}`, icon: AlertTriangle, type: "concern", text: c })
    }
    const inRegion = (alerts ?? []).filter((a) => {
      const h = new Date(a.triggered_at).getUTCHours()
      const start = REGION_START[r.region]
      return h >= start && h < start + 6
    })
    for (const a of inRegion.slice(0, 3)) {
      events.push({
        region: r.region,
        regionLabel: `${meta.label} ${meta.hours}`,
        icon: BellRing,
        type: "alert",
        text: `${a.rule_name || "Alert"}${a.service ? ` · ${a.service}` : ""}`,
      })
    }
  }

  if (events.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-8 text-muted-foreground">
        <CalendarClock className="mb-2 h-8 w-8 opacity-25" />
        <p className="text-sm">A quiet day — no notable events per region.</p>
      </div>
    )
  }

  return (
    <div className="relative space-y-3 before:absolute before:left-2.5 before:top-1 before:bottom-1 before:w-px before:bg-border">
      {events.map((e, i) => (
        <div key={i} className="relative flex items-start gap-3 pl-8">
          <span
            className={cn(
              "absolute left-0 top-0.5 z-10 inline-flex h-5 w-5 items-center justify-center rounded-full border",
              e.type === "concern" && "border-destructive/30 bg-destructive/10 text-red-500",
              e.type === "highlight" && "border-green-500/30 bg-green-500/10 text-green-500",
              e.type === "alert" && "border-amber-500/30 bg-amber-500/10 text-amber-500"
            )}
          >
            <e.icon className="h-3 w-3" />
          </span>
          <div className="min-w-0">
            <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
              {e.regionLabel}
            </span>
            <p className="text-sm leading-relaxed text-foreground/90">{e.text}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

const REGION_START: Record<string, number> = {
  night: 0,
  morning: 6,
  afternoon: 12,
  evening: 18,
}
