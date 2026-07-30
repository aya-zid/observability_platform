import { useState, useMemo } from "react"
import { getNowSeconds, dateTimeInputToSeconds, getStep } from "@/lib/queryHelpers"
import { TimeRangeSelector, TIME_RANGES } from "@/components/shared/TimeRangeSelector"
import type { TimeSelection } from "@/components/shared/TimeRangeSelector"
import { SummaryCards } from "@/pages/hosts/sections/SummaryCards"
import { CPUSection } from "@/pages/hosts/sections/CPUSection"
import { MemorySection } from "@/pages/hosts/sections/MemorySection"
import { DiskSection } from "@/pages/hosts/sections/DiskSection"
import { NetworkSection } from "@/pages/hosts/sections/NetworkSection"
import { ProcessesSection } from "@/pages/hosts/sections/ProcessesSection"
import { LogsHealthSection } from "@/pages/hosts/sections/LogsHealthSection"

function SectionHeading({ title }: { title: string }) {
  return (
    <div className="border-b border-border pb-1">
      <h2 className="text-sm font-bold text-foreground">{title}</h2>
    </div>
  )
}

function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const end = getNowSeconds()
    const start = end - selection.preset.seconds
    return { start: String(start), end: String(end), step: getStep(selection.preset.seconds), error: null as string | null }
  }
  const start = dateTimeInputToSeconds(selection.date, selection.startTime)
  const end = dateTimeInputToSeconds(selection.date, selection.endTime)
  if (start == null || end == null) return { start: "0", end: "0", step: "30s", error: "Invalid date/time" }
  if (end <= start) return { start: String(start), end: String(end), step: "30s", error: "End must be after start" }
  return { start: String(start), end: String(end), step: getStep(end - start), error: null as string | null }
}

export function MetricsTab({ machineId }: { machineId: string }) {
  const [timeSelection, setTimeSelection] = useState<TimeSelection>({
    mode: "preset",
    preset: TIME_RANGES[1],
  })

  const params = useMemo(() => {
    const w = getTimeWindow(timeSelection)
    return { start: w.start, end: w.end, step: w.step }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [timeSelection])

  return (
    <div className="grid gap-5">
      <TimeRangeSelector selected={timeSelection} onChange={setTimeSelection} />

      <SummaryCards machineId={machineId} />

      <div>
        <SectionHeading title="CPU" />
        <div className="mt-3">
          <CPUSection machineId={machineId} params={params} />
        </div>
      </div>

      <div>
        <SectionHeading title="Memory" />
        <div className="mt-3">
          <MemorySection machineId={machineId} params={params} />
        </div>
      </div>

      <div>
        <SectionHeading title="Disk" />
        <div className="mt-3">
          <DiskSection machineId={machineId} params={params} />
        </div>
      </div>

      <div>
        <SectionHeading title="Network" />
        <div className="mt-3">
          <NetworkSection machineId={machineId} params={params} />
        </div>
      </div>

      <div>
        <SectionHeading title="Processes" />
        <div className="mt-3">
          <ProcessesSection machineId={machineId} params={params} />
        </div>
      </div>

      <div>
        <SectionHeading title="Log Health" />
        <div className="mt-3">
          <LogsHealthSection machineId={machineId} params={params} />
        </div>
      </div>
    </div>
  )
}
