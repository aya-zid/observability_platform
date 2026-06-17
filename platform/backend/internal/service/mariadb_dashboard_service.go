package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type MariaDBDashboardService struct {
	base  *NginxDashboardService
	redis *redis.Client
}

type vmQueryResult struct {
	data []VMSeries
	err  error
}

type MariaDBDashboardSummary struct {
	Up                         *float64 `json:"up,omitempty"`
	ScrapeError                *float64 `json:"scrape_error,omitempty"`
	ThreadsCreatedPerSec       *float64 `json:"threads_created_per_sec,omitempty"`
	ConnectionsPerSec          *float64 `json:"connections_per_sec,omitempty"`
	SelectScanPerSec           *float64 `json:"select_scan_per_sec,omitempty"`
	OpenedTablesPerSec         *float64 `json:"opened_tables_per_sec,omitempty"`
	RowsReadPerSec             *float64 `json:"rows_read_per_sec,omitempty"`
	RowsModifiedPerSec         *float64 `json:"rows_modified_per_sec,omitempty"`
	SortRowsPerSec             *float64 `json:"sort_rows_per_sec,omitempty"`
	UptimeSeconds              *float64 `json:"uptime_seconds,omitempty"`
	MaxConnections             *float64 `json:"max_connections,omitempty"`
	MaxUsedConnections         *float64 `json:"max_used_connections,omitempty"`
	ThreadsConnected           *float64 `json:"threads_connected,omitempty"`
	ThreadsRunning             *float64 `json:"threads_running,omitempty"`
	ThreadsCached              *float64 `json:"threads_cached,omitempty"`
	ConnectionSaturationPct    *float64 `json:"connection_saturation_pct,omitempty"`
	AbortedConnectsPerMin      *float64 `json:"aborted_connects_per_min,omitempty"`
	AbortedClientsPerMin       *float64 `json:"aborted_clients_per_min,omitempty"`
	QPS                        *float64 `json:"qps,omitempty"`
	SlowQueriesPerMin          *float64 `json:"slow_queries_per_min,omitempty"`
	SelectsPerSec              *float64 `json:"selects_per_sec,omitempty"`
	InsertsPerSec              *float64 `json:"inserts_per_sec,omitempty"`
	UpdatesPerSec              *float64 `json:"updates_per_sec,omitempty"`
	DeletesPerSec              *float64 `json:"deletes_per_sec,omitempty"`
	BufferPoolHitRatioPct      *float64 `json:"buffer_pool_hit_ratio_pct,omitempty"`
	InnodbBufferPoolSizeBytes  *float64 `json:"innodb_buffer_pool_size_bytes,omitempty"`
	BufferPoolDirtyPct         *float64 `json:"buffer_pool_dirty_pct,omitempty"`
	BufferPoolUsedPct          *float64 `json:"buffer_pool_used_pct,omitempty"`
	BufferPoolWaitFreePerSec   *float64 `json:"buffer_pool_wait_free_per_sec,omitempty"`
	InnodbDataReadsPerSec      *float64 `json:"innodb_data_reads_per_sec,omitempty"`
	InnodbDataWritesPerSec     *float64 `json:"innodb_data_writes_per_sec,omitempty"`
	InnodbFsyncsPerSec         *float64 `json:"innodb_fsyncs_per_sec,omitempty"`
	InnodbLogWritesPerSec      *float64 `json:"innodb_log_writes_per_sec,omitempty"`
	InnodbLogWaitsPerSec       *float64 `json:"innodb_log_waits_per_sec,omitempty"`
	InnodbPendingReads         *float64 `json:"innodb_pending_reads,omitempty"`
	InnodbPendingWrites        *float64 `json:"innodb_pending_writes,omitempty"`
	InnodbPendingFsyncs        *float64 `json:"innodb_pending_fsyncs,omitempty"`
	InnodbHistoryListLength    *float64 `json:"innodb_history_list_length,omitempty"`
	TableLocksWaitedPerMin     *float64 `json:"table_locks_waited_per_min,omitempty"`
	InnodbDeadlocksPerMin      *float64 `json:"innodb_deadlocks_per_min,omitempty"`
	InnodbRowLockWaitsPerMin   *float64 `json:"innodb_row_lock_waits_per_min,omitempty"`
	InnodbRowLockTimeAvgMs     *float64 `json:"innodb_row_lock_time_avg_ms,omitempty"`
	TmpTablesPerSec            *float64 `json:"tmp_tables_per_sec,omitempty"`
	TmpDiskTablesPerSec        *float64 `json:"tmp_disk_tables_per_sec,omitempty"`
	NetworkRxBytesPerSec       *float64 `json:"network_rx_bytes_per_sec,omitempty"`
	NetworkTxBytesPerSec       *float64 `json:"network_tx_bytes_per_sec,omitempty"`
	ReplicationLagSeconds      *float64 `json:"replication_lag_seconds,omitempty"`
	SlaveRunning               *float64 `json:"slave_running,omitempty"`
	QcacheHitRatioPct          *float64 `json:"qcache_hit_ratio_pct,omitempty"`
	QcacheFreeMemBytes         *float64 `json:"qcache_free_mem_bytes,omitempty"`
	OpenTables                 *float64 `json:"open_tables,omitempty"`
	TableCacheHitRatioPct      *float64 `json:"table_cache_hit_ratio_pct,omitempty"`
	QueryDigestTotalTimeSeconds *float64 `json:"query_digest_total_time_seconds,omitempty"`
	TableScanRatioPct          *float64 `json:"table_scan_ratio_pct,omitempty"`
	HandlerReadKeyPerSec       *float64 `json:"handler_read_key_per_sec,omitempty"`
	HandlerReadRndNextPerSec   *float64 `json:"handler_read_rnd_next_per_sec,omitempty"`
	SortMergePassesPerSec      *float64 `json:"sort_merge_passes_per_sec,omitempty"`
	BinlogSizeBytes            *float64 `json:"binlog_size_bytes,omitempty"`
	BinlogFiles                *float64 `json:"binlog_files,omitempty"`
	WsrepClusterSize           *float64 `json:"wsrep_cluster_size,omitempty"`
	WsrepConnected             *float64 `json:"wsrep_connected,omitempty"`
	WsrepReady                 *float64 `json:"wsrep_ready,omitempty"`

	ConnectionsTimeseries []TimePoint `json:"connections_timeseries"`
	QPSTimeseries         []TimePoint `json:"qps_timeseries"`
	SlowTimeseries        []TimePoint `json:"slow_timeseries"`
	BufferPoolTimeseries  []TimePoint `json:"buffer_pool_timeseries"`
	LocksTimeseries       []TimePoint `json:"locks_timeseries"`
	QueryTimeTimeseries   []TimePoint `json:"query_time_timeseries"`
	QueryCountTimeseries  []TimePoint `json:"query_count_timeseries"`
	IOTimeseries          []TimePoint `json:"io_timeseries"`
	NetworkTimeseries     []TimePoint `json:"network_timeseries"`
	TmpTablesTimeseries   []TimePoint `json:"tmp_tables_timeseries"`
	RWTimeseries          []TimePoint `json:"rw_timeseries"`
	DeadlocksTimeseries   []TimePoint `json:"deadlocks_timeseries"`
	HandlerReadTimeseries []TimePoint `json:"handler_read_timeseries"`
	SortTimeseries        []TimePoint `json:"sort_timeseries"`
	InnodbPendingTimeseries []TimePoint `json:"innodb_pending_timeseries"`
	BinlogTimeseries      []TimePoint `json:"binlog_timeseries"`

	TopQueryDigests        []MariaDBQueryDigest `json:"top_query_digests"`
	InefficientDigests     []MariaDBQueryDigest `json:"inefficient_digests"`
	NoIndexDigests         []MariaDBQueryDigest `json:"no_index_digests"`
	TopTableSizes          []LabelCount         `json:"top_table_sizes"`
	ConnectionErrorsByType []LabelCount         `json:"connection_errors_by_type"`
	TopDBsByQueries        []LabelCount         `json:"top_dbs_by_queries"`
	TopDBsByQueryTime      []LabelCount         `json:"top_dbs_by_query_time"`
	TopDBsByRowsExamined   []LabelCount         `json:"top_dbs_by_rows_examined"`
	TopDBsByNoIndex        []LabelCount         `json:"top_dbs_by_no_index"`
	DBStorageSizes         []LabelCount         `json:"db_storage_sizes"`
	SlowQueriesByDB        []LabelCount         `json:"slow_queries_by_db"`
	LogSummary            MariaDBLogSummary    `json:"log_summary"`
	RecentLogs            []MariaDBLogEvent    `json:"recent_logs"`
	SlowQueryLogs         []MariaDBLogEvent    `json:"slow_query_logs"`
}

type MariaDBQueryDigest struct {
	Digest              string  `json:"digest"`
	DigestText          string  `json:"digest_text"`
	Database            string  `json:"database"`
	ExecCount           float64 `json:"exec_count"`
	TotalTimeSeconds    float64 `json:"total_time_seconds"`
	AvgTimeSeconds      float64 `json:"avg_time_seconds"`
	MaxTimeSeconds      float64 `json:"max_time_seconds"`
	RowsExamined        float64 `json:"rows_examined"`
	RowsSent            float64 `json:"rows_sent"`
	NoIndexUsedCount    float64 `json:"no_index_used_count"`
	TempTables          float64 `json:"tmp_tables"`
	TempDiskTables      float64 `json:"tmp_disk_tables"`
	RowsExaminedPerSent float64 `json:"rows_examined_per_sent"`
}

type MariaDBLogSummary struct {
	SlowQueries  float64 `json:"slow_queries"`
	Errors       float64 `json:"errors"`
	Warnings     float64 `json:"warnings"`
	AuthFailures float64 `json:"auth_failures"`
	Deadlocks    float64 `json:"deadlocks"`
	Replication  float64 `json:"replication"`
	Startup      float64 `json:"startup"`
	Shutdown     float64 `json:"shutdown"`
}

type MariaDBLogEvent struct {
	Timestamp  string            `json:"timestamp"`
	Severity   string            `json:"severity"`
	Category   string            `json:"category"`
	User       string            `json:"user,omitempty"`
	Host       string            `json:"host,omitempty"`
	ThreadID   string            `json:"thread_id,omitempty"`
	Subsystem  string            `json:"subsystem,omitempty"`
	DurationMs *float64          `json:"duration_ms,omitempty"`
	Message    string            `json:"message"`
	Labels     map[string]string `json:"labels,omitempty"`
}

type SlowQueryEntry struct {
	Timestamp    string   `json:"timestamp"`
	User         string   `json:"user"`
	Host         string   `json:"host"`
	ThreadID     string   `json:"thread_id"`
	QueryTimeS   float64  `json:"query_time_s"`
	LockTimeS    float64  `json:"lock_time_s"`
	RowsSent     int64    `json:"rows_sent"`
	RowsExamined int64    `json:"rows_examined"`
	DB           string   `json:"db"`
	SQL          string   `json:"sql"`
	Lines        []string `json:"lines,omitempty"`
}

type MariaDBLogsPage struct {
	Events   []MariaDBLogEvent `json:"events"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
	HasMore  bool              `json:"has_more"`
}

type SlowQueryLogsPage struct {
	Entries  []SlowQueryEntry `json:"entries"`
	Total    int              `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	HasMore  bool             `json:"has_more"`
}

type MariaDBConfigSetting struct {
	Name        string   `json:"name"`
	Value       *float64 `json:"value,omitempty"`
	Unit        string   `json:"unit"`
	Description string   `json:"description"`
	Warn        bool     `json:"warn"`
}

type MariaDBConfigGroup struct {
	Key      string                  `json:"key"`
	Title    string                  `json:"title"`
	Settings []MariaDBConfigSetting `json:"settings"`
}

type MariaDBConfigSnapshot struct {
	GeneratedAt string               `json:"generated_at"`
	Groups      []MariaDBConfigGroup `json:"groups"`
}

func mariaDBCfgSetting(name, unit, description string) MariaDBConfigSetting {
	return MariaDBConfigSetting{Name: name, Unit: unit, Description: description}
}

func mariaDBConfigCatalog() []MariaDBConfigGroup {
	return []MariaDBConfigGroup{
		{
			Key:   "connection",
			Title: "Connection & Network",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_max_connections", "number", "Maximum simultaneous client connections. Each connection uses memory; stay under available RAM."),
				mariaDBCfgSetting("mysql_global_variables_extra_max_connections", "number", "Additional connections allowed beyond max_connections for emergency access."),
				mariaDBCfgSetting("mysql_global_variables_connect_timeout", "seconds", "How long mysqld waits for a connection packet before dropping."),
				mariaDBCfgSetting("mysql_global_variables_wait_timeout", "seconds", "Non-interactive connection is closed after this idle time."),
				mariaDBCfgSetting("mysql_global_variables_interactive_timeout", "seconds", "Interactive connection (mysql CLI) is closed after this idle time."),
				mariaDBCfgSetting("mysql_global_variables_max_connect_errors", "number", "Max failed connection attempts per host before blocking. Flush with FLUSH HOSTS."),
				mariaDBCfgSetting("mysql_global_variables_back_log", "number", "Size of the listen queue for incoming TCP connections."),
				mariaDBCfgSetting("mysql_global_variables_thread_cache_size", "number", "Threads to keep cached for reuse. Reduces connect/disconnect overhead."),
				mariaDBCfgSetting("mysql_global_variables_max_allowed_packet", "bytes", "Maximum size of a single packet or generated string. Affects large BLOB transfers."),
				mariaDBCfgSetting("mysql_global_variables_net_buffer_length", "bytes", "Buffer for TCP/IP and socket communication per thread."),
				mariaDBCfgSetting("mysql_global_variables_net_read_timeout", "seconds", "Timeout for reading data from a TCP connection during queries."),
				mariaDBCfgSetting("mysql_global_variables_net_retry_count", "number", "Number of retries for interrupted reads on network connections."),
				mariaDBCfgSetting("mysql_global_variables_max_prepared_stmt_count", "number", "Maximum prepared statements across all sessions. Prevents memory exhaustion."),
			},
		},
		{
			Key:   "innodb",
			Title: "InnoDB Engine",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_innodb_buffer_pool_size", "bytes", "Main InnoDB memory pool for cached data and indexes. Typically 60-80% of available RAM."),
				mariaDBCfgSetting("mysql_global_variables_innodb_log_file_size", "bytes", "Size of each redo log file. Larger = better write throughput, longer crash recovery."),
				mariaDBCfgSetting("mysql_global_variables_innodb_log_buffer_size", "bytes", "Buffer for uncommitted redo log writes. Larger for long-running transactions."),
				mariaDBCfgSetting("mysql_global_variables_innodb_flush_log_at_trx_commit", "number", "1 = full ACID (fsync per commit), 2 = flush per sec, 0 = write per sec."),
				mariaDBCfgSetting("mysql_global_variables_innodb_io_capacity", "number", "Approximate I/O operations per second the disk can handle. Base for background flushing."),
				mariaDBCfgSetting("mysql_global_variables_innodb_io_capacity_max", "number", "Max I/O capacity during emergency flushing. Should be higher than innodb_io_capacity."),
				mariaDBCfgSetting("mysql_global_variables_innodb_lock_wait_timeout", "seconds", "How long a transaction waits for a row lock before rolling back."),
				mariaDBCfgSetting("mysql_global_variables_innodb_autoinc_lock_mode", "number", "0 = traditional (table-level), 1 = consecutive (bulk only), 2 = interleaved (max concurrency)."),
				mariaDBCfgSetting("mysql_global_variables_innodb_file_per_table", "boolean", "Each table gets its own .ibd file. ON by default; simplifies transport and reclaims space."),
				mariaDBCfgSetting("mysql_global_variables_innodb_flush_neighbors", "boolean", "Flush nearby dirty pages from buffer pool. ON for HDD, OFF for SSD."),
				mariaDBCfgSetting("mysql_global_variables_innodb_max_dirty_pages_pct", "number", "Max percentage of dirty pages in buffer pool before InnoDB flushes aggressively."),
				mariaDBCfgSetting("mysql_global_variables_innodb_adaptive_hash_index", "boolean", "Adaptive hash index speeds up lookups. May cause contention on write-heavy workloads."),
				mariaDBCfgSetting("mysql_global_variables_innodb_doublewrite", "boolean", "Doublewrite buffer prevents torn pages. InnoDB default; disable only with battery-backed cache."),
				mariaDBCfgSetting("mysql_global_variables_innodb_read_only", "boolean", "Set to ON for read-only replicas to reduce InnoDB memory overhead."),
				mariaDBCfgSetting("mysql_global_variables_innodb_strict_mode", "boolean", "Strict InnoDB error checking. Rejects invalid CREATE/ALTER options."),
				mariaDBCfgSetting("mysql_global_variables_innodb_compression_level", "number", "zlib compression level for InnoDB pages (0-9). Higher = smaller but slower."),
				mariaDBCfgSetting("mysql_global_variables_innodb_encrypt_tables", "boolean", "Enable InnoDB tablespace encryption. Requires encryption key management."),
				mariaDBCfgSetting("mysql_global_variables_innodb_sort_buffer_size", "bytes", "Buffer for sorting during index creation. Larger = faster ALTER/optimize."),
				mariaDBCfgSetting("mysql_global_variables_innodb_old_blocks_time", "milliseconds", "How long a page stays in the old sublist after first access before promotion to young."),
				mariaDBCfgSetting("mysql_global_variables_innodb_open_files", "number", "Max .ibd files InnoDB can keep open at once. Limits table cache recycling."),
				mariaDBCfgSetting("mysql_global_variables_innodb_write_io_threads", "number", "Number of write I/O threads. Higher for write-heavy workloads on fast storage."),
				mariaDBCfgSetting("mysql_global_variables_innodb_rollback_on_timeout", "boolean", "ON = rollback entire transaction on lock timeout. OFF = just the last statement."),
			},
		},
		{
			Key:   "temp_sort",
			Title: "Temporary Tables & Sort",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_tmp_table_size", "bytes", "Max size for in-memory temporary tables. Larger = fewer disk spills."),
				mariaDBCfgSetting("mysql_global_variables_tmp_memory_table_size", "bytes", "Max size for explicit MEMORY tables. Also limits derived tmp tables."),
				mariaDBCfgSetting("mysql_global_variables_tmp_disk_table_size", "bytes", "Max total size for on-disk temporary tables before error."),
				mariaDBCfgSetting("mysql_global_variables_sort_buffer_size", "bytes", "Buffer per thread for ORDER BY / GROUP BY. Large values multiply per connection."),
				mariaDBCfgSetting("mysql_global_variables_join_buffer_size", "bytes", "Minimum buffer for indexless joins. Large = faster but per-connection."),
				mariaDBCfgSetting("mysql_global_variables_join_buffer_space_limit", "bytes", "Max total join buffer space across all joins in a query."),
				mariaDBCfgSetting("mysql_global_variables_join_cache_level", "number", "Hash join cache level. 0-8; higher values enable more advanced hash join optimizations."),
				mariaDBCfgSetting("mysql_global_variables_max_heap_table_size", "bytes", "Limits MEMORY table max size. Also caps tmp_table_size effectively."),
				mariaDBCfgSetting("mysql_global_variables_max_tmp_total_space_usage", "bytes", "Total on-disk temp table space allowed per query before error."),
				mariaDBCfgSetting("mysql_global_variables_max_tmp_session_space_usage", "bytes", "Session limit for total tmp table disk space before error."),
			},
		},
		{
			Key:   "logging",
			Title: "Logging & Slow Query",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_slow_query_log", "boolean", "Enable the slow query log. Must be ON for slow query analysis."),
				mariaDBCfgSetting("mysql_global_variables_long_query_time", "seconds", "Queries exceeding this time are logged as slow. Microsecond precision possible."),
				mariaDBCfgSetting("mysql_global_variables_log_slow_query", "boolean", "Whether the slow query log is enabled (alternative variable name)."),
				mariaDBCfgSetting("mysql_global_variables_log_slow_admin_statements", "boolean", "Log administrative statements (ALTER, ANALYZE, etc.) to slow log."),
				mariaDBCfgSetting("mysql_global_variables_log_queries_not_using_indexes", "boolean", "Log queries that don't use indexes to the slow query log."),
				mariaDBCfgSetting("mysql_global_variables_min_examined_row_limit", "number", "Only log slow queries examining at least this many rows. Reduces noise."),
				mariaDBCfgSetting("mysql_global_variables_log_warnings", "number", "Log warning messages. 1 = warnings only, 2 = also aborted connections."),
				mariaDBCfgSetting("mysql_global_variables_general_log", "boolean", "Log every query to the general log. Heavy on I/O; mainly for debugging."),
			},
		},
		{
			Key:   "binlog",
			Title: "Binary Log",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_log_bin", "boolean", "Enable binary logging. Required for replication and point-in-time recovery."),
				mariaDBCfgSetting("mysql_global_variables_binlog_cache_size", "bytes", "Cache for uncommitted binary log transactions per session."),
				mariaDBCfgSetting("mysql_global_variables_binlog_stmt_cache_size", "bytes", "Cache for statement-based binary log entries per session."),
				mariaDBCfgSetting("mysql_global_variables_binlog_expire_logs_seconds", "seconds", "Binary log files older than this are auto-purged. Replaces expire_logs_days."),
				mariaDBCfgSetting("mysql_global_variables_binlog_space_limit", "bytes", "Max total storage for binary log files. 0 = no limit."),
				mariaDBCfgSetting("mysql_global_variables_sync_binlog", "number", "0 = OS flushes, 1 = fsync per commit (safe), N = fsync every N commits."),
				mariaDBCfgSetting("mysql_global_variables_binlog_commit_wait_count", "number", "Wait for N transactions before flushing binlog. Improves group commit throughput."),
				mariaDBCfgSetting("mysql_global_variables_binlog_commit_wait_usec", "microseconds", "Max wait time for binlog group commit. Set with binlog_commit_wait_count."),
				mariaDBCfgSetting("mysql_global_variables_binlog_annotate_row_events", "boolean", "Annotate row-based events with original SQL for readability."),
			},
		},
		{
			Key:   "replication",
			Title: "Replication",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_server_id", "number", "Unique server ID in a replication topology. Must be set for replication."),
				mariaDBCfgSetting("mysql_global_variables_log_slave_updates", "boolean", "Whether the slave writes replicated events to its own binary log. Needed for chaining."),
				mariaDBCfgSetting("mysql_global_variables_slave_net_timeout", "seconds", "Seconds before the slave considers the connection dead after no data."),
				mariaDBCfgSetting("mysql_global_variables_slave_parallel_threads", "number", "Number of parallel worker threads on the slave for applying events."),
				mariaDBCfgSetting("mysql_global_variables_slave_parallel_workers", "number", "Max parallel workers for slave SQL thread processing."),
				mariaDBCfgSetting("mysql_global_variables_slave_transaction_retries", "number", "Max retries on deadlock or lock timeout before giving up."),
				mariaDBCfgSetting("mysql_global_variables_slave_transaction_retry_interval", "milliseconds", "Delay between slave transaction retries."),
				mariaDBCfgSetting("mysql_global_variables_slave_compressed_protocol", "boolean", "Compress replication traffic. Saves bandwidth, costs CPU."),
				mariaDBCfgSetting("mysql_global_variables_sync_relay_log", "number", "0 = OS, 1 = fsync per event, N = per N events."),
				mariaDBCfgSetting("mysql_global_variables_sync_master_info", "number", "How often the slave syncs master info file. 0 = never."),
				mariaDBCfgSetting("mysql_global_variables_rpl_semi_sync_master_enabled", "boolean", "Enable semi-sync replication on the master."),
				mariaDBCfgSetting("mysql_global_variables_rpl_semi_sync_master_timeout", "milliseconds", "Timeout for semi-sync ACK from slave before reverting to async."),
				mariaDBCfgSetting("mysql_global_variables_rpl_semi_sync_slave_enabled", "boolean", "Enable semi-sync replication on the slave."),
			},
		},
		{
			Key:   "cache",
			Title: "Cache & Query Cache",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_query_cache_size", "bytes", "Query cache memory. 0 = disabled (recommended on MariaDB 10.6+/MySQL 8+)."),
				mariaDBCfgSetting("mysql_global_variables_query_cache_limit", "bytes", "Maximum result set size to cache in query cache."),
				mariaDBCfgSetting("mysql_global_variables_query_cache_strip_comments", "boolean", "Strip comments before caching query results. Improves cache hit rate."),
				mariaDBCfgSetting("mysql_global_variables_query_cache_wlock_invalidate", "boolean", "Invalidate query cache entries when a WRITE lock is taken on the table."),
				mariaDBCfgSetting("mysql_global_variables_table_open_cache", "number", "Max table descriptors to keep open. Reduces open/close overhead."),
				mariaDBCfgSetting("mysql_global_variables_table_definition_cache", "number", "Number of .frm/.ibd file definitions to cache."),
				mariaDBCfgSetting("mysql_global_variables_key_buffer_size", "bytes", "MyISAM key cache size. Important if MyISAM tables exist."),
				mariaDBCfgSetting("mysql_global_variables_key_cache_block_size", "number", "MyISAM key cache block size. Use with key cache."),
				mariaDBCfgSetting("mysql_global_variables_aria_pagecache_buffer_size", "bytes", "Aria page cache buffer size. Aria is the default MariaDB storage engine."),
			},
		},
		{
			Key:   "security",
			Title: "Security",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_local_infile", "boolean", "Permit LOAD DATA LOCAL INFILE. Can be a security risk; disable unless needed."),
				mariaDBCfgSetting("mysql_global_variables_skip_name_resolve", "boolean", "Don't resolve hostnames in connection authentication. Faster, and avoids DNS issues."),
				mariaDBCfgSetting("mysql_global_variables_skip_show_database", "boolean", "Users without SHOW DATABASES privilege only see their own databases."),
				mariaDBCfgSetting("mysql_global_variables_require_secure_transport", "boolean", "Reject non-TLS connections. Requires SSL configuration."),
				mariaDBCfgSetting("mysql_global_variables_strict_password_validation", "boolean", "Reject weak passwords."),
				mariaDBCfgSetting("mysql_global_variables_read_only", "boolean", "ON = no writes allowed except from replication or SUPER users. Use for read replicas."),
				mariaDBCfgSetting("mysql_global_variables_automatic_sp_privileges", "boolean", "Automatically grant EXECUTE and ALTER ROUTINE to the routine creator."),
				mariaDBCfgSetting("mysql_global_variables_old_passwords", "boolean", "Use old pre-4.1 password hashing. Should be OFF in modern deployments."),
				mariaDBCfgSetting("mysql_global_variables_disconnect_on_expired_password", "boolean", "Disconnect clients whose password has expired."),
			},
		},
		{
			Key:   "performance",
			Title: "Performance Schema & Misc",
			Settings: []MariaDBConfigSetting{
				mariaDBCfgSetting("mysql_global_variables_performance_schema", "boolean", "Enable Performance Schema. ON needed for query digest collection."),
				mariaDBCfgSetting("mysql_global_variables_event_scheduler", "boolean", "Enable the event scheduler for scheduled tasks."),
				mariaDBCfgSetting("mysql_global_variables_autocommit", "boolean", "Default autocommit mode for new connections. ON = each statement is its own transaction."),
				mariaDBCfgSetting("mysql_global_variables_lower_case_table_names", "number", "0 = case-sensitive, 1 = case-insensitive, 2 = case-insensitive on OS with mixed case."),
				mariaDBCfgSetting("mysql_global_variables_group_concat_max_len", "bytes", "Maximum result length of GROUP_CONCAT(). Increase for large concat results."),
				mariaDBCfgSetting("mysql_global_variables_max_sort_length", "number", "Bytes per column used for sorting BLOB/TEXT values."),
				mariaDBCfgSetting("mysql_global_variables_default_week_format", "number", "Default mode for WEEK() function. Range 0-7."),
				mariaDBCfgSetting("mysql_global_variables_delay_key_write", "boolean", "Delay MyISAM key writes until table close. ON = faster, less durable."),
				mariaDBCfgSetting("mysql_global_variables_flush_time", "seconds", "Close idle tables after this many seconds to free memory. 0 = never."),
				mariaDBCfgSetting("mysql_global_variables_foreign_key_checks", "boolean", "Enable foreign key constraint checking. OFF for bulk data loads."),
				mariaDBCfgSetting("mysql_global_variables_unique_checks", "boolean", "Enable unique constraint checking. OFF for bulk loads into InnoDB."),
				mariaDBCfgSetting("mysql_global_variables_sql_safe_updates", "boolean", "ON = UPDATE/DELETE require a WHERE with a key. Prevents accidental mass changes."),
			},
		},
	}
}

func NewMariaDBDashboardService(cfg *config.Config, redisClient *redis.Client) *MariaDBDashboardService {
	return &MariaDBDashboardService{
		base:  NewNginxDashboardService(cfg, redisClient),
		redis: redisClient,
	}
}

func (s *MariaDBDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
	minDurationSec float64,
) (*MariaDBDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)

	minDurSuffix := ""
	if minDurationSec > 0 {
		minDurSuffix = fmt.Sprintf(":minDur=%0.2f", minDurationSec)
	}
	cacheKey := fmt.Sprintf("mariadb:dash:%s:%s:%d:%d:%s%s", tenantUUID, serviceName, startSec, endSec, step, minDurSuffix)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result MariaDBDashboardSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	summary, err := s.fetchDashboard(ctx, tenantUUID, tenantID, serviceName, startNs, endNs, step, minDurationSec)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(summary); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, dashboardCacheTTL).Err()
		}
	}
	return summary, nil
}

func (s *MariaDBDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	serviceName string,
	startNs, endNs int64,
	step string,
	minDurationSec float64,
) (*MariaDBDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	rw := step
	if strings.TrimSpace(rw) == "" {
		rw = "30s"
	}

	type logR struct {
		entries []LokiLogEntry
		err     error
	}
	query := func(q string) vmQueryResult {
		data, err := s.base.vmQueryRange(ctx, tenantID, q, startSec, endSec, step)
		return vmQueryResult{data: data, err: err}
	}
	logQuery := func(q string, limit int) logR {
		entries, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, q, startNs, endNs, limit)
		return logR{entries: entries, err: err}
	}

	queries := map[string]string{
		"up":           "mysql_up",
		"scrape_error": `(1 - mysql_exporter_collector_success{collector="collect.global_status"}) or vector(0)`,
		"uptime":       "mysql_global_status_uptime",

		"max_connections":      "mysql_global_variables_max_connections",
		"max_used_connections": "mysql_global_status_max_used_connections",
		"threads_connected":    "mysql_global_status_threads_connected",
		"threads_running":      "mysql_global_status_threads_running",
		"threads_cached":       "mysql_global_status_threads_cached",
		"conn_saturation":      "100 * mysql_global_status_threads_connected / clamp_min(mysql_global_variables_max_connections, 1)",
		"aborted_connects":     "rate(mysql_global_status_aborted_connects[" + rw + "]) * 60",
		"aborted_clients":      "rate(mysql_global_status_aborted_clients[" + rw + "]) * 60",
		"threads_created":      "rate(mysql_global_status_threads_created[" + rw + "])",
		"connections":          "rate(mysql_global_status_connections[" + rw + "])",
		"select_scan":          "rate(mysql_global_status_select_scan[" + rw + "])",
		"opened_tables":        "rate(mysql_global_status_opened_tables[" + rw + "])",
		"rows_read":            "rate(mysql_global_status_rows_read[" + rw + "])",
		"rows_modified":        `sum(rate(mysql_global_status_handlers_total{handler=~"write|update|delete"}[` + rw + `])) or vector(0)`,
		"sort_rows":            "rate(mysql_global_status_sort_rows[" + rw + "])",
		"sort_merge_passes":    "rate(mysql_global_status_sort_merge_passes[" + rw + "])",
		"sort_range":           "rate(mysql_global_status_sort_range[" + rw + "])",
		"sort_scan":            "rate(mysql_global_status_sort_scan[" + rw + "])",

		"handler_read_rnd_next": "rate(mysql_global_status_handlers_total{handler=\"read_rnd_next\"}[" + rw + "])",
		"handler_read_key":      "rate(mysql_global_status_handlers_total{handler=\"read_key\"}[" + rw + "])",
		"handler_read_first":    "rate(mysql_global_status_handlers_total{handler=\"read_first\"}[" + rw + "])",
		"handler_read_next":     "rate(mysql_global_status_handlers_total{handler=\"read_next\"}[" + rw + "])",

		"qps":      "rate(mysql_global_status_questions[" + rw + "])",
		"slow_qpm": "rate(mysql_global_status_slow_queries[" + rw + "]) * 60",

		"selects": `sum(rate(mysql_global_status_commands_total{command=~"select|select_for_update"}[` + rw + `]))`,
		"inserts": `sum(rate(mysql_global_status_commands_total{command=~"insert|insert_select"}[` + rw + `]))`,
		"updates": `sum(rate(mysql_global_status_commands_total{command=~"update|update_multi"}[` + rw + `]))`,
		"deletes": `sum(rate(mysql_global_status_commands_total{command=~"delete|delete_multi"}[` + rw + `]))`,

		"bp_hit_ratio": `100 * (1 - (rate(mysql_global_status_innodb_buffer_pool_reads[` + rw + `]) / rate(mysql_global_status_innodb_buffer_pool_read_requests[` + rw + `]))) and on() (rate(mysql_global_status_innodb_buffer_pool_read_requests[` + rw + `]) > 0)`,
		"bp_reads":     "rate(mysql_global_status_innodb_buffer_pool_reads[" + rw + "])",
		"bp_requests":  "rate(mysql_global_status_innodb_buffer_pool_read_requests[" + rw + "])",
		"bp_size":      "mysql_global_variables_innodb_buffer_pool_size",
		"bp_dirty_pct": `100 * sum(mysql_global_status_innodb_buffer_pool_bytes_dirty) / clamp_min(sum(mysql_global_variables_innodb_buffer_pool_size), 1)`,
		"bp_used_pct":  `100 * (1 - (sum(mysql_global_status_buffer_pool_pages{state="free"}) / clamp_min(sum(mysql_global_status_buffer_pool_pages{state=~"data|free|misc"}), 1)))`,
		"bp_wait_free": "rate(mysql_global_status_innodb_buffer_pool_wait_free[" + rw + "])",

		"innodb_reads":        "rate(mysql_global_status_innodb_data_read[" + rw + "])",
		"innodb_writes":       "rate(mysql_global_status_innodb_data_written[" + rw + "])",
		"innodb_fsyncs":       "rate(mysql_global_status_innodb_data_fsyncs[" + rw + "])",
		"innodb_log_writes":   "rate(mysql_global_status_innodb_log_writes[" + rw + "])",
		"innodb_log_waits":    "rate(mysql_global_status_innodb_log_waits[" + rw + "])",
		"innodb_pending_reads":  "mysql_global_status_innodb_data_pending_reads",
		"innodb_pending_writes": "mysql_global_status_innodb_data_pending_writes",
		"innodb_pending_fsyncs": "mysql_global_status_innodb_data_pending_fsyncs",
		"innodb_history_list":   "mysql_global_status_innodb_history_list_length",

		"locks_waited":          "rate(mysql_global_status_table_locks_waited[" + rw + "]) * 60",
		"innodb_deadlocks_pm":   "rate(mysql_global_status_innodb_deadlocks[" + rw + "]) * 60",
		"row_lock_waits_pm":     "rate(mysql_global_status_innodb_row_lock_waits[" + rw + "]) * 60",
		"row_lock_time_avg_ms":  "mysql_global_status_innodb_row_lock_time_avg",

		"tmp_tables":      "sum(rate(mysql_global_status_created_tmp_tables[" + rw + "]))",
		"tmp_disk_tables": "sum(rate(mysql_global_status_created_tmp_disk_tables[" + rw + "]))",

		"net_rx": "sum(rate(mysql_global_status_bytes_received[" + rw + "]))",
		"net_tx": "sum(rate(mysql_global_status_bytes_sent[" + rw + "]))",

		"repl_lag":      "mysql_slave_lag_seconds or mysql_slave_status_seconds_behind_master",
		"slave_running": "mysql_global_status_slave_running",

		"qcache_hits":     "rate(mysql_global_status_qcache_hits[" + rw + "])",
		"qcache_inserts":  "rate(mysql_global_status_qcache_inserts[" + rw + "])",
		"qcache_free_mem": "mysql_global_status_qcache_free_memory",

		"open_tables":          "mysql_global_status_open_tables",
		"table_cache_hits":     "rate(mysql_global_status_table_open_cache_hits[" + rw + "])",
		"table_cache_misses":   "rate(mysql_global_status_table_open_cache_misses[" + rw + "])",

		"query_total_time": "sum(mariadb_query_digest_time_seconds_total)",
		"query_exec_count": "sum(mariadb_query_digest_exec_count_total)",
		"digest_total":     "mariadb_query_digest_time_seconds_total",
		"digest_exec":      "mariadb_query_digest_exec_count_total",
		"digest_avg":       "mariadb_query_digest_avg_time_seconds",
		"digest_max":       "mariadb_query_digest_max_time_seconds",
		"digest_rows_exam": "mariadb_query_digest_rows_examined_total",
		"digest_rows_sent": "mariadb_query_digest_rows_sent_total",
		"digest_no_index":  "mariadb_query_digest_no_index_used_count_total",
		"digest_tmp":       "mariadb_query_digest_tmp_tables_total",
		"digest_tmp_disk":  "mariadb_query_digest_tmp_disk_tables_total",

		"binlog_size":     "mysql_binlog_size_bytes",
		"binlog_files":    "mysql_binlog_files",
		"conn_errors":     "rate(mysql_global_status_connection_errors_total[" + rw + "])",
		"table_sizes":     "mysql_info_schema_table_size",
		"wsrep_cluster_size": "mysql_global_status_wsrep_cluster_size",
		"wsrep_connected": "mysql_global_status_wsrep_connected",
		"wsrep_ready":     "mysql_global_status_wsrep_ready",

		// Per-DB metrics from digest data
		"db_qps":        `sum by (db) (rate(mariadb_query_digest_exec_count_total[` + rw + `]))`,
		"db_total_time": `sum by (db) (mariadb_query_digest_time_seconds_total)`,
		"db_rows_exam":  `sum by (db) (mariadb_query_digest_rows_examined_total)`,
		"db_no_index":   `sum by (db) (mariadb_query_digest_no_index_used_count_total)`,
		"db_size":       `sum by (schema) (mysql_info_schema_table_size)`,
	}

	results := make(map[string]vmQueryResult)
	logResults := make(map[string]logR)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, q := range queries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := query(q)
			mu.Lock()
			results[name] = r
			mu.Unlock()
		}()
	}

	baseSelector := mariaDBLokiSelector(serviceName)
	logQueries := buildMariaDBLogQueries(baseSelector)
	for name, q := range logQueries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := logQuery(q, 20)
			mu.Lock()
			logResults[name] = r
			mu.Unlock()
		}()
	}

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	val := func(k string) *float64 { return latestVMValue(results[k].data) }

	qcHit := latestVMValue(results["qcache_hits"].data)
	qcIns := latestVMValue(results["qcache_inserts"].data)
	var qcHitRatio *float64
	if qcHit != nil && qcIns != nil {
		total := *qcHit + *qcIns
		if total > 0 {
			r := *qcHit / total * 100
			qcHitRatio = &r
		}
	}

	tcHit := latestVMValue(results["table_cache_hits"].data)
	tcMiss := latestVMValue(results["table_cache_misses"].data)
	var tcHitRatio *float64
	if tcHit != nil && tcMiss != nil {
		total := *tcHit + *tcMiss
		if total > 0 {
			r := *tcHit / total * 100
			tcHitRatio = &r
		}
	}

	handlerReadRndNext := val("handler_read_rnd_next")
	handlerReadKey := val("handler_read_key")
	var tableScanRatio *float64
	if handlerReadRndNext != nil && handlerReadKey != nil && *handlerReadKey > 0 {
		r := *handlerReadRndNext / (*handlerReadRndNext + *handlerReadKey) * 100
		tableScanRatio = &r
	}

	summary := &MariaDBDashboardSummary{
		Up:          val("up"),
		ScrapeError: val("scrape_error"),
		UptimeSeconds: val("uptime"),

		MaxConnections:          val("max_connections"),
		MaxUsedConnections:      val("max_used_connections"),
		ThreadsConnected:        val("threads_connected"),
		ThreadsRunning:          val("threads_running"),
		ThreadsCached:           val("threads_cached"),
		ConnectionSaturationPct: val("conn_saturation"),
		AbortedConnectsPerMin:   val("aborted_connects"),
		AbortedClientsPerMin:    val("aborted_clients"),
		ThreadsCreatedPerSec:    val("threads_created"),
		ConnectionsPerSec:       val("connections"),
		SelectScanPerSec:        val("select_scan"),
		OpenedTablesPerSec:      val("opened_tables"),
		RowsReadPerSec:          val("rows_read"),
		RowsModifiedPerSec:      val("rows_modified"),
		SortRowsPerSec:          val("sort_rows"),
		SortMergePassesPerSec:   val("sort_merge_passes"),

		QPS:               val("qps"),
		SlowQueriesPerMin: val("slow_qpm"),
		SelectsPerSec:     val("selects"),
		InsertsPerSec:     val("inserts"),
		UpdatesPerSec:     val("updates"),
		DeletesPerSec:     val("deletes"),

		BufferPoolHitRatioPct:      val("bp_hit_ratio"),
		InnodbBufferPoolSizeBytes:  val("bp_size"),
		BufferPoolDirtyPct:         val("bp_dirty_pct"),
		BufferPoolUsedPct:          val("bp_used_pct"),
		BufferPoolWaitFreePerSec:   val("bp_wait_free"),

		InnodbDataReadsPerSec:     val("innodb_reads"),
		InnodbDataWritesPerSec:    val("innodb_writes"),
		InnodbFsyncsPerSec:        val("innodb_fsyncs"),
		InnodbLogWritesPerSec:     val("innodb_log_writes"),
		InnodbLogWaitsPerSec:      val("innodb_log_waits"),
		InnodbPendingReads:        val("innodb_pending_reads"),
		InnodbPendingWrites:       val("innodb_pending_writes"),
		InnodbPendingFsyncs:       val("innodb_pending_fsyncs"),
		InnodbHistoryListLength:   val("innodb_history_list"),

		TableLocksWaitedPerMin:   val("locks_waited"),
		InnodbDeadlocksPerMin:    val("innodb_deadlocks_pm"),
		InnodbRowLockWaitsPerMin: val("row_lock_waits_pm"),
		InnodbRowLockTimeAvgMs:   val("row_lock_time_avg_ms"),

		TmpTablesPerSec:     val("tmp_tables"),
		TmpDiskTablesPerSec: val("tmp_disk_tables"),

		NetworkRxBytesPerSec: val("net_rx"),
		NetworkTxBytesPerSec: val("net_tx"),

		ReplicationLagSeconds: val("repl_lag"),
		SlaveRunning:          val("slave_running"),

		QcacheHitRatioPct:  qcHitRatio,
		QcacheFreeMemBytes: val("qcache_free_mem"),

		OpenTables:            val("open_tables"),
		TableCacheHitRatioPct: tcHitRatio,

		QueryDigestTotalTimeSeconds: val("query_total_time"),
		TableScanRatioPct:           tableScanRatio,
		HandlerReadKeyPerSec:        handlerReadKey,
		HandlerReadRndNextPerSec:    handlerReadRndNext,
		BinlogSizeBytes:             val("binlog_size"),
		BinlogFiles:                 val("binlog_files"),
		WsrepClusterSize:            val("wsrep_cluster_size"),
		WsrepConnected:              val("wsrep_connected"),
		WsrepReady:                  val("wsrep_ready"),
	}

	summary.ConnectionsTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"connected": results["threads_connected"].data,
		"running":   results["threads_running"].data,
		"cached":    results["threads_cached"].data,
	})
	summary.QPSTimeseries = singleVMSeries(results["qps"].data, "qps")
	summary.SlowTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"slow_queries_per_min":     results["slow_qpm"].data,
		"aborted_connects_per_min": results["aborted_connects"].data,
		"aborted_clients_per_min":  results["aborted_clients"].data,
	})
	summary.BufferPoolTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"hit_ratio": results["bp_hit_ratio"].data,
		"dirty_pct": results["bp_dirty_pct"].data,
		"used_pct":  results["bp_used_pct"].data,
	})
	summary.LocksTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"table_locks_waited_pm": results["locks_waited"].data,
		"innodb_deadlocks_pm":   results["innodb_deadlocks_pm"].data,
		"row_lock_waits_pm":     results["row_lock_waits_pm"].data,
	})
	summary.QueryTimeTimeseries = singleVMSeries(results["query_total_time"].data, "total_seconds")
	summary.QueryCountTimeseries = singleVMSeries(results["query_exec_count"].data, "exec_count")

	summary.IOTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"reads_bps":  results["innodb_reads"].data,
		"writes_bps": results["innodb_writes"].data,
		"log_writes": results["innodb_log_writes"].data,
	})
	summary.NetworkTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"rx_bps": results["net_rx"].data,
		"tx_bps": results["net_tx"].data,
	})
	summary.TmpTablesTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"memory_per_sec": results["tmp_tables"].data,
		"disk_per_sec":   results["tmp_disk_tables"].data,
	})
	summary.RWTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"selects": results["selects"].data,
		"inserts": results["inserts"].data,
		"updates": results["updates"].data,
		"deletes": results["deletes"].data,
	})
	summary.DeadlocksTimeseries = singleVMSeries(results["innodb_deadlocks_pm"].data, "deadlocks_pm")

	summary.HandlerReadTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"read_rnd_next": results["handler_read_rnd_next"].data,
		"read_key":      results["handler_read_key"].data,
		"read_first":    results["handler_read_first"].data,
	})
	summary.SortTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"merge_passes": results["sort_merge_passes"].data,
		"range":        results["sort_range"].data,
		"scan":         results["sort_scan"].data,
	})
	summary.InnodbPendingTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"pending_reads":  results["innodb_pending_reads"].data,
		"pending_writes": results["innodb_pending_writes"].data,
		"pending_fsyncs": results["innodb_pending_fsyncs"].data,
	})
	summary.BinlogTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"size_bytes": results["binlog_size"].data,
	})
	summary.TopTableSizes = latestLabelCounts(results["table_sizes"].data, "schema", 10)
	summary.ConnectionErrorsByType = latestLabelCounts(results["conn_errors"].data, "error", 10)

	summary.TopDBsByQueries = latestLabelCounts(results["db_qps"].data, "db", 10)
	summary.TopDBsByQueryTime = latestLabelCounts(results["db_total_time"].data, "db", 10)
	summary.TopDBsByRowsExamined = latestLabelCounts(results["db_rows_exam"].data, "db", 10)
	summary.TopDBsByNoIndex = latestLabelCounts(results["db_no_index"].data, "db", 10)
	summary.DBStorageSizes = latestLabelCounts(results["db_size"].data, "schema", 10)

	slowDBEntries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, mariaDBLokiSelector(serviceName)+` |~ "(?i)(Query_time|slow query)"`, startNs, endNs)
	if err == nil {
		parsedSlow := parseSlowQueryEntries(slowDBEntries)
		dbCounts := make(map[string]float64)
		for _, p := range parsedSlow {
			db := p.DB
			if db == "" {
				db = "(unknown)"
			}
			dbCounts[db]++
		}
		summary.SlowQueriesByDB = topN(dbCounts, 10)
	}

	digests := mariaDBQueryDigests(results)
	summary.TopQueryDigests = topMariaDBDigests(digests, func(d MariaDBQueryDigest) float64 { return d.TotalTimeSeconds }, 20)
	summary.InefficientDigests = topMariaDBDigests(digests, func(d MariaDBQueryDigest) float64 { return d.RowsExaminedPerSent }, 20)
	summary.NoIndexDigests = topMariaDBDigests(digests, func(d MariaDBQueryDigest) float64 { return d.NoIndexUsedCount }, 20)

	summary.LogSummary = s.countParsedLogCategories(ctx, tenantUUID, serviceName, startNs, endNs, minDurationSec)
	summary.RecentLogs = mergeMariaDBLogEvents(20,
		logResults["errors"].entries,
		logResults["warnings"].entries,
		logResults["deadlocks"].entries,
		logResults["auth"].entries,
	)
	summary.SlowQueryLogs = mergeMariaDBLogEvents(20, logResults["slow"].entries)

	return summary, nil
}

func (s *MariaDBDashboardService) GetConfigSnapshot(
	ctx context.Context,
	tenantID uint,
) (*MariaDBConfigSnapshot, error) {
	now := time.Now()
	result := &MariaDBConfigSnapshot{
		GeneratedAt: now.Format(time.RFC3339),
		Groups:      mariaDBConfigCatalog(),
	}

	for groupIndex := range result.Groups {
		for settingIndex := range result.Groups[groupIndex].Settings {
			setting := &result.Groups[groupIndex].Settings[settingIndex]
			series, err := s.base.vmQueryInstant(ctx, tenantID, setting.Name, now.Unix())
			if err != nil {
				continue
			}
			if value := latestVMValue(series); value != nil {
				setting.Value = value
			}
		}
	}
	return result, nil
}

func (s *MariaDBDashboardService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	category string,
	search string,
	page int,
	pageSize int,
	minDurationMs float64,
) (*MariaDBLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID,
		mariaDBLogsLokiQuery(mariaDBLokiSelector(serviceName), category),
		startNs, endNs)
	if err != nil {
		return nil, err
	}

	// stitchLogLines needs ascending order
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := strconv.ParseInt(entries[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(entries[j].TimestampNs, 10, 64)
		return ti < tj
	})
	merged := stitchLogLines(entries)
	sortLokiLogEntriesDesc(merged)

	events := make([]MariaDBLogEvent, 0, len(merged))
	for _, entry := range merged {
		events = append(events, parseMariaDBLogEvent(entry))
	}
	if category != "" && category != "all" {
		filtered := events[:0]
		for _, ev := range events {
			if ev.Category == category {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}
	if minDurationMs > 0 {
		filtered := events[:0]
		for _, ev := range events {
			if ev.DurationMs != nil && *ev.DurationMs >= minDurationMs {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}
	if needle := strings.ToLower(strings.TrimSpace(search)); needle != "" {
		filtered := events[:0]
		for _, ev := range events {
			if mariaDBLogMatchesSearch(ev, needle) {
				filtered = append(filtered, ev)
			}
		}
		events = filtered
	}
	total := len(events)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &MariaDBLogsPage{Events: []MariaDBLogEvent{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return &MariaDBLogsPage{Events: events[offset:end], Total: total, Page: page, PageSize: pageSize, HasMore: end < total}, nil
}

func (s *MariaDBDashboardService) GetSlowQueryLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	page, pageSize int,
	minDurationMs float64,
) (*SlowQueryLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	sel := mariaDBLokiSelector(serviceName) + ` |~ "(?i)(Query_time|slow query)"`
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, sel, startNs, endNs)
	if err != nil {
		return nil, err
	}

	// stitchSlowQueryLines needs ascending order
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := strconv.ParseInt(entries[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(entries[j].TimestampNs, 10, 64)
		return ti < tj
	})

	parsed := parseSlowQueryEntries(entries)

	// Filter by minimum duration
	if minDurationMs > 0 {
		filtered := parsed[:0]
		for _, p := range parsed {
			if p.QueryTimeS*1000 >= minDurationMs {
				filtered = append(filtered, p)
			}
		}
		parsed = filtered
	}

	sort.Slice(parsed, func(i, j int) bool { return parsed[i].Timestamp > parsed[j].Timestamp })

	total := len(parsed)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &SlowQueryLogsPage{Entries: []SlowQueryEntry{}, Total: total, Page: page, PageSize: pageSize}, nil
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	return &SlowQueryLogsPage{
		Entries:  parsed[offset:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

func (s *MariaDBDashboardService) GetErrorLogs(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	startNs, endNs int64,
	limit int,
) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	sel := mariaDBLokiSelector(serviceName) + ` | log_kind="error"`
	return s.base.lokiQueryLogLines(ctx, tenantUUID, sel, startNs, endNs, limit)
}

// ── Multi-line log stitching ─────────────────────────────────

var (
	slowTimeRE      = regexp.MustCompile(`(?i)#\s*Time:\s*(.+)`)
	slowUserRE      = regexp.MustCompile(`(?i)#\s*User@Host:\s*(\S+)\[.*?\]\s*@\s*(\S*)`)
	slowThreadRE    = regexp.MustCompile(`(?i)#\s*Thread_id:\s*(\d+)`)
	slowQueryTimeRE = regexp.MustCompile(`(?i)Query_time:\s*([0-9.]+)\s+Lock_time:\s*([0-9.]+)\s+Rows_sent:\s*(\d+)\s+Rows_examined:\s*(\d+)`)
	slowSchemaRE    = regexp.MustCompile(`(?i)use\s+(\S+)\s*;`)
	slowSetTsRE     = regexp.MustCompile(`(?i)SET\s+timestamp=\d+;`)
)

type logGroup struct {
	ts    string
	lines []string
	meta  map[string]string
}

func stitchLogLines(entries []LokiLogEntry) []LokiLogEntry {
	out := make([]LokiLogEntry, 0, len(entries))
	var cur *logGroup

	flush := func() {
		if cur == nil {
			return
		}
		out = append(out, LokiLogEntry{
			TimestampNs: cur.ts,
			Line:        strings.Join(cur.lines, "\n"),
			Metadata:    cur.meta,
		})
		cur = nil
	}

	for _, e := range entries {
		line := strings.TrimSpace(e.Line)
		isTimeHeader := strings.HasPrefix(line, "# Time:")
		isUserHeader := strings.HasPrefix(line, "# User@Host:")

		if isTimeHeader || isUserHeader {
			if cur != nil && isTimeHeader {
				flush()
			}
			if cur == nil {
				cur = &logGroup{ts: e.TimestampNs, meta: e.Metadata}
			}
			cur.lines = append(cur.lines, line)
			continue
		}

		if cur != nil {
			cur.lines = append(cur.lines, line)
			if strings.HasSuffix(line, ";") && !strings.HasPrefix(line, "#") && !slowSetTsRE.MatchString(line) {
				flush()
			}
			continue
		}

		// General stitching: if this line doesn't end with ';' and isn't a
		// complete log entry (no ISO/classic timestamp prefix), it may be a
		// SQL fragment belonging to a multi-line statement — group it.
		hasTerminator := strings.HasSuffix(line, ";")
		isLogLine := mariaDBISOLogRE.MatchString(line) || mariaDBClassicLogRE.MatchString(line)

		if hasTerminator || isLogLine || slowSetTsRE.MatchString(line) || line == "" {
			flush()
			out = append(out, e)
			continue
		}

		cur = &logGroup{ts: e.TimestampNs, meta: e.Metadata}
		cur.lines = append(cur.lines, line)
	}
	flush()
	return out
}

func parseSlowQueryEntries(entries []LokiLogEntry) []SlowQueryEntry {
	stitched := stitchLogLines(entries)
	var result []SlowQueryEntry
	for _, e := range stitched {
		lines := strings.Split(e.Line, "\n")
		if len(lines) < 2 {
			continue
		}
		sqe := SlowQueryEntry{Lines: lines}

		if ts, err := strconv.ParseInt(e.TimestampNs, 10, 64); err == nil {
			sqe.Timestamp = time.Unix(0, ts).UTC().Format(time.RFC3339Nano)
		}
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if m := slowTimeRE.FindStringSubmatch(l); len(m) == 2 {
				sqe.Timestamp = strings.TrimSpace(m[1])
			}
			if m := slowUserRE.FindStringSubmatch(l); len(m) >= 3 {
				sqe.User = strings.TrimSpace(m[1])
				sqe.Host = strings.TrimSpace(m[2])
			}
			if m := slowThreadRE.FindStringSubmatch(l); len(m) == 2 {
				sqe.ThreadID = m[1]
			}
			if m := slowQueryTimeRE.FindStringSubmatch(l); len(m) == 5 {
				sqe.QueryTimeS, _ = strconv.ParseFloat(m[1], 64)
				sqe.LockTimeS, _ = strconv.ParseFloat(m[2], 64)
				sqe.RowsSent, _ = strconv.ParseInt(m[3], 10, 64)
				sqe.RowsExamined, _ = strconv.ParseInt(m[4], 10, 64)
			}
			if m := slowSchemaRE.FindStringSubmatch(l); len(m) == 2 {
				sqe.DB = m[1]
			}
		}

		var sqlLines []string
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "#") || slowSetTsRE.MatchString(t) || t == "" {
				continue
			}
			sqlLines = append(sqlLines, t)
		}
		sqe.SQL = strings.TrimSpace(strings.Join(sqlLines, " "))

		if sqe.SQL != "" || sqe.QueryTimeS > 0 {
			result = append(result, sqe)
		}
	}
	return result
}

// ── Loki helpers ────────────────────────────────────────────────────────

func (s *MariaDBDashboardService) lokiQueryAllLogEntries(ctx context.Context, tenantUUID, query string, startNs, endNs int64) ([]LokiLogEntry, error) {
	const batchSize = 2000
	var all []LokiLogEntry
	seen := make(map[string]struct{}, 4096)
	cursorEnd := endNs
	for {
		batch, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, query, startNs, cursorEnd, batchSize)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		minTs := cursorEnd
		for _, e := range batch {
			key := e.TimestampNs + "\x00" + e.Line
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, e)
			if ts, err := strconv.ParseInt(e.TimestampNs, 10, 64); err == nil && ts < minTs {
				minTs = ts
			}
		}
		if minTs <= startNs || minTs >= cursorEnd || len(batch) < batchSize {
			break
		}
		cursorEnd = minTs - 1
	}
	sortLokiLogEntriesDesc(all)
	return all, nil
}

func (s *MariaDBDashboardService) countParsedLogCategories(ctx context.Context, tenantUUID, serviceName string, startNs, endNs int64, minDurationSec float64) MariaDBLogSummary {
	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, mariaDBLokiSelector(serviceName), startNs, endNs)
	if err != nil {
		return MariaDBLogSummary{}
	}
	var summary MariaDBLogSummary
	minDurMs := minDurationSec * 1000
	for _, entry := range entries {
		ev := parseMariaDBLogEvent(entry)
		switch ev.Category {
		case "slow query":
			if minDurMs > 0 && (ev.DurationMs == nil || *ev.DurationMs < minDurMs) {
				continue
			}
			summary.SlowQueries++
		case "error":
			summary.Errors++
		case "warning":
			summary.Warnings++
		case "auth failure":
			summary.AuthFailures++
		case "deadlock":
			summary.Deadlocks++
		case "replication":
			summary.Replication++
		case "startup":
			summary.Startup++
		case "shutdown":
			summary.Shutdown++
		}
	}
	return summary
}

// ── Loki selector / query builders ──────────────────────────────────────

func mariaDBLokiSelector(serviceName string) string {
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return `{service_name=~"mysql.*|mariadb.*|mysqld.*"}`
	}
	return fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
}

func mariaDBLogsLokiQuery(baseSelector, category string) string {
	return baseSelector
}

func buildMariaDBLogQueries(baseSelector string) map[string]string {
	return map[string]string{
		"slow":      baseSelector + ` |~ "(?i)(Query_time|slow query)"`,
		"errors":    baseSelector + ` |~ "(?i)(\\[ERROR\\]| ERROR |error:|crash|failed)"`,
		"warnings":  baseSelector + ` |~ "(?i)(\\[Warning\\]| warning | log_warnings)"`,
		"deadlocks": baseSelector + ` |~ "(?i)(deadlock|lock wait timeout)"`,
		"auth":      baseSelector + ` |~ "(?i)(access denied|authentication|denied for user)"`,
	}
}

func mergeMariaDBLogEvents(limit int, groups ...[]LokiLogEntry) []MariaDBLogEvent {
	seen := map[string]struct{}{}
	entries := []LokiLogEntry{}
	for _, group := range groups {
		for _, entry := range group {
			key := entry.TimestampNs + "\x00" + entry.Line
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			entries = append(entries, entry)
		}
	}
	sortLokiLogEntriesDesc(entries)
	if len(entries) > limit {
		entries = entries[:limit]
	}
	events := make([]MariaDBLogEvent, 0, len(entries))
	for _, entry := range entries {
		events = append(events, parseMariaDBLogEvent(entry))
	}
	return events
}

// ── Log parsing ─────────────────────────────────────────────────────────

func parseMariaDBLogEvent(entry LokiLogEntry) MariaDBLogEvent {
	event := MariaDBLogEvent{
		Timestamp: entry.TimestampNs,
		Severity:  "INFO",
		Category:  "event",
		Message:   strings.TrimSpace(entry.Line),
		Labels:    map[string]string{},
	}
	for key, value := range entry.Metadata {
		if strings.TrimSpace(value) != "" {
			event.Labels[key] = value
		}
	}
	if ts, err := strconv.ParseInt(entry.TimestampNs, 10, 64); err == nil {
		event.Timestamp = time.Unix(0, ts).Format(time.RFC3339)
	}
	line := strings.TrimSpace(entry.Line)
	if matches := mariaDBISOLogRE.FindStringSubmatch(line); len(matches) == 6 {
		event.Timestamp = matches[1]
		event.ThreadID = matches[2]
		event.Severity = normalizeMariaDBSeverity(matches[3])
		event.Subsystem = matches[4]
		event.Message = strings.TrimSpace(matches[5])
	} else if matches := mariaDBClassicLogRE.FindStringSubmatch(line); len(matches) == 4 {
		event.Timestamp = matches[1]
		event.Severity = normalizeMariaDBSeverity(matches[2])
		event.Message = strings.TrimSpace(matches[3])
	}
	if matches := mariaDBSlowUserRE.FindStringSubmatch(line); len(matches) == 3 {
		event.User = matches[1]
		event.Host = strings.TrimSpace(matches[2])
	}
	if matches := mariaDBQueryTimeRE.FindStringSubmatch(line); len(matches) == 2 {
		if value, err := strconv.ParseFloat(matches[1], 64); err == nil {
			ms := value * 1000
			event.DurationMs = &ms
		}
	}
	event.Category = classifyMariaDBLog(event.Message, line, event.Severity)
	if len(event.Labels) == 0 {
		event.Labels = nil
	}
	return event
}

func classifyMariaDBLog(message, raw, severity string) string {
	lower := strings.ToLower(message + " " + raw)
	switch {
	case strings.Contains(lower, "query_time:") || strings.Contains(lower, "slow query"):
		return "slow query"
	case strings.Contains(lower, "access denied") || strings.Contains(lower, "denied for user") || strings.Contains(lower, "authentication"):
		return "auth failure"
	case strings.Contains(lower, "deadlock") || strings.Contains(lower, "lock wait timeout"):
		return "deadlock"
	case strings.Contains(lower, "replication") || strings.Contains(lower, "slave") || strings.Contains(lower, "relay log") || strings.Contains(lower, "binlog"):
		return "replication"
	case strings.Contains(lower, "ready for connections") || strings.Contains(lower, "starting mariadb") || strings.Contains(lower, "starting mysql"):
		return "startup"
	case strings.Contains(lower, "shutdown") || strings.Contains(lower, "normal shutdown"):
		return "shutdown"
	case severity == "ERROR" || severity == "FATAL":
		return "error"
	case severity == "WARNING":
		return "warning"
	default:
		return "event"
	}
}

func normalizeMariaDBSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "error":
		return "ERROR"
	case "warning":
		return "WARNING"
	case "system", "note":
		return "INFO"
	default:
		return strings.ToUpper(strings.TrimSpace(value))
	}
}

func mariaDBLogMatchesSearch(ev MariaDBLogEvent, needle string) bool {
	for _, field := range []string{ev.Timestamp, ev.Severity, ev.Category, ev.User, ev.Host, ev.ThreadID, ev.Subsystem, ev.Message} {
		if strings.Contains(strings.ToLower(field), needle) {
			return true
		}
	}
	return false
}

func mariaDBQueryDigests(results map[string]vmQueryResult) []MariaDBQueryDigest {
	latest := func(name string) map[string]float64 {
		out := make(map[string]float64)
		for _, series := range results[name].data {
			if len(series.Values) == 0 {
				continue
			}
			val := series.Values[len(series.Values)-1][1]
			key := series.Metric["digest"]
			if key == "" {
				key = series.Metric["digest_text"]
			}
			if key == "" {
				continue
			}
			out[key] = val
		}
		return out
	}

	totalTimes := latest("digest_total")
	execCounts := latest("digest_exec")
	avgTimes := latest("digest_avg")
	maxTimes := latest("digest_max")
	rowsExam := latest("digest_rows_exam")
	rowsSent := latest("digest_rows_sent")
	noIndex := latest("digest_no_index")
	tmpTables := latest("digest_tmp")
	tmpDisk := latest("digest_tmp_disk")

	digestTotal := results["digest_total"].data
	if len(digestTotal) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var out []MariaDBQueryDigest
	for _, series := range digestTotal {
		if len(series.Values) == 0 {
			continue
		}
		key := series.Metric["digest"]
		if key == "" {
			key = series.Metric["digest_text"]
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true

		exam := rowsSent[key]
		perSent := 0.0
		if exam > 0 {
			perSent = rowsExam[key] / exam
		}

		out = append(out, MariaDBQueryDigest{
			Digest:              key,
			DigestText:          series.Metric["digest_text"],
			Database:            series.Metric["db"],
			TotalTimeSeconds:    totalTimes[key],
			ExecCount:           execCounts[key],
			AvgTimeSeconds:      avgTimes[key],
			MaxTimeSeconds:      maxTimes[key],
			RowsExamined:        rowsExam[key],
			RowsSent:            rowsSent[key],
			NoIndexUsedCount:    noIndex[key],
			TempTables:          tmpTables[key],
			TempDiskTables:      tmpDisk[key],
			RowsExaminedPerSent: perSent,
		})
	}
	return out
}

func topMariaDBDigests(items []MariaDBQueryDigest, score func(MariaDBQueryDigest) float64, limit int) []MariaDBQueryDigest {
	out := append([]MariaDBQueryDigest(nil), items...)
	sort.Slice(out, func(i, j int) bool { return score(out[i]) > score(out[j]) })
	filtered := out[:0]
	for _, item := range out {
		if score(item) <= 0 {
			continue
		}
		filtered = append(filtered, item)
	}
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

var (
	mariaDBISOLogRE     = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}T[^\s]+)\s+(\d+)\s+\[(System|Warning|ERROR|Note)\](?:\s+\[[^\]]+\])?\s+\[([^\]]+)\]\s+(.*)$`)
	mariaDBClassicLogRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}\s+[0-9:.]+)\s+\d*\s*\[(ERROR|Warning|Note|System)\]\s+(.*)$`)
	mariaDBSlowUserRE   = regexp.MustCompile(`(?i)#\s*User@Host:\s*([^\[]+)\[[^\]]*\]\s*@\s*(.*)$`)
	mariaDBQueryTimeRE  = regexp.MustCompile(`(?i)Query_time:\s*([0-9.]+)`)
)
