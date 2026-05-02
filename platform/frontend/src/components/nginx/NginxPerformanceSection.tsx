import { useEffect, useMemo, useRef, useState, type ReactNode, Fragment } from 'react'
import { useQuery } from '@tanstack/react-query'

import {
  getNginxRTDistribution,
  getNginxRTHeatmap,
  getNginxRTScatter,
} from '@/api/nginx'
import type {
  NginxRTDistribution,
  NginxRTHeatmap,
  NginxRTScatter,
  NginxScatterPoint,
} from '@/api/nginx'
import { formatNumber, secondsToNanos } from '@/lib/queryHelpers'
import { cn } from '@/lib/utils'

type Props = {
  id: string
  domain: string
}

type ScatterTooltip = {
  x: number
  y: number
  point: NginxScatterPoint
}

const STATUS_CLASS_COLORS = {
  '2xx': '#22c55e',
  '3xx': '#3b82f6',
  '4xx': '#f59e0b',
  '5xx': '#ef4444',
} as const

function getStatusClass(status: number): keyof typeof STATUS_CLASS_COLORS {
  if (status >= 500) return '5xx'
  if (status >= 400) return '4xx'
  if (status >= 300) return '3xx'
  return '2xx'
}

function pad2(value: number) {
  return value.toString().padStart(2, '0')
}

function formatHourMinute(date: Date) {
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}`
}

function formatBucketLabel(label: string) {
  return label // already in local Tunis time from backend
}

function formatPointTime(unixMs: number) {
  const date = new Date(unixMs)
  return `${formatHourMinute(date)}:${pad2(date.getSeconds())}`
}

function formatHourOptionLabel(startMs: number, endMs: number, isCurrentHour: boolean) {
  const start = new Date(startMs)
  const end = new Date(endMs)
  if (isCurrentHour) {
    return `${formatHourMinute(start)} - now`
  }
  return `${formatHourMinute(start)} - ${formatHourMinute(end)}`
}

function getTodayHourOptions(nowMs: number) {
  const now = new Date(nowMs)
  const startOfDay = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 0, 0, 0, 0)
  const currentHourStart = new Date(now.getFullYear(), now.getMonth(), now.getDate(), now.getHours(), 0, 0, 0)

  const options: Array<{
    value: number
    label: string
    startMs: number
    endMs: number
    isCurrentHour: boolean
  }> = []

  for (let hour = 0; hour <= now.getHours(); hour += 1) {
    const hourStart = new Date(startOfDay)
    hourStart.setHours(hour, 0, 0, 0)

    const nextHour = new Date(hourStart)
    nextHour.setHours(hour + 1, 0, 0, 0)

    const isCurrentHour = hourStart.getTime() === currentHourStart.getTime()
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

function normalizeHeatmap(data?: NginxRTHeatmap) {
  return {
    paths: Array.isArray(data?.paths) ? data.paths : [],
    buckets: Array.isArray(data?.buckets) ? data.buckets : [],
    matrix: Array.isArray(data?.matrix) ? data.matrix : [],
  }
}

function normalizeScatter(data?: NginxRTScatter) {
  return {
    points: Array.isArray(data?.points) ? data.points : [],
  }
}

function HeatmapChart({
  data,
  selectedPath,
  onSelectPath,
}: {
  data?: NginxRTHeatmap
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
        <span>Click a path row to filter the scatter chart.</span>
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
                {(Array.isArray(heatmap.matrix[rowIndex]) ? heatmap.matrix[rowIndex] : []).map((value, bucketIndex) => (
                  <div
                    key={`${path}-${bucketIndex}`}
                    className="flex h-9 items-center justify-center rounded-md text-[10px] font-semibold text-slate-950"
                    style={{ backgroundColor: heatColor(value, maxValue) }}
                    title={`${path}\n${heatmap.buckets[bucketIndex]}\nP95 ${value > 0 ? formatMs(value) : 'No data'}`}
                  >
                    {value > 0 ? Math.round(value) : ''}
                  </div>
                ))}
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
      </div>
    </div>
  )
}

function ScatterChart({
  data,
  selectedPath,
  selectedHour,
}: {
  data?: NginxRTScatter
  selectedPath: string | null
  selectedHour: { startMs: number; endMs: number; label: string } | null
}) {
  const wrapperRef = useRef<HTMLDivElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const [tooltip, setTooltip] = useState<ScatterTooltip | null>(null)
  const [useLogScale, setUseLogScale] = useState(false)
  const scatter = useMemo(() => normalizeScatter(data), [data])

  const filteredPoints = useMemo(() => {
    const points = scatter.points
    if (!selectedPath) return points
    return points.filter((point) => point.normalized_path === selectedPath || point.is_aggregate)
  }, [scatter.points, selectedPath])

  const metrics = useMemo(() => {
    if (!filteredPoints.length) return null

    const startMs = selectedHour?.startMs ?? filteredPoints.reduce((m, p) => Math.min(m, p.time), filteredPoints[0].time)
    const endMs = selectedHour?.endMs ?? filteredPoints.reduce((m, p) => Math.max(m, p.time), filteredPoints[0].time)
    let maxRt = 1
    for (const point of filteredPoints) {
      if (point.rt > maxRt) maxRt = point.rt
    }

    return {
      minTime: startMs,
      maxTime: Math.max(endMs, startMs + 1),
      maxRt: Math.max(maxRt, 1),
    }
  }, [filteredPoints, selectedHour])

  const toLogFrac = (rt: number, maxRt: number) => {
    const lo = Math.log(Math.max(rt, 0.1))
    const hi = Math.log(Math.max(maxRt, 1))
    return hi <= 0 ? 0 : Math.max(0, Math.min(1, lo / hi))
  }

  useEffect(() => {
    const wrapper = wrapperRef.current
    const canvas = canvasRef.current
    if (!wrapper || !canvas) return

    const draw = () => {
      const ctx = canvas.getContext('2d')
      if (!ctx) return

      const dpr = window.devicePixelRatio || 1
      const width = wrapper.clientWidth
      const height = 400
      canvas.width = width * dpr
      canvas.height = height * dpr
      canvas.style.width = `${width}px`
      canvas.style.height = `${height}px`
      ctx.scale(dpr, dpr)
      ctx.clearRect(0, 0, width, height)

      ctx.fillStyle = 'rgba(148, 163, 184, 0.06)'
      ctx.fillRect(0, 0, width, height)

      const padding = { top: 20, right: 20, bottom: 40, left: 56 }
      const plotWidth = width - padding.left - padding.right
      const plotHeight = height - padding.top - padding.bottom

      ctx.font = '11px system-ui, sans-serif'

      // Y-axis
      const yTicks = 5
      ctx.strokeStyle = 'rgba(148, 163, 184, 0.18)'
      ctx.lineWidth = 1

      for (let i = 0; i <= yTicks; i += 1) {
        const frac = i / yTicks
        const y = padding.top + plotHeight - frac * plotHeight

        ctx.beginPath()
        ctx.moveTo(padding.left, y)
        ctx.lineTo(width - padding.right, y)
        ctx.stroke()

        if (metrics) {
          let rt: number
          if (useLogScale) {
            rt = Math.exp(frac * Math.log(Math.max(metrics.maxRt, 1)))
          } else {
            rt = frac * metrics.maxRt
          }
          ctx.fillStyle = '#94a3b8'
          ctx.textAlign = 'right'
          ctx.fillText(formatMs(rt), padding.left - 6, y + 4)
        }
      }

      // Window separators
      if (metrics) {
        const windowMs = 3 * 60 * 1000
        const timeSpan = metrics.maxTime - metrics.minTime
        const toX = (t: number) => padding.left + ((t - metrics.minTime) / timeSpan) * plotWidth

        ctx.strokeStyle = 'rgba(148, 163, 184, 0.12)'
        ctx.lineWidth = 1
        ctx.setLineDash([3, 4])
        for (let t = metrics.minTime; t <= metrics.maxTime; t += windowMs) {
          const x = toX(t)
          ctx.beginPath()
          ctx.moveTo(x, padding.top)
          ctx.lineTo(x, padding.top + plotHeight)
          ctx.stroke()
        }
        ctx.setLineDash([])
      }

      if (!metrics || !filteredPoints.length) {
        ctx.fillStyle = 'rgba(148, 163, 184, 0.6)'
        ctx.textAlign = 'center'
        ctx.font = '13px system-ui, sans-serif'
        ctx.fillText('No data', width / 2, height / 2)
        return
      }

      const timeSpan = metrics.maxTime - metrics.minTime
      const toX = (t: number) => padding.left + ((t - metrics.minTime) / timeSpan) * plotWidth
      const toY = (rt: number) => {
        const frac = useLogScale ? toLogFrac(rt, metrics.maxRt) : rt / metrics.maxRt
        return padding.top + plotHeight - frac * plotHeight
      }

      // X-axis labels
      ctx.fillStyle = '#94a3b8'
      ctx.textAlign = 'center'
      ctx.font = '11px system-ui, sans-serif'
      const xTicks = Math.min(6, Math.floor(plotWidth / 80))
      for (let i = 0; i <= xTicks; i += 1) {
        const t = metrics.minTime + (timeSpan * i) / xTicks
        const x = toX(t)
        ctx.fillText(formatPointTime(t), x, height - 10)
      }

      // Aggregate dots
      const aggPoints = filteredPoints.filter((p) => p.is_aggregate)
      const indivPoints = filteredPoints.filter((p) => !p.is_aggregate)

      for (const point of aggPoints) {
        const x = toX(point.time)
        const y = toY(point.rt)
        const statusClass = getStatusClass(point.status)
        const color = STATUS_CLASS_COLORS[statusClass]

        ctx.beginPath()
        ctx.arc(x, y, 7, 0, Math.PI * 2)
        ctx.fillStyle = color
        ctx.globalAlpha = 0.55
        ctx.fill()
        ctx.globalAlpha = 1
        ctx.strokeStyle = color
        ctx.lineWidth = 1.5
        ctx.stroke()
      }

      // Individual dots
      for (const point of indivPoints) {
        const x = toX(point.time)
        const y = toY(point.rt)
        const statusClass = getStatusClass(point.status)
        const radius = statusClass === '5xx' ? 4.5 : statusClass === '4xx' ? 4 : 3.5

        ctx.beginPath()
        ctx.arc(x, y, radius, 0, Math.PI * 2)
        ctx.fillStyle = STATUS_CLASS_COLORS[statusClass]
        ctx.globalAlpha = 0.8
        ctx.fill()
        ctx.globalAlpha = 1
      }
    }

    draw()
    const observer = new ResizeObserver(draw)
    observer.observe(wrapper)
    return () => observer.disconnect()
  }, [filteredPoints, metrics, useLogScale])

  const handleMouseMove = (event: React.MouseEvent<HTMLCanvasElement>) => {
    const canvas = canvasRef.current
    if (!canvas || !metrics || !filteredPoints.length) {
      setTooltip(null)
      return
    }

    const rect = canvas.getBoundingClientRect()
    const mx = event.clientX - rect.left
    const my = event.clientY - rect.top
    const padding = { top: 20, right: 20, bottom: 40, left: 56 }
    const plotWidth = rect.width - padding.left - padding.right
    const plotHeight = rect.height - padding.top - padding.bottom
    const timeSpan = metrics.maxTime - metrics.minTime
    const toX = (t: number) => padding.left + ((t - metrics.minTime) / timeSpan) * plotWidth
    const toY = (rt: number) => {
      const frac = useLogScale ? toLogFrac(rt, metrics.maxRt) : rt / metrics.maxRt
      return padding.top + plotHeight - frac * plotHeight
    }

    let match: NginxScatterPoint | null = null
    let bestDist = 16
    for (const point of filteredPoints) {
      const dx = toX(point.time) - mx
      const dy = toY(point.rt) - my
      const dist = Math.sqrt(dx * dx + dy * dy)
      if (dist < bestDist) {
        bestDist = dist
        match = point
      }
    }

    if (!match) {
      setTooltip(null)
      return
    }
    setTooltip({ x: mx, y: my, point: match })
  }

  if (!data) {
    return <LoadingState height="h-[25rem]" />
  }

  if (!scatter.points.length) {
    return (
      <EmptyState
        height="h-[25rem]"
        message="No scatter data returned from the backend for this window."
      />
    )
  }

  const indivCount = filteredPoints.filter((p) => !p.is_aggregate).length
  const aggCount = filteredPoints.filter((p) => p.is_aggregate).length

  return (
    <div className="relative">
      <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
          <span>
            <span className="font-medium text-foreground">{formatNumber(indivCount, 0)}</span> individual ·{' '}
            <span className="font-medium text-foreground">{formatNumber(aggCount, 0)}</span> aggregate dots
          </span>
          {selectedPath ? (
            <span className="rounded border border-border px-2 py-0.5 font-mono text-foreground">
              {selectedPath}
            </span>
          ) : (
            <span>All paths</span>
          )}
        </div>

        <button
          type="button"
          onClick={() => setUseLogScale((prev) => !prev)}
          className={cn(
            'rounded border px-3 py-1 text-xs transition-colors',
            useLogScale
              ? 'border-blue-500 bg-blue-500/10 text-blue-400'
              : 'border-border text-muted-foreground hover:bg-muted',
          )}
        >
          {useLogScale ? 'Log scale ✓' : 'Log scale'}
        </button>
      </div>

      <div className="mb-3 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
        {[
          { color: STATUS_CLASS_COLORS['2xx'], label: '2xx normal (avg dot)', agg: true },
          { color: STATUS_CLASS_COLORS['3xx'], label: '3xx normal (avg dot)', agg: true },
          { color: STATUS_CLASS_COLORS['4xx'], label: '4xx error', agg: false },
          { color: STATUS_CLASS_COLORS['5xx'], label: '5xx error', agg: false },
          { color: STATUS_CLASS_COLORS['2xx'], label: 'outlier (slow 2xx/3xx)', agg: false },
        ].map(({ color, label, agg }) => (
          <span key={label} className="flex items-center gap-1.5">
            <span
              className="inline-block rounded-full"
              style={{
                width: agg ? 12 : 8,
                height: agg ? 12 : 8,
                backgroundColor: color,
                opacity: agg ? 0.55 : 0.8,
                outline: agg ? `1.5px solid ${color}` : 'none',
              }}
            />
            {label}
          </span>
        ))}
      </div>

      <div ref={wrapperRef} className="relative overflow-hidden rounded-lg border border-border/70 bg-slate-950/5">
        <canvas
          ref={canvasRef}
          onMouseLeave={() => setTooltip(null)}
          onMouseMove={handleMouseMove}
          className="block w-full"
        />

        {tooltip && (
          <div
            className="pointer-events-none absolute z-10 w-80 rounded-lg border border-border bg-card p-3 text-xs shadow-lg"
            style={{
              left: Math.min(tooltip.x + 14, Math.max(14, (wrapperRef.current?.clientWidth ?? 320) - 336)),
              top: tooltip.y > 220 
                ? tooltip.y - 180   // flip upward if near bottom
                : Math.max(12, tooltip.y - 16),
            }}
          >
            <div className="mb-2 flex items-center gap-2">
              <span
                className="inline-block h-2.5 w-2.5 rounded-full"
                style={{ backgroundColor: STATUS_CLASS_COLORS[getStatusClass(tooltip.point.status)] }}
              />
              <span className="font-semibold text-foreground">
                {tooltip.point.is_aggregate
                  ? `Avg of ${formatNumber(tooltip.point.count ?? 0, 0)} ${getStatusClass(tooltip.point.status)} requests`
                  : 'Individual request'}
              </span>
            </div>
            <div className="space-y-1 text-muted-foreground">
              {tooltip.point.is_aggregate ? (
                <>
                  <div>Window: <span className="font-mono text-foreground">{tooltip.point.window_label ?? '—'}</span></div>
                  <div>Avg RT: <span className="font-mono text-foreground">{formatMs(tooltip.point.rt)}</span></div>
                  <div>Count: <span className="font-mono text-foreground">{formatNumber(tooltip.point.count ?? 0, 0)}</span></div>
                </>
              ) : (
                <>
                  <div>Time: <span className="font-mono text-foreground">{formatPointTime(tooltip.point.time)}</span></div>
                  <div>RT: <span className="font-mono text-foreground">{formatMs(tooltip.point.rt)}</span></div>
                  <div>Status: <span className="font-mono text-foreground">{tooltip.point.status}</span></div>
                  <div>Method: <span className="font-mono text-foreground">{tooltip.point.method || '—'}</span></div>
                  <div>Path: <span className="font-mono text-foreground break-all">{tooltip.point.path || '—'}</span></div>
                  {tooltip.point.normalized_path && (
                    <div>Normalized: <span className="font-mono text-foreground break-all">{tooltip.point.normalized_path}</span></div>
                  )}
                </>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function DistributionChart({
  data,
  error,
}: {
  data?: NginxRTDistribution
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
    const values = metrics.flatMap((group) => [group.values.avg, group.values.p95, group.values.p99]).filter((value) => value > 0)
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

  const chartHeight = 380
  const chartWidth = 920
  const padding = { top: 28, right: 28, bottom: 86, left: 64 }
  const plotHeight = chartHeight - padding.top - padding.bottom
  const plotWidth = chartWidth - padding.left - padding.right
  const y = (value: number) => padding.top + plotHeight - (value / maxValue) * plotHeight

  return (
    <div className="overflow-x-auto">
      <svg viewBox={`0 0 ${chartWidth} ${chartHeight}`} className="h-[360px] min-w-[920px] w-full">
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
          const barGap = 18
          const barWidth = Math.min(34, (innerWidth - barGap * 2) / 3)
          const bars = [
            { label: 'Avg', value: group.values.avg, fill: group.color },
            { label: 'P95', value: group.values.p95, fill: group.color },
            { label: 'P99', value: group.values.p99, fill: group.color },
          ]

          return (
            <g key={group.key}>
              <rect
                x={groupX}
                y={padding.top}
                width={innerWidth}
                height={plotHeight}
                rx="16"
                fill={group.tint}
                stroke="rgba(148, 163, 184, 0.12)"
              />

              {bars.map((bar, barIndex) => {
                const rawHeight = plotHeight * (bar.value / maxValue)
                const barHeight = bar.value > 0 ? Math.max(14, rawHeight) : 0
                const barX = groupX + barIndex * (barWidth + barGap) + 18
                const barY = padding.top + plotHeight - barHeight
                const textY = Math.max(padding.top + 16, barY - 14)

                return (
                  <g key={`${group.key}-${bar.label}`}>
                    <rect
                      x={barX}
                      y={barY}
                      width={barWidth}
                      height={barHeight}
                      rx="8"
                      fill={bar.fill}
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
                      y={chartHeight - 50}
                      textAnchor="middle"
                      fontSize="11"
                      fill="#94a3b8"
                    >
                      {bar.label}
                    </text>
                  </g>
                )
              })}

              <text
                x={groupX + innerWidth / 2}
                y={chartHeight - 36}
                textAnchor="middle"
                fontSize="13"
                fontWeight="700"
                fill={group.color}
              >
                {group.key}
              </text>
              <text
                x={groupX + innerWidth / 2}
                y={chartHeight - 18}
                textAnchor="middle"
                fontSize="11"
                fill="#94a3b8"
              >
                {formatNumber(group.values.count ?? 0, 0)} req
              </text>
            </g>
          )
        })}
      </svg>

      <div className="mt-3 flex flex-wrap gap-4 text-xs text-muted-foreground">
        <span>Each status group contains 3 bars: Avg, P95, and P99.</span>
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

export function NginxPerformanceSection({ id, domain }: Props) {
  const [selectedPath, setSelectedPath] = useState<string | null>(null)
  const [selectedHourStartMs, setSelectedHourStartMs] = useState<number | null>(null)
  const [nowMs, setNowMs] = useState(() => Date.now())

  const hourOptions = useMemo(() => getTodayHourOptions(nowMs), [nowMs])
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
    if (!selectedHour && hourOptions.length > 0) {
      setSelectedHourStartMs(hourOptions[0].value)
    }
  }, [hourOptions, selectedHour])

  const heatmapQuery = useQuery({
    queryKey: [
      'nginx-performance-heatmap',
      id,
      domain,
      selectedHour?.startMs,
      selectedHour?.isCurrentHour ? 'current-hour' : selectedHour?.endMs,
    ],
    queryFn: () => {
      return getNginxRTHeatmap(id, {
        start: secondsToNanos(Math.floor((selectedHour?.startMs ?? nowMs) / 1000)),
        end: secondsToNanos(Math.floor((selectedHour?.endMs ?? nowMs) / 1000)),
        domain,
      })
    },
    enabled: Boolean(id && domain && selectedHour),
    staleTime: 20_000,
    refetchInterval: 25_000,
    refetchOnWindowFocus: false,
  })

  const scatterQuery = useQuery({
    queryKey: [
      'nginx-performance-scatter',
      id,
      domain,
      selectedHour?.startMs,
      selectedHour?.isCurrentHour ? 'current-hour' : selectedHour?.endMs,
    ],
    queryFn: () => {
      return getNginxRTScatter(id, {
        start: secondsToNanos(Math.floor((selectedHour?.startMs ?? nowMs) / 1000)),
        end: secondsToNanos(Math.floor((selectedHour?.endMs ?? nowMs) / 1000)),
        domain,
      })
    },
    enabled: Boolean(id && domain && selectedHour),
    staleTime: 20_000,
    refetchInterval: 25_000,
    refetchOnWindowFocus: false,
  })

  const distributionQuery = useQuery({
    queryKey: [
      'nginx-performance-distribution',
      id,
      domain,
      selectedHour?.startMs,
      selectedHour?.isCurrentHour ? 'current-hour' : selectedHour?.endMs,
    ],
    queryFn: () => {
      return getNginxRTDistribution(id, {
        start: secondsToNanos(Math.floor((selectedHour?.startMs ?? nowMs) / 1000)),
        end: secondsToNanos(Math.floor((selectedHour?.endMs ?? nowMs) / 1000)),
        domain,
      })
    },
    enabled: Boolean(id && domain && selectedHour),
    staleTime: 20_000,
    refetchInterval: 25_000,
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
              Heatmap for hot paths, scatter for sampled raw requests, and a status-class RT distribution snapshot for one selected hour today.
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              <span>Performance hour</span>
              <select
                value={selectedHour?.value ?? ''}
                onChange={(event) => setSelectedHourStartMs(Number(event.target.value))}
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
        subtitle="Top normalized paths in the selected window. Each cell shows the path P95 response time for that bucket."
      >
        <HeatmapChart
          data={heatmapQuery.data}
          selectedPath={selectedPath}
          onSelectPath={setSelectedPath}
        />
      </SectionCard>

      <SectionCard
        title="Raw RT Scatter"
        subtitle="Canvas-rendered sampled dots across the selected window. Colors follow HTTP status class."
      >
        <ScatterChart data={scatterQuery.data} selectedPath={selectedPath} selectedHour={selectedHour} />
      </SectionCard>

      <SectionCard
        title="RT Distribution by Status"
        subtitle="Each status class shows Avg, P95, and P99 on one shared response-time axis."
      >
        <DistributionChart
          data={distributionQuery.data}
          error={distributionQuery.isError ? distributionQuery.error?.message ?? 'Failed to load RT distribution.' : null}
        />
      </SectionCard>
    </div>
  )
}