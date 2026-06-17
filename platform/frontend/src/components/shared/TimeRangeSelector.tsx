import { formatDateInputValue, formatTimeInputValue } from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

export const TIME_RANGES = [
  { label: '15m', seconds: 900 },
  { label: '1h', seconds: 3600 },
  { label: '6h', seconds: 21600 },
  { label: '24h', seconds: 86400 },
] as const

export type TimeRange = (typeof TIME_RANGES)[number]

export type TimeSelection =
  | { mode: 'preset'; preset: TimeRange }
  | { mode: 'manual'; date: string; startTime: string; endTime: string }

export function createDefaultManualSelection(): TimeSelection {
  const end = new Date()
  const start = new Date(end.getTime() - 60 * 60 * 1000)
  return {
    mode: 'manual',
    date: formatDateInputValue(end),
    startTime: formatTimeInputValue(start),
    endTime: formatTimeInputValue(end),
  }
}

export function TimeRangeSelector({
  selected,
  onChange,
}: {
  selected: TimeSelection
  onChange: (selection: TimeSelection) => void
}) {
  const manual = selected.mode === 'manual' ? selected : createDefaultManualSelection()

  return (
    <div className="flex flex-wrap items-center gap-2">
      <div className="flex gap-1 rounded-lg border border-border bg-muted p-1 shadow-sm">
        {TIME_RANGES.map((range) => (
          <button
            key={range.label}
            type="button"
            onClick={() => onChange({ mode: 'preset', preset: range })}
            className={cn(
              'rounded px-3 py-1.5 text-sm font-medium transition-all duration-200',
              selected.mode === 'preset' && selected.preset.seconds === range.seconds
                ? 'bg-primary text-primary-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground hover:bg-muted/70',
            )}
          >
            {range.label}
          </button>
        ))}
        <button
          type="button"
          onClick={() => onChange(manual)}
          className={cn(
            'rounded px-3 py-1.5 text-sm font-medium transition-all duration-200',
            selected.mode === 'manual'
              ? 'bg-primary text-primary-foreground shadow-sm'
              : 'text-muted-foreground hover:text-foreground hover:bg-muted/70',
          )}
        >
          Manual
        </button>
      </div>

      {selected.mode === 'manual' ? (
        <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card px-2 py-1 shadow-sm">
          <label className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            Date
            <input
              type="text"
              inputMode="numeric"
              placeholder="DD-MM-YYYY"
              value={selected.date}
              onChange={(event) => onChange({ ...selected, date: event.target.value })}
              className="h-8 w-28 rounded-md border border-border bg-background px-2 text-sm text-foreground outline-none focus:border-primary"
            />
          </label>
          <label className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            Start
            <input
              type="time"
              value={selected.startTime}
              onChange={(event) => onChange({ ...selected, startTime: event.target.value })}
              className="h-8 rounded-md border border-border bg-background px-2 text-sm text-foreground outline-none focus:border-primary"
            />
          </label>
          <label className="flex items-center gap-1 text-xs font-medium text-muted-foreground">
            End
            <input
              type="time"
              value={selected.endTime}
              onChange={(event) => onChange({ ...selected, endTime: event.target.value })}
              className="h-8 rounded-md border border-border bg-background px-2 text-sm text-foreground outline-none focus:border-primary"
            />
          </label>
        </div>
      ) : null}
    </div>
  )
}
