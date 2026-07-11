import { useMemo } from "react"
import type { PHPFPMDashboardSummary, PHPFPMTimePoint } from "@/api/phpfpm"
import { formatDuration, formatNumber, formatPercent, formatBytes } from "@/lib/queryHelpers"
import { cn } from "@/lib/utils"

type Severity = "critical" | "warning" | "notice"

interface Finding {
  severity: Severity
  title: string
  detail: string
  category?: string
}

interface Insight {
  title: string
  detail: string
}

interface Profile {
  hasMultiplePools: boolean
  opcacheEnabled: boolean
  accessLogsAvailable: boolean
  isUp: boolean
  pools: number
}

function classify(data: PHPFPMDashboardSummary): Profile {
  return {
    hasMultiplePools: (data.pools?.length ?? 0) > 1,
    opcacheEnabled: data.opcache_enabled === 1,
    accessLogsAvailable: (data.req_rate_timeseries?.length ?? 0) > 0,
    isUp: data.up === 1,
    pools: data.pools?.length ?? 0,
  }
}

function tsStartEnd(series: PHPFPMTimePoint[], key: string): { first: number | null; last: number | null } {
  if (!series || series.length < 2) return { first: null, last: null }
  let first: number | null = null
  let last: number | null = null
  for (const pt of series) {
    const v = pt[key]
    if (v != null && first == null) first = v as number
  }
  for (let i = series.length - 1; i >= 0; i--) {
    const v = series[i][key]
    if (v != null && last == null) last = v as number
  }
  return { first, last }
}

function trendWord(series: PHPFPMTimePoint[], key: string, higherIsBad: boolean): string {
  const { first, last } = tsStartEnd(series, key)
  if (first == null || last == null) return ""
  const pct = ((last - first) / Math.abs(first || 1)) * 100
  if (Math.abs(pct) < 5) return ""
  const going = pct > 0 ? "up" : "down"
  if (higherIsBad) return going === "up" ? ` (rising: ${formatNumber(first, 1)} → ${formatNumber(last, 1)})` : ""
  return ` (${going}: ${formatNumber(first, 1)} → ${formatNumber(last, 1)})`
}

type Tone = "ok" | "warn" | "critical" | "na" | "info"

const TONE = {
  ok:       { dot: "bg-emerald-400",   text: "text-emerald-300",  ring: "ring-emerald-400/30",  bar: "bg-emerald-400",  glow: "shadow-[0_0_24px_-8px_rgba(52,211,153,0.45)]",  surface: "bg-emerald-400/[0.04] border-emerald-400/15" },
  warn:     { dot: "bg-amber-400",     text: "text-amber-300",    ring: "ring-amber-400/30",    bar: "bg-amber-400",    glow: "shadow-[0_0_24px_-8px_rgba(251,191,36,0.5)]",   surface: "bg-amber-400/[0.04] border-amber-400/15" },
  critical: { dot: "bg-rose-500",      text: "text-rose-300",     ring: "ring-rose-500/30",     bar: "bg-rose-500",     glow: "shadow-[0_0_28px_-8px_rgba(244,63,94,0.55)]",   surface: "bg-rose-500/[0.05] border-rose-500/20" },
  info:     { dot: "bg-sky-400",       text: "text-sky-300",      ring: "ring-sky-400/30",      bar: "bg-sky-400",      glow: "shadow-[0_0_24px_-8px_rgba(56,189,248,0.45)]",  surface: "bg-sky-400/[0.04] border-sky-400/15" },
  na:       { dot: "bg-zinc-600",      text: "text-zinc-500",     ring: "ring-zinc-700/40",     bar: "bg-zinc-600",     glow: "",                                              surface: "bg-zinc-500/[0.03] border-zinc-700/40" },
} as const

const SEV_BY_OVERALL: Record<Severity, Tone> = { critical: "critical", warning: "warn", notice: "ok" }

const Icon = {
  Pulse:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M3 12h4l3-8 4 16 3-8h4"/></svg>),
  Bolt:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M13 2 4 14h7l-1 8 9-12h-7l1-8z"/></svg>),
  Chip:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><rect x="4" y="4" width="16" height="16" rx="2"/><path d="M9 4v-2M15 4v-2M9 22v-2M15 22v-2M4 9h-2M4 15h-2M22 9h-2M22 15h-2"/><rect x="8" y="8" width="8" height="8" rx="1"/></svg>),
  Db:     (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><ellipse cx="12" cy="5" rx="8" ry="3"/><path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5"/><path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6"/></svg>),
  Link:   (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1"/></svg>),
  Flame:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 2s4 4 4 8a4 4 0 0 1-8 0c0-1.5 1-2.5 1-2.5S5 11 5 15a7 7 0 0 0 14 0c0-6-7-13-7-13z"/></svg>),
  Eye:    (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z"/><circle cx="12" cy="12" r="3"/></svg>),
  Check:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="m5 12 5 5L20 7"/></svg>),
  Alert:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 9v4M12 17h.01"/><path d="M10.3 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/></svg>),
  Spark:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M5.6 18.4l2.1-2.1M16.3 7.7l2.1-2.1"/></svg>),
  Diamond:(p: any) => (<svg viewBox="0 0 24 24" fill="currentColor" {...p}><path d="M12 2 22 12 12 22 2 12z"/></svg>),
  Users:  (p: any) => (<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" {...p}><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>),
}

export function PHPFPMSummaryTab({ data }: { data: PHPFPMDashboardSummary }) {
  const profile = useMemo(() => classify(data), [data])

  const uptimeMin = (data.uptime_seconds ?? 0) / 60
  const saturation = data.worker_saturation_pct ?? 0
  const queueUtil = data.queue_utilization_pct ?? 0
  const queueDepth = data.listen_queue ?? 0
  const throughput = data.accepted_conn_per_sec ?? 0
  const avgDur = data.avg_duration_ms ?? 0
  const avgCpu = data.avg_cpu_pct ?? 0
  const opcacheHit = data.opcache_hit_rate ?? 1
  const opcacheMemUsed = data.opcache_memory_used_ratio ?? 0
  const opcacheWasted = data.opcache_wasted_ratio ?? 0
  const opcacheRestarts = data.opcache_restarts_total ?? 0
  const latencyP95 = data.latency_p95 ?? 0
  const latencyP99 = data.latency_p99 ?? 0
  const status5xx = data.status_5xx ?? 0
  const maxChildren = data.max_children_reached ?? 0
  const slowReqs = data.slow_requests ?? 0
  const errors = data.log_summary?.errors ?? 0
  const warnings = data.log_summary?.warnings ?? 0

  const findings = useMemo<Finding[]>(() => {
    const out: Finding[] = []
    if (!profile.isUp) {
      out.push({ severity: "critical", category: "Availability", title: "PHP-FPM unreachable", detail: "phpfpm_up is 0 — the server is not responding." })
      return out
    }
    if ((data.scrape_failures ?? 0) > 0) {
      out.push({ severity: "warning", category: "Monitoring", title: "Scrape failures detected", detail: `${data.scrape_failures} scrape failure(s) in this window. Exporter connectivity may be unstable.` })
    }
    if (uptimeMin < 15 && (data.log_summary?.startups ?? 0) > 0) {
      out.push({ severity: "notice", category: "Availability", title: "Server restarted", detail: `PHP-FPM restarted ${formatDuration(data.uptime_seconds ?? 0)} ago.` })
    }

    if (saturation > 85) {
      out.push({ severity: "critical", category: "Worker Pool", title: `Worker pool ${formatPercent(saturation)} saturated`, detail: `${data.active_processes ?? "?"} active of ${data.total_processes ?? "?"} total — add workers or investigate slow handlers.` })
    } else if (saturation > 70) {
      out.push({ severity: "warning", category: "Worker Pool", title: `Worker pool ${formatPercent(saturation)} saturated`, detail: `${data.active_processes ?? "?"} active of ${data.total_processes ?? "?"} total.` })
    }

    if (queueDepth > 0) {
      out.push({ severity: "critical", category: "Queue", title: `Queue depth ${formatNumber(queueDepth, 0)}`, detail: `Requests waiting for a free worker. Queue utilization ${formatPercent(queueUtil)}. Workers may be saturated.` })
    }
    if (queueUtil > 50 && queueDepth === 0) {
      out.push({ severity: "warning", category: "Queue", title: `Queue utilization ${formatPercent(queueUtil)}`, detail: `Configured queue limit is ${data.listen_queue_len ?? "?"}.` })
    }

    if (maxChildren > 0) {
      out.push({ severity: "critical", category: "Worker Pool", title: `Max children reached ${formatNumber(maxChildren, 0)} times`, detail: `Pool capacity exhausted. Increase pm.max_children or optimize memory usage.` })
    }

    if (slowReqs > 0) {
      out.push({
        severity: slowReqs > 10 ? "warning" : "notice",
        category: "Performance",
        title: `${formatNumber(slowReqs, 0)} slow request${slowReqs > 1 ? "s" : ""}`,
        detail: slowReqs > 10 ? "Elevated slow request count — check slowlog for bottlenecks." : "Minimal slow request activity.",
      })
    }

    if (avgDur > 1000) {
      out.push({ severity: "warning", category: "Performance", title: `Avg duration ${formatNumber(avgDur, 1)}ms`, detail: "Requests are slow on average. Review slow scripts and external service calls." })
    } else if (avgDur > 500) {
      out.push({ severity: "notice", category: "Performance", title: `Avg duration ${formatNumber(avgDur, 1)}ms`, detail: "Elevated average duration — monitor for increasing trend." })
    }

    if (avgCpu > 25) {
      out.push({ severity: "warning", category: "Performance", title: `Avg CPU ${formatPercent(avgCpu)}`, detail: "High CPU usage per process — investigate CPU-bound scripts." })
    } else if (avgCpu > 5) {
      out.push({ severity: "notice", category: "Performance", title: `Avg CPU ${formatPercent(avgCpu)}`, detail: "Moderate CPU usage." })
    }

    if (profile.opcacheEnabled) {
      if (opcacheHit < 0.85) {
        out.push({ severity: "critical", category: "OPcache", title: `Hit rate ${formatPercent(opcacheHit * 100)}`, detail: "Below 85% — increase opcache.memory_consumption or opcache.max_accelerated_files." })
      } else if (opcacheHit < 0.9) {
        out.push({ severity: "warning", category: "OPcache", title: `Hit rate ${formatPercent(opcacheHit * 100)}`, detail: "Below 90% — monitor and tune OPcache settings." })
      }
      if (opcacheMemUsed > 0.8) {
        out.push({ severity: "warning", category: "OPcache", title: `Memory ${formatPercent(opcacheMemUsed * 100)} used`, detail: `${data.opcache_memory_free_bytes ? formatBytes(data.opcache_memory_free_bytes) : "0"} free. Increase opcache.memory_consumption.` })
      }
      if (opcacheWasted > 0.05) {
        out.push({ severity: "warning", category: "OPcache", title: `Wasted memory ${formatPercent(opcacheWasted * 100)}`, detail: "Elevated waste ratio may trigger OOM restarts. opcache.revalidate_freq may need tuning." })
      }
      if (opcacheRestarts > 0) {
        out.push({ severity: "critical", category: "OPcache", title: `${formatNumber(opcacheRestarts, 0)} OPcache restart${opcacheRestarts > 1 ? "s" : ""}`, detail: "OPcache was flushed — likely caused by memory pressure or explicit reset." })
      }
    }

    if (latencyP99 > 5000) {
      out.push({ severity: "critical", category: "Latency", title: `p99 ${formatNumber(latencyP99, 1)}ms`, detail: "Severe latency spikes — investigate slow queries and upstream dependencies." })
    } else if (latencyP99 > 2000) {
      out.push({ severity: "warning", category: "Latency", title: `p99 ${formatNumber(latencyP99, 1)}ms`, detail: "High tail latency — review slow scripts and database queries." })
    } else if (latencyP95 > 1000) {
      out.push({ severity: "notice", category: "Latency", title: `p95 ${formatNumber(latencyP95, 1)}ms`, detail: "Elevated p95 latency." })
    }

    if (status5xx > 0) {
      out.push({ severity: "warning", category: "Errors", title: `5xx rate ${formatNumber(status5xx, 2)}/s`, detail: "Server errors detected — check application error logs." })
    }

    if (errors > 5) {
      out.push({ severity: "warning", category: "Logs", title: `${formatNumber(errors, 0)} error${errors > 1 ? "s" : ""} in logs`, detail: "Review recent logs for application errors." })
    } else if (errors > 0) {
      out.push({ severity: "notice", category: "Logs", title: `${formatNumber(errors, 0)} log error${errors > 1 ? "s" : ""}`, detail: "Minor error activity." })
    }

    if (warnings > 0) {
      out.push({ severity: "notice", category: "Logs", title: `${formatNumber(warnings, 0)} warning${warnings > 1 ? "s" : ""} in logs`, detail: "Review warning events for potential issues." })
    }

    return out
  }, [data, profile, uptimeMin, saturation, queueDepth, queueUtil, maxChildren, slowReqs, avgDur, avgCpu, opcacheHit, opcacheMemUsed, opcacheWasted, opcacheRestarts, latencyP95, latencyP99, status5xx, errors, warnings])

  const insights = useMemo<Insight[]>(() => {
    const out: Insight[] = []

    if (saturation > 70 && queueDepth > 0) {
      out.push({
        title: "Worker saturation driving queue buildup",
        detail: `Pool is ${formatPercent(saturation)} saturated with ${formatNumber(queueDepth, 0)} queued requests. Adding workers or optimizing slow handlers will reduce both.`,
      })
    }
    if (avgDur > 500 && avgCpu > 10) {
      out.push({
        title: "Slow requests correlate with high CPU",
        detail: `Average duration (${formatNumber(avgDur, 1)}ms) and CPU (${formatPercent(avgCpu)}) are both elevated — likely CPU-bound scripts or inefficient code paths.`,
      })
    }
    if (latencyP99 > 2000 && status5xx > 0) {
      out.push({
        title: "Latency spike coinciding with 5xx errors",
        detail: `p99 latency (${formatNumber(latencyP99, 1)}ms) and 5xx errors (${formatNumber(status5xx, 2)}/s) overlap — timeouts may be causing request failures.`,
      })
    }
    if (profile.opcacheEnabled && opcacheHit < 0.9 && opcacheWasted > 0.03) {
      out.push({
        title: "OPcache pressure: low hit rate + high waste",
        detail: `Hit rate ${formatPercent(opcacheHit * 100)} with ${formatPercent(opcacheWasted * 100)} waste. Tune opcache.max_accelerated_files and opcache.memory_consumption.`,
      })
    }
    if (maxChildren > 0) {
      const trend = trendWord(data.running_procs_timeseries ?? [], "running", true)
      out.push({
        title: "Pool capacity reached",
        detail: `pm.max_children was hit ${formatNumber(maxChildren, 0)} times.${trend} Consider increasing pm.max_children or reducing memory per process.`,
      })
    }

    return out
  }, [data, saturation, queueDepth, avgDur, avgCpu, latencyP99, status5xx, opcacheHit, opcacheWasted, maxChildren])

  const contextSignals = useMemo<Insight[]>(() => {
    const out: Insight[] = []
    if (throughput > 0) {
      const eff = (data.active_processes ?? 1) > 0 ? (throughput / (data.active_processes ?? 1)) : 0
      out.push({ title: "Worker efficiency", detail: `${formatNumber(throughput, 2)} req/s across ${data.active_processes ?? 0} active workers = ${formatNumber(eff, 2)} req/s per worker.` })
    }
    if (profile.hasMultiplePools) {
      out.push({ title: "Multi-pool config", detail: `${data.pools?.length ?? 0} pools configured — ${data.pools?.join(", ") ?? ""}.` })
    } else if (data.pools?.length === 1) {
      out.push({ title: "Single pool", detail: `Pool "${data.pools[0]}" is the only pool.` })
    }
    if (profile.opcacheEnabled && opcacheHit >= 0.95) {
      out.push({ title: "OPcache healthy", detail: `${formatPercent(opcacheHit * 100)} hit rate — cache is well-tuned.` })
    }
    if (latencyP95 < 200 && latencyP99 < 500) {
      out.push({ title: "Excellent latency", detail: `p95 ${formatNumber(latencyP95, 1)}ms · p99 ${formatNumber(latencyP99, 1)}ms — tail latency is well within bounds.` })
    }
    return out
  }, [data, profile, throughput, opcacheHit, latencyP95, latencyP99])

  const sorted = useMemo(() => {
    const bySev: Record<Severity, Finding[]> = { critical: [], warning: [], notice: [] }
    for (const f of findings) bySev[f.severity].push(f)
    return bySev
  }, [findings])

  const overall: Severity = sorted.critical.length ? "critical" : sorted.warning.length ? "warning" : "notice"
  const overallTone = SEV_BY_OVERALL[overall]

  const healthScore = useMemo(() => {
    if (!profile.isUp) return 0
    const penalty = sorted.critical.length * 18 + sorted.warning.length * 7 + sorted.notice.length * 2
    return Math.max(5, Math.min(100, 100 - penalty))
  }, [profile.isUp, sorted])

  const verdict = !profile.isUp
    ? { headline: "Instance is DOWN", sub: "PHP-FPM is unreachable. Restore the server before further triage." }
    : overall === "critical" ? { headline: "Action required", sub: `${sorted.critical.length} critical issue${sorted.critical.length > 1 ? "s" : ""} blocking healthy operation.` }
    : overall === "warning"  ? { headline: "Operating degraded", sub: `${sorted.warning.length} warning${sorted.warning.length > 1 ? "s" : ""} — performance impacted but stable.` }
    : sorted.notice.length    ? { headline: "All systems nominal", sub: `${sorted.notice.length} informational notice${sorted.notice.length > 1 ? "s" : ""} — no action required.` }
    :                            { headline: "All systems nominal", sub: "Every health check passed in this window." }

  const pulse = useMemo(() => {
    const items: Array<{ label: string; value: string; sub: string; tone: Tone; icon: any }> = []

    items.push({
      label: "Saturation",
      value: formatPercent(saturation),
      sub: `${data.active_processes ?? "?"} of ${data.total_processes ?? "?"} workers`,
      tone: saturation > 85 ? "critical" : saturation > 70 ? "warn" : "ok",
      icon: Icon.Link,
    })

    items.push({
      label: "Throughput",
      value: `${formatNumber(throughput, 1)}`,
      sub: "requests / sec",
      tone: profile.isUp ? "info" : "critical",
      icon: Icon.Bolt,
    })

    items.push({
      label: "Queue",
      value: queueDepth > 0 ? `${formatNumber(queueDepth, 0)}` : "0",
      sub: queueDepth > 0 ? `${formatPercent(queueUtil)} utilized` : "empty",
      tone: queueDepth > 0 ? "critical" : "ok",
      icon: Icon.Pulse,
    })

    items.push({
      label: "Latency",
      value: latencyP99 > 0 ? `${formatNumber(latencyP99, 1)}ms` : "—",
      sub: `p99 · ${latencyP95 > 0 ? `p95 ${formatNumber(latencyP95, 1)}ms` : ""}`,
      tone: latencyP99 > 5000 ? "critical" : latencyP99 > 2000 ? "warn" : latencyP99 > 0 ? "ok" : "na",
      icon: Icon.Flame,
    })

    if (profile.opcacheEnabled) {
      items.push({
        label: "OPcache",
        value: formatPercent(opcacheHit * 100),
        sub: `${formatPercent(opcacheMemUsed * 100)} used · ${formatPercent(opcacheWasted * 100)} waste`,
        tone: opcacheHit < 0.85 ? "critical" : opcacheHit < 0.9 ? "warn" : "ok",
        icon: Icon.Chip,
      })
    } else {
      items.push({
        label: "OPcache",
        value: "Off",
        sub: "not enabled",
        tone: "na",
        icon: Icon.Chip,
      })
    }

    items.push({
      label: "Logs",
      value: `${formatNumber(errors, 0)} err`,
      sub: `${formatNumber(warnings, 0)} warn · ${formatNumber(slowReqs, 0)} slow`,
      tone: errors > 5 ? "warn" : errors > 0 ? "info" : "ok",
      icon: Icon.Alert,
    })

    return items
  }, [data, profile, saturation, throughput, queueDepth, queueUtil, latencyP95, latencyP99, opcacheHit, opcacheMemUsed, opcacheWasted, errors, warnings, slowReqs])

  const overview = useMemo(() => {
    const rows: Array<{ key: string; label: string; status: Tone; value: string; group: string }> = [
      { key: "status", group: "Service", label: "Availability", status: profile.isUp ? "ok" : "critical",
        value: profile.isUp ? `Up ${formatDuration(data.uptime_seconds ?? 0)}` : "Down" },
      { key: "scrape", group: "Service", label: "Scrape Health", status: (data.scrape_failures ?? 0) > 0 ? "warn" : "ok",
        value: (data.scrape_failures ?? 0) > 0 ? `${data.scrape_failures} failures` : "Healthy" },
      { key: "uptime", group: "Service", label: "Uptime", status: uptimeMin < 15 ? "info" : "ok",
        value: formatDuration(data.uptime_seconds ?? 0) },
      { key: "workers", group: "Worker Pool", label: "Active Workers", status: saturation > 85 ? "critical" : saturation > 70 ? "warn" : "ok",
        value: `${data.active_processes ?? "?"} / ${data.total_processes ?? "?"}` },
      { key: "saturation", group: "Worker Pool", label: "Saturation", status: saturation > 85 ? "critical" : saturation > 70 ? "warn" : "ok",
        value: formatPercent(saturation) },
      { key: "max_children", group: "Worker Pool", label: "Max Children Reached", status: maxChildren > 0 ? "critical" : "ok",
        value: maxChildren > 0 ? `${formatNumber(maxChildren, 0)} times` : "Never" },
      { key: "queue", group: "Queue", label: "Queue Depth", status: queueDepth > 0 ? "critical" : "ok",
        value: queueDepth > 0 ? `${formatNumber(queueDepth, 0)} (${formatPercent(queueUtil)})` : "Empty" },
      { key: "throughput", group: "Queue", label: "Throughput", status: "ok",
        value: `${formatNumber(throughput, 2)}/s` },
      { key: "duration", group: "Performance", label: "Avg Duration", status: avgDur > 1000 ? "warn" : avgDur > 500 ? "info" : "ok",
        value: `${formatNumber(avgDur, 1)}ms` },
      { key: "memory", group: "Performance", label: "Avg Memory", status: "ok",
        value: data.avg_memory_kb != null ? `${formatNumber(data.avg_memory_kb, 0)} KB` : data.avg_memory_bytes != null ? formatBytes(data.avg_memory_bytes) : "—" },
      { key: "cpu", group: "Performance", label: "Avg CPU", status: avgCpu > 25 ? "warn" : avgCpu > 5 ? "info" : "ok",
        value: `${formatPercent(avgCpu)}` },
      { key: "p50", group: "Latency", label: "p50", status: "ok",
        value: data.latency_p50 != null ? `${formatNumber(data.latency_p50, 1)}ms` : "—" },
      { key: "p95", group: "Latency", label: "p95", status: latencyP95 > 1000 ? "warn" : latencyP95 > 500 ? "info" : "ok",
        value: `${formatNumber(latencyP95, 1)}ms` },
      { key: "p99", group: "Latency", label: "p99", status: latencyP99 > 5000 ? "critical" : latencyP99 > 2000 ? "warn" : "ok",
        value: `${formatNumber(latencyP99, 1)}ms` },
    ]

    if (profile.opcacheEnabled) {
      rows.push(
        { key: "opcache_hit", group: "OPcache", label: "Hit Rate", status: opcacheHit < 0.85 ? "critical" : opcacheHit < 0.9 ? "warn" : "ok",
          value: formatPercent(opcacheHit * 100) },
        { key: "opcache_mem", group: "OPcache", label: "Memory Used", status: opcacheMemUsed > 0.8 ? "warn" : "ok",
          value: formatPercent(opcacheMemUsed * 100) },
        { key: "opcache_waste", group: "OPcache", label: "Wasted", status: opcacheWasted > 0.05 ? "warn" : "ok",
          value: formatPercent(opcacheWasted * 100) },
        { key: "opcache_restarts", group: "OPcache", label: "Restarts", status: opcacheRestarts > 0 ? "critical" : "ok",
          value: opcacheRestarts > 0 ? `${formatNumber(opcacheRestarts, 0)} total` : "None" },
      )
    }

    rows.push(
      { key: "errors", group: "Logs", label: "Errors", status: errors > 5 ? "warn" : errors > 0 ? "info" : "ok",
        value: `${formatNumber(errors, 0)} in window` },
      { key: "warnings", group: "Logs", label: "Warnings", status: warnings > 0 ? "info" : "ok",
        value: `${formatNumber(warnings, 0)} in window` },
      { key: "slow", group: "Logs", label: "Slow Requests", status: slowReqs > 0 ? "info" : "ok",
        value: `${formatNumber(slowReqs, 0)} total` },
      { key: "pools", group: "Platform", label: "Pools", status: profile.hasMultiplePools ? "info" : "ok",
        value: profile.pools > 0 ? data.pools!.join(", ") : "None" },
      { key: "opcache", group: "Platform", label: "OPcache", status: profile.opcacheEnabled ? "ok" : "na",
        value: profile.opcacheEnabled ? "Enabled" : "Disabled" },
      { key: "logs_source", group: "Platform", label: "Access Logs", status: profile.accessLogsAvailable ? "ok" : "na",
        value: profile.accessLogsAvailable ? "Available" : "Not detected" },
    )
    return rows
  }, [data, profile, uptimeMin, saturation, queueDepth, queueUtil, throughput, avgDur, avgCpu, latencyP95, latencyP99, opcacheHit, opcacheMemUsed, opcacheWasted, opcacheRestarts, maxChildren, errors, warnings, slowReqs])

  const criticalPoolIssues = useMemo(() => {
    if (!data.pool_details) return []
    const issues: Array<{ pool: string; saturation: number; slowReqs: number; errors: number; errorRate: number }> = []
    for (const [pool, pd] of Object.entries(data.pool_details)) {
      const sat = pd.saturation_pct ?? 0
      issues.push({
        pool,
        saturation: sat,
        slowReqs: pd.slow_requests ?? 0,
        errors: pd.error_count ?? 0,
        errorRate: pd.status_5xx ?? 0,
      })
    }
    return issues.sort((a, b) => b.saturation - a.saturation)
  }, [data])

  const capacityPlanning = useMemo(() => {
    if (!data.workers_timeseries || data.workers_timeseries.length < 2) return null
    const peak = data.workers_timeseries.reduce((max, pt) => {
      const v = pt["total"]
      return v != null ? Math.max(max, v as number) : max
    }, 0)
    const current = data.total_processes ?? 0
    const limit = data.max_active_processes ?? data.total_processes ?? 0
    const headroomPct = limit > 0 ? ((limit - peak) / limit) * 100 : 0
    return { peak, current, limit, headroomPct }
  }, [data])

  const topErrorMessages = useMemo(() => {
    if (!data.recent_logs || data.recent_logs.length === 0) return []
    const count = new Map<string, number>()
    for (const log of data.recent_logs) {
      if (log.severity === "error" || log.severity === "warning") {
        const msg = log.message.length > 80 ? log.message.slice(0, 80) + "…" : log.message
        count.set(msg, (count.get(msg) ?? 0) + 1)
      }
    }
    return Array.from(count.entries())
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8)
  }, [data])

  const trends = useMemo(() => {
    const items: Array<{ label: string; direction: "up" | "down" | "flat"; pct: number; isBad: boolean; first: number; last: number }> = []
    const add = (label: string, series: PHPFPMTimePoint[] | undefined, key: string, higherIsBad: boolean) => {
      const { first, last } = tsStartEnd(series ?? [], key)
      if (first == null || last == null || first === 0) return
      const pct = ((last - first) / Math.abs(first)) * 100
      const direction = Math.abs(pct) < 3 ? "flat" : pct > 0 ? "up" : "down"
      items.push({ label, direction, pct, isBad: higherIsBad, first, last })
    }
    add("Worker Saturation", data.workers_timeseries, "total", false)
    add("Queue Depth", data.queue_timeseries, "queue", true)
    add("p99 Latency", data.latency_percentiles_timeseries, "p99", true)
    add("Throughput", data.throughput_timeseries, "accepted", false)
    add("Slow Requests", data.slow_req_rate_timeseries, "slow_req_rate", true)
    add("Active Workers", data.workers_timeseries, "active", false)
    if (profile.opcacheEnabled) add("OPcache Hit Rate", data.opcache_hit_rate_timeseries, "hit_rate", true)
    return items
  }, [data, profile])

  const topBottlenecks = useMemo(() => {
    const rank: Array<{ label: string; detail: string; severity: Severity }> = []
    if (saturation > 85) rank.push({
      label: `Worker pool ${formatPercent(saturation)} saturated`,
      detail: `${data.active_processes ?? 0} active of ${data.total_processes ?? 0} total workers — at this level, incoming requests may queue up and timeout. Consider adding workers or optimizing slow handlers.`,
      severity: "critical",
    })
    if (queueDepth > 0) rank.push({
      label: `${formatNumber(queueDepth, 0)} requests queued`,
      detail: `Queue utilization is ${formatPercent(queueUtil)}. Requests are waiting for a free worker because all ${data.total_processes ?? 0} workers are busy. This causes response delays and potential timeouts.`,
      severity: "critical",
    })
    if (maxChildren > 0) rank.push({
      label: `pm.max_children hit ${formatNumber(maxChildren, 0)}×`,
      detail: `The pool hit its configured process limit (${data.total_processes ?? 0}) — new requests were forced to wait or were rejected. Increase pm.max_children or reduce memory per process.`,
      severity: "critical",
    })
    if (latencyP99 > 5000) rank.push({
      label: `p99 latency at ${formatNumber(latencyP99, 1)}ms`,
      detail: `99th percentile response time is ${formatNumber(latencyP99, 1)}ms — well above the 2s threshold. The slowest 1% of requests take >${formatNumber(Math.round(latencyP99 / 1000), 0)}s, causing severe user-facing slowdowns. Investigate slow DB queries and upstream HTTP calls.`,
      severity: "critical",
    })
    if (opcacheRestarts > 0) rank.push({
      label: `OPcache restarted ${formatNumber(opcacheRestarts, 0)}×`,
      detail: `OPcache was flushed ${formatNumber(opcacheRestarts, 0)} time(s) in this window, likely due to memory pressure (${formatPercent(opcacheWasted * 100)} waste) or explicit opcache_reset(). This forces re-caching of all PHP scripts, temporarily increasing CPU load.`,
      severity: "critical",
    })
    if (saturation > 70 && saturation <= 85) rank.push({
      label: `Worker pool ${formatPercent(saturation)} saturated`,
      detail: `${data.active_processes ?? 0} active of ${data.total_processes ?? 0} workers — approaching capacity. If traffic increases, requests will start queueing.`,
      severity: "warning",
    })
    if (latencyP99 > 2000 && latencyP99 <= 5000) rank.push({
      label: `p99 ${formatNumber(latencyP99, 1)}ms tail latency`,
      detail: `The slowest 1% of requests take >${formatNumber(latencyP99, 1)}ms (threshold is 2s). This indicates intermittent slow code paths, DB queries, or upstream dependencies causing bottlenecks.`,
      severity: "warning",
    })
    if (avgDur > 1000) rank.push({
      label: `Avg duration ${formatNumber(avgDur, 1)}ms`,
      detail: `The average request takes ${formatNumber(avgDur, 1)}ms — above the 1s threshold. Most users experience sluggish responses. Review slow scripts and external service call latency.`,
      severity: "warning",
    })
    if (avgCpu > 25) rank.push({
      label: `Avg CPU ${formatPercent(avgCpu)} per process`,
      detail: `Each PHP process uses ${formatPercent(avgCpu)} CPU on average (>25% threshold). Likely CPU-bound scripts — consider opcache tuning, caching, or optimizing hot code paths.`,
      severity: "warning",
    })
    if (profile.opcacheEnabled && opcacheHit < 0.9) rank.push({
      label: `OPcache hit rate ${formatPercent(opcacheHit * 100)}`,
      detail: `Only ${formatPercent(opcacheHit * 100)} of file lookups hit the cache (target: 90%+). Each miss reads and compiles PHP files from disk. Increase opcache.memory_consumption or opcache.max_accelerated_files.`,
      severity: "warning",
    })
    if (status5xx > 0) rank.push({
      label: `5xx errors at ${formatNumber(status5xx, 2)}/s`,
      detail: `Server errors are occurring at ${formatNumber(status5xx, 2)} per second. Application code, upstream timeouts, or resource exhaustion may be causing request failures.`,
      severity: "warning",
    })
    if (slowReqs > 0) rank.push({
      label: `${formatNumber(slowReqs, 0)} slow request${slowReqs > 1 ? "s" : ""} in window`,
      detail: `${formatNumber(slowReqs, 0)} request${slowReqs > 1 ? "s" : ""} exceeded PHP-FPM's slow request timeout. Check php-slow.log for the specific script paths and stack traces.`,
      severity: "notice",
    })
    if (errors > 0) rank.push({
      label: `${formatNumber(errors, 0)} application error${errors > 1 ? "s" : ""} in logs`,
      detail: `${formatNumber(errors, 0)} PHP error${errors > 1 ? "s" : ""} detected in the current window. Review php-error.log for details on warnings, notices, or fatal errors.`,
      severity: "notice",
    })
    return rank.slice(0, 5)
  }, [saturation, queueDepth, maxChildren, latencyP99, opcacheRestarts, avgDur, avgCpu, opcacheHit, profile.opcacheEnabled, status5xx, slowReqs, errors, queueUtil, opcacheWasted, data])

  const recommendations = useMemo(() => {
    const recs: string[] = []
    if (saturation > 85) recs.push("Increase pm.max_children or optimize slow handlers to reduce worker saturation.")
    if (queueDepth > 0) recs.push("Requests are queueing — add workers or investigate blocking operations causing queue buildup.")
    if (maxChildren > 0) recs.push("Pool reached pm.max_children limit. Increase it or reduce memory per process.")
    if (avgDur > 1000) recs.push("Review slow scripts and external API calls contributing to high average duration.")
    if (avgCpu > 25) recs.push("Investigate CPU-bound scripts — consider caching or optimizing database queries.")
    if (profile.opcacheEnabled && opcacheHit < 0.9) recs.push("Tune opcache.max_accelerated_files and opcache.memory_consumption to improve hit rate.")
    if (profile.opcacheEnabled && opcacheWasted > 0.05) recs.push("OPcache waste ratio elevated — check opcache.revalidate_freq and script churn.")
    if (latencyP99 > 2000) recs.push("Investigate slow database queries and external service calls causing p99 latency spikes.")
    if (status5xx > 0) recs.push("Check application error logs for 5xx response causes.")
    if (errors > 5) recs.push("Review recent error log entries for application-level issues.")
    return recs.slice(0, 5)
  }, [saturation, queueDepth, maxChildren, avgDur, avgCpu, opcacheHit, profile.opcacheEnabled, opcacheWasted, latencyP99, status5xx, errors])

  const resourceWaste = useMemo(() => {
    const items: Array<{ label: string; value: string; tone: Tone }> = []
    const idle = data.idle_processes ?? 0
    const total = data.total_processes ?? 0
    if (idle > 0 && total > 0) {
      const idlePct = (idle / total) * 100
      items.push({ label: "Idle Workers", value: `${formatNumber(idle, 0)} of ${formatNumber(total, 0)} (${formatPercent(idlePct)})`, tone: idlePct > 50 ? "warn" : idlePct > 30 ? "info" : "ok" })
    }
    if (profile.opcacheEnabled) {
      items.push({ label: "OPcache Waste", value: formatPercent(opcacheWasted * 100), tone: opcacheWasted > 0.05 ? "warn" : "ok" })
      if (data.avg_memory_kb != null && idle > 0) {
        const estimatedWaste = (data.avg_memory_kb * idle) / (1024 * 1024)
        items.push({ label: "Est. Idle Memory", value: `${formatNumber(estimatedWaste, 2)} GB`, tone: estimatedWaste > 1 ? "warn" : "ok" })
      }
    }
    if (data.opcache_memory_free_bytes != null && data.opcache_memory_total_bytes != null && data.opcache_memory_total_bytes > 0) {
      const freeGb = data.opcache_memory_free_bytes / (1024 * 1024 * 1024)
      items.push({ label: "OPcache Memory Free", value: freeGb > 0.1 ? `${formatNumber(freeGb, 2)} GB` : formatBytes(data.opcache_memory_free_bytes), tone: "ok" })
    }
    return items
  }, [data, profile, opcacheWasted])

  const errorImpact = useMemo(() => {
    if (!data.recent_logs || data.recent_logs.length === 0) return null
    const errorCounts = new Map<string, { count: number; pools: Set<string> }>()
    for (const log of data.recent_logs) {
      if (log.severity === "error") {
        const msg = log.message.length > 60 ? log.message.slice(0, 60) + "…" : log.message
        if (!errorCounts.has(msg)) errorCounts.set(msg, { count: 0, pools: new Set() })
        const entry = errorCounts.get(msg)!
        entry.count++
        if (log.pool) entry.pools.add(log.pool)
      }
    }
    const ranked = Array.from(errorCounts.entries()).sort((a, b) => b[1].count - a[1].count)
    if (ranked.length === 0) return null
    const top = ranked[0]
    return {
      topError: top[0],
      topCount: top[1].count,
      topPools: Array.from(top[1].pools),
      errorRate5xx: status5xx,
      totalErrors: data.log_summary?.errors ?? 0,
    }
  }, [data, status5xx])

  return (
    <div className="space-y-6">

      <section className={cn(
        "relative overflow-hidden rounded-2xl border bg-gradient-to-br from-zinc-950 via-zinc-900 to-zinc-950 p-6 md:p-7",
        TONE[overallTone].surface.split(" ")[1],
      )}>
        <div className={cn("pointer-events-none absolute -top-24 -right-24 h-72 w-72 rounded-full blur-3xl opacity-30",
          overall === "critical" ? "bg-rose-500" : overall === "warning" ? "bg-amber-400" : "bg-emerald-400")} />
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_30%_0%,rgba(255,255,255,0.04),transparent_50%)]" />

        <div className="relative grid gap-6 md:grid-cols-[auto,1fr,auto] md:items-center">
          <ScoreRing score={healthScore} tone={overallTone} />

          <div className="min-w-0">
            <div className="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
              <span className={cn("h-1.5 w-1.5 rounded-full animate-pulse", TONE[overallTone].dot)} />
              PHP-FPM · Summary
            </div>
            <h2 className="mt-2 text-2xl md:text-3xl font-semibold tracking-tight text-zinc-50">
              {verdict.headline}
            </h2>
            <p className="mt-1.5 text-sm text-zinc-400 leading-relaxed max-w-xl">{verdict.sub}</p>

            <div className="mt-4 flex flex-wrap items-center gap-1.5">
              <Badge>{profile.hasMultiplePools ? "Multi-pool" : "Single pool"}</Badge>
              <Badge>{profile.opcacheEnabled ? "OPcache on" : "OPcache off"}</Badge>
              {profile.accessLogsAvailable && <Badge>Access logs</Badge>}
              <Badge>Uptime {formatDuration(data.uptime_seconds ?? 0)}</Badge>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-2 md:w-56">
            <Tally tone="critical" label="Critical" count={sorted.critical.length} />
            <Tally tone="warn"     label="Warning"  count={sorted.warning.length} />
            <Tally tone="info"     label="Notice"   count={sorted.notice.length} />
          </div>
        </div>
      </section>

      <section className="grid gap-3 grid-cols-2 md:grid-cols-3 xl:grid-cols-6">
        {pulse.map((p, i) => (
          <KpiCard key={i} {...p} />
        ))}
      </section>

      {trends.length > 0 && (
        <Panel title="Trend Summary" sub="How key metrics are changing in this window" icon={<Icon.Spark className="h-4 w-4 text-zinc-400" />}>
          <div className="grid gap-2 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-6">
            {trends.map((t, i) => {
              const trendTone: Tone = t.direction === "flat" ? "ok" : t.isBad && t.direction === "up" ? "critical" : t.isBad && t.direction === "down" ? "warn" : !t.isBad && t.direction === "up" ? "ok" : "info"
              const arrow = t.direction === "up" ? "▲" : t.direction === "down" ? "▼" : "→"
              const dirLabel = t.direction === "up" ? "Rising" : t.direction === "down" ? "Falling" : "Steady"
              return (
                <div key={i} className="rounded-lg border border-zinc-800 bg-zinc-950/60 px-3 py-3">
                  <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-500">{t.label}</div>
                  <div className={cn("mt-1 flex items-baseline gap-1.5 text-sm font-semibold tabular-nums", TONE[trendTone].text)}>
                    <span>{formatNumber(t.first, 1)}</span>
                    <span className="text-xs text-zinc-600">{arrow}</span>
                    <span>{formatNumber(t.last, 1)}</span>
                  </div>
                  <div className={cn("mt-0.5 flex items-center gap-1 text-[10px] font-medium", TONE[trendTone].text)}>
                    <span>{dirLabel}</span>
                    {t.direction !== "flat" && (
                      <span>{formatNumber(Math.abs(t.pct), 1)}%</span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </Panel>
      )}

      {criticalPoolIssues.length > 1 && (
        <Panel title="Pool Comparison Heatmap" sub={`${criticalPoolIssues.length} pools · color-coded by severity`} icon={<Icon.Users className="h-4 w-4 text-zinc-400" />}>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-zinc-500 border-b border-zinc-800">
                  <th className="pb-2 pr-4 font-medium">Pool</th>
                  <th className="pb-2 pr-2 font-medium text-center">Saturation</th>
                  <th className="pb-2 pr-2 font-medium text-center">Slow Reqs</th>
                  <th className="pb-2 font-medium text-center">Errors</th>
                </tr>
              </thead>
              <tbody>
                {criticalPoolIssues.map((p) => {
                  const satTone: Tone = p.saturation > 85 ? "critical" : p.saturation > 70 ? "warn" : p.saturation > 0 ? "info" : "ok"
                  const slowTone: Tone = p.slowReqs > 10 ? "critical" : p.slowReqs > 0 ? "warn" : "ok"
                  const errTone: Tone = p.errorRate > 0 ? "critical" : p.errors > 5 ? "warn" : "ok"
                  return (
                    <tr key={p.pool} className="border-b border-zinc-800/50">
                      <td className="py-3 pr-4 font-mono text-zinc-200">{p.pool}</td>
                      <td className="py-3 pr-2 text-center">
                        <span className={cn("inline-block w-full rounded-md px-2 py-1 text-xs font-semibold tabular-nums", TONE[satTone].surface, TONE[satTone].text)}>
                          {formatPercent(p.saturation)}
                        </span>
                      </td>
                      <td className="py-3 pr-2 text-center">
                        <span className={cn("inline-block w-full rounded-md px-2 py-1 text-xs font-semibold tabular-nums", TONE[slowTone].surface, TONE[slowTone].text)}>
                          {p.slowReqs > 0 ? formatNumber(p.slowReqs, 0) : "—"}
                        </span>
                      </td>
                      <td className="py-3 text-center">
                        <span className={cn("inline-block w-full rounded-md px-2 py-1 text-xs font-semibold tabular-nums", TONE[errTone].surface, TONE[errTone].text)}>
                          {p.errors > 0 || p.errorRate > 0 ? formatNumber(p.errors + p.errorRate, 0) : "—"}
                        </span>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </Panel>
      )}

      {capacityPlanning && (
        <Panel title="Capacity Planning" sub="Worker headroom and peak usage" icon={<Icon.Bolt className="h-4 w-4 text-zinc-400" />}>
          <div className="grid gap-4 md:grid-cols-3">
            <CapacityCard label="Current" value={formatNumber(capacityPlanning.current, 0)} sub="workers active" tone={capacityPlanning.current > 0 ? "ok" : "na"} />
            <CapacityCard label="Peak (window)" value={formatNumber(capacityPlanning.peak, 0)}
              sub={capacityPlanning.limit > 0 ? `Limit: ${formatNumber(capacityPlanning.limit, 0)}` : undefined}
              tone={capacityPlanning.limit > 0 && capacityPlanning.peak / capacityPlanning.limit > 0.85 ? "critical" : capacityPlanning.limit > 0 && capacityPlanning.peak / capacityPlanning.limit > 0.7 ? "warn" : "ok"} />
            <CapacityCard label="Headroom" value={capacityPlanning.limit > 0 ? formatPercent(capacityPlanning.headroomPct) : "—"}
              sub={capacityPlanning.limit > 0 ? `${formatNumber(capacityPlanning.limit - capacityPlanning.peak, 0)} available` : undefined}
              tone={capacityPlanning.headroomPct < 15 ? "critical" : capacityPlanning.headroomPct < 30 ? "warn" : "ok"} />
          </div>
        </Panel>
      )}

      {resourceWaste.length > 0 && (
        <Panel title="Resource Efficiency" sub="Waste and underutilization" icon={<Icon.Chip className="h-4 w-4 text-zinc-400" />}>
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {resourceWaste.map((r, i) => (
              <div key={i} className={cn("rounded-lg border bg-zinc-900/40 px-3.5 py-3", TONE[r.tone].surface)}>
                <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-500">{r.label}</div>
                <div className={cn("mt-1 text-base font-semibold tabular-nums", TONE[r.tone].text)}>{r.value}</div>
              </div>
            ))}
          </div>
        </Panel>
      )}

      <section className="grid gap-4 lg:grid-cols-5">
        <div className="lg:col-span-3 space-y-4">
          <Panel
            title="Action Center"
            sub={findings.length ? `${findings.length} findings · sorted by severity` : "No findings"}
            icon={<Icon.Alert className="h-4 w-4 text-zinc-400" />}
          >
            {findings.length === 0 ? (
              <EmptyState
                icon={<Icon.Check className="h-6 w-6 text-emerald-400" />}
                title="Nothing to act on"
                detail="All health checks passed in the current window."
              />
            ) : (
              <div className="space-y-5">
                {(["critical", "warning", "notice"] as Severity[]).map((sev) => {
                  const list = sorted[sev]
                  if (!list.length) return null
                  const tone = SEV_BY_OVERALL[sev]
                  const heading = sev === "critical" ? "Critical" : sev === "warning" ? "Warnings" : "Notices"
                  return (
                    <div key={sev}>
                      <div className="mb-2 flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.2em]">
                        <span className={cn("h-1.5 w-1.5 rounded-full", TONE[tone].dot)} />
                        <span className={TONE[tone].text}>{heading}</span>
                        <span className="text-zinc-600">· {list.length}</span>
                      </div>
                      <ol className="space-y-1.5">
                        {list.map((f, i) => (
                          <FindingRow key={i} index={i + 1} finding={f} tone={tone} />
                        ))}
                      </ol>
                    </div>
                  )
                })}
              </div>
            )}
          </Panel>

          {topBottlenecks.length > 0 && (
            <Panel
              title="Top Bottleneck"
              sub={topBottlenecks[0].severity === "critical" ? "Critical issue requiring immediate attention" : "Area to address next"}
              icon={<Icon.Flame className={cn("h-4 w-4", topBottlenecks[0].severity === "critical" ? "text-rose-400" : "text-amber-400")} />}
            >
              <ol className="space-y-2">
                {topBottlenecks.map((b, i) => {
                  const bTone: Tone = b.severity === "critical" ? "critical" : b.severity === "warning" ? "warn" : "info"
                  return (
                    <li key={i} className={cn("rounded-lg border bg-zinc-900/40 px-3 py-2.5", TONE[bTone].surface)}>
                      <div className="flex items-center gap-2.5">
                        <span className={cn("grid h-5 w-5 shrink-0 place-items-center rounded text-[10px] font-bold tabular-nums", TONE[bTone].surface, TONE[bTone].text)}>
                          {i + 1}
                        </span>
                        <span className={cn("text-xs font-semibold", TONE[bTone].text)}>{b.label}</span>
                      </div>
                      <p className="mt-1.5 text-[12px] leading-relaxed text-zinc-400">{b.detail}</p>
                    </li>
                  )
                })}
              </ol>
            </Panel>
          )}

          {errorImpact && (
            <Panel
              title="Error Impact"
              sub={`${formatNumber(errorImpact.totalErrors, 0)} errors in window`}
              icon={<Icon.Alert className="h-4 w-4 text-rose-400" />}
            >
              <div className="space-y-3">
                <div className="flex items-baseline justify-between rounded-lg border border-zinc-800 bg-zinc-900/40 px-3 py-2">
                  <span className="text-xs text-zinc-400">5xx Rate</span>
                  <span className={cn("text-sm font-semibold tabular-nums", errorImpact.errorRate5xx > 0 ? "text-rose-300" : "text-zinc-400")}>
                    {errorImpact.errorRate5xx > 0 ? `${formatNumber(errorImpact.errorRate5xx, 2)}/s` : "None"}
                  </span>
                </div>
                <div className="rounded-lg border border-zinc-800 bg-zinc-900/40 px-3 py-2">
                  <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-500">Top Error</div>
                  <div className="mt-0.5 text-xs text-zinc-300 leading-relaxed">{errorImpact.topError}</div>
                  <div className="mt-1 flex items-center gap-2 text-[11px] text-zinc-500">
                    <span>{formatNumber(errorImpact.topCount, 0)} occurrences</span>
                    {errorImpact.topPools.length > 0 && (
                      <><span>·</span><span>Pool: {errorImpact.topPools.join(", ")}</span></>
                    )}
                  </div>
                </div>
              </div>
            </Panel>
          )}
        </div>

        <div className="lg:col-span-2 space-y-4">
          <Panel
            title="Correlated Insights"
            sub={insights.length ? `${insights.length} cross-signal pattern${insights.length > 1 ? "s" : ""}` : "Cross-signal analysis"}
            icon={<Icon.Diamond className="h-3.5 w-3.5 text-violet-400" />}
          >
            {insights.length === 0 ? (
              <div className="rounded-lg border border-dashed border-zinc-700/60 bg-zinc-900/20 p-3.5">
                <div className="flex items-center gap-1.5 text-[11px] font-medium text-zinc-500 mb-2">
                  <Icon.Spark className="h-3 w-3" />
                  No correlations yet — how this works
                </div>
                <ul className="space-y-2">
                  {[
                    ["Worker saturation", "Queue depth", "When both rise together, workers can't keep up"],
                    ["Request duration", "CPU usage", "Both elevated means CPU-bound scripts are slowing requests"],
                    ["p99 latency", "5xx errors", "Latency spikes paired with errors suggest timeouts"],
                    ["OPcache hit rate", "OPcache waste", "Low hit rate with high waste signals memory pressure"],
                    ["Max children", "Active workers", "Limit reached while saturated means pool is exhausted"],
                  ].map(([a, b, desc], i) => (
                    <li key={i} className="flex items-start gap-2 text-[11px] text-zinc-600">
                      <span className="mt-0.5 shrink-0 font-mono text-zinc-700">{a}</span>
                      <span className="shrink-0 text-zinc-700">+</span>
                      <span className="shrink-0 font-mono text-zinc-700">{b}</span>
                      <span className="text-zinc-600">→</span>
                      <span>{desc}</span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : (
              <ul className="space-y-2.5">
                {insights.map((ins, i) => (
                  <li key={i} className="rounded-lg border border-violet-400/15 bg-violet-400/[0.04] p-3.5">
                    <div className="flex items-start gap-2.5">
                      <Icon.Diamond className="mt-1 h-2.5 w-2.5 shrink-0 text-violet-400" />
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-zinc-100">{ins.title}</div>
                        <p className="mt-1 text-[13px] leading-relaxed text-zinc-400">{ins.detail}</p>
                      </div>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </Panel>

          <Panel
            title="Pool Hotspots"
            sub={criticalPoolIssues.length > 1 ? "Saturation by pool" : criticalPoolIssues.length === 1 ? "1 pool configured" : "No pool data"}
            icon={<Icon.Flame className="h-4 w-4 text-orange-400" />}
          >
            {criticalPoolIssues.length === 0 ? (
              <p className="text-sm text-zinc-500">No pool data available.</p>
            ) : (
              <ul className="space-y-3">
                {criticalPoolIssues.slice(0, 5).map((p) => {
                  const pTone: Tone = p.saturation > 85 ? "critical" : p.saturation > 70 ? "warn" : "ok"
                  return (
                    <li key={p.pool} className="space-y-1.5">
                      <div className="flex items-baseline justify-between gap-3">
                        <span className="font-mono text-[12px] text-zinc-200">{p.pool}</span>
                        <span className={cn("shrink-0 text-xs tabular-nums", TONE[pTone].text)}>{formatPercent(p.saturation)}</span>
                      </div>
                      <div className="h-1 w-full overflow-hidden rounded-full bg-zinc-800">
                        <div className={cn("h-full rounded-full", TONE[pTone].bar)} style={{ width: `${Math.max(2, p.saturation)}%` }} />
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </Panel>

          <Panel
            title="Latency Experience Score"
            sub="Estimated · Satisfied &lt;500ms · Frustrated &gt;2s"
            icon={<Icon.Eye className="h-4 w-4 text-sky-400" />}
          >
            <LatencyExperienceScore data={data} />
          </Panel>

          {topErrorMessages.length > 0 && (
            <Panel
              title="Top Error Messages"
              sub={`${topErrorMessages.length} distinct message patterns`}
              icon={<Icon.Alert className="h-4 w-4 text-rose-400" />}
            >
              <ul className="space-y-2">
                {topErrorMessages.map(([msg, count], i) => (
                  <li key={i} className="flex items-start gap-2.5 rounded-lg border border-zinc-800 bg-zinc-900/40 px-3 py-2">
                    <span className={cn("mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded text-[10px] font-semibold tabular-nums", count > 3 ? "text-rose-300 bg-rose-500/10" : "text-zinc-400 bg-zinc-800")}>
                      {count}
                    </span>
                    <span className="text-[12px] text-zinc-300 leading-relaxed">{msg}</span>
                  </li>
                ))}
              </ul>
            </Panel>
          )}

          {data.error_type_breakdown && data.error_type_breakdown.length > 0 && (
            <Panel
              title="Error Type Breakdown"
              sub={`${data.error_type_breakdown.length} categories`}
              icon={<Icon.Flame className="h-4 w-4 text-orange-400" />}
            >
              <ul className="space-y-2">
                {data.error_type_breakdown!.map((e, i) => {
                  const total = data.error_type_breakdown!.reduce((s, x) => s + x.count, 0)
                  const pct = total > 0 ? (e.count / total) * 100 : 0
                  return (
                    <li key={i} className="space-y-1">
                      <div className="flex items-baseline justify-between gap-3">
                        <span className="text-xs text-zinc-300 truncate">{e.label}</span>
                        <span className="shrink-0 text-xs tabular-nums text-zinc-400">{formatNumber(e.count, 0)}</span>
                      </div>
                      <div className="h-1 w-full overflow-hidden rounded-full bg-zinc-800">
                        <div className="h-full rounded-full bg-gradient-to-r from-orange-500 to-rose-500" style={{ width: `${Math.max(2, pct)}%` }} />
                      </div>
                    </li>
                  )
                })}
              </ul>
            </Panel>
          )}

          {contextSignals.length > 0 && (
            <Panel
              title="Context Signals"
              sub="Non-alert intelligence"
              icon={<Icon.Spark className="h-4 w-4 text-zinc-400" />}
            >
              <div className="flex flex-wrap gap-1.5">
                {contextSignals.map((c, i) => (
                  <span key={i} className="rounded-md border border-zinc-800 bg-zinc-900/60 px-2.5 py-1.5 text-[12px] text-zinc-400">
                    <span className="font-medium text-zinc-200">{c.title}:</span> {c.detail}
                  </span>
                ))}
              </div>
            </Panel>
          )}
        </div>
      </section>

      <Panel title="System Map" sub="Full subsystem state" icon={<Icon.Db className="h-4 w-4 text-zinc-400" />}>
        <div className="grid gap-px overflow-hidden rounded-lg border border-zinc-800 bg-zinc-800 sm:grid-cols-2 lg:grid-cols-3">
          {overview.map((row) => (
            <div key={row.key} className="group flex items-center justify-between gap-3 bg-zinc-950 px-3.5 py-3 transition-colors hover:bg-zinc-900">
              <div className="flex min-w-0 items-center gap-2.5">
                <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", TONE[row.status].dot)} />
                <div className="min-w-0">
                  <div className="text-[10px] font-medium uppercase tracking-wider text-zinc-500">{row.group}</div>
                  <div className="truncate text-sm text-zinc-200">{row.label}</div>
                </div>
              </div>
              <span className={cn("shrink-0 text-right text-xs font-medium tabular-nums", TONE[row.status].text)}>
                {row.status === "na" ? "—" : row.value}
              </span>
            </div>
          ))}
        </div>
      </Panel>
    </div>
  )
}

function LatencyExperienceScore({ data }: { data: PHPFPMDashboardSummary }) {
  const threshold = 500
  const p50 = data.latency_p50 ?? 0
  const p95 = data.latency_p95 ?? 0
  const p99 = data.latency_p99 ?? 0
  const hasData = p50 > 0 || p95 > 0 || p99 > 0

  if (!hasData) {
    return (
      <div className="space-y-3">
        <div className="flex items-baseline justify-between">
          <span className="text-2xl font-semibold tabular-nums text-zinc-600">—</span>
          <span className="text-xs font-medium text-zinc-600">Insufficient data</span>
        </div>
        <p className="text-[11px] text-zinc-600 leading-relaxed">
          No latency measurements available for this window. Score estimates user experience from the share of requests completing under {threshold}ms (Satisfied) vs exceeding {threshold * 4}ms (Frustrated).
        </p>
      </div>
    )
  }

  let satisfied: number
  let frustrated: number

  if (p95 <= threshold) {
    satisfied = 0.95
  } else if (p50 <= threshold) {
    satisfied = 0.5 + 0.45 * (1 - Math.min(1, (p95 - threshold) / (threshold * 3)))
  } else {
    satisfied = 0.5 * Math.max(0, 1 - (p50 - threshold) / (threshold * 3))
  }

  if (p99 > threshold * 4) {
    frustrated = Math.min(0.3, 0.01 + 0.29 * (p99 - threshold * 4) / (threshold * 20))
  } else if (p95 > threshold * 4) {
    frustrated = 0.03
  } else {
    frustrated = 0
  }

  satisfied = Math.max(0.01, Math.min(0.99, satisfied))
  frustrated = Math.max(0, Math.min(0.99 - satisfied, frustrated))
  const tolerated = 1 - satisfied - frustrated
  const apdex = Math.round((satisfied + tolerated / 2) * 100)

  const label = apdex >= 90 ? "Excellent" : apdex >= 75 ? "Good" : apdex >= 50 ? "Fair" : apdex >= 25 ? "Poor" : "Critical"
  const tone: Tone = apdex >= 90 ? "ok" : apdex >= 75 ? "info" : apdex >= 50 ? "warn" : "critical"

  return (
    <div className="space-y-3">
      <div className="flex items-baseline justify-between">
        <span className={cn("text-2xl font-semibold tabular-nums", TONE[tone].text)}>
          {(apdex / 100).toFixed(2)}
        </span>
        <span className={cn("rounded-md px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider", TONE[tone].surface, TONE[tone].text)}>
          {label}
        </span>
      </div>
      <div className="flex h-2 w-full overflow-hidden rounded-full bg-zinc-800">
        <div className="bg-emerald-500 transition-all" style={{ width: `${satisfied * 100}%` }} title="Satisfied" />
        <div className="bg-amber-500 transition-all" style={{ width: `${tolerated * 100}%` }} title="Tolerated" />
        <div className="bg-rose-500 transition-all" style={{ width: `${frustrated * 100}%` }} title="Frustrated" />
      </div>
      <div className="flex justify-between text-[11px] text-zinc-500">
        <span><span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500 mr-1" />Satisfied {'<'}500ms</span>
        <span><span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500 mr-1" />Tolerated</span>
        <span><span className="inline-block h-1.5 w-1.5 rounded-full bg-rose-500 mr-1" />Frustrated {'>'}2s</span>
      </div>
      <div className="grid grid-cols-3 gap-2 border-t border-zinc-800 pt-3">
        <div>
          <div className="text-[10px] text-zinc-600">p50</div>
          <div className={cn("text-sm font-semibold tabular-nums", p50 <= threshold ? "text-emerald-400" : p50 <= threshold * 2 ? "text-amber-400" : "text-rose-400")}>
            {formatNumber(p50, 1)}ms
          </div>
        </div>
        <div>
          <div className="text-[10px] text-zinc-600">p95</div>
          <div className={cn("text-sm font-semibold tabular-nums", p95 <= threshold ? "text-emerald-400" : p95 <= threshold * 2 ? "text-amber-400" : "text-rose-400")}>
            {formatNumber(p95, 1)}ms
          </div>
        </div>
        <div>
          <div className="text-[10px] text-zinc-600">p99</div>
          <div className={cn("text-sm font-semibold tabular-nums", p99 <= threshold ? "text-emerald-400" : p99 <= threshold * 2 ? "text-amber-400" : "text-rose-400")}>
            {formatNumber(p99, 1)}ms
          </div>
        </div>
      </div>
      <p className="text-[11px] text-zinc-600 leading-relaxed">
        Score of 1.00 = all requests under {threshold}ms. Currently {formatNumber(satisfied * 100, 0)}% of requests are Satisfied (under {threshold}ms), {formatNumber(frustrated * 100, 0)}% are Frustrated (over {threshold * 4}ms). Estimated from percentile latencies.
      </p>
    </div>
  )
}

function Panel({ title, sub, icon, children }: { title: string; sub?: string; icon?: React.ReactNode; children: React.ReactNode }) {
  return (
    <section className="rounded-2xl border border-zinc-800/80 bg-zinc-950/40 p-5">
      <header className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2.5">
          {icon}
          <div>
            <h3 className="text-sm font-semibold text-zinc-100">{title}</h3>
            {sub && <p className="text-[11px] text-zinc-500">{sub}</p>}
          </div>
        </div>
      </header>
      {children}
    </section>
  )
}

function Badge({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center rounded-md border border-zinc-800 bg-zinc-900/70 px-2 py-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-400">
      {children}
    </span>
  )
}

function Tally({ tone, label, count }: { tone: Tone; label: string; count: number }) {
  const active = count > 0
  return (
    <div className={cn(
      "rounded-xl border bg-zinc-950/60 px-3 py-3 text-center transition-colors",
      active ? TONE[tone].surface : "border-zinc-800/60",
    )}>
      <div className={cn("text-2xl font-semibold tabular-nums", active ? TONE[tone].text : "text-zinc-600")}>{count}</div>
      <div className="mt-0.5 text-[10px] font-medium uppercase tracking-wider text-zinc-500">{label}</div>
    </div>
  )
}

function KpiCard({ label, value, sub, tone, icon: I }: { label: string; value: string; sub: string; tone: Tone; icon: any }) {
  const t = TONE[tone]
  return (
    <div className={cn(
      "group relative overflow-hidden rounded-xl border border-zinc-800/80 bg-zinc-950/60 p-4 transition-all hover:border-zinc-700",
      tone !== "ok" && tone !== "na" && t.glow,
    )}>
      <div className="flex items-center justify-between">
        <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">{label}</span>
        <I className={cn("h-3.5 w-3.5", t.text)} />
      </div>
      <div className={cn("mt-2 text-2xl font-semibold tabular-nums tracking-tight", tone === "ok" ? "text-zinc-100" : t.text)}>
        {value}
      </div>
      <div className="mt-0.5 text-[11px] text-zinc-500">{sub}</div>
      <div className={cn("pointer-events-none absolute inset-x-0 bottom-0 h-px", t.bar, "opacity-60")} />
    </div>
  )
}

function FindingRow({ index, finding, tone }: { index: number; finding: Finding; tone: Tone }) {
  const t = TONE[tone]
  return (
    <li className={cn("group relative overflow-hidden rounded-lg border bg-zinc-950/40 pl-4 pr-4 py-3 transition-colors hover:bg-zinc-900/40", t.surface)}>
      <span className={cn("absolute inset-y-0 left-0 w-[3px]", t.bar)} />
      <div className="flex items-start gap-3">
        <span className={cn("mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-md text-[10px] font-semibold tabular-nums", t.text, "bg-zinc-900/80 border border-zinc-800")}>
          {index}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-2">
            <span className={cn("text-sm font-medium", "text-zinc-100")}>{finding.title}</span>
            {finding.category && (
              <span className="text-[10px] uppercase tracking-wider text-zinc-500">· {finding.category}</span>
            )}
          </div>
          <p className="mt-1 text-[13px] leading-relaxed text-zinc-400">{finding.detail}</p>
        </div>
      </div>
    </li>
  )
}

function EmptyState({ icon, title, detail }: { icon: React.ReactNode; title: string; detail: string }) {
  return (
    <div className="flex flex-col items-center justify-center gap-2 py-10 text-center">
      <div className="grid h-12 w-12 place-items-center rounded-full border border-emerald-400/20 bg-emerald-400/5">
        {icon}
      </div>
      <div className="text-sm font-medium text-zinc-200">{title}</div>
      <p className="max-w-xs text-[13px] text-zinc-500">{detail}</p>
    </div>
  )
}

function CapacityCard({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone: Tone }) {
  const t = TONE[tone]
  return (
    <div className={cn("rounded-xl border bg-zinc-950/60 p-4 text-center transition-all", t.surface)}>
      <div className="text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">{label}</div>
      <div className={cn("mt-1 text-2xl font-semibold tabular-nums tracking-tight", t.text)}>{value}</div>
      {sub && <div className="mt-0.5 text-[11px] text-zinc-500">{sub}</div>}
      <div className={cn("pointer-events-none absolute inset-x-0 bottom-0 h-px", t.bar, "opacity-60")} />
    </div>
  )
}

function ScoreRing({ score, tone }: { score: number; tone: Tone }) {
  const r = 44
  const c = 2 * Math.PI * r
  const offset = c - (score / 100) * c
  const stroke = tone === "critical" ? "#f43f5e" : tone === "warn" ? "#fbbf24" : tone === "info" ? "#38bdf8" : "#34d399"
  return (
    <div className="relative h-[112px] w-[112px] shrink-0">
      <svg viewBox="0 0 100 100" className="h-full w-full -rotate-90">
        <circle cx="50" cy="50" r={r} stroke="rgba(255,255,255,0.06)" strokeWidth="6" fill="none" />
        <circle
          cx="50" cy="50" r={r}
          stroke={stroke}
          strokeWidth="6"
          strokeLinecap="round"
          fill="none"
          strokeDasharray={c}
          strokeDashoffset={offset}
          style={{ transition: "stroke-dashoffset 600ms ease, stroke 300ms" }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <div className="text-2xl font-semibold tabular-nums text-zinc-100">{score}</div>
        <div className="text-[9px] font-semibold uppercase tracking-[0.18em] text-zinc-500">Health</div>
      </div>
    </div>
  )
}
