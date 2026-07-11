package catalog

// ─────────────────────────────────────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────────────────────────────────────

type LoggingMode string
type MetricsPattern string

const (
	LoggingModeFile     LoggingMode = "file"
	LoggingModeJournald LoggingMode = "journald"
	LoggingModeSyslog   LoggingMode = "syslog"
	LoggingModeStdout   LoggingMode = "stdout"
)

const (
	MetricsPatternBuiltIn  MetricsPattern = "builtin"
	MetricsPatternExporter MetricsPattern = "exporter"
	MetricsPatternNone     MetricsPattern = "none"
)

type NVDProduct struct {
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
}

type ServiceDef struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`

	// Logging
	DefaultLogPaths    []string    `json:"default_log_paths"`
	DefaultLoggingMode LoggingMode `json:"default_logging_mode"`

	// Metrics
	MetricsPattern         MetricsPattern `json:"metrics_pattern"`
	DefaultMetricsEndpoint string         `json:"default_metrics_endpoint"`
	ExporterName           string         `json:"exporter_name,omitempty"`
	ExporterPort           int            `json:"exporter_port,omitempty"`

	// Guides
	MetricsGuide      string `json:"metrics_guide"`
	LogsGuide         string `json:"logs_guide"`
	JournaledLogGuide string `json:"journaled_log_guide,omitempty"`

	// Detection
	DetectionHints []string `json:"detection_hints"`

	// CVE checking
	NVD *NVDProduct `json:"nvd,omitempty"`
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog — Only nginx and postgresql
// ─────────────────────────────────────────────────────────────────────────────

var Catalog = map[string]ServiceDef{
	"nginx": {
		Name:                   "nginx",
		DisplayName:            "Nginx",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/nginx/access.log", "/var/log/nginx/error.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:80/nginx_status",
		MetricsGuide: `## Nginx Metrics (stub_status)

Nginx exposes basic connection metrics via the built-in stub_status module.

**Step 1 — Add the status location block**

Edit your nginx config and add inside a server block:

` + "```nginx" + `
location /nginx_status {
    stub_status on;
    access_log off;
    allow 127.0.0.1;
    deny all;
}
` + "```" + `

**Step 2 — Test and reload**

` + "```bash" + `
sudo nginx -t
sudo nginx -s reload
` + "```" + `

**Step 3 — Verify**

` + "```bash" + `
curl http://127.0.0.1:80/nginx_status
` + "```" + `

The **access_log off** directive keeps metric scrapes out of your access logs.`,
		LogsGuide: `## Nginx Log Collection

Nginx writes access logs by default. Typically no changes are needed.

**Verify log files exist:**

` + "```bash" + `
ls -la /var/log/nginx/
` + "```" + `

If you configured the stub_status location block, it already includes **access_log off** to prevent metric scrapes from polluting your access logs.`,
		DetectionHints: []string{"nginx", "openresty"},
		NVD:            &NVDProduct{Vendor: "f5", Product: "nginx_open_source"},
	},

	"apache": {
		Name:                   "apache",
		DisplayName:            "Apache",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/apache2/*.access.log", "/var/log/apache2/*.error.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8081/server-status?auto",
		MetricsGuide: `## Apache Metrics (mod_status)

Apache exposes server metrics via the built-in mod_status module.

**Step 1 — Enable the module**

` + "```bash" + `
sudo a2enmod status
sudo systemctl restart apache2
` + "```" + `

**Step 2 — Verify**

` + "```bash" + `
curl http://127.0.0.1:8081/server-status?auto
` + "```" + `

Use the port your Apache instance is listening on. The ?auto parameter returns a machine-readable metrics format.`,
		LogsGuide: `## Apache Log Collection

**Step 1 — Add custom log format**

Edit **/etc/apache2/apache2.conf** and add after the existing LogFormat definitions:

` + "```apache" + `
LogFormat "%a - %u %t \"%r\" %>s %B \"%{Referer}i\" \"%{User-Agent}i\" rt=%{ms}T" platform
` + "```" + `

**Step 2 — Apply to each VirtualHost**

For each domain, add the platform format and exclude **/server-status** from logs:

` + "```apache" + `
<VirtualHost *:80>
    ServerName your-domain.com

    SetEnvIf Request_URI "^/server-status" dontlog

    ErrorLog  ${APACHE_LOG_DIR}/your-domain.error.log
    CustomLog ${APACHE_LOG_DIR}/your-domain.access.log platform env=!dontlog
</VirtualHost>
` + "```" + `

**Step 3 — Test and restart**

` + "```bash" + `
sudo apache2ctl configtest
sudo systemctl restart apache2
` + "```" + `

**Step 4 — Verify**

` + "```bash" + `
sudo tail -n 1 /var/log/apache2/your-domain.access.log
` + "```" + `

Expected output:

` + "```" + `
192.168.1.10 - - [15/May/2026:16:33:00 +0000] "GET / HTTP/1.1" 200 612 "-" "curl/7.88" rt=12
` + "```" + ``,

		DetectionHints: []string{"apache", "apache2", "httpd"},
		NVD:            &NVDProduct{Vendor: "apache", Product: "http_server"},
	},

	"postgresql": {
		Name:                   "postgresql",
		DisplayName:            "PostgreSQL",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/postgresql/postgresql.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9187/metrics",
		ExporterName:           "postgres_exporter",
		ExporterPort:           9187,
		MetricsGuide: `## Enable PostgreSQL Metrics (postgres_exporter + pg_stat_statements)

Follow the complete instructions in the LogsGuide below (Parts 2 and 3).`,
		LogsGuide: `Complete PostgreSQL Integration Guide
BEFORE YOU START — DO NOT

* ❌ Do not use log_filename = 'postgresql-%Y-%m-%d.log' — daily rotation breaks log collection

* ❌ Do not set log_statement = 'all' unless you specifically want security auditing — it creates massive log volume

* ❌ Do not skip the log_line_prefix — without it logs cannot be parsed

* ❌ Do not set logging_collector = off — logs go to journald mixed with everything else

* ❌ Do not skip pg_stat_statements — without it you lose per-query performance metrics

PART 1 — Configure Log File
Step 1 — Create log directory

` + "```bash" + `
sudo mkdir -p /var/log/postgresql
sudo chown postgres:postgres /var/log/postgresql
sudo chmod 755 /var/log/postgresql
` + "```" + `

Step 2 — Find your postgresql.conf

` + "```bash" + `
# Ubuntu/Debian
sudo find /etc/postgresql -name "postgresql.conf"

# RHEL/CentOS
sudo find /var/lib/pgsql -name "postgresql.conf"
` + "```" + `

Step 3 — Edit postgresql.conf

` + "```ini" + `
# ── Logging destination ──────────────────────────────────────────
logging_collector = on
log_destination = 'stderr'
log_directory = '/var/log/postgresql'
log_filename = 'postgresql.log'        # STATIC name — never change this
log_rotation_age = 0                   # disable time-based rotation
log_rotation_size = 0                  # disable size-based rotation
log_truncate_on_rotation = off

# ── Log format (REQUIRED — do not change this prefix) ───────────
log_line_prefix = '%m [%p] %u@%d %r %a '

# ── What to log ──────────────────────────────────────────────────
log_min_duration_statement = 1000      # log queries slower than 1 second
log_connections = on
log_disconnections = on
log_lock_waits = on
log_checkpoints = on
log_temp_files = 0
log_statement = 'ddl'                  # schema changes only
` + "```" + `

For security auditing (logs every query — high volume):

` + "```ini" + `
log_statement = 'all'
log_min_duration_statement = -1
` + "```" + `

Step 4 — Reload PostgreSQL

` + "```bash" + `
sudo systemctl reload postgresql
` + "```" + `

Step 5 — Verify

` + "```bash" + `
ls -la /var/log/postgresql/
tail -n 20 /var/log/postgresql/postgresql.log
` + "```" + `

You should see lines like:

` + "```" + `
2025-01-15 14:23:01.123 UTC [1234] app@mydb 192.168.1.5(52341) rails connection received: ...
` + "```" + `

If file is empty, restart instead:

` + "```bash" + `
sudo systemctl restart postgresql
` + "```" + `

PART 2 — Configure Metrics
Step 1 — Enable pg_stat_statements

` + "```bash" + `
sudo -u postgres psql <<SQL
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
ALTER SYSTEM SET pg_stat_statements.max = '10000';
ALTER SYSTEM SET pg_stat_statements.track = 'all';
SQL
` + "```" + `

Step 2 — Restart PostgreSQL (required)

` + "```bash" + `
sudo systemctl restart postgresql
` + "```" + `

Step 3 — Create the extension and monitoring user

` + "```bash" + `
sudo -u postgres psql <<SQL
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

CREATE USER monitoring WITH PASSWORD 'choose_a_strong_password_here';
GRANT pg_monitor TO monitoring;
GRANT CONNECT ON DATABASE postgres TO monitoring;
SQL
` + "```" + `

Step 4 — Verify monitoring user works

` + "```bash" + `
psql -h localhost -U monitoring -d postgres \
  -c "SELECT count(*) FROM pg_stat_statements;"
` + "```" + `

PART 3 — Install postgres_exporter
Step 1 — Download postgres_exporter

` + "```bash" + `
# Check latest version at https://github.com/prometheus-community/postgres_exporter/releases
wget https://github.com/prometheus-community/postgres_exporter/releases/download/v0.15.0/postgres_exporter-0.15.0.linux-amd64.tar.gz
tar xzf postgres_exporter-0.15.0.linux-amd64.tar.gz
sudo mv postgres_exporter-0.15.0.linux-amd64/postgres_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/postgres_exporter
` + "```" + `

Step 2 — Create environment file

` + "```bash" + `
sudo tee /etc/postgres_exporter.env <<EOF
DATA_SOURCE_NAME=postgresql://monitoring:choose_a_strong_password_here@localhost/postgres?sslmode=disable
EOF
sudo chmod 600 /etc/postgres_exporter.env
` + "```" + `

Step 3 — Create systemd service

` + "```bash" + `
sudo tee /etc/systemd/system/postgres_exporter.service <<EOF
[Unit]
Description=PostgreSQL Exporter
After=network.target postgresql.service

[Service]
User=root
EnvironmentFile=/etc/postgres_exporter.env
ExecStart=/usr/local/bin/postgres_exporter
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

Step 4 — Start the exporter

` + "```bash" + `
sudo systemctl daemon-reload
sudo systemctl enable --now postgres_exporter
` + "```" + `

Step 5 — Verify exporter is working

` + "```bash" + `
sudo systemctl status postgres_exporter
curl -s http://localhost:9187/metrics | grep pg_up
` + "```" + `

PART 4 — If PostgreSQL Is Currently Logging to journald
Check:

` + "```bash" + `
sudo journalctl -u postgresql --no-pager | head -5
` + "```" + `

PART 5 — Configure in the Platform
After completing Parts 1, 2, and 3:

1. Go to the service config page for PostgreSQL
2. Set Log Path to /var/log/postgresql/postgresql.log
3. Set Metrics Endpoint to http://localhost:9187/metrics
4. Set Metrics Mode to prometheus
5. Click Install Integration

PART 6 — What You Will See in the Platform

From logs:

| Event | What you learn |
|-------|----------------|
| Slow query | Which query, how slow, which user, which database, which app |
| Auth failure | Failed login with username and client IP |
| Connection | Every connection with user, database, client IP, application name |
| Lock wait | Which query is blocked and for how long |
| Schema change | Every CREATE, DROP, ALTER with full SQL |
| Errors | Application errors at database level |

From metrics:

| Metric | What it tells you |
|--------|-------------------|
| Cache hit ratio | Below 95% means reading from disk — need more RAM or better indexes |
| Active connections | How close to max_connections limit |
| Query avg execution time | Which query patterns are slowest |
| Query call count | Which queries run most frequently |
| Sequential scans | Which tables are missing indexes |
| Dead rows | Whether VACUUM needs to run |
| Database size | How fast data is growing |
| Deadlocks | Application-level transaction conflicts |
| Temp files | PostgreSQL spilling to disk — memory pressure |
| Transaction rate | Commits and rollbacks per second |

PART 7 — Checklist Before Clicking Install Integration

- [ ] /var/log/postgresql/postgresql.log exists and has content
- [ ] log_line_prefix is exactly '%m [%p] %u@%d %r %a '
- [ ] log_filename is exactly 'postgresql.log'
- [ ] curl http://localhost:9187/metrics | grep pg_up returns pg_up 1
- [ ] You have entered the log path and metrics endpoint in the platform config page`,
		JournaledLogGuide: `PostgreSQL is currently logging to journald.

Follow PART 1 in the LogsGuide above to switch to a static file for proper collection.`,
		DetectionHints: []string{"postgres", "postgresql"},
		NVD:            &NVDProduct{Vendor: "postgresql", Product: "postgresql"},
	},

	"mariadb": {
		Name:                   "mariadb",
		DisplayName:            "MariaDB",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/mysql/error.log", "/var/log/mysql/slow.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9104/metrics",
		ExporterName:           "mysqld_exporter",
		ExporterPort:           9104,
		MetricsGuide: `## Enable MariaDB Metrics

Follow the complete instructions in the LogsGuide below. Metrics come from:

- mysqld_exporter on http://127.0.0.1:9104/metrics
- the query digest collector writing Prometheus text metrics to /var/lib/agent-prom/mariadb_query_digests.prom`,
		LogsGuide: `Complete MariaDB Integration Guide

BEFORE YOU START

This guide assumes MariaDB is already installed. Do not use root for collectors. Create a least-privilege monitor user and use that user for mysqld_exporter and the query digest collector.

PART 1 — Configure MariaDB for Observability

Step 1 — Edit the server config

` + "```bash" + `
sudo nano /etc/mysql/mariadb.conf.d/50-server.cnf
` + "```" + `

Add or update these lines under [mariadb]:

` + "```ini" + `
[mariadb]
bind-address            = 0.0.0.0
performance_schema      = ON

slow_query_log                  = 1
slow_query_log_file             = /var/log/mysql/slow.log
long_query_time                 = 1
log_queries_not_using_indexes   = 1
log_slow_admin_statements       = 1

log_error               = /var/log/mysql/error.log
log_warnings            = 3

server-id               = 1
log_bin                 = /var/log/mysql/mariadb-bin
binlog_format           = ROW
` + "```" + `

Do not set log_error_verbosity on MariaDB 11.x. MariaDB uses log_warnings; log_error_verbosity can prevent the service from starting.

Step 2 — Create the log directory and restart

` + "```bash" + `
sudo mkdir -p /var/log/mysql
sudo chown -R mysql:mysql /var/log/mysql
sudo chmod 755 /var/log/mysql
sudo systemctl restart mariadb
` + "```" + `

Step 3 — Verify the observability streams

` + "```bash" + `
sudo mariadb -u root -p -e "SHOW VARIABLES LIKE 'performance_schema';"
sudo mariadb -u root -p -e "SHOW VARIABLES LIKE 'slow_query_log%';"
sudo mariadb -u root -p -e "SHOW VARIABLES LIKE 'log_error%';"
sudo mariadb -u root -p -e "SHOW VARIABLES LIKE 'log_warnings';"
` + "```" + `

Expected: performance_schema is ON, slow_query_log is ON, slow_query_log_file is /var/log/mysql/slow.log, log_error is /var/log/mysql/error.log.

PART 2 — Create the Monitoring User

` + "```bash" + `
sudo mariadb -u root -p
` + "```" + `

Run:

` + "```sql" + `
CREATE USER IF NOT EXISTS 'monitor'@'%' IDENTIFIED BY 'choose_a_strong_password_here';
CREATE USER IF NOT EXISTS 'monitor'@'localhost' IDENTIFIED BY 'choose_a_strong_password_here';

GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'monitor'@'%';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'monitor'@'localhost';

GRANT SELECT ON performance_schema.* TO 'monitor'@'%';
GRANT SELECT ON performance_schema.* TO 'monitor'@'localhost';

FLUSH PRIVILEGES;
EXIT;
` + "```" + `

Verify:

` + "```bash" + `
mariadb -u monitor -p -e "SELECT * FROM performance_schema.events_statements_summary_by_digest LIMIT 1;"
` + "```" + `

PART 3 — Install mysqld_exporter

Step 1 — Download and install

` + "```bash" + `
cd /tmp
wget https://github.com/prometheus/mysqld_exporter/releases/download/v0.15.1/mysqld_exporter-0.15.1.linux-amd64.tar.gz
tar xzf mysqld_exporter-0.15.1.linux-amd64.tar.gz
sudo mv mysqld_exporter-0.15.1.linux-amd64/mysqld_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/mysqld_exporter
` + "```" + `

Step 2 — Create credentials file

` + "```bash" + `
sudo mkdir -p /etc/mysqld_exporter
sudo tee /etc/mysqld_exporter/.my.cnf <<EOF
[client]
user=monitor
password=choose_a_strong_password_here
host=localhost
port=3306
EOF
sudo chmod 600 /etc/mysqld_exporter/.my.cnf
` + "```" + `

Step 3 — Create systemd service

` + "```bash" + `
sudo tee /etc/systemd/system/mysqld_exporter.service <<EOF
[Unit]
Description=MySQL Exporter
After=network.target mariadb.service

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/mysqld_exporter \
  --config.my-cnf=/etc/mysqld_exporter/.my.cnf \
  --collect.info_schema.processlist \
  --collect.info_schema.innodb_metrics \
  --collect.engine_innodb_status \
  --collect.perf_schema.tablelocks \
  --collect.perf_schema.file_events \
  --collect.perf_schema.events_statements_summary_by_digest \
  --collect.binlog_size \
  --collect.info_schema.tables \
  --collect.info_schema.userstats
Restart=always

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

Step 4 — Start and verify

` + "```bash" + `
sudo systemctl daemon-reload
sudo systemctl enable --now mysqld_exporter
curl -s http://127.0.0.1:9104/metrics | grep mysql_up
` + "```" + `

PART 4 — Add Query Digest Metrics

The platform agent already exposes /var/lib/agent-prom/*.prom through its local Prometheus file server. This collector writes query digest metrics there, so they flow into VictoriaMetrics through the existing agent pipeline.

Step 1 — Create collector credentials

` + "```bash" + `
sudo mkdir -p /opt/observability
sudo tee /opt/observability/.my.cnf <<EOF
[client]
user=monitor
password=choose_a_strong_password_here
host=localhost
port=3306
EOF
sudo chmod 600 /opt/observability/.my.cnf
sudo chown root:root /opt/observability/.my.cnf
` + "```" + `

Step 2 — Create /opt/observability/query_collector.sh

` + "```bash" + `
sudo tee /opt/observability/query_collector.sh <<'EOF'
#!/bin/bash
set -euo pipefail

CONFIG_FILE="/opt/observability/.my.cnf"
PROM_DIR="/var/lib/agent-prom"
PROM_FILE="${PROM_DIR}/mariadb_query_digests.prom"
TMP_FILE=$(mktemp)

mkdir -p "$PROM_DIR"
COLLECTED_EPOCH=$(date +%s)000

cat > "$TMP_FILE" <<HEADER
# HELP mariadb_query_digest_exec_count Total executions of this query digest since server start
# TYPE mariadb_query_digest_exec_count counter
# HELP mariadb_query_digest_total_time_seconds Total time spent on this digest since server start
# TYPE mariadb_query_digest_total_time_seconds counter
# HELP mariadb_query_digest_avg_time_seconds Average execution time for this digest
# TYPE mariadb_query_digest_avg_time_seconds gauge
# HELP mariadb_query_digest_max_time_seconds Max execution time seen for this digest
# TYPE mariadb_query_digest_max_time_seconds gauge
# HELP mariadb_query_digest_rows_examined_total Total rows examined by this digest
# TYPE mariadb_query_digest_rows_examined_total counter
# HELP mariadb_query_digest_rows_sent_total Total rows sent by this digest
# TYPE mariadb_query_digest_rows_sent_total counter
# HELP mariadb_query_digest_no_index_used_count Executions that did not use an index
# TYPE mariadb_query_digest_no_index_used_count counter
# HELP mariadb_query_digest_tmp_tables_total Temporary tables created by this digest
# TYPE mariadb_query_digest_tmp_tables_total counter
# HELP mariadb_query_digest_tmp_disk_tables_total Temporary disk tables created by this digest
# TYPE mariadb_query_digest_tmp_disk_tables_total counter
HEADER

mariadb --defaults-file="$CONFIG_FILE" -D performance_schema -BNe "
SELECT
    DIGEST,
    COALESCE(SCHEMA_NAME, 'none'),
    LEFT(REPLACE(REPLACE(REPLACE(COALESCE(DIGEST_TEXT, ''), '\n', ' '), '\r', ' '), '\t', ' '), 200),
    COUNT_STAR,
    ROUND(SUM_TIMER_WAIT / 1000000000000, 6),
    ROUND(AVG_TIMER_WAIT / 1000000000000, 6),
    ROUND(MAX_TIMER_WAIT / 1000000000000, 6),
    SUM_ROWS_EXAMINED,
    SUM_ROWS_SENT,
    SUM_NO_INDEX_USED,
    SUM_CREATED_TMP_TABLES,
    SUM_CREATED_TMP_DISK_TABLES
FROM performance_schema.events_statements_summary_by_digest
WHERE DIGEST IS NOT NULL
ORDER BY SUM_TIMER_WAIT DESC
LIMIT 100;
" | while IFS=$'\t' read -r digest db query_text exec_count total_time avg_time max_time rows_examined rows_sent no_index tmp_tables tmp_disk; do
    qt=${query_text//\\/\\\\}
    qt=${qt//\"/\\\"}
    labels="digest=\"${digest}\",db=\"${db}\",digest_text=\"${qt}\""
    printf 'mariadb_query_digest_exec_count{%s} %s %s\n' "$labels" "$exec_count" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_total_time_seconds{%s} %s %s\n' "$labels" "$total_time" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_avg_time_seconds{%s} %s %s\n' "$labels" "$avg_time" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_max_time_seconds{%s} %s %s\n' "$labels" "$max_time" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_rows_examined_total{%s} %s %s\n' "$labels" "$rows_examined" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_rows_sent_total{%s} %s %s\n' "$labels" "$rows_sent" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_no_index_used_count{%s} %s %s\n' "$labels" "$no_index" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_tmp_tables_total{%s} %s %s\n' "$labels" "$tmp_tables" "$COLLECTED_EPOCH" >> "$TMP_FILE"
    printf 'mariadb_query_digest_tmp_disk_tables_total{%s} %s %s\n' "$labels" "$tmp_disk" "$COLLECTED_EPOCH" >> "$TMP_FILE"
done

mv "$TMP_FILE" "$PROM_FILE"
echo "Emitted $(grep -c 'mariadb_query_digest_exec_count' "$PROM_FILE" 2>/dev/null || echo 0) query digest metric sets to $PROM_FILE"
EOF
sudo chmod +x /opt/observability/query_collector.sh
` + "```" + `

Step 3 — Create timer

` + "```bash" + `
sudo tee /etc/systemd/system/query-collector.service <<EOF
[Unit]
Description=Query Digest Collector (runs once per trigger)
After=mariadb.service

[Service]
Type=oneshot
ExecStart=/bin/bash /opt/observability/query_collector.sh
StandardOutput=journal
StandardError=journal
EOF

sudo tee /etc/systemd/system/query-collector.timer <<EOF
[Unit]
Description=Run Query Digest Collector every 60 seconds

[Timer]
OnBootSec=30
OnUnitActiveSec=60s
AccuracySec=1s

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now query-collector.timer
` + "```" + `

Step 4 — Verify

` + "```bash" + `
sudo bash /opt/observability/query_collector.sh
head -20 /var/lib/agent-prom/mariadb_query_digests.prom
sudo systemctl list-timers --all | grep query-collector
` + "```" + `

PART 5 — Configure in the Platform

1. Go to the service config page for MariaDB / MySQL
2. Set Log Path to /var/log/mysql/error.log,/var/log/mysql/slow.log if both are available
3. Set Metrics Endpoint to http://127.0.0.1:9104/metrics
4. Set Metrics Mode to prometheus or catalog
5. Click Install Integration

PART 6 — Checklist Before Clicking Install Integration

- [ ] performance_schema is ON
- [ ] /var/log/mysql/error.log exists and has content
- [ ] /var/log/mysql/slow.log exists after running a slow query
- [ ] curl http://127.0.0.1:9104/metrics | grep mysql_up returns mysql_up 1
- [ ] /var/lib/agent-prom/mariadb_query_digests.prom exists
- [ ] The platform service config has the MariaDB log paths and metrics endpoint`,
		DetectionHints: []string{"mysql", "mariadb", "mysqld", "mariadbd"},
		NVD:            &NVDProduct{Vendor: "mariadb", Product: "mariadb"},
	},

	"redis": {
		Name:                   "redis",
		DisplayName:            "Redis",
		Category:               "cache",
		DefaultLogPaths:        []string{"/var/log/redis/redis-server.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9121/metrics",
		ExporterName:           "redis_exporter",
		ExporterPort:           9121,
		MetricsGuide: `## Redis Metrics (redis_exporter)

Redis exposes metrics via a Prometheus exporter — there is no built-in HTTP endpoint.

**Step 1 — Install redis_exporter**

` + "```bash" + `
wget https://github.com/oliver006/redis_exporter/releases/download/v1.58.0/redis_exporter-v1.58.0.linux-amd64.tar.gz
tar xzf redis_exporter-v1.58.0.linux-amd64.tar.gz
sudo mv redis_exporter-v1.58.0.linux-amd64/redis_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/redis_exporter
` + "```" + `

**Step 2 — Create systemd service**

` + "```bash" + `
sudo tee /etc/systemd/system/redis_exporter.service <<EOF
[Unit]
Description=Redis Exporter
After=network.target redis-server.service

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/redis_exporter \\
  --redis.addr redis://localhost:6379 \\
  --web.listen-address 127.0.0.1:9121 \\
  --include-system-metrics
Restart=always

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

**Step 3 — Start and verify**

` + "```bash" + `
sudo systemctl daemon-reload
sudo systemctl enable --now redis_exporter
curl -s http://127.0.0.1:9121/metrics | grep redis_up
` + "```" + `

The **--web.listen-address 127.0.0.1:9121** flag binds the exporter to localhost only — same security model as Nginx stub_status.`,
		LogsGuide: `## Redis Log Collection

**Step 1 — Configure logging in redis.conf**

Edit **/etc/redis/redis.conf**:

` + "```conf" + `
logfile /var/log/redis/redis-server.log
loglevel notice
slowlog-log-slower-than 10000
slowlog-max-len 256
` + "```" + `

**Step 2 — Create log directory**

` + "```bash" + `
sudo mkdir -p /var/log/redis
sudo chown redis:redis /var/log/redis
sudo chmod 755 /var/log/redis
` + "```" + `

**Step 3 — Restart and verify**

` + "```bash" + `
sudo systemctl restart redis-server
sudo tail -n 20 /var/log/redis/redis-server.log
` + "```" + `

Expected output:

` + "```" + `
1016:M 20 May 2026 11:38:45.591 * Ready to accept connections tcp
` + "```" + `

Log lines prefixed with **#** are warnings and errors — these are your actionable alerts.`,
		DetectionHints: []string{"redis", "redis-server"},
		NVD:            &NVDProduct{Vendor: "redis", Product: "redis"},
	},

	"phpfpm": {
		Name:                   "phpfpm",
		DisplayName:            "PHP-FPM",
		Category:               "appserver",
		DefaultLogPaths:        []string{"/var/log/php-fpm/error.log", "/var/log/php-fpm/access.log", "/var/log/php-fpm/slow.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9253/metrics",
		ExporterName:           "php-fpm_exporter",
		ExporterPort:           9253,
		MetricsGuide: `## Enable PHP-FPM Metrics

Follow the complete instructions in the LogsGuide below. Metrics come from:

- php-fpm_exporter on http://127.0.0.1:9253/metrics (pool health, workers, queue)
- the OPcache collector writing Prometheus text metrics to /var/lib/agent-prom/phpfpm_opcache.prom (optional)`,
		LogsGuide: `Complete PHP-FPM Integration Guide

BEFORE YOU START — DO NOT

* ❌ Do not skip enabling pm.status_path — without it the exporter cannot collect any pool metrics
* ❌ Do not use the default access.log format — it omits memory and CPU which are the most valuable PHP-specific fields
* ❌ Do not set request_slowlog_timeout without also setting a slowlog path — it silently does nothing
* ❌ Do not use root for the exporter — create a dedicated system user with socket read access
* ❌ Do not skip the access log — it provides script-level memory and CPU data that Nginx/Apache cannot provide

PART 1 — Configure PHP-FPM Pool

PHP-FPM configuration is split between a main config file (php-fpm.conf) and per-pool files (e.g. www.conf).
Almost everything you need to change is in the pool file.

Step 1 — Find your pool config file

` + "```bash" + `
# Debian/Ubuntu — list all pool configs (pool name may differ from "www")
sudo ls /etc/php/*/fpm/pool.d/*.conf 2>/dev/null

# RHEL/CentOS/Fedora
sudo ls /etc/php-fpm.d/*.conf 2>/dev/null
` + "```" + `

Common pool file paths (your pool name may differ):
- /etc/php/8.x/fpm/pool.d/www.conf   (Debian/Ubuntu)
- /etc/php-fpm.d/www.conf             (RHEL/CentOS)

**Important**: If you have multiple pools, repeat Steps 2–4 for each pool config file. Each pool gets its own status endpoint, socket, and log files.

Step 2 — Enable the status endpoint

Open the pool config and find or add these lines:

` + "```ini" + `
; Metrics endpoint — required for php-fpm_exporter
pm.status_path = /status

; Optional ping endpoint for uptime checks
ping.path = /ping
ping.response = pong

; Suppress health-check noise from the access log (PHP-FPM 8.0+ only)
; Check version first: php-fpm --version | head -1
; On PHP 7.4 these lines crash PHP-FPM on startup — skip this block entirely.
; Note: /status?json&full (with query string) cannot be suppressed here.
; The platform agent filters those lines automatically.
access.suppress_path[] = /status
access.suppress_path[] = /ping
access.suppress_path[] = /internal/opcache-collector
` + "```" + `

Step 3 — Enable the access log with the platform format

Find the access.log and access.format lines (they are commented out by default) and set them.
Use the $pool variable so each pool gets its own log file automatically:

` + "```ini" + `
; $pool expands to pool name (e.g. www.access.log, api.access.log)
access.log = /var/log/php-fpm/$pool.access.log

; Platform format — fields: pool | pid | method+uri | status | script | ms | KB | CPU%
access.format = "%n %p \"%m %r%Q%q\" %s %f %{milli}d %{kilo}M %C"
` + "```" + `

Step 4 — Enable the slow log

` + "```ini" + `
; $pool expands to pool name (e.g. www.slow.log, api.slow.log)
request_slowlog_timeout = 5
slowlog = /var/log/php-fpm/$pool.slow.log
` + "```" + `

Step 5 — Check and configure the error log path

The error log is set in php-fpm.conf (the main config), not in pool files. Check where it is currently:

` + "```bash" + `
grep "^error_log" /etc/php/*/fpm/php-fpm.conf     # Debian/Ubuntu
grep "^error_log" /etc/php-fpm.conf                # RHEL/CentOS
` + "```" + `

Three possible results and what to do:

- Set to a file path (e.g. /var/log/php-fpm.log): change it to the unified path below.
- Set to /dev/stderr (journald): change it to the path below (see JournaledLogGuide if you prefer journald).
- Missing or commented out: add the line below.

` + "```ini" + `
error_log = /var/log/php-fpm/error.log
` + "```" + `

After any change to php-fpm.conf, restart PHP-FPM (see Step 6).

Step 6 — Create the log directory and restart

Always test the config before restarting — a typo can take your site down.

` + "```bash" + `
sudo mkdir -p /var/log/php-fpm
sudo chown www-data:www-data /var/log/php-fpm
sudo chmod 755 /var/log/php-fpm

# Test config before restarting (prevents downtime from config errors)
# Replace X.Y with your PHP version (e.g. php-fpm8.2 --test)
sudo php-fpmX.Y --test   # Debian/Ubuntu
# OR
sudo php-fpm --test      # RHEL/CentOS

# Restart only if the test passes
sudo systemctl restart phpX.Y-fpm   # Debian/Ubuntu — replace X.Y
# OR
sudo systemctl restart php-fpm      # RHEL/CentOS
` + "```" + `

Step 7 — Verify log files are being created

Replace [pool] with your actual pool name (e.g. www, api, etc.).

` + "```bash" + `
# 1. List all PHP-FPM log files
sudo ls -la /var/log/php-fpm/

# 2. Access log (generate a request first, then check)
sudo tail -n 5 /var/log/php-fpm/[pool].access.log

# 3. Error log (should exist even if empty)
sudo ls -la /var/log/php-fpm/error.log

# 4. Slow log (will be empty until a slow request is made)
sudo ls -la /var/log/php-fpm/[pool].slow.log
` + "```" + `

If you have multiple pools, repeat for each pool: ` + "`[pool1].access.log`" + `, ` + "`[pool2].access.log`" + `, etc.

Expected access log line:

` + "```" + `
[pool-name] 12345 "GET /index.php?page=1" 200 /var/www/html/index.php 45 2048 1.23
` + "```" + `

Fields: pool | pid | "method uri?querystring" | status | script | duration_ms | memory_kb | cpu_pct

If the access log is empty, check that access.log is uncommented and the log directory is writable.

Pool metrics are verified in Part 2 Step 5 after the exporter is installed.

PART 2 — Install php-fpm_exporter

The exporter connects directly to the PHP-FPM Unix socket and exposes pool metrics in Prometheus format on port 9253. No web server is required — it speaks FastCGI directly.

Step 1 — Find your PHP-FPM socket path

` + "```bash" + `
# Check what socket PHP-FPM is using
# Check all pool configs (replace *.conf with your actual pool file)
sudo grep "listen\s*=" /etc/php/*/fpm/pool.d/*.conf 2>/dev/null || \
sudo grep "listen\s*=" /etc/php-fpm.d/*.conf 2>/dev/null
` + "```" + `

Common paths (these are examples — your actual paths will differ):
- /run/php/php-fpm.sock or /run/php/phpX.Y-fpm.sock
- /var/run/php-fpm/www.sock

Note your socket path — you will need it in Step 3.

Step 2 — Download and install the exporter

` + "```bash" + `
cd /tmp

# Check latest version at https://github.com/hipages/php-fpm_exporter/releases
wget https://github.com/hipages/php-fpm_exporter/releases/download/v2.2.0/php-fpm_exporter_2.2.0_linux_amd64.tar.gz
tar xzf php-fpm_exporter_2.2.0_linux_amd64.tar.gz
sudo mv php-fpm_exporter /usr/local/bin/
sudo chmod +x /usr/local/bin/php-fpm_exporter
` + "```" + `

Step 3 — Create a system user with socket access

` + "```bash" + `
sudo useradd --system --no-create-home --shell /usr/sbin/nologin phpfpm-exporter
sudo usermod -aG www-data phpfpm-exporter
` + "```" + `

Step 4 — Create the systemd service

Create one --phpfpm.scrape-uri flag per pool, using the socket paths you found in Step 1.

` + "```bash" + `
sudo tee /etc/systemd/system/phpfpm-exporter.service <<EOF
[Unit]
Description=PHP-FPM Exporter
After=network.target

[Service]
Type=simple
User=phpfpm-exporter
Group=phpfpm-exporter
ExecStart=/usr/local/bin/php-fpm_exporter server \\
  --phpfpm.scrape-uri "unix:///SOCKET_PATH_1;/status" \\
  --phpfpm.scrape-uri "unix:///SOCKET_PATH_2;/status" \\
  --web.listen-address "127.0.0.1:9253"
Restart=always
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

Replace SOCKET_PATH_1, SOCKET_PATH_2 etc. with the actual socket paths from Step 1.
If you have only one pool, just use a single --phpfpm.scrape-uri flag.

Step 5 — Start and verify

` + "```bash" + `
sudo systemctl daemon-reload
sudo systemctl enable --now phpfpm-exporter
sudo systemctl status phpfpm-exporter
curl -s http://127.0.0.1:9253/metrics | grep phpfpm_up
` + "```" + `

Expected output:

` + "```" + `
phpfpm_up{pool="www"} 1
phpfpm_up{pool="laravel"} 1
` + "```" + `

If phpfpm_up is 0 for a pool, the exporter cannot reach that pool's socket. Check the socket path and that the phpfpm-exporter user is in the web server group.

PART 3 — Configure Log Collection

Your platform agent already monitors the log paths set in Part 1. No additional software is needed — just verify the paths are correct.

If you used the ` + "`$pool`" + ` variable as recommended, each pool gets its own access and slow log file:

| Log | Path Pattern | What It Contains |
|-----|-------------|-----------------|
| Error log | /var/log/php-fpm/error.log | Worker crashes, max children events, startup errors (one per FPM master) |
| Access log | /var/log/php-fpm/[pool].access.log | Per-request: URI, status, script, duration, memory, CPU (one per pool) |
| Slow log | /var/log/php-fpm/[pool].slow.log | Requests exceeding slowlog timeout with PHP backtrace (one per pool) |

Example with two pools (www and api):
- /var/log/php-fpm/www.access.log
- /var/log/php-fpm/www.slow.log
- /var/log/php-fpm/api.access.log
- /var/log/php-fpm/api.slow.log
- /var/log/php-fpm/error.log

If your distro uses different paths, update them in the platform config page:

` + "```bash" + `
# Check actual log paths (replace X.Y with your PHP version)
sudo php-fpmX.Y --test 2>&1 | grep -i log   # Debian/Ubuntu
# OR
sudo php-fpm --test 2>&1 | grep -i log      # RHEL/CentOS
` + "```" + `

PART 4 — OPcache Collector (Optional — Bonus Dashboard)

This is the same pattern as the MariaDB query digest collector. A PHP script calls opcache_get_status() and the collector writes Prometheus metrics to /var/lib/agent-prom/phpfpm_opcache.prom, which the platform agent already picks up through its prom-file-server.

Skip this part if you do not need the OPcache dashboard.

Step 1 — Install cgi-fcgi (needed to call PHP-FPM from bash)

` + "```bash" + `
# Debian/Ubuntu
sudo apt-get install -y libfcgi-bin

# RHEL/CentOS
sudo yum install -y fcgi
` + "```" + `

Step 2 — Create the PHP script

` + "```bash" + `
sudo mkdir -p /opt/observability
sudo tee /opt/observability/opcache_status.php <<'EOF'
<?php
// Called by opcache_collector.sh via cgi-fcgi
// Outputs Prometheus metrics from opcache_get_status()

if (!function_exists('opcache_get_status')) {
    // OPcache not enabled — emit a single gauge so the dashboard shows clearly
    echo "# HELP phpfpm_opcache_enabled Whether OPcache is enabled\n";
    echo "# TYPE phpfpm_opcache_enabled gauge\n";
    echo "phpfpm_opcache_enabled 0\n";
    exit(0);
}

$s = opcache_get_status(false);
$m = $s['memory_usage'];
$st = $s['opcache_statistics'];

$used   = $m['used_memory'];
$free   = $m['free_memory'];
$wasted = $m['wasted_memory'];
$total  = $used + $free + $wasted;

echo "# HELP phpfpm_opcache_enabled Whether OPcache is enabled\n";
echo "# TYPE phpfpm_opcache_enabled gauge\n";
echo "phpfpm_opcache_enabled " . ($s['opcache_enabled'] ? '1' : '0') . "\n";

echo "# HELP phpfpm_opcache_memory_used_bytes Memory used by OPcache\n";
echo "# TYPE phpfpm_opcache_memory_used_bytes gauge\n";
echo "phpfpm_opcache_memory_used_bytes {$used}\n";

echo "# HELP phpfpm_opcache_memory_free_bytes Memory free in OPcache\n";
echo "# TYPE phpfpm_opcache_memory_free_bytes gauge\n";
echo "phpfpm_opcache_memory_free_bytes {$free}\n";

echo "# HELP phpfpm_opcache_memory_wasted_bytes Wasted memory in OPcache\n";
echo "# TYPE phpfpm_opcache_memory_wasted_bytes gauge\n";
echo "phpfpm_opcache_memory_wasted_bytes {$wasted}\n";

echo "# HELP phpfpm_opcache_memory_total_bytes Total OPcache memory allocated\n";
echo "# TYPE phpfpm_opcache_memory_total_bytes gauge\n";
echo "phpfpm_opcache_memory_total_bytes {$total}\n";

echo "# HELP phpfpm_opcache_memory_used_ratio Memory used ratio (0–1)\n";
echo "# TYPE phpfpm_opcache_memory_used_ratio gauge\n";
echo "phpfpm_opcache_memory_used_ratio " . round($total > 0 ? $used / $total : 0, 6) . "\n";

echo "# HELP phpfpm_opcache_wasted_ratio Wasted memory ratio (0–1)\n";
echo "# TYPE phpfpm_opcache_wasted_ratio gauge\n";
echo "phpfpm_opcache_wasted_ratio " . round($total > 0 ? $wasted / $total : 0, 6) . "\n";

echo "# HELP phpfpm_opcache_hits_total Total cache hits since reset\n";
echo "# TYPE phpfpm_opcache_hits_total counter\n";
echo "phpfpm_opcache_hits_total {$st['hits']}\n";

echo "# HELP phpfpm_opcache_misses_total Total cache misses since reset\n";
echo "# TYPE phpfpm_opcache_misses_total counter\n";
echo "phpfpm_opcache_misses_total {$st['misses']}\n";

echo "# HELP phpfpm_opcache_hit_rate Cache hit rate (0–1)\n";
echo "# TYPE phpfpm_opcache_hit_rate gauge\n";
echo "phpfpm_opcache_hit_rate " . round($st['opcache_hit_rate'] / 100, 6) . "\n";

echo "# HELP phpfpm_opcache_cached_scripts_total Number of scripts currently cached\n";
echo "# TYPE phpfpm_opcache_cached_scripts_total gauge\n";
echo "phpfpm_opcache_cached_scripts_total {$st['num_cached_scripts']}\n";

echo "# HELP phpfpm_opcache_restarts_total Number of cache restarts since server start\n";
echo "# TYPE phpfpm_opcache_restarts_total counter\n";
echo "phpfpm_opcache_restarts_total {$st['oom_restarts']}\n";

echo "# HELP phpfpm_opcache_manual_restarts_total Manual restarts (opcache_reset calls)\n";
echo "# TYPE phpfpm_opcache_manual_restarts_total counter\n";
echo "phpfpm_opcache_manual_restarts_total {$st['manual_restarts']}\n";
EOF
sudo chmod 644 /opt/observability/opcache_status.php
` + "```" + `

Step 3 — Create /opt/observability/opcache_collector.sh

` + "```bash" + `
sudo tee /opt/observability/opcache_collector.sh <<'EOF'
#!/bin/bash
# OPcache Collector — emits Prometheus metrics for the platform agent to scrape
# Calls opcache_status.php through PHP-FPM via FastCGI (no web server needed)
# Destination: /var/lib/agent-prom/phpfpm_opcache.prom

set -euo pipefail

SOCKET="YOUR_POOL_SOCKET"   # Replace with any pool socket path from Step 1
PHP_SCRIPT="/opt/observability/opcache_status.php"
PROM_DIR="/var/lib/agent-prom"
PROM_FILE="${PROM_DIR}/phpfpm_opcache.prom"
TMP_FILE=$(mktemp)

mkdir -p "$PROM_DIR"

# Call PHP-FPM via FastCGI — cgi-fcgi handles the FastCGI protocol
# SCRIPT_FILENAME tells PHP-FPM which script to execute
# SCRIPT_NAME + REQUEST_URI are set to a unique path so you can suppress
# these collector requests from the access log (see Part 1 Step 2).
SCRIPT_FILENAME="$PHP_SCRIPT" \
SCRIPT_NAME="/internal/opcache-collector" \
REQUEST_URI="/internal/opcache-collector" \
QUERY_STRING="" \
REQUEST_METHOD="GET" \
cgi-fcgi -bind -connect "$SOCKET" 2>/dev/null \
  | sed '1,/^\r$/d' \
  > "$TMP_FILE"

# Verify the output looks like Prometheus metrics before replacing
if grep -q "^# HELP phpfpm_opcache" "$TMP_FILE"; then
    mv "$TMP_FILE" "$PROM_FILE"
    COUNT=$(grep -c "^phpfpm_opcache_enabled" "$PROM_FILE" 2>/dev/null || echo 0)
    echo "Emitted OPcache metrics to $PROM_FILE (opcache_enabled lines: $COUNT)"
else
    rm -f "$TMP_FILE"
    echo "ERROR: PHP script did not return valid Prometheus metrics — check SOCKET path and PHP-FPM status" >&2
    exit 1
fi
EOF
sudo chmod +x /opt/observability/opcache_collector.sh
` + "```" + `

Replace YOUR_POOL_SOCKET with any pool socket path from Part 2 Step 1.

If you followed Part 1 Step 2, the /internal/opcache-collector path is
already suppressed from the access log — no extra log noise from this collector.

Step 4 — Create the timer

` + "```bash" + `
sudo tee /etc/systemd/system/opcache-collector.service <<EOF
[Unit]
Description=OPcache Metrics Collector (runs once per trigger)
After=php-fpm.service

[Service]
Type=oneshot
ExecStart=/bin/bash /opt/observability/opcache_collector.sh
StandardOutput=journal
StandardError=journal
EOF

sudo tee /etc/systemd/system/opcache-collector.timer <<EOF
[Unit]
Description=Run OPcache Collector every 60 seconds

[Timer]
OnBootSec=30
OnUnitActiveSec=60s
AccuracySec=1s

[Install]
WantedBy=timers.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now opcache-collector.timer
` + "```" + `

Step 5 — Verify

` + "```bash" + `
sudo bash /opt/observability/opcache_collector.sh
head -20 /var/lib/agent-prom/phpfpm_opcache.prom
sudo systemctl list-timers --all | grep opcache
` + "```" + `

Expected output:

` + "```" + `
# HELP phpfpm_opcache_enabled Whether OPcache is enabled
# TYPE phpfpm_opcache_enabled gauge
phpfpm_opcache_enabled 1
# HELP phpfpm_opcache_hit_rate Cache hit rate (0-1)
# TYPE phpfpm_opcache_hit_rate gauge
phpfpm_opcache_hit_rate 0.971200
` + "```" + `

PART 5 — Configure in the Platform

After completing Parts 1-3 (and optionally Part 4):

1. Go to the service config page for PHP-FPM
2. Set Log Paths to all paths from Part 3 — error log plus one access and slow log per pool
3. Set Metrics Endpoint to http://127.0.0.1:9253/metrics
4. Set Metrics Mode to prometheus
5. Click Install Integration

PART 6 — What You Will See in the Platform

From metrics (php-fpm_exporter):

| Metric | What it tells you |
|--------|-------------------|
| Active workers | How many PHP processes are currently handling requests |
| Idle workers | How many PHP processes are waiting for work |
| Worker saturation % | active / total × 100 — alert above 85% |
| Queue length | Requests waiting because all workers are busy — alert above 0 |
| Queue utilization % | listen_queue / listen_queue_len × 100 — alert above 50% |
| Max children reached | How often PHP wanted more workers but hit the pm.max_children limit |
| Accepted connections/s | Overall request throughput handled by PHP-FPM |
| Slow requests | Cumulative count of requests that exceeded request_slowlog_timeout |

From access log:

| Event | What you learn |
|-------|----------------|
| High memory per request | Which script is allocating excessive memory — not visible in Nginx logs |
| High CPU per request | Which script is burning CPU — not visible in Nginx logs |
| Slow duration | Response time at the PHP layer (excludes Nginx overhead) |
| 5xx status codes | PHP-level errors before Nginx sees them |
| Top URIs | Which endpoints receive the most PHP traffic |

From slow log:

| Event | What you learn |
|-------|----------------|
| Slow request entry | Exact script path, elapsed time, and which PHP function caused the slowness |
| Backtrace | Full call stack at the moment the threshold was exceeded |

From error log:

| Event | What you learn |
|-------|----------------|
| WARNING: ...max_children | PHP-FPM rejected requests because the worker pool was full |
| child ... exited | A worker process crashed — check the PHP error log for the cause |
| Allowed memory size exhausted | A request hit the PHP memory_limit |
| PHP Fatal error | Uncaught exception or fatal stopping a worker |

From OPcache collector (if enabled):

| Metric | What it tells you |
|--------|-------------------|
| Hit rate | Below 90% means PHP is re-compiling scripts — check memory_consumption in php.ini |
| Memory used % | Above 80% means OPcache is evicting scripts to make room |
| Wasted memory | High wasted % after deployments — consider opcache_reset() in deploy scripts |
| Restarts | OOM restarts mean OPcache ran out of memory and had to clear itself |

PART 7 — Checklist Before Clicking Install Integration

- [ ] pm.status_path = /status is set in the pool config
- [ ] access.suppress_path[] entries are set for /status, /ping, /internal/opcache-collector (/status?json&full handled by agent filter)
- [ ] pm.status_path = /status is set in every pool config
- [ ] access.suppress_path[] entries are set (PHP 8.0+ only) or you know they are absent on 7.4
- [ ] access.log is set and /var/log/php-fpm/[pool].access.log has lines after a test request (repeat per pool)
- [ ] access.format is exactly "%n %p \"%m %r%Q%q\" %s %f %{milli}d %{kilo}M %C"
- [ ] slowlog and request_slowlog_timeout are both set in every pool config
- [ ] curl http://127.0.0.1:9253/metrics | grep phpfpm_up returns phpfpm_up{pool="..."} 1 for every pool
- [ ] /var/log/php-fpm/error.log exists
- [ ] All log paths and the metrics endpoint are entered in the platform config page
- [ ] (Optional) /var/lib/agent-prom/phpfpm_opcache.prom exists if OPcache collector is enabled`,
		JournaledLogGuide: `PHP-FPM may be logging to journald if error_log is not set or points to /dev/stderr.

Check (replace the unit name with your PHP-FPM service name):

` + "```bash" + `
sudo journalctl -u php-fpm --no-pager | head -10    # RHEL/CentOS
# OR
sudo journalctl -u phpX.Y-fpm --no-pager | head -10  # Debian/Ubuntu — replace X.Y
` + "```" + `

If you see PHP-FPM output there, follow PART 1 Step 5 to set error_log to a file path.`,
		DetectionHints: []string{"php-fpm", "php8", "php7", "php-fpm8", "php-fpm7"},
		NVD:            &NVDProduct{Vendor: "php", Product: "php"},
	},
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func Get(name string) (ServiceDef, bool) {
	def, ok := Catalog[name]
	return def, ok
}

func IsSupported(name string) bool {
	_, ok := Catalog[name]
	return ok
}

func List() []ServiceDef {
	out := make([]ServiceDef, 0, len(Catalog))
	for _, v := range Catalog {
		out = append(out, v)
	}
	return out
}

func ByCategory(category string) []ServiceDef {
	var out []ServiceDef
	for _, v := range Catalog {
		if v.Category == category {
			out = append(out, v)
		}
	}
	return out
}

func NVDServices() []ServiceDef {
	var out []ServiceDef
	for _, v := range Catalog {
		if v.NVD != nil {
			out = append(out, v)
		}
	}
	return out
}


