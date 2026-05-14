import client from "@/api/client"

export type PostgresTimePoint = Record<string, number | null>

export type PostgresLabelCount = {
  label: string
  count: number
}

export type PostgresLogSummary = {
  slow_queries: number
  errors: number
  auth_failures: number
  connections: number
  disconnections: number
  lock_waits: number
  temp_files: number
  schema_changes: number
  checkpoints: number
}

export type PostgresDatabaseHealth = {
  database: string
  size_bytes: number
  connections: number
  cache_hit_ratio_pct: number
  temp_files_per_second: number
  temp_bytes_per_second: number
  rollback_rate_pct: number
}

export type PostgresLogEvent = {
  timestamp: string
  severity: string
  user: string
  database: string
  client: string
  app: string
  category: string
  duration_ms?: number
  message: string
  labels?: Record<string, string>
}

export type PostgresLogsPage = {
  events: PostgresLogEvent[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type PostgresConfigSetting = {
  name: string
  value?: number
  unit: "bytes" | "seconds" | "boolean" | "number" | string
  description: string
  warn: boolean
}

export type PostgresConfigGroup = {
  key: string
  title: string
  settings: PostgresConfigSetting[]
}

export type PostgresConfigSnapshot = {
  generated_at: string
  groups: PostgresConfigGroup[]
}

export type PostgresDashboardSummary = {
  up?: number
  scrape_error?: number
  max_connections?: number
  connection_saturation_pct?: number
  cache_hit_ratio_pct?: number
  database_size_bytes?: number
  wal_size_bytes?: number
  replication_lag_seconds?: number
  max_tx_duration_seconds?: number
  rollback_rate_pct?: number
  deadlocks_per_min?: number
  archive_failed?: number
  shared_buffers_bytes?: number
  work_mem_bytes?: number
  effective_cache_size_bytes?: number

  connections_timeseries: PostgresTimePoint[]
  activity_timeseries: PostgresTimePoint[]
  transaction_timeseries: PostgresTimePoint[]
  cache_timeseries: PostgresTimePoint[]
  temp_timeseries: PostgresTimePoint[]
  locks_timeseries: PostgresTimePoint[]
  size_timeseries: PostgresTimePoint[]
  wal_timeseries: PostgresTimePoint[]
  row_ops_timeseries: PostgresTimePoint[]
  seq_vs_index_timeseries: PostgresTimePoint[]
  bgwriter_timeseries: PostgresTimePoint[]
  archiver_timeseries: PostgresTimePoint[]
  conn_by_db_timeseries: PostgresTimePoint[]

  database_sizes: PostgresLabelCount[]
  locks_by_mode: PostgresLabelCount[]
  conn_by_database: PostgresLabelCount[]
  database_health: PostgresDatabaseHealth[]
  log_summary: PostgresLogSummary
  recent_logs: PostgresLogEvent[]
  slow_query_logs: PostgresLogEvent[]
}

export async function getPostgresDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string },
): Promise<PostgresDashboardSummary> {
  const { data } = await client.get<PostgresDashboardSummary>(`/api/postgres/${id}/dashboard`, {
    params,
  })
  return data
}

export async function getPostgresConfigSnapshot(id: string): Promise<PostgresConfigSnapshot> {
  const { data } = await client.get<PostgresConfigSnapshot>(`/api/postgres/${id}/config`)
  return data
}

export type PostgresLogsParams = {
  start: string
  end: string
  category?: string
  search?: string
  include_monitoring?: boolean
  page?: number
  page_size?: number
}

// New dedicated logs endpoint — returns ALL logs for the window with pagination.
// Does NOT reuse the dashboard endpoint. This is what fixes the missing logs bug.
export async function getPostgresLogs(
  id: string,
  params: PostgresLogsParams,
): Promise<PostgresLogsPage> {
  const { data } = await client.get<PostgresLogsPage>(`/api/postgres/${id}/logs`, {
    params: {
      start: params.start,
      end: params.end,
      category: params.category ?? "all",
      search: params.search ?? "",
      include_monitoring: params.include_monitoring ?? false,
      page: params.page ?? 1,
      page_size: params.page_size ?? 100,
    },
  })
  return data
}
