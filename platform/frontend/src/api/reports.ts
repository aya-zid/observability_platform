import client from "@/api/client"

export type ReportStatus = "generating" | "ready" | "error"
export type ReportType = "today" | "compare"

export type ReportPriority = "critical" | "high" | "medium" | "low"
export type ReportEffort = "low" | "medium" | "high"

// ── numeric payload (numbers) ────────────────────────────────────────────────

export type ReportServiceMetrics = {
  // web
  total_requests?: number | null
  avg_req_rate?: number | null
  total_5xx?: number | null
  total_4xx?: number | null
  p95_rt_ms?: number | null
  bandwidth_bytes?: number | null
  bot_rate?: number | null
  top_error_path?: string
  // postgres
  cache_hit_ratio_pct?: number | null
  slow_queries?: number | null
  errors?: number | null
  auth_failures?: number | null
  lock_waits?: number | null
  deadlocks_per_min?: number | null
  connection_saturation_pct?: number | null
  db_size_bytes?: number | null
  // mariadb
  qps?: number | null
  slow_queries_per_min?: number | null
  buffer_pool_hit_ratio_pct?: number | null
  threads_connected?: number | null
  aborted_connects_per_min?: number | null
  table_scan_ratio_pct?: number | null
  // redis
  memory_usage_pct?: number | null
  commands_per_sec?: number | null
  connected_clients?: number | null
  client_saturation_pct?: number | null
  evict_rate?: number | null
  error_replies_per_sec?: number | null
  slow_log_count?: number | null
  // php-fpm
  worker_saturation_pct?: number | null
  accepted_conn_per_sec?: number | null
  slow_requests?: number | null
  status_5xx?: number | null
  latency_p95_ms?: number | null
  opcache_hit_rate?: number | null
  max_children_reached?: number | null
}

export type ReportFleetMetrics = {
  avg_cpu_pct?: number | null
  avg_mem_pct?: number | null
  avg_disk_pct?: number | null
  net_in_bps?: number | null
  net_out_bps?: number | null
  load_avg_1m?: number | null
  containers?: number
  avg_container_cpu_pct?: number | null
  avg_container_mem_pct?: number | null
}

export type ReportLogSignals = {
  errors: number
  warnings: number
}

export type ReportWindow = {
  services?: Record<string, ReportServiceMetrics>
  fleet?: ReportFleetMetrics | null
  log_signals?: Record<string, ReportLogSignals>
  alerts_fired?: number
}

export type ReportRegion = {
  region: string
  day_a: ReportWindow
  day_b?: ReportWindow | null
}

export type ReportAlertPayload = {
  severity: string
  rule_name: string
  host_machine_id: string
  service: string
  status: string
  triggered_at: string
  resolved_at?: string
  actual_value?: number | null
  domain?: string
  pool?: string
  container_name?: string
}

export type ReportIncidentPayload = {
  id: string
  title: string
  severity: string
  category: string
  status: string
  hostname: string
  firing_since: string
  resolved_at?: string
  members: number
}

export type ReportServiceHealth = {
  service: string
  present: boolean
  up_instances: number
}

export type ReportHosts = {
  total: number
  online: number
}

export type ReportDelta = {
  region: string
  service: string
  metric: string
  day_a_value?: number | null
  day_b_value?: number | null
  delta_pct?: number | null
}

export type ReportNumbers = {
  type: ReportType
  day_a: string
  day_b?: string
  generated_at: string
  regions: ReportRegion[]
  alerts_a?: ReportAlertPayload[]
  alerts_b?: ReportAlertPayload[]
  incidents?: ReportIncidentPayload[]
  service_health?: ReportServiceHealth[]
  hosts?: ReportHosts
  fleet?: ReportFleetMetrics | null
  fleet_day_b?: ReportFleetMetrics | null
  deltas?: ReportDelta[]
}

// ── AI report (ai_report) ────────────────────────────────────────────────────

export type Recommendation = {
  priority: ReportPriority
  title: string
  action: string
  rationale: string
  expected_impact: string
  effort: ReportEffort
}

export type KeyFinding = {
  severity: "critical" | "warning" | "info"
  title: string
  detail: string
  day_a_value?: string
  day_b_value?: string
  delta_pct?: number | null
}

export type RegionAI = {
  region: string
  summary: string
  highlights?: string[]
  concerns?: string[]
  notable_metrics?: { service: string; metric: string; value: string }[]
  biggest_differences?: {
    metric: string
    day_a: string
    day_b: string
    delta_pct?: number | null
    direction: "up" | "down" | "flat"
  }[]
}

export type TodayAIReport = {
  overall_status: string
  executive_summary: string
  key_findings: KeyFinding[]
  regions: RegionAI[]
  alerts_insight: string
  recommendations: Recommendation[]
  outlook: string
}

export type ServiceComparison = {
  service: string
  metrics: {
    metric: string
    day_a: string
    day_b: string
    delta_pct?: number | null
  }[]
}

export type CompareAIReport = {
  overall_status: string
  executive_summary: string
  key_findings: KeyFinding[]
  regions: RegionAI[]
  service_comparisons: ServiceComparison[]
  alerts_diff: string
  recommendations: Recommendation[]
}

export type AIReport = TodayAIReport | CompareAIReport

// ── Report record ────────────────────────────────────────────────────────────

export type ReportSummary = {
  id: number
  type: ReportType
  date_a: string
  date_b: string
  status: ReportStatus
  error?: string
  generated_by: number
  created_at: string
  finished_at?: string | null
}

export type ReportDetail = ReportSummary & {
  numbers?: ReportNumbers | null
  ai_report?: AIReport | null
}

// ── client functions ─────────────────────────────────────────────────────────

export type GenerateReportInput = {
  type: ReportType
  date_a?: string
  date_b?: string
}

export async function generateReport(input: GenerateReportInput): Promise<ReportDetail> {
  const { data } = await client.post<ReportDetail>("/api/reports/generate", input)
  return data
}

export async function listReports(
  params?: { type?: ReportType; limit?: number; offset?: number }
): Promise<ReportDetail[]> {
  const { data } = await client.get<ReportDetail[]>("/api/reports", { params })
  return data
}

export async function getReport(id: number): Promise<ReportDetail> {
  const { data } = await client.get<ReportDetail>(`/api/reports/${id}`)
  return data
}

export async function regenerateReport(id: number): Promise<ReportDetail> {
  const { data } = await client.post<ReportDetail>(`/api/reports/${id}/regenerate`)
  return data
}

export async function deleteReport(id: number): Promise<void> {
  await client.delete(`/api/reports/${id}`)
}
