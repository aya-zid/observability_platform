// src/api/apache.ts
import client from "@/api/client"

export type ApacheTimePoint = Record<string, number | null>

export type ApacheLabelCount = {
  label: string
  count: number
}

export type ApacheErrorPathStat = {
  path: string
  errors: number
  total: number
  err_pct: number
}

export type ApacheSuspiciousIP = {
  ip: string
  errors: number
  total: number
  rate: number
}

export type ApacheSlowRequest = {
  path: string
  normalized_path: string
  method: string
  status: number
  ip: string
  rt: number // milliseconds
  referer?: string // HTTP referer header
  user_agent?: string // HTTP user-agent header
  timestamp?: string // Log timestamp
}

export type ApacheDashboardSummary = {
  // Timeseries for charts
  status_timeseries: ApacheTimePoint[]      // {time, "2xx", "3xx", "4xx", "5xx"} — req/s
  rt_timeseries: ApacheTimePoint[]          // {time, p50, p95, p99} — milliseconds
  req_rate_timeseries: ApacheTimePoint[]    // {time, req} — req/s
  bandwidth_timeseries: ApacheTimePoint[]   // {time, bytes} — bytes/s
  conn_timeseries: ApacheTimePoint[]        // {time, active, waiting, reading, writing}
  vm_total_conn_timeseries: ApacheTimePoint[] // {time, total}
  vm_requests_timeseries: ApacheTimePoint[]   // {time, requests}
  bot_timeseries: ApacheTimePoint[]           // {time, botRate, totalRate}

  // Scalar totals for status breakdown cards
  total_requests: number
  total_2xx: number
  total_3xx: number
  total_4xx: number
  total_5xx: number

  // Aggregation tables
  top_paths: ApacheLabelCount[]
  top_ips: ApacheLabelCount[]
  top_statuses: ApacheLabelCount[]
  top_methods: ApacheLabelCount[]
  top_error_paths: ApacheErrorPathStat[]
  // Normalized path aggregations
  top_normalized_paths: ApacheLabelCount[]
  top_normalized_error_paths: ApacheErrorPathStat[]
  suspicious_ips: ApacheSuspiciousIP[]

  // Latest scalar connection values for stat cards (from apache status endpoint)
  conn_active?: number
  conn_waiting?: number
  conn_reading?: number
  conn_writing?: number

  // VM cumulative counter scalars for stat cards
  vm_requests_total?: number
  vm_accepted_total?: number
  vm_handled_total?: number

  // Slow requests table (top 100, rt > slow_requests_min_rt, computed server-side)
  slow_requests: ApacheSlowRequest[]
  // Total slow requests found in the time range (before truncation to 100)
  slow_requests_total: number
  // Slow requests with normalized paths (top 100)
  slow_requests_normalized: ApacheSlowRequest[]
  // Total normalized slow requests found in the time range (before truncation to 100)
  slow_requests_normalized_total: number
  // The threshold used to compute the above two lists (seconds)
  // Echoed back so the frontend can label the section correctly, e.g. "Slow (> 2s)"
  slow_requests_min_rt: number
}

export type ApacheExplorerResult = {
  rt_timeseries: ApacheTimePoint[]
  request_rate: ApacheTimePoint[]
  top_statuses: ApacheLabelCount[]
  top_paths: ApacheLabelCount[]
  total_count: number
}

export type ApacheExplorerParams = {
  start: string
  end: string
  step: string
  domain?: string
  ip?: string
  path?: string
  status?: string
  min_rt?: string
}

export type ApacheDomainItem = {
  domain: string
  enabled: boolean
  is_default: boolean
  is_shared_log: boolean
}

export type ApacheSlowRequestsListResult = {
  items: ApacheSlowRequest[]
  total: number
  page: number
  per_page: number
  total_pages: number
  min_rt: number
}

export type ApacheSlowRequestsParams = {
  start: string
  end: string
  domain?: string
  min_rt?: string
  page?: string
  per_page?: string
}

export async function getApacheDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string; domain?: string; min_rt?: string; percentiles?: string }
): Promise<ApacheDashboardSummary> {
  const { data } = await client.get<ApacheDashboardSummary>(`/api/apache/${id}/dashboard`, {
    params,
  })
  return data
}

export async function getApacheSlowRequests(
  id: string,
  params: ApacheSlowRequestsParams
): Promise<ApacheSlowRequestsListResult> {
  const { data } = await client.get<ApacheSlowRequestsListResult>(`/api/apache/${id}/slow-requests`, {
    params,
  })
  return data
}

export async function getApacheExplorerResult(
  id: string,
  params: ApacheExplorerParams
): Promise<ApacheExplorerResult> {
  const { data } = await client.get<ApacheExplorerResult>(`/api/apache/${id}/explorer`, {
    params,
  })
  return data
}

export async function getApacheDomains(id: string): Promise<ApacheDomainItem[]> {
  const { data } = await client.get<ApacheDomainItem[]>(`/api/apache/${id}/domains`)
  return data
}

export type ApacheErrorLogsParams = {
  start: string
  end: string
  domain?: string
  limit?: number
}

export async function getApacheErrorLogs(
  id: string,
  params: ApacheErrorLogsParams,
): Promise<{ lines: string[] }> {
  const { data } = await client.get<{ lines: string[] }>(`/api/apache/${id}/error-logs`, {
    params,
  })
  return data
}

export type ApacheIPPathStat = {
  path: string
  method: string
  count: number
  errors: number
  avg_rt: number
  status: string
  status_breakdown: Record<string, number>
}

export type ApacheIPInvestigationResult = {
  total_requests: number
  total_4xx: number
  total_5xx: number
  error_rate: number
  first_seen_today: string
  last_seen_today: string
  req_rate_timeseries: ApacheTimePoint[]
  status_timeseries: ApacheTimePoint[]
  rt_timeseries: ApacheTimePoint[]
  paths: ApacheIPPathStat[]
}

export type ApacheIPInvestigationParams = {
  ip: string
  start: string
  end: string
  step: string
  domain?: string
  seen_start?: string
  seen_end?: string
}

export async function getApacheIPInvestigation(
  id: string,
  params: ApacheIPInvestigationParams,
): Promise<ApacheIPInvestigationResult> {
  const { data } = await client.get<ApacheIPInvestigationResult>(`/api/apache/${id}/ip-investigation`, {
    params,
  })
  return data
}

// ─── Performance chart types (Charts 1, 2, 3) ────────────────────────────────

/** Chart 1: RT Intensity Heatmap
 *  Top-20 normalized paths × five-minute buckets, cell = P95 RT in ms.
 */
export type ApacheRTHeatmap = {
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
export type ApacheScatterPoint = {
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

export type ApacheRTScatter = {
  points: ApacheScatterPoint[]
  /**
   * Server-computed total request count that this scatter response represents.
   * = Σ(individual dots × 1) + Σ(aggregate dot.count)
   * Use this to verify no requests were silently dropped.
   */
  total_requests: number
}

/** Chart 3: RT Distribution by Status — Avg / P95 / P99 per status class. */
export type ApacheRTStatusMetrics = {
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

export type ApacheRTDistribution = {
  "2xx": ApacheRTStatusMetrics
  "3xx": ApacheRTStatusMetrics
  "4xx": ApacheRTStatusMetrics
  "5xx": ApacheRTStatusMetrics
}

// Common params for all three performance endpoints
export type ApachePerformanceParams = {
  start: string   // nanoseconds as string
  end: string     // nanoseconds as string
  domain?: string
}

export async function getApacheRTHeatmap(
  id: string,
  params: ApachePerformanceParams,
): Promise<ApacheRTHeatmap> {
  const { data } = await client.get<ApacheRTHeatmap>(`/api/apache/${id}/performance/heatmap`, {
    params,
  })
  return data
}

export async function getApacheRTScatter(
  id: string,
  params: ApachePerformanceParams,
): Promise<ApacheRTScatter> {
  const { data } = await client.get<ApacheRTScatter>(`/api/apache/${id}/performance/scatter`, {
    params,
  })
  return data
}

export async function getApacheRTDistribution(
  id: string,
  params: ApachePerformanceParams,
): Promise<ApacheRTDistribution> {
  const { data } = await client.get<ApacheRTDistribution>(`/api/apache/${id}/performance/dist`, {
    params,
  })
  return data
}
