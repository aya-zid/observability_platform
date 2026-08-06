import type { ReportServiceMetrics, ReportFleetMetrics } from "@/api/reports"

// Region display metadata
export const REGION_META: Record<string, { label: string; hours: string; icon: string }> = {
  night: { label: "Night", hours: "00:00 – 06:00", icon: "Moon" },
  morning: { label: "Morning", hours: "06:00 – 12:00", icon: "Sunrise" },
  afternoon: { label: "Afternoon", hours: "12:00 – 18:00", icon: "Sun" },
  evening: { label: "Evening", hours: "18:00 – 24:00", icon: "Sunset" },
}

export const SERVICE_LABELS: Record<string, string> = {
  nginx: "Nginx",
  apache: "Apache",
  postgresql: "PostgreSQL",
  mariadb: "MariaDB",
  redis: "Redis",
  phpfpm: "PHP-FPM",
  fleet: "Hosts",
  container: "Containers",
}

export function formatNumber(v: number | null | undefined, digits = 0): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—"
  if (Math.abs(v) >= 1e9) return `${(v / 1e9).toFixed(1)}B`
  if (Math.abs(v) >= 1e6) return `${(v / 1e6).toFixed(1)}M`
  if (Math.abs(v) >= 1e3) return `${(v / 1e3).toFixed(1)}k`
  return v.toLocaleString(undefined, { maximumFractionDigits: digits })
}

export function formatBytes(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—"
  const units = ["B", "KB", "MB", "GB", "TB"]
  let i = 0
  let n = v
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(1)} ${units[i]}`
}

export function formatRate(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—"
  return `${Number(v).toFixed(2)}/s`
}

export function formatPct(v: number | null | undefined, digits = 1): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—"
  return `${Number(v).toFixed(digits)}%`
}

export function formatMs(v: number | null | undefined): string {
  if (v === null || v === undefined || Number.isNaN(v)) return "—"
  return `${Number(v).toFixed(1)} ms`
}

// Metric display name → value formatter for KPI cards
export type MetricDef = { label: string; fmt: (v: number) => string }

export const METRIC_DEFS: Record<string, MetricDef> = {
  total_requests: { label: "Total Requests", fmt: (v) => formatNumber(v) },
  avg_req_rate: { label: "Avg Request Rate", fmt: (v) => formatRate(v) },
  total_5xx: { label: "5xx", fmt: (v) => formatNumber(v) },
  total_4xx: { label: "4xx", fmt: (v) => formatNumber(v) },
  p95_rt_ms: { label: "P95 Latency", fmt: (v) => formatMs(v) },
  bandwidth_bytes: { label: "Bandwidth", fmt: (v) => formatBytes(v) },
  bot_rate: { label: "Bot Rate", fmt: (v) => formatRate(v) },
  cache_hit_ratio_pct: { label: "Cache Hit", fmt: (v) => formatPct(v) },
  slow_queries: { label: "Slow Queries", fmt: (v) => formatNumber(v) },
  errors: { label: "Errors", fmt: (v) => formatNumber(v) },
  auth_failures: { label: "Auth Failures", fmt: (v) => formatNumber(v) },
  lock_waits: { label: "Lock Waits", fmt: (v) => formatNumber(v) },
  deadlocks_per_min: { label: "Deadlocks/min", fmt: (v) => formatNumber(v, 1) },
  connection_saturation_pct: { label: "Conn Saturation", fmt: (v) => formatPct(v) },
  db_size_bytes: { label: "DB Size", fmt: (v) => formatBytes(v) },
  qps: { label: "QPS", fmt: (v) => formatNumber(v, 1) },
  slow_queries_per_min: { label: "Slow/min", fmt: (v) => formatNumber(v, 1) },
  buffer_pool_hit_ratio_pct: { label: "Buffer Pool Hit", fmt: (v) => formatPct(v) },
  threads_connected: { label: "Threads", fmt: (v) => formatNumber(v) },
  aborted_connects_per_min: { label: "Aborted/min", fmt: (v) => formatNumber(v, 1) },
  table_scan_ratio_pct: { label: "Table Scan", fmt: (v) => formatPct(v) },
  memory_usage_pct: { label: "Memory", fmt: (v) => formatPct(v) },
  commands_per_sec: { label: "Commands/s", fmt: (v) => formatNumber(v, 1) },
  connected_clients: { label: "Clients", fmt: (v) => formatNumber(v) },
  client_saturation_pct: { label: "Client Saturation", fmt: (v) => formatPct(v) },
  evict_rate: { label: "Evict/s", fmt: (v) => formatNumber(v, 1) },
  error_replies_per_sec: { label: "Error Replies/s", fmt: (v) => formatNumber(v, 1) },
  slow_log_count: { label: "Slow Log", fmt: (v) => formatNumber(v) },
  worker_saturation_pct: { label: "Worker Saturation", fmt: (v) => formatPct(v) },
  accepted_conn_per_sec: { label: "Accepted/s", fmt: (v) => formatNumber(v, 1) },
  slow_requests: { label: "Slow Requests", fmt: (v) => formatNumber(v) },
  status_5xx: { label: "5xx", fmt: (v) => formatNumber(v) },
  latency_p95_ms: { label: "P95 Latency", fmt: (v) => formatMs(v) },
  opcache_hit_rate: { label: "Opcache Hit", fmt: (v) => formatPct(v) },
  max_children_reached: { label: "Max Children", fmt: (v) => formatNumber(v) },
}

export const FLEET_METRIC_DEFS: Record<string, MetricDef> = {
  avg_cpu_pct: { label: "Avg CPU", fmt: (v) => formatPct(v) },
  avg_mem_pct: { label: "Avg Memory", fmt: (v) => formatPct(v) },
  avg_disk_pct: { label: "Avg Disk", fmt: (v) => formatPct(v) },
  net_in_bps: { label: "Net In", fmt: (v) => formatRate(v) },
  net_out_bps: { label: "Net Out", fmt: (v) => formatRate(v) },
  load_avg_1m: { label: "Load Avg (1m)", fmt: (v) => formatNumber(v, 2) },
  containers: { label: "Containers", fmt: (v) => formatNumber(v) },
  avg_container_cpu_pct: { label: "Container CPU", fmt: (v) => formatPct(v) },
  avg_container_mem_pct: { label: "Container Memory", fmt: (v) => formatPct(v) },
}

// Extract non-null metric values from a service metrics object
export function serviceMetricEntries(m: ReportServiceMetrics): { key: string; def: MetricDef; value: number }[] {
  const out: { key: string; def: MetricDef; value: number }[] = []
  for (const [key, value] of Object.entries(m)) {
    const def = METRIC_DEFS[key]
    if (def && typeof value === "number" && Number.isFinite(value)) {
      out.push({ key, def, value })
    }
  }
  return out
}

export function fleetMetricEntries(f: ReportFleetMetrics): { key: string; def: MetricDef; value: number }[] {
  const out: { key: string; def: MetricDef; value: number }[] = []
  for (const [key, value] of Object.entries(f)) {
    const def = FLEET_METRIC_DEFS[key]
    if (def && typeof value === "number" && Number.isFinite(value)) {
      out.push({ key, def, value })
    }
  }
  return out
}

// Severity → tailwind classes
export const SEVERITY_STYLES: Record<string, string> = {
  critical: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30",
  high: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30",
  warning: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30",
  medium: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30",
  info: "text-blue-600 dark:text-blue-400 bg-blue-500/10 border-blue-500/30",
  low: "text-slate-600 dark:text-slate-400 bg-slate-500/10 border-slate-500/30",
}

export const STATUS_STYLES: Record<string, string> = {
  healthy: "text-green-600 dark:text-green-400 bg-green-500/10 border-green-500/30",
  degraded: "text-amber-600 dark:text-amber-400 bg-amber-500/10 border-amber-500/30",
  critical: "text-red-600 dark:text-red-400 bg-red-500/10 border-red-500/30",
}

export function dateLabel(dateA: string, dateB?: string): string {
  if (dateB) return `${dateA} → ${dateB}`
  return dateA
}
