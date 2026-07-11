export const TIME_RANGES = [
  { label: "15m", seconds: 900 },
  { label: "1h", seconds: 3600 },
  { label: "6h", seconds: 21600 },
  { label: "24h", seconds: 86400 },
] as const

export type PresetRange = (typeof TIME_RANGES)[number]

export interface ManualSelection {
  start: string
  end: string
  startSeconds: number
  endSeconds: number
}

export type TimeSelection =
  | { mode: "preset"; preset: PresetRange }
  | { mode: "manual"; manual: ManualSelection }

export function createDefaultManualSelection(): ManualSelection {
  const now = Math.floor(Date.now() / 1000)
  return {
    start: new Date((now - 3600) * 1000).toISOString().slice(0, 16),
    end: new Date(now * 1000).toISOString().slice(0, 16),
    startSeconds: now - 3600,
    endSeconds: now,
  }
}

export function getStep(rangeSeconds: number): string {
  if (rangeSeconds <= 900) return "15s"
  if (rangeSeconds <= 3600) return "30s"
  if (rangeSeconds <= 21600) return "2m"
  return "5m"
}

export function getTimeWindow(selection: TimeSelection) {
  if (selection.mode === "preset") {
    const now = Math.floor(Date.now() / 1000)
    const start = now - selection.preset.seconds
    return {
      start: String(start),
      end: String(now),
      seconds: selection.preset.seconds,
      step: getStep(selection.preset.seconds),
      isManual: false,
      error: null as string | null,
    }
  }
  const { startSeconds, endSeconds } = selection.manual
  if (!startSeconds || !endSeconds || endSeconds <= startSeconds) {
    return { start: "", end: "", seconds: 0, step: "30s", isManual: true, error: "Invalid time range" }
  }
  const diff = endSeconds - startSeconds
  return {
    start: String(startSeconds),
    end: String(endSeconds),
    seconds: diff,
    step: getStep(diff),
    isManual: true,
    error: null as string | null,
  }
}

export function formatChartTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function formatFullTime(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function mapTimeseries<T extends Record<string, number | null>>(
  points: T[],
  timeKey = "time",
): Array<Record<string, string | number>> {
  return points.map((pt) => {
    const row: Record<string, string | number> = {}
    for (const [k, v] of Object.entries(pt)) {
      if (k === timeKey && typeof v === "number") {
        row[k] = formatChartTime(v)
      } else if (v !== null && typeof v === "number" && Number.isFinite(v)) {
        row[k] = v
      }
    }
    return row
  })
}

export function latestFromSeries<T extends Record<string, number | null>>(
  data: T[] | undefined,
  key: string,
): number | null {
  if (!data || data.length === 0) return null
  return data[data.length - 1]?.[key] ?? null
}

export function nanosecToSec(ns: number): number {
  return ns / 1_000_000_000
}

export function secToNanosec(s: number): number {
  return s * 1_000_000_000
}

export function chartClickToLogsUrl(
  basePath: string,
  chartTime: number | undefined,
  timeWindowStart: string,
  timeWindowEnd: string,
): string {
  if (chartTime) {
    const t = Math.floor(chartTime)
    const start = t - 60
    const end = t + 60
    return `${basePath}?start=${start}&end=${end}`
  }
  return `${basePath}?start=${timeWindowStart}&end=${timeWindowEnd}`
}
