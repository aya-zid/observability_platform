import client from "@/api/client"

export type PHPFPMTimePoint = Record<string, number | null>

export type PHPFPMLogSummary = {
  warnings: number
  errors: number
  notices: number
  slow_requests: number
  startups: number
}

export type PHPFPMLogEvent = {
  timestamp: string
  severity: string
  pool: string
  category: string
  message: string
  labels?: Record<string, string>
  method?: string
  uri?: string
  status_code?: number
  script?: string
  duration_ms?: number
  memory_kb?: number
  cpu?: number
}

export type PHPFPMLogsPage = {
  events: PHPFPMLogEvent[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type ScriptStat = {
  script: string
  avg_duration_ms: number
  avg_memory_kb: number
  count: number
}

export type ProcessStat = {
  pid: string
  state: string
  duration_ms: number
  memory_kb: number
  cpu_pct: number
  requests: number
}

export type LabelCount = {
  label: string
  count: number
}

export type PoolDetail = {
  active_processes?: number
  idle_processes?: number
  total_processes?: number
  saturation_pct?: number
  accepted_conn_per_sec?: number
  max_children_reached?: number
  slow_requests?: number

  status_2xx?: number
  status_4xx?: number
  status_5xx?: number
  latency_p50?: number
  latency_p95?: number
  latency_p99?: number
  top_slow_scripts?: ScriptStat[]
  top_uris?: LabelCount[]
  request_count?: number

  error_count?: number
  warning_count?: number
  notice_count?: number
  slow_request_count?: number
  top_error_messages?: LabelCount[]
}

export type PHPFPMDashboardSummary = {
  // Status
  up?: number
  scrape_failures?: number
  uptime_seconds?: number
  health_score?: number

  // Worker pool
  active_processes?: number
  idle_processes?: number
  total_processes?: number
  max_active_processes?: number
  worker_saturation_pct?: number

  // Queue
  listen_queue?: number
  listen_queue_len?: number
  queue_utilization_pct?: number
  max_listen_queue?: number

  // Throughput
  accepted_connections?: number
  accepted_conn_per_sec?: number

  // Performance & Errors
  slow_requests?: number
  max_children_reached?: number
  avg_duration_ms?: number
  avg_memory_bytes?: number
  avg_memory_kb?: number
  avg_cpu_pct?: number
  running_processes?: number

  // OPcache
  opcache_enabled?: number
  opcache_hit_rate?: number
  opcache_memory_used_ratio?: number
  opcache_memory_free_bytes?: number
  opcache_memory_total_bytes?: number
  opcache_wasted_ratio?: number
  opcache_cached_scripts?: number
  opcache_hits_total?: number
  opcache_misses_total?: number
  opcache_restarts_total?: number

  // Log-derived: top scripts
  top_slow_scripts?: ScriptStat[]
  top_mem_scripts?: ScriptStat[]

  // Log-derived: per-process
  top_processes_duration?: ProcessStat[]
  top_processes_memory?: ProcessStat[]

  // Log-derived: latency
  latency_p50?: number
  latency_p95?: number
  latency_p99?: number

  // Log-derived: status codes
  status_2xx?: number
  status_4xx?: number
  status_5xx?: number

  // Log-derived: top URIs
  top_uris?: LabelCount[]
  top_error_uris?: LabelCount[]

  // VM timeseries
  workers_timeseries: PHPFPMTimePoint[]
  queue_timeseries: PHPFPMTimePoint[]
  throughput_timeseries: PHPFPMTimePoint[]
  duration_timeseries: PHPFPMTimePoint[]
  memory_timeseries: PHPFPMTimePoint[]
  cpu_timeseries: PHPFPMTimePoint[]

  // OPcache timeseries
  opcache_hit_rate_timeseries: PHPFPMTimePoint[]
  opcache_memory_timeseries: PHPFPMTimePoint[]
  opcache_scripts_timeseries: PHPFPMTimePoint[]

  // Log-derived timeseries
  status_timeseries: PHPFPMTimePoint[]
  req_rate_timeseries: PHPFPMTimePoint[]

  // Log signals
  log_summary: PHPFPMLogSummary
  recent_logs: PHPFPMLogEvent[]

  // Pools
  pools: string[]
  pool_details?: Record<string, PoolDetail>

  // Log-derived analytics
  method_distribution?: LabelCount[]
  status_distribution?: LabelCount[]
  duration_histogram?: LabelCount[]
  memory_histogram?: LabelCount[]
  cpu_histogram?: LabelCount[]
  top_error_messages?: LabelCount[]
  error_type_breakdown?: LabelCount[]
}

export async function getPHPFPMDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string },
): Promise<PHPFPMDashboardSummary> {
  const { data } = await client.get<PHPFPMDashboardSummary>(`/api/phpfpm/${id}/dashboard`, { params })
  return data
}

export type PHPFPMLogsParams = {
  start: string
  end: string
  category?: string
  search?: string
  page?: number
  page_size?: number
}

export type PHPFPMLogsTypeMeta = {
  access_count: number
  error_count: number
  slow_count: number
}

export type PHPFPMLogsParams = {
  start: string
  end: string
  log_file?: string
  pool?: string
  search?: string
  method?: string
  status_code?: string
  min_duration?: string
  max_duration?: string
  min_memory?: string
  max_memory?: string
  min_cpu?: string
  max_cpu?: string
  page?: number
  page_size?: number
}

export async function getPHPFPMLogs(
  id: string,
  params: PHPFPMLogsParams,
): Promise<PHPFPMLogsPage> {
  const { data } = await client.get<PHPFPMLogsPage>(`/api/phpfpm/${id}/logs`, {
    params: {
      start: params.start,
      end: params.end,
      log_file: params.log_file ?? "",
      pool: params.pool ?? "",
      search: params.search ?? "",
      method: params.method ?? "",
      status_code: params.status_code ?? "",
      min_duration: params.min_duration ?? "",
      max_duration: params.max_duration ?? "",
      min_memory: params.min_memory ?? "",
      max_memory: params.max_memory ?? "",
      min_cpu: params.min_cpu ?? "",
      max_cpu: params.max_cpu ?? "",
      page: params.page ?? 1,
      page_size: params.page_size ?? 100,
    },
  })
  return data
}

export async function getPHPFPMLogsMeta(
  id: string,
  params: { start: string; end: string },
): Promise<PHPFPMLogsTypeMeta> {
  const { data } = await client.get<PHPFPMLogsTypeMeta>(`/api/phpfpm/${id}/logs-meta`, { params })
  return data
}
