import client from "@/api/client"

export type RedisTimePoint = Record<string, number | null>

export type RedisLabelCount = {
  label: string
  count: number
}

export type RedisLogSummary = {
  warnings: number
  errors: number
  shutdowns: number
  startups: number
  persistence: number
  auth_failures: number
  security: number
}

export type RedisLogEvent = {
  timestamp: string
  severity: string
  role: string
  pid: string
  category: string
  message: string
  labels?: Record<string, string>
}

export type RedisLogsPage = {
  events: RedisLogEvent[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type RedisDashboardSummary = {
  up?: number
  scrape_error?: number
  memory_used_bytes?: number
  memory_rss_bytes?: number
  memory_peak_bytes?: number
  memory_frag_ratio?: number
  max_memory_bytes?: number
  system_memory_bytes?: number
  memory_usage_pct?: number
  connected_clients?: number
  max_clients?: number
  client_saturation_pct?: number
  blocked_clients?: number
  commands_per_sec?: number
  cache_hit_ratio_pct?: number
  keys_total?: number
  evict_rate?: number
  expire_rate?: number
  error_replies_per_sec?: number
  unexpected_errors_per_sec?: number
  command_failures_per_sec?: number
  command_rejects_per_sec?: number
  net_input_bytes_per_sec?: number
  net_output_bytes_per_sec?: number
  uptime_seconds?: number
  slow_log_length?: number
  rejected_conns_per_sec?: number
  cpu_sys_per_sec?: number
  cpu_user_per_sec?: number
  connected_slaves?: number
  latest_fork_seconds?: number
  rdb_changes_since_last_save?: number
  rdb_save_in_progress?: number
  aof_enabled?: number
  aof_rewrite_in_progress?: number

  memory_timeseries: RedisTimePoint[]
  clients_timeseries: RedisTimePoint[]
  commands_timeseries: RedisTimePoint[]
  cache_timeseries: RedisTimePoint[]
  error_timeseries: RedisTimePoint[]
  net_timeseries: RedisTimePoint[]
  cpu_timeseries: RedisTimePoint[]
  keys_timeseries: RedisTimePoint[]
  evict_expire_timeseries: RedisTimePoint[]
  frag_timeseries: RedisTimePoint[]
  conn_rate_timeseries: RedisTimePoint[]
  persistence_timeseries: RedisTimePoint[]

  keys_by_db: RedisLabelCount[]
  commands_top: RedisLabelCount[]
  command_rates: RedisLabelCount[]
  log_summary: RedisLogSummary
  recent_logs: RedisLogEvent[]
}

export async function getRedisDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string },
): Promise<RedisDashboardSummary> {
  const { data } = await client.get<RedisDashboardSummary>(`/api/redis/${id}/dashboard`, {
    params,
  })
  return data
}

export type RedisLogsParams = {
  start: string
  end: string
  category?: string
  search?: string
  page?: number
  page_size?: number
}

export async function getRedisLogs(
  id: string,
  params: RedisLogsParams,
): Promise<RedisLogsPage> {
  const { data } = await client.get<RedisLogsPage>(`/api/redis/${id}/logs`, {
    params: {
      start: params.start,
      end: params.end,
      category: params.category ?? "all",
      search: params.search ?? "",
      page: params.page ?? 1,
      page_size: params.page_size ?? 100,
    },
  })
  return data
}
