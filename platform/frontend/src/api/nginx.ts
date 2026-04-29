// src/api/nginx.ts
import client from "@/api/client"

export type NginxTimePoint = Record<string, number>

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
  method: string
  status: number
  ip: string
  rt: number // seconds
  referer?: string // HTTP referer header
  userAgent?: string // HTTP user-agent header
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

  // Slow requests table (top 100, rt > 1s, computed server-side)
  slow_requests: NginxSlowRequest[]
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
  is_default: boolean
  is_shared_log: boolean
}

export async function getNginxDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string; domain?: string }
): Promise<NginxDashboardSummary> {
  const { data } = await client.get<NginxDashboardSummary>(`/api/nginx/${id}/dashboard`, {
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

// src/api/nginx.ts — ADD these types and function (append to existing file)

export type NginxIPPathStat = {
  path: string
  method: string
  count: number
  errors: number
  avg_rt: number
  status: string
  status_breakdown: Record<string, number> // e.g. {"200": 42, "404": 8}
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