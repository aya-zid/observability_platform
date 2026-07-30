package domain

type AlertTemplate struct {
	ID                 string
	ServiceType        string
	Name               string
	Description        string
	Explanation        string
	Unit               string
	DefaultThreshold   float64
	ComparisonOp       string
	DefaultSeverity    string
	DefaultForDuration string
}

var AlertTemplates = []AlertTemplate{
	// ── Host templates ──────────────────────────────────────────
	{
		ID: "host_cpu", ServiceType: "host", Name: "High CPU usage", DefaultThreshold: 90, Unit: "%",
		Description: "Percentage of total CPU capacity being used — fires when sustained CPU usage exceeds the threshold.",
		Explanation: "High CPU means processes are competing for compute time. Common causes: traffic spikes, runaway processes, inefficient code (e.g. unoptimized loops, missing caching), or a DDoS attack. If sustained, services will slow down and request queues will grow.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_memory", ServiceType: "host", Name: "High memory usage", DefaultThreshold: 85, Unit: "%",
		Description: "Percentage of physical RAM in use — fires when memory consumption exceeds the threshold.",
		Explanation: "High memory usage forces the kernel to swap, which dramatically slows down every process on the host. Common causes: memory leaks in applications, too many concurrent workers, or insufficient RAM for the workload. If it reaches 100%, the OOM killer may terminate processes.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_disk", ServiceType: "host", Name: "Disk space critical", DefaultThreshold: 90, Unit: "%",
		Description: "Percentage of disk capacity used on any partition — fires when usage exceeds the threshold.",
		Explanation: "Running out of disk space causes write-heavy services to fail silently: databases can't persist transactions, log collectors drop lines, and applications throw 'no space left on device' errors. Common causes: unrotated logs, database bloat, temporary build artifacts, or insufficient allocation.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_swap", ServiceType: "host", Name: "High swap usage", DefaultThreshold: 50, Unit: "%",
		Description: "Percentage of swap space in use — fires when swap consumption exceeds the threshold.",
		Explanation: "Significant swap usage means the system is overcommitted on memory. Since swap lives on disk (thousands of times slower than RAM), any swapped-out memory pages cause severe performance degradation when accessed. Common causes: insufficient RAM, memory leak, or a workload that temporarily spikes above physical capacity.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_load1", ServiceType: "host", Name: "High CPU load average (1m)", DefaultThreshold: 4, Unit: "load",
		Description: "1-minute CPU load average — fires when the run-queue length exceeds the threshold.",
		Explanation: "Load average represents the number of processes waiting for CPU time. A load above the CPU core count means processes are queued. Common causes: CPU-bound workload spike, runaway process, or insufficient CPU cores for the current traffic. High load causes all services on the host to feel slow.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_disk_io_latency", ServiceType: "host", Name: "High disk I/O latency", DefaultThreshold: 100, Unit: "ms",
		Description: "Average time for disk I/O operations to complete — fires when latency exceeds the threshold.",
		Explanation: "High disk latency means processes are spending significant time waiting for data to be read from or written to disk. This directly impacts database query performance and any file I/O. Common causes: disk saturation from heavy read/write loads, a failing disk, or an overloaded shared storage backend.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_conntrack", ServiceType: "host", Name: "Connection tracking saturation", DefaultThreshold: 80, Unit: "%",
		Description: "Percentage of the connection tracking table in use — fires when conntrack utilization exceeds threshold.",
		Explanation: "The connection tracking table tracks all active network connections. When it fills up, new connections are dropped, causing intermittent connectivity failures. Common causes: high connection count from many short-lived connections (e.g. HTTP/1.0 without keepalive), port scanning, or DDoS attacks.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_network_errors", ServiceType: "host", Name: "Network interface errors", DefaultThreshold: 1, Unit: "errors/s",
		Description: "Rate of network interface errors (dropped packets, overruns, carrier errors) — fires when at least 1 error/s is detected (threshold 1 means any error rate triggers the alert).",
		Explanation: "Network errors indicate physical or configuration problems at the NIC or switch level. Common causes: bad cabling, duplex mismatch, saturated link, faulty NIC hardware, or MTU misconfiguration. Even a small but sustained error rate can cause packet loss and retransmissions that degrade application performance.",
		DefaultSeverity: "warning", ComparisonOp: ">=", DefaultForDuration: "5m",
	},
	{
		ID: "host_ssh_brute_force", ServiceType: "host", Name: "SSH brute-force attack detected", DefaultThreshold: 10, Unit: "attempts/5m",
		Description: "Count of failed SSH authentication attempts in the evaluation window — fires when attempts exceed the threshold (default 10 means 10+ failed attempts).",
		Explanation: "Multiple failed SSH login attempts in a short period indicate an active brute-force or credential-stuffing attack against the SSH service. Attackers scan the internet for exposed SSH ports and try common username/password combinations. Immediate action: enable fail2ban, restrict SSH to trusted IPs, or disable password authentication.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_oom_kill", ServiceType: "host", Name: "OOM killer invoked", DefaultThreshold: 1, Unit: "events/5m",
		Description: "Count of Out-Of-Memory killer events in the evaluation window — fires when at least 1 OOM event is detected (threshold 1 means any occurrence triggers the alert).",
		Explanation: "The OOM killer is the kernel's last resort when the system runs completely out of memory. It forcibly terminates processes to free RAM. This means critical services may have been killed without a clean shutdown. Common causes: memory leak in an application, a process consuming more memory than configured limits allow, or insufficient total RAM.",
		DefaultSeverity: "critical", ComparisonOp: ">=", DefaultForDuration: "5m",
	},
	{
		ID: "host_service_crash_loop", ServiceType: "host", Name: "Service crash loop detected", DefaultThreshold: 3, Unit: "events/5m",
		Description: "Count of systemd service failures or crashes in the evaluation window — fires when services are repeatedly restarting.",
		Explanation: "Services that crash and restart repeatedly indicate a serious stability problem. Each crash means the service was unavailable for some period. Common causes: application bugs (null pointer, unhandled exception), resource limits (memory, file descriptors), misconfigured unit files, or dependency failures (e.g. database unreachable at startup).",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_disk_error", ServiceType: "host", Name: "Disk I/O errors detected", DefaultThreshold: 1, Unit: "events/5m",
		Description: "Count of disk I/O errors from kernel logs in the evaluation window — fires when at least 1 error is detected (threshold 1 means any disk error triggers the alert).",
		Explanation: "Disk I/O errors are serious — they indicate the kernel is unable to read from or write to a storage device. This can corrupt data, crash databases, and cause silent data loss. Common causes: failing hard drive (bad sectors), loose cables, filesystem corruption, or an overloaded storage controller. Immediate investigation and hardware replacement may be needed.",
		DefaultSeverity: "critical", ComparisonOp: ">=", DefaultForDuration: "5m",
	},

	// ── Nginx templates ─────────────────────────────────────────
	{
		ID: "nginx_conn_saturation", ServiceType: "nginx", Name: "Connection saturation", DefaultThreshold: 2000, Unit: "connections",
		Description: "Number of currently active nginx connections — fires when active connections exceed the threshold.",
		Explanation: "Each active connection represents an in-flight request being processed. When connections grow beyond capacity, nginx starts queueing or refusing new connections. Common causes: traffic spikes, slow upstream responses holding connections open (PHP-FPM workers saturated, slow database), or too few nginx worker_connections configured. If connections stay high, response times will increase as requests wait for available workers.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "nginx_traffic_drop", ServiceType: "nginx", Name: "Traffic drop detected", DefaultThreshold: -50, Unit: "%",
		Description: "Percentage change in request rate between the current window (default 5m) and the trailing 1-hour window — fires when traffic drops more than the threshold (e.g. -50% means current rate is half the 1-hour average).",
		Explanation: "A sudden traffic drop can indicate infrastructure problems: the upstream service is unreachable (502/503), DNS resolution failures, a CDN or load balancer misconfiguration, or a routing change that stopped directing traffic to this host. The evaluation compares the current request rate (over the evaluation window, e.g. 5m) against the rate over the last hour. A value of -50% means current traffic is half of what it was over the last hour. Investigate upstream connectivity, DNS health, and load balancer configuration.",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "nginx_auth_brute_force", ServiceType: "nginx", Name: "Brute-force attack detected", DefaultThreshold: 10, Unit: "attempts/5m",
		Description: "Count of 401/403 HTTP responses in the evaluation window — fires when unauthorized/forbidden responses exceed the threshold.",
		Explanation: "A high rate of 401/403 responses indicates an active brute-force or credential-stuffing attack. Each 401/403 represents a failed authentication or access attempt. Attackers probe endpoints with stolen or guessed credentials, regardless of the specific path. Immediate actions: rate-limit endpoints returning 401/403, enable Web Application Firewall (WAF) rules, check for leaked credentials, and review access logs for the attacking IPs.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "nginx_5xx", ServiceType: "nginx", Name: "High 5xx error rate", DefaultThreshold: 5, Unit: "%",
		Description: "Percentage of HTTP responses that are 5xx errors — fires when the error rate exceeds threshold.",
		Explanation: "A rising 5xx rate means the upstream application (PHP-FPM, Node.js, etc.) is failing to handle requests. Common causes: application bugs or crashes, exhausted PHP-FPM workers, database connection failures, or a deployment that introduced errors. If 5xx errors are accompanied by high PHP-FPM saturation, the app pool likely needs more workers.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "nginx_p99", ServiceType: "nginx", Name: "High P99 latency", DefaultThreshold: 5, Unit: "seconds",
		Description: "99th percentile response time in milliseconds — fires when the slowest 1% of requests exceed threshold.",
		Explanation: "P99 latency measures the experience of your slowest users. When it spikes, it means some requests are taking much longer than normal. Common causes: slow database queries, external API calls timing out, PHP-FPM workers being queued, or a specific slow endpoint. High P99 with normal P50 suggests a specific bad path, not general overload.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "nginx_4xx", ServiceType: "nginx", Name: "High 4xx error rate", DefaultThreshold: 15, Unit: "%",
		Description: "Percentage of HTTP responses that are 4xx client errors — fires when the rate exceeds threshold.",
		Explanation: "A high 4xx rate means clients are sending bad requests. Common causes: misconfigured application (wrong base URLs, missing assets), bot/scanner traffic probing for vulnerabilities, expired or invalid API tokens, or recently changed API contracts.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},

	// ── Apache templates ────────────────────────────────────────
	{
		ID: "apache_4xx", ServiceType: "apache", Name: "High 4xx error rate", DefaultThreshold: 15, Unit: "%",
		Description: "Percentage of HTTP responses that are 4xx client errors — fires when the rate exceeds threshold.",
		Explanation: "A high 4xx rate means clients are sending bad requests. Common causes: misconfigured application (wrong base URLs, missing assets), bot/scanner traffic probing for vulnerabilities, expired or invalid API tokens, or recently changed API contracts.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "apache_auth_brute_force", ServiceType: "apache", Name: "Brute-force attack detected", DefaultThreshold: 10, Unit: "attempts/5m",
		Description: "Count of 401/403 HTTP responses in the evaluation window — fires when unauthorized/forbidden responses exceed the threshold.",
		Explanation: "A high rate of 401/403 responses indicates an active brute-force or credential-stuffing attack. Each 401/403 represents a failed authentication or access attempt. Attackers probe endpoints with stolen or guessed credentials, regardless of the specific path. Immediate actions: rate-limit endpoints returning 401/403, enable Web Application Firewall (WAF) rules, check for leaked credentials, and review access logs for the attacking IPs.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "apache_traffic_drop", ServiceType: "apache", Name: "Traffic drop detected", DefaultThreshold: -50, Unit: "%",
		Description: "Percentage change in request rate between the current window (default 5m) and the trailing 1-hour window — fires when traffic drops more than the threshold (e.g. -50% means current rate is half the 1-hour average).",
		Explanation: "A sudden traffic drop can indicate infrastructure problems: the upstream service is unreachable (502/503), DNS resolution failures, a CDN or load balancer misconfiguration, or a routing change that stopped directing traffic to this host. The evaluation compares the current request rate (over the evaluation window, e.g. 5m) against the rate over the last hour. A value of -50% means current traffic is half of what it was over the last hour. Investigate upstream connectivity, DNS health, and load balancer configuration.",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "apache_workers_saturation", ServiceType: "apache", Name: "Worker saturation", DefaultThreshold: 85, Unit: "%",
		Description: "Percentage of Apache workers currently busy — fires when worker pool utilization exceeds the threshold.",
		Explanation: "When Apache workers are near capacity, new requests queue up instead of being processed immediately. This causes response times to increase and, at 100%, new connections are rejected. Common causes: traffic spike, slow scripts blocking workers (e.g. waiting on slow database queries, external API calls), or MaxRequestWorkers / MaxClients set too low for the traffic volume. Consider increasing MaxRequestWorkers or optimizing slow scripts.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "apache_5xx", ServiceType: "apache", Name: "High 5xx error rate", DefaultThreshold: 5, Unit: "%",
		Description: "Percentage of HTTP responses that are 5xx errors — fires when the error rate exceeds threshold.",
		Explanation: "A rising 5xx rate means the upstream application is failing to handle requests. Common causes: application bugs, exhausted Apache workers, database failures, or a bad deployment. If 5xx errors are accompanied by high worker saturation, the MaxRequestWorkers setting likely needs to be increased.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "apache_p99", ServiceType: "apache", Name: "High P99 latency", DefaultThreshold: 5000, Unit: "ms",
		Description: "99th percentile response time in milliseconds — fires when the slowest 1% of requests exceed threshold.",
		Explanation: "P99 latency measures the experience of your slowest users. When it spikes, it means some requests are taking much longer than normal. Common causes: slow database queries, external API timeouts, or a specific slow endpoint. Check Apache's slow request log to identify the offending paths.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},

	// ── PostgreSQL templates ─────────────────────────────────────
	{
		ID: "postgres_connections", ServiceType: "postgres", Name: "Connection saturation", DefaultThreshold: 80, Unit: "%",
		Description: "Percentage of available database connections in use — fires when connection pool utilization exceeds threshold.",
		Explanation: "When connection utilization is high, the database is approaching its max_connections limit. New connection attempts will be rejected once the limit is reached, causing application errors. Common causes: connection leaks in application code (connections not returned to pool), too many application instances, or max_connections set too low for the workload.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_deadlocks", ServiceType: "postgres", Name: "High deadlock rate", DefaultThreshold: 1, Unit: "count/min",
		Description: "Rate of database deadlock events per minute — fires when deadlocks are detected.",
		Explanation: "Deadlocks occur when two or more transactions hold locks that the other needs, creating a circular wait. PostgreSQL resolves deadlocks by killing one of the involved transactions, causing application errors. Common causes: inconsistent lock ordering across concurrent transactions, missing indexes that cause table-level locking, or long-running transactions that hold locks for too long.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_cache_hit", ServiceType: "postgres", Name: "Low cache hit rate", DefaultThreshold: 95, Unit: "%",
		Description: "PostgreSQL shared buffer cache hit rate — fires when hit rate drops below threshold (e.g. < 95% means more than 5% of reads come from disk).",
		Explanation: "The shared buffer cache holds frequently accessed data pages in memory. A low cache hit rate means queries are reading from disk, which is thousands of times slower than memory. Common causes: insufficient shared_buffers for the working dataset, full table scans from missing indexes, or a workload that exceeds the available cache. A hit rate below 95% requires increasing shared_buffers or optimizing query patterns.",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_replication_lag", ServiceType: "postgres", Name: "Replication lag", DefaultThreshold: 30, Unit: "seconds",
		Description: "Replica lag in seconds — fires when the standby is more than threshold seconds behind the primary.",
		Explanation: "Replication lag means standby replicas have not yet applied all WAL changes from the primary. This causes stale reads on replicas and risks data loss if the primary fails before the lag is caught up. Common causes: heavy write workload on the primary, insufficient resources on the standby, or network latency between primary and standby. Investigate standby performance and WAL archiving.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_long_transaction", ServiceType: "postgres", Name: "Long-running transaction", DefaultThreshold: 300, Unit: "seconds",
		Description: "Maximum duration (in seconds) of the longest active transaction — fires when any transaction exceeds the threshold.",
		Explanation: "Long-running transactions block autovacuum, prevent bloat cleanup, and hold locks that cause other queries to wait. They also keep old row versions alive (MVCC), causing table bloat and slowing index scans. Common causes: a transaction that is open but idle (application forgot to commit), a slow batch operation, or a transaction that includes user interaction (never do this). Queries with pg_stat_activity.state='active' that run longer than a few minutes should be investigated.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_temp_files", ServiceType: "postgres", Name: "Temp file creation", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate of temporary file creation (from pg_stat_database_temp_files) — fires when any temp file is created (threshold > 0 indicates work_mem too small).",
		Explanation: "Temp files are created when PostgreSQL cannot fit a sort, hash, or materialize operation in memory (work_mem). Writing temp files to disk slows down queries dramatically and increases I/O pressure. If any temp files appear, it means the workload is spilling to disk. Common causes: insufficient work_mem for the query patterns, missing indexes causing large sorts, or queries that return too many rows without proper LIMIT or filtering.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_archive_failures", ServiceType: "postgres", Name: "WAL archive failures", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate of WAL archiving failures — fires when any archive failure is detected (threshold > 0 means archiving is failing).",
		Explanation: "WAL archiving moves completed WAL segments to a safe location (e.g. S3, NFS) for point-in-time recovery. If archiving fails, WAL segments accumulate on the primary, eventually filling the disk and crashing the database. Common causes: destination storage full, network issues to archive target, misconfigured archive_command, or permission problems. Immediate investigation is required — archiving must work for backup integrity.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_conflicts", ServiceType: "postgres", Name: "Replication conflicts", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate of replication conflicts on standby — fires when any conflict is detected (threshold > 0 means queries are being cancelled on standby).",
		Explanation: "Replication conflicts occur on standby servers when a query conflicts with WAL replay (e.g. a query is accessing a table that the primary just dropped or vacuumed). PostgreSQL cancels the conflicting query, causing application errors on read replicas. Common causes: long-running queries on the standby, aggressive vacuum on the primary, or max_standby_streaming_delay set too low. Increase max_standby_streaming_delay or ensure queries on standby finish quickly.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_auth_failures", ServiceType: "postgres", Name: "Authentication failures", DefaultThreshold: 5, Unit: "events/5m",
		Description: "Count of 'password authentication failed' errors from PostgreSQL logs — fires when auth failures exceed threshold.",
		Explanation: "Authentication failures mean client connection attempts with wrong credentials. A low rate may be a misconfigured application (wrong password cached). A sustained high rate suggests an active brute-force attack on PostgreSQL credentials. Check pg_hba.conf settings and consider restricting network access to trusted sources.",
		DefaultSeverity: "warning", ComparisonOp: ">=", DefaultForDuration: "5m",
	},
	{
		ID: "postgres_lock_waits", ServiceType: "postgres", Name: "Lock wait contention", DefaultThreshold: 5, Unit: "events/5m",
		Description: "Count of 'still waiting for' lock wait messages from PostgreSQL logs — fires when lock wait events exceed threshold.",
		Explanation: "Lock waits occur when a query cannot proceed because another transaction holds a conflicting lock. A high rate of lock waits indicates application-level contention. Common causes: long-running transactions not committing, multiple concurrent updates to the same rows, or DDL operations (ALTER TABLE) that require exclusive locks blocking all other access. Use pg_blocking_pids() to identify the blocking session.",
		DefaultSeverity: "warning", ComparisonOp: ">=", DefaultForDuration: "5m",
	},

	// ── MariaDB templates ────────────────────────────────────────
	{
		ID: "mariadb_slow_queries", ServiceType: "mariadb", Name: "Slow query spike", DefaultThreshold: 10, Unit: "count/min",
		Description: "Rate of slow queries (exceeding long_query_time) per minute — fires when slow query rate exceeds threshold.",
		Explanation: "Slow queries indicate queries that take longer than long_query_time to execute. They consume database resources for extended periods and can block other queries. Common causes: missing indexes, poorly optimized queries (full table scans), insufficient buffer pool size causing disk reads, or schema changes that invalidated query plans.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_connections", ServiceType: "mariadb", Name: "Connection saturation", DefaultThreshold: 80, Unit: "%",
		Description: "Percentage of available database connections in use — fires when connection pool utilization exceeds threshold.",
		Explanation: "When connection utilization is high, MariaDB is approaching max_connections. New connections will be rejected once the limit is reached. Common causes: connection leaks in applications, too many concurrent app instances, or insufficient max_connections. Saturated connections combined with slow queries usually means queries are taking too long, holding connections open.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_buffer_pool_hit", ServiceType: "mariadb", Name: "Low buffer pool hit rate", DefaultThreshold: 95, Unit: "%",
		Description: "InnoDB buffer pool hit rate — fires when hit rate drops below threshold (e.g. < 95% means more than 5% of pages are read from disk instead of memory).",
		Explanation: "The buffer pool caches data pages in memory for faster access. A low hit rate means queries are reading from disk more often, which is orders of magnitude slower. Common causes: insufficient innodb_buffer_pool_size for the working dataset, full table scans from missing indexes, or a sudden increase in data volume. A hit rate below 95% indicates the buffer pool is too small for the active workload.",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_deadlocks", ServiceType: "mariadb", Name: "High deadlock rate", DefaultThreshold: 5, Unit: "count/min",
		Description: "Rate of InnoDB deadlock events per minute — fires when deadlock rate exceeds threshold.",
		Explanation: "Deadlocks occur when two or more transactions hold locks that the other needs, creating a circular wait. InnoDB resolves deadlocks by rolling back one transaction, causing application errors. Common causes: inconsistent lock ordering across concurrent transactions, missing indexes that cause table-level locking, or long-running transactions holding locks for extended periods. Reduce deadlocks by ensuring consistent lock order and keeping transactions short.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_replication_lag", ServiceType: "mariadb", Name: "Replication lag", DefaultThreshold: 30, Unit: "seconds",
		Description: "Replica lag in seconds — fires when the replica is more than threshold seconds behind the primary.",
		Explanation: "Replication lag means the replica has not yet applied all changes from the primary. This can cause stale reads on replicas and, if lag grows too large, a prolonged recovery if the primary fails. Common causes: a heavy write workload on the primary, a slow replica (insufficient CPU/IO), or a long-running query on the replica blocking replication. Investigate replica performance and consider read load distribution.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_aborted_connects", ServiceType: "mariadb", Name: "High aborted connection rate", DefaultThreshold: 10, Unit: "count/min",
		Description: "Rate of aborted connection attempts per minute — fires when aborted connects exceed threshold.",
		Explanation: "Aborted connections indicate failed login attempts. A high rate may signal an application misconfiguration (wrong credentials, wrong host), a transient network issue, or an active brute-force attack on database credentials. Common causes: application with bad cached credentials, too many connection timeouts, or a misconfigured max_connect_errors limit being hit repeatedly.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_tmp_disk_tables", ServiceType: "mariadb", Name: "Temp tables on disk", DefaultThreshold: 10, Unit: "count/min",
		Description: "Rate of temporary tables created on disk (instead of in memory) — fires when disk temp table rate exceeds threshold.",
		Explanation: "Temporary tables on disk indicate that MariaDB could not fit the temporary result set in memory (tmp_table_size / max_heap_table_size). This happens with large GROUP BY, DISTINCT, ORDER BY operations without proper indexes. Disk-based temp tables are significantly slower and increase I/O pressure. Common causes: missing indexes, insufficient tmp_table_size, or queries returning too many rows without proper filtering.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_row_lock_wait", ServiceType: "mariadb", Name: "High row lock wait rate", DefaultThreshold: 5, Unit: "count/min",
		Description: "Rate of InnoDB row lock waits per minute — fires when lock wait rate exceeds threshold.",
		Explanation: "Row lock waits occur when a transaction tries to access a row locked by another transaction. High lock wait rates indicate contention — many transactions competing for the same rows. Common causes: long-running transactions holding locks, missing indexes causing full table scans (locking many rows), or application design issues with concurrent updates to the same rows. Monitor which queries are waiting and optimize them.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_auth_failures", ServiceType: "mariadb", Name: "Authentication failures", DefaultThreshold: 5, Unit: "events/5m",
		Description: "Count of 'Access denied for user' errors from MariaDB error logs — fires when authentication failures exceed threshold.",
		Explanation: "Authentication failures indicate failed login attempts. A small number may be expected (application restart with cached old credentials), but a sustained or high rate suggests a misconfigured application or an active brute-force attack on database credentials. Check which user and source IP is generating the failures and verify the application connection string.",
		DefaultSeverity: "warning", ComparisonOp: ">=", DefaultForDuration: "5m",
	},
	{
		ID: "mariadb_history_list_length", ServiceType: "mariadb", Name: "Long InnoDB history list", DefaultThreshold: 10000, Unit: "length",
		Description: "InnoDB history list length — fires when the length exceeds threshold (indicates long-running transactions blocking purge).",
		Explanation: "The InnoDB history list tracks old row versions needed for MVCC (Multi-Version Concurrency Control). A long history list means old undo records cannot be purged because a long-running transaction still needs them. This causes the InnoDB tablespace to grow, slows down queries that need to traverse the history, and can eventually lead to severe performance degradation. Check for long-running idle transactions that aren't committing.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},

	// ── Redis templates ──────────────────────────────────────────
	{
		ID: "redis_memory", ServiceType: "redis", Name: "High memory usage", DefaultThreshold: 80, Unit: "%",
		Description: "Percentage of Redis maxmemory in use — fires when memory consumption exceeds threshold.",
		Explanation: "When Redis approaches maxmemory, its eviction policy starts removing keys to free space. This reduces cache effectiveness and increases database load as more queries miss the cache. If maxmemory is reached with a noeviction policy, all write commands will fail. Common causes: cache keys without TTLs, insufficient maxmemory, or a sudden increase in cached data volume.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_evictions", ServiceType: "redis", Name: "High eviction rate", DefaultThreshold: 100, Unit: "count/min",
		Description: "Rate of keys being evicted from Redis due to memory pressure — fires when eviction rate exceeds threshold.",
		Explanation: "Evictions mean Redis is under memory pressure and is actively removing keys to stay within maxmemory limits. This is a sign that cache effectiveness is degrading — evicted keys will need to be recomputed or re-fetched from the database, increasing latency and load. Common causes: insufficient maxmemory, keys without TTLs, or a larger working set than anticipated.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_cache_hit", ServiceType: "redis", Name: "Low cache hit rate", DefaultThreshold: 90, Unit: "%",
		Description: "Redis keyspace hit rate (hits / (hits + misses) * 100) — fires when hit rate drops below threshold.",
		Explanation: "A low cache hit rate means many requested keys do not exist in Redis, forcing the application to fetch data from the primary database. This increases latency and database load. Common causes: keys expiring too early (short TTL), cache isn't being warmed after a flush, application requesting non-existent keys, or insufficient maxmemory causing eviction of useful keys. Check for cache miss patterns in application logs.",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "redis_blocked_clients", ServiceType: "redis", Name: "Blocked clients", DefaultThreshold: 5, Unit: "clients",
		Description: "Number of clients blocked (waiting on lists, streams, or locks) — fires when blocked clients exceed threshold.",
		Explanation: "Blocked clients are connections waiting for data (BRPOP, BLPOP, XREAD with BLOCK, or waiting for a lock). A high number of blocked clients may indicate slow consumers not processing data fast enough, or a deadlock where a producer crashed while holding a lock. Common causes: a consumer worker pool that is too small, slow consumer processing, or a stuck consumer that never releases a lock.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_rejected_connections", ServiceType: "redis", Name: "Rejected connections", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate of connections rejected by Redis (maxclients reached) — fires when any rejection occurs (threshold > 0 means clients are being refused).",
		Explanation: "Rejected connections mean Redis has reached its maxclients limit and is refusing new connections. This directly causes application errors since apps cannot connect to Redis. Common causes: connection leaks in application code, too many application instances, or maxclients set too low. Check for connections that are not being closed properly (TIME_WAIT, CLOSE_WAIT).",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_error_replies", ServiceType: "redis", Name: "Error replies", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate of error replies from Redis (e.g. WRONGTYPE, OOM, command errors) — fires when any error reply is detected.",
		Explanation: "Error replies mean clients are sending invalid commands to Redis — wrong data types, syntax errors, or commands that fail because of OOM conditions. A high rate of errors indicates application bugs (calling wrong commands, wrong key types) or an attack. Common causes: application code using wrong data type for a key (e.g. LPUSH on a string), sending commands that exceed maxmemory with noeviction policy, or a misconfigured client library.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_fragmentation", ServiceType: "redis", Name: "High memory fragmentation", DefaultThreshold: 1.5, Unit: "ratio",
		Description: "Redis memory fragmentation ratio (used_rss / used_memory) — fires when fragmentation exceeds threshold (> 1.5 means 50%+ memory is wasted).",
		Explanation: "Memory fragmentation occurs when Redis allocates and frees memory of different sizes, leaving gaps. High fragmentation (> 1.5) means a significant portion of RAM is wasted and cannot be used for data. Common causes: frequent key creation/deletion with varying sizes, large number of keys with different TTLs expiring at different times, or the Jemalloc allocator struggling with the allocation pattern. Restarting Redis or using MEMORY PURGE may temporarily reduce fragmentation.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "redis_slave_disconnect", ServiceType: "redis", Name: "Replica disconnected", DefaultThreshold: 1, Unit: "replicas",
		Description: "Number of connected replicas (slaves) — fires when the count is below expected (threshold < 1 means no replicas connected).",
		Explanation: "Missing replicas mean Redis replication is not working. This means there is no failover target if the primary fails, leading to data loss and downtime. Common causes: network issues between primary and replica, replica crashed or stopped, authentication mismatch, or replica configuration changes. Check redis replication info for the specific error.",
		DefaultSeverity: "critical", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "redis_auth_or_security", ServiceType: "redis", Name: "Security/Auth failures", DefaultThreshold: 1, Unit: "events/5m",
		Description: "Count of AUTH failures or security attack messages from Redis logs — fires when any event is detected.",
		Explanation: "AUTH failures mean clients are attempting to connect with wrong passwords. A single event may be a misconfigured client, but repeated failures indicate an active brute-force attack on Redis credentials. Redis is a frequent target for ransomware attacks when exposed without authentication. Check requirepass configuration and ensure Redis is not directly exposed to the internet.",
		DefaultSeverity: "critical", ComparisonOp: ">=", DefaultForDuration: "5m",
	},

	// ── Container stopped templates (status-based) ─────────────────
	{
		ID: "container_stopped", ServiceType: "container", Name: "Container stopped", DefaultThreshold: 420, Unit: "s",
		Description: "Fires if a container stops sending heartbeats via the agent.",
		Explanation: "The agent reports running containers every 5 minutes via heartbeat. If no heartbeat is received within the threshold window, the container is likely stopped, crashed, or the host is unreachable. Use this to detect down containers.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "host_service_stopped", ServiceType: "host_service", Name: "Host service stopped", DefaultThreshold: 420, Unit: "s",
		Description: "Fires if a systemd service (nginx, apache, postgres, etc.) stops sending heartbeats via the agent.",
		Explanation: "The agent reports running systemd services every 5 minutes via heartbeat. If no heartbeat is received within the threshold window, the service is likely stopped, crashed, or the host is unreachable. Use this to detect down services.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},

	// ── Container templates ────────────────────────────────────────
	{
		ID: "container_cpu", ServiceType: "container", Name: "High CPU usage", DefaultThreshold: 90, Unit: "%",
		Description: "CPU usage percentage for a container — fires when CPU exceeds threshold.",
		Explanation: "High container CPU usage means the process inside the container is consuming significant CPU time. Common causes: traffic spike, unoptimized code (infinite loops, busy waits), or insufficient CPU allocation for the workload. If sustained, the container may be throttled by cgroup limits, causing request latency to increase.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "container_memory", ServiceType: "container", Name: "High memory usage", DefaultThreshold: 85, Unit: "%",
		Description: "Memory usage percentage for a container — fires when memory exceeds threshold.",
		Explanation: "High memory usage inside a container means the application is approaching its memory limit. If the limit is reached, the container may be OOM-killed by the kernel. Common causes: memory leaks in the application, too many concurrent connections/workers, or insufficient memory limit set on the container. Monitor for OOM events and consider increasing the limit if the workload legitimately needs more memory.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},

	// ── PHP-FPM templates ────────────────────────────────────────
	{
		ID: "phpfpm_processes", ServiceType: "phpfpm", Name: "High active process usage", DefaultThreshold: 80, Unit: "%",
		Description: "Percentage of PHP-FPM worker processes currently active — fires when pool utilization exceeds threshold.",
		Explanation: "When the PHP-FPM pool is near capacity, new requests queue up instead of being processed immediately. This causes response times to increase and, at 100%, new requests are rejected with 502/503 errors. Common causes: traffic spike, slow scripts blocking workers (e.g. waiting on slow database queries), or pm.max_children set too low for the traffic volume.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_queue_saturation", ServiceType: "phpfpm", Name: "Listen queue saturation", DefaultThreshold: 50, Unit: "%",
		Description: "Percentage of the PHP-FPM listen queue capacity in use — fires when queued connections exceed threshold.",
		Explanation: "The listen queue holds incoming connections waiting for an available worker. When the queue fills up, new connections are rejected. A non-zero listen queue means all workers are busy and requests are waiting. Common causes: traffic spike, pm.max_children too low, or workers blocked on slow operations (database queries, external APIs). If the queue grows rapidly, consider increasing pm.max_children and optimizing slow scripts.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_slow_requests", ServiceType: "phpfpm", Name: "Slow request rate", DefaultThreshold: 5, Unit: "count/min",
		Description: "Rate of PHP-FPM slow requests (exceeding request_slowlog_timeout) per minute — fires when the rate exceeds threshold.",
		Explanation: "Slow requests indicate scripts that are taking longer than request_slowlog_timeout to execute. They hold worker processes for extended periods, reducing pool capacity for other requests. Common causes: slow database queries, unoptimized loops, large file processing, or external API calls with no timeout. Check the slow request log to identify the offending scripts.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_max_children_reached", ServiceType: "phpfpm", Name: "Max children limit hit", DefaultThreshold: 0, Unit: "count/s",
		Description: "Rate at which PHP-FPM reaches pm.max_children (from rate()) — fires when any event occurs (threshold > 0 means any positive rate triggers the alert).",
		Explanation: "Max children reached means the PHP-FPM pool has exhausted all available workers and is refusing new connections. This directly causes 502/503 errors for users. Common causes: traffic spike, connection leaks (workers not freed after request completion), or pm.max_children set too low for the workload. Immediate action is required — increase pm.max_children and investigate what is consuming workers.",
		DefaultSeverity: "critical", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_opcache_hit_low", ServiceType: "phpfpm", Name: "Low OPcache hit rate", DefaultThreshold: 85, Unit: "%",
		Description: "OPcache hit rate percentage — fires when the hit rate drops below threshold.",
		Explanation: "A low OPcache hit rate means PHP scripts are being recompiled frequently instead of being served from memory. This increases CPU usage and slows down every request. Common causes: opcache.memory_consumption too small causing cache churn, opcache.revalidate_freq too low forcing frequent revalidation, or PHP files changing frequently (bad in shared hosting or deployment where files are rewritten).",
		DefaultSeverity: "warning", ComparisonOp: "<", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_opcache_wasted", ServiceType: "phpfpm", Name: "OPcache wasted memory", DefaultThreshold: 5, Unit: "%",
		Description: "Percentage of OPcache memory wasted by invalidated scripts — fires when wasted memory exceeds threshold.",
		Explanation: "OPcache wasted memory accumulates when cached scripts are invalidated but their memory slots aren't immediately reused. High waste means the cache is fragmented and less effective, causing recompilation of scripts that should be cached. Common causes: frequent PHP file updates during deployments, opcache.max_wasted_percentage set too high, or opcache.memory_consumption too small for the application's script footprint.",
		DefaultSeverity: "warning", ComparisonOp: ">", DefaultForDuration: "5m",
	},
	{
		ID: "phpfpm_error_rate", ServiceType: "phpfpm", Name: "PHP-FPM error rate", DefaultThreshold: 1, Unit: "events/5m",
		Description: "Count of ERROR, ALERT, or CRITICAL level log messages from PHP-FPM error logs — fires when at least 1 error is detected (threshold 1 means any error triggers the alert).",
		Explanation: "ERROR and ALERT level messages in PHP-FPM error logs indicate serious issues: worker crashes (Segfault, SIGSEGV), pool configuration errors, or PHP fatal errors that weren't caught. A single error may indicate a transient bug, but sustained errors suggest a deeper problem — worker instability, memory corruption in an extension, or a code path that consistently crashes PHP. Check the error log for stack traces and identify the failing script or extension.",
		DefaultSeverity: "warning", ComparisonOp: ">=", DefaultForDuration: "5m",
	},
}

func FindTemplateByID(id string) *AlertTemplate {
	for _, t := range AlertTemplates {
		if t.ID == id {
			return &t
		}
	}
	return nil
}

func TemplatesByServiceType(serviceType string) []AlertTemplate {
	var result []AlertTemplate
	for _, t := range AlertTemplates {
		if t.ServiceType == serviceType {
			result = append(result, t)
		}
	}
	return result
}
