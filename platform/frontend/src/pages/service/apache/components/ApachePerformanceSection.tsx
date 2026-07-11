import { useEffect, useMemo, useState, type ReactNode, Fragment } from 'react'
import { useQuery } from '@tanstack/react-query'

import { getApacheRTHeatmap } from '@/api/apache'
import type {
  ApacheRTDistribution,
  ApacheRTHeatmap,
} from '@/api/apache'
import { formatDateInputValue, formatNumber, secondsToNanos } from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

type Props = {
  id: string
  domain: string
}

function pad2(value: number) {
  return value.toString().padStart(2, '0')
}

function formatHourMinute(date: Date) {
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`
}

function formatBucketLabel(label: string) {
  return label
}

function formatHourOptionLabel(startMs: number, endMs: number, isCurrentHour: boolean) {
  const start = new Date(startMs)
  const end = new Date(endMs)
  if (isCurrentHour) {
    return `${formatHourMinute(start)} - now`
  }
  return `${formatHourMinute(start)} - ${formatHourMinute(end)}`
}

type HourOption = {
  value: number
  label: string
  startMs: number
  endMs: number
  isCurrentHour: boolean
}

function parseDateInputValue(value: string): Date | null {
  const match = value.trim().match(/^(\d{2})-(\d{2})-(\d{4})$/)
  if (!match) return null

  const day = Number(match[1])
  const month = Number(match[2])
  const year = Number(match[3])
  const parsed = new Date(year, month - 1, day, 0, 0, 0, 0)
  if (
    parsed.getFullYear() !== year ||
    parsed.getMonth() !== month - 1 ||
    parsed.getDate() !== day
  ) {
    return null
  }

  return parsed
}

function getHourOptionsForDate(dateValue: string, nowMs: number): HourOption[] {
  const selectedDate = parseDateInputValue(dateValue)
  if (!selectedDate) return []

  const now = new Date(nowMs)
  const startOfDay = new Date(selectedDate.getFullYear(), selectedDate.getMonth(), selectedDate.getDate(), 0, 0, 0, 0)
  const currentHourStart = new Date(now.getFullYear(), now.getMonth(), now.getDate(), now.getHours(), 0, 0, 0)
  const isToday =
    selectedDate.getFullYear() === now.getFullYear() &&
    selectedDate.getMonth() === now.getMonth() &&
    selectedDate.getDate() === now.getDate()

  const options: HourOption[] = []
  const lastHour = isToday ? now.getHours() : 23

  for (let hour = 0; hour <= lastHour; hour += 1) {
    const hourStart = new Date(startOfDay)
    hourStart.setHours(hour, 0, 0, 0)

    const nextHour = new Date(hourStart)
    nextHour.setHours(hour + 1, 0, 0, 0)

    const isCurrentHour = isToday && hourStart.getTime() === currentHourStart.getTime()
    const endMs = isCurrentHour ? nowMs : nextHour.getTime()

    options.push({
      value: hourStart.getTime(),
      label: formatHourOptionLabel(hourStart.getTime(), endMs, isCurrentHour),
      startMs: hourStart.getTime(),
      endMs,
      isCurrentHour,
    })
  }

  return options.reverse()
}

function formatMs(ms: number) {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

function heatColor(value: number, max: number) {
  if (!value || max <= 0) return 'rgba(148, 163, 184, 0.12)'
  const ratio = Math.max(0, Math.min(1, value / max))
  if (ratio < 0.5) {
    const t = ratio / 0.5
    const r = Math.round(34 + (245 - 34) * t)
    const g = Math.round(197 + (158 - 197) * t)
    const b = Math.round(94 + (11 - 94) * t)
    return `rgb(${r}, ${g}, ${b})`
  }
  const t = (ratio - 0.5) / 0.5
  const r = Math.round(245 + (239 - 245) * t)
  const g = Math.round(158 + (68 - 158) * t)
  const b = Math.round(11 + (68 - 11) * t)
  return `rgb(${r}, ${g}, ${b})`
}

function SectionCard({
  title,
  subtitle,
  children,
}: {
  title: string
  subtitle?: string
  children: ReactNode
}) {
  return (
    <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
      <div className="mb-4 flex flex-col gap-1">
        <h3 className="text-sm font-semibold text-foreground">{title}</h3>
        {subtitle ? <p className="text-xs text-muted-foreground">{subtitle}</p> : null}
      </div>
      {children}
    </div>
  )
}

function LoadingState({ height = 'h-64' }: { height?: string }) {
  return <div className={cn('flex items-center justify-center text-sm text-muted-foreground', height)}>Loading...</div>
}

function EmptyState({ message, height = 'h-64' }: { message: string; height?: string }) {
  return <div className={cn('flex items-center justify-center text-center text-sm text-muted-foreground', height)}>{message}</div>
}

function ErrorState({ message, height = 'h-64' }: { message: string; height?: string }) {
  return (
    <div className={cn('flex items-center justify-center text-center text-sm text-red-600 dark:text-red-400', height)}>
      {message}
    </div>
  )
}

function normalizeHeatmap(data?: ApacheRTHeatmap) {
  return {
    paths: Array.isArray(data?.paths) ? data.paths : [],
    buckets: Array.isArray(data?.buckets) ? data.buckets : [],
    matrix: Array.isArray(data?.matrix) ? data.matrix : [],
    counts: Array.isArray(data?.counts) ? data.counts : [],
    p95Counts: Array.isArray(data?.p95_counts) ? data.p95_counts : [],
  }
}

function HeatmapChart({
  data,
  selectedPath,
  onSelectPath,
}: {
  data?: ApacheRTHeatmap
  selectedPath: string | null
  onSelectPath: (path: string | null) => void
}) {
  const heatmap = useMemo(() => normalizeHeatmap(data), [data])

  const maxValue = useMemo(() => {
    const values = heatmap.matrix.flat().filter((value) => value > 0)
    return values.length ? Math.max(...values) : 0
  }, [heatmap.matrix])

  if (!data) {
    return <LoadingState height="h-[28rem]" />
  }

  if (!heatmap.paths.length || !heatmap.buckets.length) {
    return <EmptyState height="h-[28rem]" message="No path-level response time data for the selected window." />
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
        <span>Click a path row to highlight it.</span>
        {selectedPath ? (
          <button
            type="button"
            onClick={() => onSelectPath(null)}
            className="rounded border border-border px-2 py-1 text-foreground transition-colors hover:bg-muted"
          >
            Clear path filter
          </button>
        ) : null}
      </div>

      <div className="overflow-x-auto">
        <div
          className="grid min-w-[900px] gap-2"
          style={{ gridTemplateColumns: `260px repeat(${heatmap.buckets.length}, minmax(34px, 1fr))` }}
        >
          <div />
          {heatmap.buckets.map((bucket) => (
            <div key={bucket} className="text-center text-[11px] text-muted-foreground">
              {formatBucketLabel(bucket)}
            </div>
          ))}

          {heatmap.paths.map((path, rowIndex) => {
            const isSelected = selectedPath === path
            return (
              <Fragment key={path}>
                <button
                  type="button"
                  onClick={() => onSelectPath(isSelected ? null : path)}
                  className={cn(
                    'truncate rounded-md px-3 py-2 text-left font-mono text-xs transition-colors',
                    isSelected ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900' : 'hover:bg-muted',
                  )}
                  title={path}
                >
                  {path}
                </button>
                {(Array.isArray(heatmap.matrix[rowIndex]) ? heatmap.matrix[rowIndex] : []).map((value, bucketIndex) => {
                  const count = heatmap.counts[rowIndex]?.[bucketIndex] ?? 0
                  const p95Count = heatmap.p95Counts[rowIndex]?.[bucketIndex] ?? 0
                  return (
                    <div
                      key={`${path}-${bucketIndex}`}
                      className="flex h-11 flex-col items-center justify-center rounded-md text-[10px] font-semibold leading-tight text-slate-950"
                      style={{ backgroundColor: heatColor(value, maxValue) }}
                      title={`${path}\n${heatmap.buckets[bucketIndex]}\nP95 ${value > 0 ? formatMs(value) : 'No data'}\nTotal requests in this bucket: ${formatNumber(count, 0)}\nRequests inside P95: ${formatNumber(p95Count, 0)}`}
                    >
                      {value > 0 ? (
                        <>
                          <span>{Math.round(value)}ms</span>
                          <span className="text-[9px] font-medium opacity-80">{formatNumber(p95Count, 0)} / {formatNumber(count, 0)} req</span>
                        </>
                      ) : ''}
                    </div>
                  )
                })}
              </Fragment>
            )
          })}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3 text-[11px] text-muted-foreground">
        <span className="font-medium">P95 RT scale</span>
        <div className="h-2 w-36 rounded-full bg-[linear-gradient(90deg,#22c55e_0%,#f59e0b_50%,#ef4444_100%)]" />
        <span>0ms</span>
        <span>{maxValue > 0 ? formatMs(maxValue) : 'No data'}</span>
        <span>Cell count format: requests inside P95 / total bucket requests.</span>
      </div>
    </div>
  )
}

export function ApacheRTDistributionChart({
  data,
  error,
}: {
  data?: ApacheRTDistribution
  error?: string | null
}) {
  if (error) {
    return <ErrorState message={error} />
  }

  const metrics = useMemo(() => {
    if (!data) return []
    return [
      { key: '2xx', color: '#22c55e', tint: 'rgba(34, 197, 94, 0.12)', values: data['2xx'] },
      { key: '3xx', color: '#3b82f6', tint: 'rgba(59, 130, 246, 0.12)', values: data['3xx'] },
      { key: '4xx', color: '#f59e0b', tint: 'rgba(245, 158, 11, 0.12)', values: data['4xx'] },
      { key: '5xx', color: '#ef4444', tint: 'rgba(239, 68, 68, 0.12)', values: data['5xx'] },
    ] as const
  }, [data])

  const maxValue = useMemo(() => {
    const values = metrics.flatMap((group) => [group.values.avg, group.values.p50, group.values.p95, group.values.p99]).filter((value) => value > 0)
    return values.length ? Math.max(...values) : 0
  }, [metrics])

  const breakdownRows = useMemo(() => {
    return metrics.map((group) => {
      const breakdown = Object.entries(group.values.status_breakdown ?? {})
        .sort((a, b) => Number(a[0]) - Number(b[0]))
      return { key: group.key, color: group.color, breakdown }
    })
  }, [metrics])

  if (!data) {
    return <LoadingState />
  }

  if (maxValue === 0) {
    return <EmptyState message="No response-time distribution data for the selected window." />
  }

  const chartHeight = 390
  const chartWidth = 920
  const padding = { top: 28, right: 28, bottom: 96, left: 64 }
  const plotHeight = chartHeight - padding.top - padding.bottom
  const plotWidth = chartWidth - padding.left - padding.right
  const y = (value: number) => padding.top + plotHeight - (value / maxValue) * plotHeight

  return (
    <div className="overflow-x-auto">
      <svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} className="h-[370px] min-w-[920px] w-full">
        {[0, 0.25, 0.5, 0.75, 1].map((step) => {
          const value = maxValue * step
          const lineY = y(value)
          return (
            <g key={step}>
              <line x1={padding.left} y1={lineY} x2={chartWidth - padding.right} y2={lineY} stroke="rgba(148, 163, 184, 0.25)" />
              <text x={10} y={lineY + 4} fontSize="11" fill="#94a3b8">
                {Math.round(value)}ms
              </text>
            </g>
          )
        })}

        {metrics.map((group, index) => {
          const groupWidth = plotWidth / metrics.length
          const groupGap = 34
          const innerWidth = groupWidth - groupGap
          const groupX = padding.left + groupWidth * index + groupGap / 2
          const barGap = 12
          const barWidth = Math.min(28, (innerWidth - barGap * 3) / 4)
          const bars = [
            { label: 'Avg', value: group.values.avg, fill: '#64748b', count: null },
            { label: 'P50', value: group.values.p50, fill: group.color, count: group.values.p50_count },
            { label: 'P95', value: group.values.p95, fill: group.color, count: group.values.p95_count },
            { label: 'P99', value: group.values.p99, fill: group.color, count: group.values.p99_count },
          ]

          return (
            <g key={group.key}>
              <rect
                x={groupX}
                y={padding.top}
                width={innerWidth}
                height={plotHeight}
                rx="12"
                fill={group.tint}
                stroke="rgba(148, 163, 184, 0.12)"
              />

              {bars.map((bar, barIndex) => {
                const rawHeight = plotHeight * (bar.value / maxValue)
                const barHeight = bar.value > 0 ? Math.max(14, rawHeight) : 0
                const totalBarsWidth = bars.length * barWidth + (bars.length - 1) * barGap
                const barX = groupX + (innerWidth - totalBarsWidth) / 2 + barIndex * (barWidth + barGap)
                const barY = padding.top + plotHeight - barHeight
                const textY = Math.max(padding.top + 16, barY - 14)

                return (
                  <g key={`${group.key}-${bar.label}`}>
                    <rect
                      x={barX}
                      y={barY}
                      width={barWidth}
                      height={barHeight}
                      rx="7"
                      fill={bar.fill}
                      opacity={bar.label === 'Avg' ? 0.75 : 1}
                    />
                    <text
                      x={barX + barWidth / 2}
                      y={textY}
                      textAnchor="middle"
                      fontSize="11"
                      fontWeight="600"
                      fill="#e2e8f0"
                    >
                      {Math.round(bar.value)}ms
                    </text>
                    <text
                      x={barX + barWidth / 2}
                      y={chartHeight - 66}
                      textAnchor="middle"
                      fontSize="11"
                      fill="#94a3b8"
                    >
                      {bar.label}
                    </text>
                    {bar.count !== null ? (
                      <text
                        x={barX + barWidth / 2}
                        y={chartHeight - 51}
                        textAnchor="middle"
                        fontSize="10"
                        fill="#94a3b8"
                      >
                        {formatNumber(bar.count, 0)}
                      </text>
                    ) : null}
                  </g>
                )
              })}

              <text
                x={groupX + innerWidth / 2}
                y={chartHeight - 32}
                textAnchor="middle"
                fontSize="13"
                fontWeight="700"
                fill={group.color}
              >
                {group.key}
              </text>
            </g>
          )
        })}
      </svg>

      <div className="mt-3 flex flex-wrap gap-4 text-xs text-muted-foreground">
        <span>Each status group contains Avg, P50, P95, and P99 bars.</span>
        <span>Only percentile bars show request counts; Avg does not repeat the total.</span>
        <span>Shared Y-axis = response time in ms.</span>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {breakdownRows.map((group) => (
          <div
            key={group.key}
            className="rounded-lg border border-border/70 bg-muted/20 p-3"
          >
            <div className="mb-2 flex items-center justify-between gap-2">
              <span className="text-sm font-semibold" style={{ color: group.color }}>
                {group.key} breakdown
              </span>
            </div>
            {group.breakdown.length > 0 ? (
              <div className="flex flex-col gap-2">
                {group.breakdown.map(([code, count]) => (
                  <div key={`${group.key}-${code}`} className="rounded-md border border-border/50 bg-background/50 p-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-semibold text-foreground">
                        Status <span className="font-mono" style={{ color: group.color }}>{code}</span>
                      </span>
                      <span className="text-xs font-medium text-muted-foreground">{formatNumber(count, 0)} req</span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-xs text-muted-foreground">No status details</div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

export function ApachePerformanceSection({ id, domain }: Props) {
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [selectedDate, setSelectedDate] = useState(() => formatDateInputValue(new Date()))
  const [selectedHourStartMs, setSelectedHourStartMs] = useState<number | null>(null)
  const [nowMs, setNowMs] = useState(() => Date.now())

  const isDateValid = parseDateInputValue(selectedDate) !== null
  const hourOptions = useMemo(() => getHourOptionsForDate(selectedDate, nowMs), [selectedDate, nowMs])
  const selectedHour =
    hourOptions.find((option) => option.value === selectedHourStartMs) ??
    hourOptions[0] ??
    null

  useEffect(() => {
    const timer = window.setInterval(() => {
      setNowMs(Date.now())
    }, 30_000)
    return () => window.clearInterval(timer)
  }, [])

  useEffect(() => {
    if (!hourOptions.some((option) => option.value === selectedHourStartMs) && hourOptions.length > 0) {
      setSelectedHourStartMs(hourOptions[0].value)
    }
  }, [hourOptions, selectedHourStartMs])

  const heatmapQuery = useQuery({
    queryKey: [
      'apache-performance-heatmap',
      id,
      domain,
      selectedHour?.startMs,
      selectedHour?.isCurrentHour ? 'current-hour' : selectedHour?.endMs,
    ],
    queryFn: () => {
      return getApacheRTHeatmap(id, {
        start: secondsToNanos(Math.floor((selectedHour?.startMs ?? nowMs) / 1000)),
        end: secondsToNanos(Math.floor((selectedHour?.endMs ?? nowMs) / 1000)),
        domain,
      })
    },
    enabled: Boolean(id && domain && selectedHour && isDateValid),
    staleTime: 20_000,
    refetchInterval: selectedHour?.isCurrentHour ? 25_000 : false,
    refetchOnWindowFocus: false,
  })

  useEffect(() => {
    const paths = normalizeHeatmap(heatmapQuery.data).paths
    if (selectedPath && !paths.includes(selectedPath)) {
      setSelectedPath(null)
    }
  }, [heatmapQuery.data, selectedPath])

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-border bg-card p-5 shadow-sm">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <h2 className="text-lg font-semibold text-foreground">Response-Time Deep Dive</h2>
            <p className="text-sm text-muted-foreground">
              Independent heatmap window for hot paths. This date/hour control does not change the dashboard range above.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              <span>Performance date</span>
              <input
                value={selectedDate}
                onChange={(event) => setSelectedDate(event.target.value)}
                placeholder="DD-MM-YYYY"
                className={cn(
                  'rounded-md border bg-card px-3 py-2 text-sm text-foreground outline-none transition-colors focus:border-blue-500',
                  isDateValid ? 'border-border' : 'border-red-500',
                )}
              />
            </label>
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              <span>Performance hour</span>
              <select
                value={selectedHour?.value ?? ''}
                onChange={(event) => setSelectedHourStartMs(Number(event.target.value))}
                disabled={!isDateValid || hourOptions.length === 0}
                className="rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground outline-none transition-colors focus:border-blue-500"
              >
                {hourOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <div className="rounded-md border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
              Domain: <span className="font-mono text-foreground">{domain}</span>
            </div>
          </div>
        </div>
      </div>

      <SectionCard
        title="RT Intensity Heatmap"
        subtitle="Top normalized paths in the selected window. Each 5-minute cell shows P95 response time and request count."
      >
        <HeatmapChart
          data={heatmapQuery.data}
          selectedPath={selectedPath}
          onSelectPath={setSelectedPath}
        />
      </SectionCard>
    </div>
  )
}
