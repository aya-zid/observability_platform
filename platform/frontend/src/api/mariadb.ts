import client from "@/api/client"

export type MariaDBTimePoint = Record<string, number | null>

export type MariaDBQueryDigest = {
  digest: string
  digest_text: string
  database: string
  exec_count: number
  total_time_seconds: number
  avg_time_seconds: number
  max_time_seconds: number
  rows_examined: number
  rows_sent: number
  no_index_used_count: number
  tmp_tables: number
  tmp_disk_tables: number
  rows_examined_per_sent: number
}

export type MariaDBLogSummary = {
  slow_queries: number
  errors: number
  warnings: number
  auth_failures: number
  deadlocks: number
  replication: number
  startup: number
  shutdown: number
}

export type MariaDBLogEvent = {
  timestamp: string
  severity: string
  category: string
  user?: string
  host?: string
  thread_id?: string
  subsystem?: string
  duration_ms?: number
  message: string
  labels?: Record<string, string>
}

export type MariaDBLogsPage = {
  events: MariaDBLogEvent[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type MariaDBDashboardSummary = {
  up?: number
  scrape_error?: number
  threads_created_per_sec?: number
  connections_per_sec?: number
  select_scan_per_sec?: number
  opened_tables_per_sec?: number
  rows_read_per_sec?: number
  rows_modified_per_sec?: number
  sort_rows_per_sec?: number
  uptime_seconds?: number
  max_connections?: number
  max_used_connections?: number
  threads_connected?: number
  threads_running?: number
  threads_cached?: number
  connection_saturation_pct?: number
  aborted_connects_per_min?: number
  aborted_clients_per_min?: number
  qps?: number
  slow_queries_per_min?: number
  selects_per_sec?: number
  inserts_per_sec?: number
  updates_per_sec?: number
  deletes_per_sec?: number
  buffer_pool_hit_ratio_pct?: number
  innodb_buffer_pool_size_bytes?: number
  buffer_pool_dirty_pct?: number
  buffer_pool_used_pct?: number
  buffer_pool_wait_free_per_sec?: number
  innodb_data_reads_per_sec?: number
  innodb_data_writes_per_sec?: number
  innodb_fsyncs_per_sec?: number
  innodb_log_writes_per_sec?: number
  innodb_log_waits_per_sec?: number
  innodb_pending_reads?: number
  innodb_pending_writes?: number
  innodb_pending_fsyncs?: number
  innodb_history_list_length?: number
  table_locks_waited_per_min?: number
  innodb_deadlocks_per_min?: number
  innodb_row_lock_waits_per_min?: number
  innodb_row_lock_time_avg_ms?: number
  tmp_tables_per_sec?: number
  tmp_disk_tables_per_sec?: number
  network_rx_bytes_per_sec?: number
  network_tx_bytes_per_sec?: number
  replication_lag_seconds?: number
  slave_running?: number
  qcache_hit_ratio_pct?: number
  qcache_free_mem_bytes?: number
  open_tables?: number
  table_cache_hit_ratio_pct?: number
  query_digest_total_time_seconds?: number
  table_scan_ratio_pct?: number
  handler_read_key_per_sec?: number
  handler_read_rnd_next_per_sec?: number
  sort_merge_passes_per_sec?: number
  binlog_size_bytes?: number
  binlog_files?: number
  wsrep_cluster_size?: number
  wsrep_connected?: number
  wsrep_ready?: number

  connections_timeseries: MariaDBTimePoint[]
  qps_timeseries: MariaDBTimePoint[]
  slow_timeseries: MariaDBTimePoint[]
  buffer_pool_timeseries: MariaDBTimePoint[]
  locks_timeseries: MariaDBTimePoint[]
  query_time_timeseries: MariaDBTimePoint[]
  query_count_timeseries: MariaDBTimePoint[]
  io_timeseries: MariaDBTimePoint[]
  network_timeseries: MariaDBTimePoint[]
  tmp_tables_timeseries: MariaDBTimePoint[]
  rw_timeseries: MariaDBTimePoint[]
  deadlocks_timeseries: MariaDBTimePoint[]
  handler_read_timeseries: MariaDBTimePoint[]
  sort_timeseries: MariaDBTimePoint[]
  innodb_pending_timeseries: MariaDBTimePoint[]
  binlog_timeseries: MariaDBTimePoint[]

  top_query_digests: MariaDBQueryDigest[]
  inefficient_digests: MariaDBQueryDigest[]
  no_index_digests: MariaDBQueryDigest[]
  top_table_sizes: { label: string; count: number }[]
  connection_errors_by_type: { label: string; count: number }[]
  top_dbs_by_queries: { label: string; count: number }[]
  top_dbs_by_query_time: { label: string; count: number }[]
  top_dbs_by_rows_examined: { label: string; count: number }[]
  top_dbs_by_no_index: { label: string; count: number }[]
  db_storage_sizes: { label: string; count: number }[]
  slow_queries_by_db: { label: string; count: number }[]
  log_summary: MariaDBLogSummary
  recent_logs: MariaDBLogEvent[]
  slow_query_logs: MariaDBLogEvent[]
}

export type MariaDBConfigSetting = {
  name: string
  value?: number
  unit: string
  description: string
  warn: boolean
}

export type MariaDBConfigGroup = {
  key: string
  title: string
  settings: MariaDBConfigSetting[]
}

export type MariaDBConfigSnapshot = {
  generated_at: string
  groups: MariaDBConfigGroup[]
}

export async function getMariaDBConfigSnapshot(id: string): Promise<MariaDBConfigSnapshot> {
  const { data } = await client.get<MariaDBConfigSnapshot>(`/api/mariadb/${id}/config`)
  return data
}

export async function getMariaDBDashboardSummary(
  id: string,
  params: { start: string; end: string; step: string; min_duration_seconds?: number },
): Promise<MariaDBDashboardSummary> {
  const { data } = await client.get<MariaDBDashboardSummary>(`/api/mariadb/${id}/dashboard`, {
    params,
  })
  return data
}

export type MariaDBLogsParams = {
  start: string
  end: string
  category?: string
  search?: string
  page?: number
  page_size?: number
  min_duration_ms?: number
}

export type MariaDBErrorLogsResponse = {
  lines: string[]
}

export async function getMariaDBErrorLogs(
  id: string,
  params: { start: string; end: string; limit?: number },
): Promise<MariaDBErrorLogsResponse> {
  const { data } = await client.get<MariaDBErrorLogsResponse>(`/api/mariadb/${id}/error-logs`, {
    params,
  })
  return data
}

export type SlowQueryEntry = {
  timestamp: string
  user: string
  host: string
  thread_id: string
  query_time_s: number
  lock_time_s: number
  rows_sent: number
  rows_examined: number
  db: string
  sql: string
  lines?: string[]
}

export type SlowQueryLogsPage = {
  entries: SlowQueryEntry[]
  total: number
  page: number
  page_size: number
  has_more: boolean
}

export type SlowQueryLogsParams = {
  start: string
  end: string
  page?: number
  page_size?: number
  min_duration_ms?: number
}

export async function getMariaDBLogs(
  id: string,
  params: MariaDBLogsParams,
): Promise<MariaDBLogsPage> {
  const { data } = await client.get<MariaDBLogsPage>(`/api/mariadb/${id}/logs`, {
    params: {
      start: params.start,
      end: params.end,
      category: params.category ?? "all",
      search: params.search ?? "",
      page: params.page ?? 1,
      page_size: params.page_size ?? 100,
      min_duration_ms: params.min_duration_ms,
    },
  })
  return data
}

export async function getMariaDBSlowQueryLogs(
  id: string,
  params: SlowQueryLogsParams,
): Promise<SlowQueryLogsPage> {
  const { data } = await client.get<SlowQueryLogsPage>(`/api/mariadb/${id}/slow-query-logs`, {
    params: {
      start: params.start,
      end: params.end,
      page: params.page ?? 1,
      page_size: params.page_size ?? 50,
      min_duration_ms: params.min_duration_ms,
    },
  })
  return data
}
