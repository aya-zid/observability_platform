// src/api/nginx.ts
import client from "@/api/client"

export type NginxTimePoint = Record<string, number | null>

export type NginxLabelCount = {
  label: string
  count: number
}

export type NginxErrorPathStat = {
  path: string
  errors: number
  total: number
  err_pct: number
}

export type NginxSuspiciousIP = {
  ip: string
  errors: number
  total: number
  rate: number
}

export type NginxSlowRequest = {
  path: string
  normalized_path: string
  method: string
  status: number
  ip: string
  rt: number // seconds
  referer?: string // HTTP referer header
  user_agent?: string // HTTP user-agent header
  timestamp?: string // Log timestamp
}

export type NginxDashboardSummary = {
  // Timeseries for charts
  status_timeseries: NginxTimePoint[]      // {time, "2xx", "3xx", "4xx", "5xx"} — req/s
  rt_timeseries: NginxTimePoint[]          // {time, p50, p95, p99} — milliseconds
  req_rate_timeseries: NginxTimePoint[]    // {time, req} — req/s
  bandwidth_timeseries: NginxTimePoint[]   // {time, bytes} — bytes/s
  conn_timeseries: NginxTimePoint[]        // {time, active, waiting, reading, writing}
  vm_total_conn_timeseries: NginxTimePoint[] // {time, total}
  vm_requests_timeseries: NginxTimePoint[]   // {time, requests}
  bot_timeseries: NginxTimePoint[]           // {time, botRate, totalRate}

  // Scalar totals for status breakdown cards
  total_requests: number
  total_2xx: number
  total_3xx: number
  total_4xx: number
  total_5xx: number

  // Aggregation tables
  top_paths: NginxLabelCount[]
  top_ips: NginxLabelCount[]
  top_statuses: NginxLabelCount[]
  top_methods: NginxLabelCount[]
  top_error_paths: NginxErrorPathStat[]
  // Normalized path aggregations
  top_normalized_paths: NginxLabelCount[]
  top_normalized_error_paths: NginxErrorPathStat[]
  suspicious_ips: NginxSuspiciousIP[]

  // Latest scalar connection values for stat cards (from nginx status endpoint)
  conn_active?: number
  conn_waiting?: number
  conn_reading?: number
  conn_writing?: number

  // VM cumulative counter scalars for stat cards
  vm_requests_total?: number
  vm_accepted_total?: number
  vm_handled_total?: number

  // Slow requests table (top 100, rt > slow_requests_min_rt, computed server-side)
  slow_requests: NginxSlowRequest[]
  // Total slow requests found in the time range (before truncation to 100)
  slow_requests_total: number
  // Slow requests with normalized paths (top 100)
  slow_requests_normalized: NginxSlowRequest[]
  // Total normalized slow requests found in the time range (before truncation to 100)
  slow_requests_normalized_total: number
  // The threshold used to compute the above two lists (seconds)
  // Echoed back so the frontend can label the section correctly, e.g. "Slow (> 2s)"
  slow_requests_min_rt: number
}

export type NginxExplorerResult = {
  rt_timeseries: NginxTimePoint[]
  request_rate: NginxTimePoint[]
  top_statuses: NginxLabelCount[]
  top_paths: NginxLabelCount[]
  total_count: number
}

export type NginxExplorerParams = {
  start: string
  end: string
  step: string
  domain?: string
  ip?: string
  path?: string
  status?: string
  min_rt?: string
}

export type NginxDomainItem = {
  domain: string
  enabled: boolean
}

export type NginxSlowRequestsListResult = {
  items: NginxSlowRequest[]
  total: number
  page: number
  per_page: number
  total_pages: number
  min_rt: number
}

export type NginxSlowRequestsParams = {
  start: string
  end: string
  domain?: string
  min_rt?: string
  page?: string
  per_page?: string
}

export async function getNginxDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string; domain?: string; min_rt?: string; percentiles?: string }
): Promise<NginxDashboardSummary> {
  const { data } = await client.get<NginxDashboardSummary>(`/api/nginx/${id}/dashboard`, {
    params,
  })
  return data
}

export async function getNginxSlowRequests(
  id: string,
  params: NginxSlowRequestsParams
): Promise<NginxSlowRequestsListResult> {
  const { data } = await client.get<NginxSlowRequestsListResult>(`/api/nginx/${id}/slow-requests`, {
    params,
  })
  return data
}

export async function getNginxExplorerResult(
  id: string,
  params: NginxExplorerParams
): Promise<NginxExplorerResult> {
  const { data } = await client.get<NginxExplorerResult>(`/api/nginx/${id}/explorer`, {
    params,
  })
  return data
}

export async function getNginxDomains(id: string): Promise<NginxDomainItem[]> {
  const { data } = await client.get<NginxDomainItem[]>(`/api/nginx/${id}/domains`)
  return data
}

export type NginxErrorLogsParams = {
  start: string
  end: string
  domain?: string
  limit?: number
}

export async function getNginxErrorLogs(
  id: string,
  params: NginxErrorLogsParams,
): Promise<{ lines: string[] }> {
  const { data } = await client.get<{ lines: string[] }>(`/api/nginx/${id}/error-logs`, {
    params,
  })
  return data
}

export type NginxIPPathStat = {
  path: string
  method: string
  count: number
  errors: number
  avg_rt: number
  status: string
  status_breakdown: Record<string, number>
}

export type NginxIPInvestigationResult = {
  total_requests: number
  total_4xx: number
  total_5xx: number
  error_rate: number
  first_seen_today: string
  last_seen_today: string
  req_rate_timeseries: NginxTimePoint[]
  status_timeseries: NginxTimePoint[]
  rt_timeseries: NginxTimePoint[]
  paths: NginxIPPathStat[]
}

export type NginxIPInvestigationParams = {
  ip: string
  start: string
  end: string
  step: string
  domain?: string
  seen_start?: string
  seen_end?: string
}

export async function getNginxIPInvestigation(
  id: string,
  params: NginxIPInvestigationParams,
): Promise<NginxIPInvestigationResult> {
  const { data } = await client.get<NginxIPInvestigationResult>(`/api/nginx/${id}/ip-investigation`, {
    params,
  })
  return data
}

// ─── Performance chart types (Charts 1, 2, 3) ────────────────────────────────

/** Chart 1: RT Intensity Heatmap
 *  Top-20 normalized paths × five-minute buckets, cell = P95 RT in ms.
 */
export type NginxRTHeatmap = {
  paths: string[]    // length ≤ 20, y-axis labels
  buckets: string[]  // x-axis labels e.g. "09:00"
  matrix: number[][] // matrix[pathIdx][bucketIdx] = P95 RT ms (0 = no data)
  counts: number[][] // counts[pathIdx][bucketIdx] = request count in bucket
  p95_counts: number[][] // p95_counts[pathIdx][bucketIdx] = requests at or below P95
}

/** Chart 2: Raw RT Scatter — individual/aggregated request dots.
 *
 * Individual dots  → real request, carries path / method / ip / status.
 * Aggregate dots   → avg of requests that exceeded the per-window budget.
 *   - 2xx / 3xx normals: one avg dot per window (status 200 / 301).
 *   - 4xx / 5xx overflow + outliers: one avg dot PER EXACT STATUS CODE per window
 *     (e.g. separate dots for 401, 403, 404 within the same window).
 */
export type NginxScatterPoint = {
  time: number              // unix ms within the selected hour
  rt: number                // response time in ms
  status: number            // HTTP status code
  path: string              // raw path (individual dots) or "" (aggregate dots)
  method: string
  normalized_path?: string
  ip?: string               // client IP — individual dots only
  is_aggregate?: boolean    // true = avg dot, false = individual dot
  count?: number            // for aggregate dots: how many requests it represents
  window_label?: string     // e.g. "09:00–09:03" for aggregate dots
}

export type NginxRTScatter = {
  points: NginxScatterPoint[]
  /**
   * Server-computed total request count that this scatter response represents.
   * = Σ(individual dots × 1) + Σ(aggregate dot.count)
   * Use this to verify no requests were silently dropped.
   */
  total_requests: number
}

/** Chart 3: RT Distribution by Status — Avg / P95 / P99 per status class. */
export type NginxRTStatusMetrics = {
  avg: number
  p50: number
  p95: number
  p99: number
  count: number
  p50_count: number
  p95_count: number
  p99_count: number
  status_breakdown?: Record<string, number>
}

export type NginxRTDistribution = {
  "2xx": NginxRTStatusMetrics
  "3xx": NginxRTStatusMetrics
  "4xx": NginxRTStatusMetrics
  "5xx": NginxRTStatusMetrics
}

// Common params for all three performance endpoints
export type NginxPerformanceParams = {
  start: string   // nanoseconds as string
  end: string     // nanoseconds as string
  domain?: string
}

export async function getNginxRTHeatmap(
  id: string,
  params: NginxPerformanceParams,
): Promise<NginxRTHeatmap> {
  const { data } = await client.get<NginxRTHeatmap>(`/api/nginx/${id}/performance/heatmap`, {
    params,
  })
  return data
}

export async function getNginxRTScatter(
  id: string,
  params: NginxPerformanceParams,
): Promise<NginxRTScatter> {
  const { data } = await client.get<NginxRTScatter>(`/api/nginx/${id}/performance/scatter`, {
    params,
  })
  return data
}

export async function getNginxRTDistribution(
  id: string,
  params: NginxPerformanceParams,
): Promise<NginxRTDistribution> {
  const { data } = await client.get<NginxRTDistribution>(`/api/nginx/${id}/performance/dist`, {
    params,
  })
  return data
}
