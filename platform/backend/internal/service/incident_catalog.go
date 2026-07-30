package service

// incident_catalog.go — static tier/flavor classification, 46-case title lookup,
// and fallback title builder for the incident grouping engine.

// ─── Tier classification ─────────────────────────────────────────────────────

type IncidentTier string

const (
	TierHost      IncidentTier = "host"
	TierHostFault IncidentTier = "host_fault"
	TierData      IncidentTier = "data"
	TierApp       IncidentTier = "app"
	TierWeb       IncidentTier = "web"
	TierSecurity  IncidentTier = "security"
	TierContainer IncidentTier = "container"
)

// templateTier maps every template_id to its tier.
var templateTier = map[string]IncidentTier{
	// HOST
	"host_cpu":            TierHost,
	"host_memory":         TierHost,
	"host_disk":           TierHost,
	"host_swap":           TierHost,
	"host_load1":          TierHost,
	"host_disk_io_latency": TierHost,
	"host_conntrack":      TierHost,
	"host_network_errors": TierHost,

	// HOST_FAULT (event-based, used to enrich narratives)
	"host_oom_kill":           TierHostFault,
	"host_service_crash_loop": TierHostFault,
	"host_disk_error":         TierHostFault,

	// DATA — Postgres
	"postgres_connections":    TierData,
	"postgres_deadlocks":      TierData,
	"postgres_cache_hit":      TierData,
	"postgres_replication_lag": TierData,
	"postgres_long_transaction": TierData,
	"postgres_temp_files":     TierData,
	"postgres_archive_failures": TierData,
	"postgres_conflicts":      TierData,
	"postgres_lock_waits":     TierData,

	// DATA — MariaDB
	"mariadb_slow_queries":       TierData,
	"mariadb_connections":        TierData,
	"mariadb_buffer_pool_hit":    TierData,
	"mariadb_deadlocks":          TierData,
	"mariadb_replication_lag":    TierData,
	"mariadb_aborted_connects":   TierData,
	"mariadb_tmp_disk_tables":    TierData,
	"mariadb_row_lock_wait":      TierData,
	"mariadb_history_list_length": TierData,

	// DATA — Redis
	"redis_memory":               TierData,
	"redis_evictions":            TierData,
	"redis_cache_hit":            TierData,
	"redis_blocked_clients":      TierData,
	"redis_rejected_connections": TierData,
	"redis_error_replies":        TierData,
	"redis_fragmentation":        TierData,
	"redis_slave_disconnect":     TierData,

	// APP — PHP-FPM
	"phpfpm_processes":            TierApp,
	"phpfpm_queue_saturation":     TierApp,
	"phpfpm_slow_requests":        TierApp,
	"phpfpm_max_children_reached": TierApp,
	"phpfpm_opcache_hit_low":      TierApp,
	"phpfpm_opcache_wasted":       TierApp,
	"phpfpm_error_rate":           TierApp,

	// WEB — Nginx
	"nginx_5xx":              TierWeb,
	"nginx_4xx":              TierWeb,
	"nginx_p99":              TierWeb,
	"nginx_conn_saturation":  TierWeb,
	"nginx_traffic_drop":     TierWeb,

	// WEB — Apache
	"apache_4xx":                TierWeb,
	"apache_5xx":                TierWeb,
	"apache_p99":                TierWeb,
	"apache_workers_saturation": TierWeb,
	"apache_traffic_drop":       TierWeb,

	// SECURITY
	"host_ssh_brute_force":     TierSecurity,
	"nginx_auth_brute_force":   TierSecurity,
	"apache_auth_brute_force":  TierSecurity,
	"postgres_auth_failures":   TierSecurity,
	"mariadb_auth_failures":    TierSecurity,
	"redis_auth_or_security":   TierSecurity,

	// CONTAINER
	"container_cpu":    TierContainer,
	"container_memory": TierContainer,
}

func getTier(templateID string) IncidentTier {
	if t, ok := templateTier[templateID]; ok {
		return t
	}
	return ""
}

// ─── Flavor classification ───────────────────────────────────────────────────

type IncidentFlavor string

const (
	FlavorSaturation  IncidentFlavor = "SATURATION"
	FlavorLocked      IncidentFlavor = "LOCKED"
	FlavorSlowIO      IncidentFlavor = "SLOW_IO"
	FlavorReplication IncidentFlavor = "REPLICATION"
	FlavorBackup      IncidentFlavor = "BACKUP"
	FlavorConflict    IncidentFlavor = "CONFLICT"
	FlavorMemory      IncidentFlavor = "MEMORY"
	FlavorErrors      IncidentFlavor = "ERRORS"
	FlavorSlow        IncidentFlavor = "SLOW"
	FlavorOpcache     IncidentFlavor = "OPCACHE"
	FlavorTrafficDrop IncidentFlavor = "TRAFFIC_DROP"
)

// templateFlavor maps DATA/APP/WEB template_ids to their flavor.
var templateFlavor = map[string]IncidentFlavor{
	// Postgres
	"postgres_connections":       FlavorSaturation,
	"postgres_lock_waits":        FlavorLocked,
	"postgres_deadlocks":         FlavorLocked,
	"postgres_cache_hit":         FlavorSlowIO,
	"postgres_temp_files":        FlavorSlowIO,
	"postgres_replication_lag":   FlavorReplication,
	"postgres_archive_failures":  FlavorBackup,
	"postgres_conflicts":         FlavorConflict,
	// postgres_long_transaction is always a contributing detail, not its own flavor

	// MariaDB
	"mariadb_connections":        FlavorSaturation,
	"mariadb_aborted_connects":   FlavorSaturation,
	"mariadb_row_lock_wait":      FlavorLocked,
	"mariadb_deadlocks":          FlavorLocked,
	"mariadb_slow_queries":       FlavorSlowIO,
	"mariadb_buffer_pool_hit":    FlavorSlowIO,
	"mariadb_tmp_disk_tables":    FlavorSlowIO,
	"mariadb_history_list_length": FlavorSlowIO,
	"mariadb_replication_lag":    FlavorReplication,

	// Redis
	"redis_memory":               FlavorMemory,
	"redis_evictions":            FlavorMemory,
	"redis_fragmentation":        FlavorMemory,
	"redis_blocked_clients":      FlavorSaturation,
	"redis_rejected_connections": FlavorSaturation,
	"redis_error_replies":        FlavorErrors,
	"redis_slave_disconnect":     FlavorReplication,
	"redis_cache_hit":            FlavorSlowIO, // special co-root case handled separately

	// PHP-FPM
	"phpfpm_processes":            FlavorSaturation,
	"phpfpm_queue_saturation":     FlavorSaturation,
	"phpfpm_max_children_reached": FlavorSaturation,
	"phpfpm_slow_requests":        FlavorSlow,
	"phpfpm_opcache_hit_low":      FlavorOpcache,
	"phpfpm_opcache_wasted":       FlavorOpcache,
	"phpfpm_error_rate":           FlavorErrors,

	// Nginx
	"nginx_5xx":              FlavorErrors,
	"nginx_4xx":              FlavorErrors,
	"nginx_p99":              FlavorSlow,
	"nginx_conn_saturation":  FlavorSaturation,
	"nginx_traffic_drop":     FlavorTrafficDrop,

	// Apache
	"apache_5xx":                FlavorErrors,
	"apache_4xx":                FlavorErrors,
	"apache_p99":                FlavorSlow,
	"apache_workers_saturation": FlavorSaturation,
	"apache_traffic_drop":       FlavorTrafficDrop,
}

func getFlavor(templateID string) IncidentFlavor {
	if f, ok := templateFlavor[templateID]; ok {
		return f
	}
	return ""
}

// ─── Flavor priority per service ─────────────────────────────────────────────

// flavorPriority defines which flavor wins when multiple fire on the same service.
// First element = highest priority.
var flavorPriority = map[string][]IncidentFlavor{
	"postgres": {FlavorSaturation, FlavorLocked, FlavorSlowIO, FlavorReplication, FlavorBackup, FlavorConflict},
	"mariadb":  {FlavorSaturation, FlavorLocked, FlavorSlowIO, FlavorReplication},
	"redis":    {FlavorMemory, FlavorSaturation, FlavorErrors, FlavorReplication},
	"phpfpm":   {FlavorSaturation, FlavorSlow, FlavorOpcache, FlavorErrors},
	"web":      {FlavorErrors, FlavorSlow, FlavorSaturation, FlavorTrafficDrop},
}

// pickWinningFlavor returns the highest-priority flavor from a set of firing flavors
// for a given service kind.
func pickWinningFlavor(serviceKind string, firingFlavors map[IncidentFlavor]bool) IncidentFlavor {
	prio, ok := flavorPriority[serviceKind]
	if !ok {
		// Unknown service, just return whichever flavor we find
		for f := range firingFlavors {
			return f
		}
		return ""
	}
	for _, f := range prio {
		if firingFlavors[f] {
			return f
		}
	}
	// Shouldn't happen, but return first firing flavor
	for f := range firingFlavors {
		return f
	}
	return ""
}

// ─── Service kind from template_id ───────────────────────────────────────────

// templateServiceKind returns the service kind ("postgres", "mariadb", "redis")
// from a DATA-tier template_id.
func templateServiceKind(templateID string) string {
	switch {
	case len(templateID) > 9 && templateID[:9] == "postgres_":
		return "postgres"
	case len(templateID) > 8 && templateID[:8] == "mariadb_":
		return "mariadb"
	case len(templateID) > 6 && templateID[:6] == "redis_":
		return "redis"
	case len(templateID) > 7 && templateID[:7] == "phpfpm_":
		return "phpfpm"
	case len(templateID) > 6 && templateID[:6] == "nginx_":
		return "nginx"
	case len(templateID) > 7 && templateID[:7] == "apache_":
		return "apache"
	default:
		return ""
	}
}

// ─── Symptom set classification ──────────────────────────────────────────────

type symptomSet int

const (
	symptomNone    symptomSet = 0
	symptomApp     symptomSet = 1
	symptomWeb     symptomSet = 2
	symptomAppWeb  symptomSet = 3
)

// ─── Catalog entries — 46 cases ──────────────────────────────────────────────

type catalogEntry struct {
	title     string
	narrative string
}

// The incident catalog. Keys are case IDs from the plan.
// Placeholders: {host}, {domain}, {service}, {container}, {dbname}
var incidentCatalog = map[int]catalogEntry{
	// ── A. Postgres as root ──────────────────────────────────────────────
	1: {
		title:     "Postgres is out of connections — the app and web layer are backing up because of it",
		narrative: "Connection pool exhaustion on Postgres is causing PHP-FPM to queue requests and the web server to return errors.",
	},
	2: {
		title:     "Postgres is out of connections, and it's already spilling into PHP-FPM",
		narrative: "PHP-FPM workers are waiting on Postgres connections, causing request queuing.",
	},
	3: {
		title:     "Postgres queries are stuck behind locks — slow responses are a downstream effect",
		narrative: "Lock contention in Postgres is blocking queries, which is slowing down the application layer.",
	},
	4: {
		title:     "Postgres is under memory pressure — cache hit rate has dropped and queries are slower everywhere",
		narrative: "Postgres shared buffer misses are forcing disk reads, degrading query performance across the stack.",
	},
	5: {
		title:     "Postgres cache hit rate is low, which is why PHP-FPM is running slow",
		narrative: "Low buffer cache effectiveness is making queries slower, which is consuming PHP-FPM workers longer.",
	},
	6: {
		title:     "Postgres replication is falling behind",
		narrative: "The standby replica is lagging behind the primary, risking stale reads and data loss on failover.",
	},
	7: {
		title:     "Postgres backups are failing",
		narrative: "WAL archiving is failing — backup integrity is at risk and WAL segments may fill the disk.",
	},
	8: {
		title:     "Postgres is generating temp files under load — running low on working memory",
		narrative: "Queries are spilling sort/hash operations to disk because work_mem is insufficient.",
	},
	9: {
		title:     "Postgres replica is hitting query conflicts with replication",
		narrative: "Queries on the standby are being cancelled because they conflict with WAL replay.",
	},

	// ── B. MariaDB as root ──────────────────────────────────────────────
	10: {
		title:     "MariaDB has run out of connections — PHP-FPM and your web app are queuing behind it",
		narrative: "Connection exhaustion on MariaDB is cascading into application and web tier degradation.",
	},
	11: {
		title:     "MariaDB is rejecting connections, and PHP-FPM is feeling it",
		narrative: "MariaDB connection failures are causing PHP-FPM workers to stall.",
	},
	12: {
		title:     "MariaDB queries are blocked on row locks — that's what's slowing PHP-FPM down",
		narrative: "Row lock contention in InnoDB is blocking queries and degrading application response times.",
	},
	13: {
		title:     "MariaDB is running slow queries under load — this is the source of the app slowdown",
		narrative: "Elevated slow query rate is consuming database resources and making the application wait.",
	},
	14: {
		title:     "MariaDB's buffer pool hit rate has dropped — expect slower queries across the board",
		narrative: "InnoDB is reading more pages from disk, which increases query latency.",
	},
	15: {
		title:     "MariaDB is spilling temp tables to disk — a sign it's short on memory",
		narrative: "Temporary tables can't fit in memory, forcing disk I/O during query execution.",
	},
	16: {
		title:     "MariaDB replication is falling behind",
		narrative: "The replica is lagging behind the primary, risking stale reads.",
	},
	17: {
		title:     "MariaDB's undo history is growing — long-running transactions are piling up",
		narrative: "The InnoDB history list is long, meaning old row versions can't be purged.",
	},

	// ── C. Redis as root ─────────────────────────────────────────────────
	18: {
		title:     "Redis is nearly out of memory and evicting keys — your cache is no longer helping",
		narrative: "Redis memory pressure is causing key evictions, degrading cache effectiveness and pushing load to the database.",
	},
	19: {
		title:     "Redis's cache miss rate is pushing extra load onto the database, and both are now behind",
		narrative: "A cold or evicted Redis cache is causing more requests to hit the database directly.",
	},
	20: {
		title:     "Redis is fragmented and using more memory than it needs to",
		narrative: "Memory fragmentation means Redis is wasting RAM on allocator overhead.",
	},
	21: {
		title:     "Redis is rejecting or blocking client connections",
		narrative: "Redis connection limits are being hit, causing application connection failures.",
	},
	22: {
		title:     "Redis is throwing errors on client commands",
		narrative: "Redis is returning error replies to client requests, indicating command or OOM issues.",
	},
	23: {
		title:     "Redis replica has disconnected — you're running unprotected right now",
		narrative: "No failover target is available if the primary fails.",
	},

	// ── D. PHP-FPM as root (no DATA tier) ─────────────────────────────────
	24: {
		title:     "PHP-FPM has run out of workers — requests are queuing at the web server",
		narrative: "All PHP-FPM workers are busy and new requests are waiting in the listen queue.",
	},
	25: {
		title:     "PHP-FPM is slow to respond, and it's not the database — workers are just overloaded",
		narrative: "Slow request processing is consuming workers without a database bottleneck.",
	},
	26: {
		title:     "PHP-FPM's opcache hit rate is low — PHP is recompiling scripts on every request",
		narrative: "Script recompilation overhead is increasing CPU usage and slowing responses.",
	},
	27: {
		title:     "PHP-FPM error rate has spiked",
		narrative: "PHP-FPM workers are logging errors at an elevated rate, indicating instability or a bad deploy.",
	},

	// ── E. Web as root (standalone) ────────────────────────────────────────
	28: {
		title:     "Elevated error rate with no backend strain behind it — check for a bad deploy or malformed requests",
		narrative: "HTTP errors are occurring without corresponding application or database issues.",
	},
	29: {
		title:     "Slow response times, but PHP-FPM and the database look healthy — likely a network or upstream issue",
		narrative: "High latency without backend degradation suggests a network or configuration problem.",
	},
	30: {
		title:     "Connections are saturating — this looks like a traffic spike, not a backend failure",
		narrative: "Connection pool is near capacity from traffic volume, not from slow backends.",
	},
	31: {
		title:     "Traffic has dropped sharply compared to its normal baseline",
		narrative: "Request rate is significantly below the recent average, possibly indicating an upstream routing problem.",
	},

	// ── F. Host as root ───────────────────────────────────────────────────
	32: {
		title:     "The host itself is under resource pressure — that's likely dragging down everything running on it",
		narrative: "Host-level resource exhaustion is degrading all services on this machine.",
	},
	33: {
		title:     "Disk space is running low, and it's starting to affect services that write to disk",
		narrative: "Low disk space is impacting write-heavy services like databases and log collectors.",
	},
	34: {
		title:     "Disk I/O latency is spiking — this is likely why database and app response times are up",
		narrative: "Slow disk operations are bottlenecking all services that depend on I/O.",
	},
	35: {
		title:     "Network errors on the host are likely causing timeouts upstream",
		narrative: "NIC-level errors are causing packet loss and connection timeouts.",
	},
	36: {
		title:     "Host is under resource pressure, standalone — nothing downstream is degraded yet",
		narrative: "Resource usage is elevated but services haven't started degrading. This is an early warning.",
	},

	// ── G. Host-fault standalone cards ──────────────────────────────────────
	// (When host-fault events fire without any other tier — they get their own card.)
	// These are handled inline via enrichment logic; see hostFaultStandaloneTitle.

	// ── H. Security ─────────────────────────────────────────────────────────
	37: {
		title:     "Brute-force SSH attempts detected on {host}",
		narrative: "Multiple failed SSH authentication attempts indicate an active credential attack.",
	},
	38: {
		title:     "Credential-stuffing attempts detected on {domain}",
		narrative: "Elevated 401/403 responses suggest automated credential testing or brute-force attempts.",
	},
	39: {
		title:     "Repeated failed logins against Postgres",
		narrative: "PostgreSQL is logging authentication failures from client connection attempts.",
	},
	40: {
		title:     "Repeated failed logins against MariaDB",
		narrative: "MariaDB is logging access denied errors from connection attempts.",
	},
	41: {
		title:     "Repeated failed AUTH attempts against Redis",
		narrative: "Redis is logging AUTH failures, which may indicate an attack on exposed Redis.",
	},
	42: {
		title:     "Coordinated attack detected — SSH and services are both under brute-force attempts on {host}",
		narrative: "Multiple authentication attack vectors are active simultaneously on the same host.",
	},

	// ── I. Container ────────────────────────────────────────────────────────
	43: {
		title:     "Container {container} is maxing out CPU",
		narrative: "The container process is consuming significant CPU time and may be throttled.",
	},
	44: {
		title:     "Container {container} is close to its memory limit",
		narrative: "The container is approaching its memory limit and may be OOM-killed.",
	},

	// ── J. Multi-root ───────────────────────────────────────────────────────
	45: {
		title:     "Both Postgres and MariaDB are under strain on {host} at the same time",
		narrative: "Two database services are degraded simultaneously on the same host.",
	},
	46: {
		title:     "Multiple domains are returning errors on the same web server",
		narrative: "More than one virtual host is experiencing elevated error rates: {domains}.",
	},
}

// ─── Host-fault enrichment clauses ───────────────────────────────────────────

var hostFaultEnrichment = map[string]string{
	"host_oom_kill":           "A process was OOM-killed on this host, which may explain service restarts.",
	"host_service_crash_loop": "A service is stuck in a restart loop on this host, consistent with a crash loop.",
	"host_disk_error":         "Disk I/O errors were logged on this host, which can cause slowdowns or data corruption.",
}

var hostFaultStandaloneTitle = map[string]catalogEntry{
	"host_oom_kill": {
		title:     "A process was killed for using too much memory (OOM) on {host}",
		narrative: "The kernel's OOM killer terminated a process to free memory.",
	},
	"host_service_crash_loop": {
		title:     "A service is stuck in a restart loop on {host}",
		narrative: "A systemd service is repeatedly crashing and restarting.",
	},
	"host_disk_error": {
		title:     "Disk I/O errors detected on {host}",
		narrative: "The kernel logged disk read/write errors, indicating potential hardware failure.",
	},
}

// ─── Flavor labels for fallback titles ───────────────────────────────────────

var flavorLabel = map[IncidentFlavor]string{
	FlavorSaturation:  "connection/resource saturation",
	FlavorLocked:      "lock contention",
	FlavorSlowIO:      "memory/IO pressure",
	FlavorReplication: "replication lag",
	FlavorBackup:      "backup failures",
	FlavorConflict:    "replication conflicts",
	FlavorMemory:      "memory pressure",
	FlavorErrors:      "elevated errors",
	FlavorSlow:        "slow responses",
	FlavorOpcache:     "opcache inefficiency",
	FlavorTrafficDrop: "traffic anomaly",
}

// ─── Case-selection helpers ──────────────────────────────────────────────────

// selectPostgresCase picks the catalog case ID for postgres as root.
func selectPostgresCase(flavor IncidentFlavor, symptoms symptomSet) int {
	switch flavor {
	case FlavorSaturation:
		switch symptoms {
		case symptomAppWeb:
			return 1
		case symptomApp:
			return 2
		default:
			return 1 // saturation with any combo
		}
	case FlavorLocked:
		return 3
	case FlavorSlowIO:
		switch symptoms {
		case symptomAppWeb:
			return 4
		case symptomApp:
			return 5
		default:
			return 4
		}
	case FlavorReplication:
		return 6
	case FlavorBackup:
		return 7
	case FlavorConflict:
		return 9
	}
	return 0 // fallback
}

// selectMariaDBCase picks the catalog case ID for mariadb as root.
func selectMariaDBCase(flavor IncidentFlavor, symptoms symptomSet) int {
	switch flavor {
	case FlavorSaturation:
		switch symptoms {
		case symptomAppWeb:
			return 10
		case symptomApp:
			return 11
		default:
			return 10
		}
	case FlavorLocked:
		return 12
	case FlavorSlowIO:
		switch symptoms {
		case symptomAppWeb:
			return 13
		case symptomApp:
			return 15
		default:
			return 14
		}
	case FlavorReplication:
		return 16
	}
	return 0 // fallback — let caller use generic template
}

// selectRedisCase picks the catalog case ID for redis as root.
func selectRedisCase(flavor IncidentFlavor, symptoms symptomSet) int {
	switch flavor {
	case FlavorMemory:
		return 18
	case FlavorSlowIO:
		return 19 // co-root case, handled specially by caller
	case FlavorSaturation:
		return 21
	case FlavorErrors:
		return 22
	case FlavorReplication:
		return 23
	}
	return 20
}

// selectPHPFPMCase picks the catalog case ID for phpfpm as root (no DATA firing).
func selectPHPFPMCase(flavor IncidentFlavor, webFiring bool) int {
	switch flavor {
	case FlavorSaturation:
		return 24
	case FlavorSlow:
		if webFiring {
			return 25
		}
		return 25
	case FlavorOpcache:
		return 26
	case FlavorErrors:
		return 27
	}
	return 0
}

// selectWebCase picks the catalog case ID for web as root (standalone).
func selectWebCase(flavor IncidentFlavor) int {
	switch flavor {
	case FlavorErrors:
		return 28
	case FlavorSlow:
		return 29
	case FlavorSaturation:
		return 30
	case FlavorTrafficDrop:
		return 31
	}
	return 28
}

// selectHostCase picks the catalog case ID for host as root.
func selectHostCase(hostTemplates map[string]bool, otherTiersFiring bool) int {
	if !otherTiersFiring {
		return 36 // standalone early warning
	}
	// Check specific host sub-cases first
	if hostTemplates["host_disk"] {
		return 33
	}
	if hostTemplates["host_disk_io_latency"] {
		return 34
	}
	if hostTemplates["host_network_errors"] || hostTemplates["host_conntrack"] {
		return 35
	}
	return 32 // generic host pressure
}

// Helpers for disk-related checks
var diskRelatedTemplates = map[string]bool{
	"postgres_archive_failures": true,
	"mariadb_tmp_disk_tables":   true,
	"postgres_temp_files":       true,
}
