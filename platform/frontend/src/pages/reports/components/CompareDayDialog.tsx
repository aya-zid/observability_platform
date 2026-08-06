import { useState } from "react"
import { format } from "date-fns"
import { CalendarRange } from "lucide-react"

import { Button } from "@/components/ui/button"

function toISODate(d: Date): string {
  return format(d, "yyyy-MM-dd")
}

function daysAgo(n: number): Date {
  const d = new Date()
  d.setUTCDate(d.getUTCDate() - n)
  d.setUTCHours(0, 0, 0, 0)
  return d
}

/**
 * Small modal with two date pickers (default today vs yesterday) to start a
 * day-comparison report. On generate, resolves UTC YYYY-MM-DD dates.
 */
export default function CompareDayDialog({
  onClose,
  onGenerate,
}: {
  onClose: () => void
  onGenerate: (dateA: string, dateB: string) => void
}) {
  const [dayA, setDayA] = useState<string>(toISODate(daysAgo(1)))
  const [dayB, setDayB] = useState<string>(toISODate(daysAgo(0)))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")

  const sameDay = dayA === dayB

  function submit() {
    if (sameDay) {
      setError("The two dates must differ.")
      return
    }
    setBusy(true)
    onGenerate(dayA, dayB)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-2xl border border-border bg-card p-6 shadow-card">
        <div className="mb-4 flex items-center gap-2">
          <CalendarRange className="h-4 w-4 text-primary" />
          <h3 className="text-base font-semibold">Compare Days</h3>
        </div>
        <p className="mb-4 text-sm text-muted-foreground">
          Pick two days (UTC). Day B is compared against Day A; deltas are
          precomputed and shown in the report.
        </p>

        <div className="grid grid-cols-2 gap-3">
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-muted-foreground">Day A (reference)</span>
            <input
              type="date"
              value={dayA}
              onChange={(e) => setDayA(e.target.value)}
              className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-medium text-muted-foreground">Day B (current)</span>
            <input
              type="date"
              value={dayB}
              onChange={(e) => setDayB(e.target.value)}
              className="w-full rounded-lg border border-border bg-background px-3 py-2 text-sm"
            />
          </label>
        </div>

        {error && (
          <div className="mt-3 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
            {error}
          </div>
        )}

        <div className="mt-5 flex justify-end gap-2">
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button onClick={submit} disabled={busy || sameDay}>
            {busy ? "Generating…" : "Generate Report"}
          </Button>
        </div>
      </div>
    </div>
  )
}
