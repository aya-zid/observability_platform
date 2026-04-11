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
	LoggingModeStdout   LoggingMode = "stdout" // goes to journald via systemd
)

const (
	// MetricsPatternBuiltIn — service already exposes /metrics or a stats endpoint.
	// User only needs to enable it in config. otelcol scrapes it directly.
	MetricsPatternBuiltIn MetricsPattern = "builtin"

	// MetricsPatternExporter — no built-in Prometheus endpoint.
	// A separate exporter process must be installed alongside the service.
	MetricsPatternExporter MetricsPattern = "exporter"

	// MetricsPatternNone — service has no useful metrics endpoint.
	// Log collection only.
	MetricsPatternNone MetricsPattern = "none"
)

// NVDProduct is the CPE vendor/product pair used to query the NVD API for CVEs.
type NVDProduct struct {
	Vendor  string `json:"vendor"`
	Product string `json:"product"`
}

type ServiceDef struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"` // web, database, queue, infrastructure, security, observability, appserver, mail, dns, secrets

	// Logging
	DefaultLogPaths    []string    `json:"default_log_paths"`
	DefaultLoggingMode LoggingMode `json:"default_logging_mode"`

	// Metrics
	MetricsPattern         MetricsPattern `json:"metrics_pattern"`
	DefaultMetricsEndpoint string         `json:"default_metrics_endpoint"` // empty if PatternNone
	ExporterName           string         `json:"exporter_name,omitempty"`  // e.g. "postgres_exporter"
	ExporterPort           int            `json:"exporter_port,omitempty"`

	// Guides — plain text with markdown, rendered in the UI
	MetricsGuide      string `json:"metrics_guide"`
	LogsGuide         string `json:"logs_guide"`
	JournaledLogGuide string `json:"journaled_log_guide,omitempty"` // only set when DefaultLoggingMode is journald/stdout

	// Detection
	DetectionHints []string `json:"detection_hints"` // process names / systemd unit name fragments

	// CVE checking
	NVD *NVDProduct `json:"nvd,omitempty"` // nil = skip CVE check for this service
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog
// ─────────────────────────────────────────────────────────────────────────────

var Catalog = map[string]ServiceDef{

	// ═══════════════════════════════════════════════════════════════════
	// WEB SERVERS & PROXIES
	// ═══════════════════════════════════════════════════════════════════

	"nginx": {
		Name:                   "nginx",
		DisplayName:            "Nginx",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/nginx/access.log", "/var/log/nginx/error.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:80/nginx_status",
		MetricsGuide: `## Enable Nginx Metrics (stub_status)

Nginx exposes basic connection metrics via the built-in stub_status module.

**Step 1 — Add the status location block**

Edit your nginx server config (usually /etc/nginx/sites-enabled/default or /etc/nginx/nginx.conf) and add inside a server block:

` + "```" + `nginx
location /nginx_status {
    stub_status on;
    allow 127.0.0.1;
    deny all;
}
` + "```" + `

**Step 2 — Test and reload**

` + "```" + `bash
sudo nginx -t
sudo nginx -s reload
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:80/nginx_status
` + "```" + `

Expected output:
` + "```" + `
Active connections: 3
server accepts handled requests
 10 10 15
Reading: 0 Writing: 1 Waiting: 2
` + "```" + `

Once you see this output, click "Install Integration" and the platform will start collecting metrics automatically.`,
		LogsGuide: `## Nginx Log Collection

Nginx writes to files by default. No changes are usually needed.

**Verify log files exist:**

` + "```" + `bash
ls -la /var/log/nginx/
` + "```" + `

You should see access.log and error.log. If they are empty, nginx may not have received any requests yet — that is normal.

**If logs are in a custom location**, update the log path in the platform using the edit button above.

**Log format note:** The platform expects Combined Log Format (the nginx default). If you use a custom log_format, update your nginx.conf to include the standard fields.`,
		DetectionHints: []string{"nginx", "openresty"},
		NVD:            &NVDProduct{Vendor: "f5", Product: "nginx_open_source"},
	},

	"apache": {
		Name:                   "apache",
		DisplayName:            "Apache HTTP Server",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/apache2/access.log", "/var/log/apache2/error.log", "/var/log/httpd/access_log", "/var/log/httpd/error_log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:80/server-status?auto",
		MetricsGuide: `## Enable Apache Metrics (mod_status)

**Step 1 — Enable mod_status**

` + "```" + `bash
# Debian/Ubuntu
sudo a2enmod status

# RHEL/CentOS — mod_status is usually compiled in, skip this step
` + "```" + `

**Step 2 — Add the status location**

Edit /etc/apache2/mods-enabled/status.conf (Debian) or add to your VirtualHost config:

` + "```" + `apache
<Location "/server-status">
    SetHandler server-status
    Require ip 127.0.0.1
</Location>
` + "```" + `

**Step 3 — Restart Apache**

` + "```" + `bash
# Debian/Ubuntu
sudo systemctl restart apache2

# RHEL/CentOS
sudo systemctl restart httpd
` + "```" + `

**Step 4 — Verify**

` + "```" + `bash
curl http://127.0.0.1:80/server-status?auto
` + "```" + ``,
		LogsGuide: `## Apache Log Collection

Apache writes access and error logs to files by default.

**Common log locations:**
- Debian/Ubuntu: /var/log/apache2/
- RHEL/CentOS: /var/log/httpd/

**Verify:**

` + "```" + `bash
ls -la /var/log/apache2/
# or
ls -la /var/log/httpd/
` + "```" + `

If your logs are in a custom location (set via CustomLog/ErrorLog directives), update the path in the platform.`,
		DetectionHints: []string{"apache2", "httpd", "apache"},
		NVD:            &NVDProduct{Vendor: "apache", Product: "http_server"},
	},

	"caddy": {
		Name:                   "caddy",
		DisplayName:            "Caddy",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/caddy/access.log"},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:2019/metrics",
		MetricsGuide: `## Enable Caddy Metrics

Caddy has a built-in Prometheus metrics endpoint via its admin API.

**Step 1 — The admin API is enabled by default on port 2019**

Verify it is running:

` + "```" + `bash
curl http://127.0.0.1:2019/metrics
` + "```" + `

If that works, you are done — click "Install Integration".

**If the admin API is disabled**, add this to your Caddyfile:

` + "```" + `
{
    admin 127.0.0.1:2019
}
` + "```" + `

Then reload: ` + "`sudo systemctl reload caddy`",
		LogsGuide: `## Caddy Log Collection

By default Caddy logs to stdout (captured by systemd/journald). To collect structured logs, configure file logging in your Caddyfile.

**Step 1 — Add file logging to your Caddyfile**

` + "```" + `
{
    log {
        output file /var/log/caddy/access.log {
            roll_size 100mb
            roll_keep 5
        }
        format json
    }
}
` + "```" + `

**Step 2 — Create the log directory**

` + "```" + `bash
sudo mkdir -p /var/log/caddy
sudo chown caddy:caddy /var/log/caddy
` + "```" + `

**Step 3 — Reload Caddy**

` + "```" + `bash
sudo systemctl reload caddy
` + "```" + ``,
		JournaledLogGuide: `Caddy logs to stdout by default, which is captured by systemd journald.

To redirect to a file, add file logging to your Caddyfile as shown in the Logs Guide above, then update the log path in the platform.`,
		DetectionHints: []string{"caddy"},
		NVD:            &NVDProduct{Vendor: "caddyserver", Product: "caddy"},
	},

	"traefik": {
		Name:                   "traefik",
		DisplayName:            "Traefik",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/traefik/traefik.log", "/var/log/traefik/access.log"},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8080/metrics",
		MetricsGuide: `## Enable Traefik Metrics

**Step 1 — Enable Prometheus metrics in traefik.yml**

` + "```" + `yaml
metrics:
  prometheus:
    entryPoint: metrics
    addEntryPointsLabels: true
    addServicesLabels: true

entryPoints:
  metrics:
    address: ":8082"
` + "```" + `

Or if using the dashboard entryPoint (port 8080):

` + "```" + `yaml
api:
  dashboard: true
  insecure: true   # only for internal/dev use

metrics:
  prometheus: {}
` + "```" + `

**Step 2 — Restart Traefik**

` + "```" + `bash
sudo systemctl restart traefik
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:8080/metrics | head -20
` + "```" + ``,
		LogsGuide: `## Traefik Log Collection

Traefik logs to stdout by default. To write to files, configure file logging.

**Step 1 — Add to traefik.yml**

` + "```" + `yaml
log:
  filePath: /var/log/traefik/traefik.log
  level: INFO

accessLog:
  filePath: /var/log/traefik/access.log
  bufferingSize: 100
` + "```" + `

**Step 2 — Create the directory and restart**

` + "```" + `bash
sudo mkdir -p /var/log/traefik
sudo chown traefik:traefik /var/log/traefik
sudo systemctl restart traefik
` + "```" + ``,
		JournaledLogGuide: `Traefik logs to stdout which goes to journald. Configure file logging in traefik.yml as shown in the Logs Guide, then update the log path in the platform.`,
		DetectionHints:    []string{"traefik"},
		NVD:               &NVDProduct{Vendor: "traefik", Product: "traefik"},
	},

	"haproxy": {
		Name:                   "haproxy",
		DisplayName:            "HAProxy",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/haproxy.log"},
		DefaultLoggingMode:     LoggingModeSyslog,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8404/metrics",
		MetricsGuide: `## Enable HAProxy Metrics

**Step 1 — Add a Prometheus frontend to haproxy.cfg**

` + "```" + `
frontend stats
    bind 127.0.0.1:8404
    mode http
    http-request use-service prometheus-exporter if { path /metrics }
    stats enable
    stats uri /stats
    stats refresh 10s
` + "```" + `

**Step 2 — Restart HAProxy**

` + "```" + `bash
sudo systemctl restart haproxy
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:8404/metrics | head -20
` + "```" + `

Note: The Prometheus metrics endpoint is built into HAProxy 2.0+. If you are on an older version, upgrade or use haproxy_exporter separately.`,
		LogsGuide: `## HAProxy Log Collection

HAProxy sends logs to syslog by default (local0 facility).

**Step 1 — Verify haproxy.cfg has logging configured**

` + "```" + `
global
    log 127.0.0.1 local0
    log 127.0.0.1 local1 notice
` + "```" + `

**Step 2 — Configure rsyslog to write HAProxy logs to a file**

Create /etc/rsyslog.d/haproxy.conf:

` + "```" + `
local0.* /var/log/haproxy.log
& stop
` + "```" + `

**Step 3 — Restart rsyslog**

` + "```" + `bash
sudo systemctl restart rsyslog
` + "```" + `

**Step 4 — Verify**

` + "```" + `bash
tail -f /var/log/haproxy.log
` + "```" + ``,
		JournaledLogGuide: `HAProxy logs via syslog (local0). If no file exists at /var/log/haproxy.log, configure rsyslog to write it as shown in the Logs Guide, then update the path in the platform.`,
		DetectionHints:    []string{"haproxy"},
		NVD:               &NVDProduct{Vendor: "haproxy", Product: "haproxy"},
	},

	"varnish": {
		Name:                   "varnish",
		DisplayName:            "Varnish Cache",
		Category:               "web",
		DefaultLogPaths:        []string{"/var/log/varnish/varnishncsa.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9131/metrics",
		ExporterName:           "varnish_exporter",
		ExporterPort:           9131,
		MetricsGuide: `## Enable Varnish Metrics (varnish_exporter)

Varnish does not expose Prometheus metrics natively. Install varnish_exporter.

**Step 1 — Download and install**

` + "```" + `bash
wget https://github.com/jonnenauha/prometheus_varnish_exporter/releases/latest/download/prometheus_varnish_exporter_linux_amd64.tar.gz
tar xzf prometheus_varnish_exporter_linux_amd64.tar.gz
sudo mv prometheus_varnish_exporter /usr/local/bin/varnish_exporter
` + "```" + `

**Step 2 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/varnish_exporter.service > /dev/null <<EOF
[Unit]
Description=Varnish Exporter
After=varnish.service

[Service]
User=nobody
ExecStart=/usr/local/bin/varnish_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

**Step 3 — Start the exporter**

` + "```" + `bash
sudo systemctl daemon-reload
sudo systemctl enable --now varnish_exporter
` + "```" + `

**Step 4 — Verify**

` + "```" + `bash
curl http://127.0.0.1:9131/metrics | head -20
` + "```" + ``,
		LogsGuide: `## Varnish Log Collection

Varnish logs via its own logging daemon (varnishncsa for access logs).

**Step 1 — Enable varnishncsa to write to file**

` + "```" + `bash
sudo tee /etc/systemd/system/varnishncsa.service > /dev/null <<EOF
[Unit]
Description=Varnish NCSA Log Daemon
After=varnish.service

[Service]
ExecStart=/usr/bin/varnishncsa -a -w /var/log/varnish/varnishncsa.log -D -P /run/varnishncsa.pid
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

**Step 2 — Create log dir and start**

` + "```" + `bash
sudo mkdir -p /var/log/varnish
sudo systemctl daemon-reload
sudo systemctl enable --now varnishncsa
` + "```" + ``,
		DetectionHints: []string{"varnish", "varnishd"},
		NVD:            &NVDProduct{Vendor: "varnish-cache", Product: "varnish"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// DATABASES
	// ═══════════════════════════════════════════════════════════════════

	"postgresql": {
		Name:                   "postgresql",
		DisplayName:            "PostgreSQL",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/postgresql/*.log", "/var/lib/pgsql/data/log/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9187/metrics",
		ExporterName:           "postgres_exporter",
		ExporterPort:           9187,
		MetricsGuide: `## Enable PostgreSQL Metrics (postgres_exporter)

**Step 1 — Create a monitoring user in PostgreSQL**

` + "```" + `bash
sudo -u postgres psql <<SQL
CREATE USER monitoring WITH PASSWORD 'monitoring_password';
GRANT pg_monitor TO monitoring;
SQL
` + "```" + `

**Step 2 — Install postgres_exporter**

` + "```" + `bash
wget https://github.com/prometheus-community/postgres_exporter/releases/latest/download/postgres_exporter-linux-amd64.tar.gz
tar xzf postgres_exporter-linux-amd64.tar.gz
sudo mv postgres_exporter-linux-amd64/postgres_exporter /usr/local/bin/
` + "```" + `

**Step 3 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/postgres_exporter.service > /dev/null <<EOF
[Unit]
Description=PostgreSQL Exporter
After=postgresql.service

[Service]
User=postgres
Environment=DATA_SOURCE_NAME=postgresql://monitoring:monitoring_password@localhost:5432/postgres?sslmode=disable
ExecStart=/usr/local/bin/postgres_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF
` + "```" + `

**Step 4 — Start the exporter**

` + "```" + `bash
sudo systemctl daemon-reload
sudo systemctl enable --now postgres_exporter
` + "```" + `

**Step 5 — Verify**

` + "```" + `bash
curl http://127.0.0.1:9187/metrics | grep pg_up
` + "```" + ``,
		LogsGuide: `## PostgreSQL Log Collection

PostgreSQL must have logging_collector enabled to write logs to files.

**Step 1 — Check current status**

` + "```" + `bash
sudo -u postgres psql -c "SHOW logging_collector;"
` + "```" + `

**Step 2 — If logging_collector is OFF, enable it**

` + "```" + `bash
sudo -u postgres psql <<SQL
ALTER SYSTEM SET logging_collector = on;
ALTER SYSTEM SET log_directory = 'log';
ALTER SYSTEM SET log_filename = 'postgresql-%Y-%m-%d.log';
ALTER SYSTEM SET log_min_messages = 'info';
ALTER SYSTEM SET log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h ';
SQL
` + "```" + `

**Step 3 — Restart PostgreSQL**

` + "```" + `bash
sudo systemctl restart postgresql
` + "```" + `

**Step 4 — Find log path**

` + "```" + `bash
sudo -u postgres psql -c "SHOW data_directory;"
# Logs will be at <data_directory>/log/postgresql-YYYY-MM-DD.log
` + "```" + `

Update the log path in the platform with the full path, e.g. /var/lib/postgresql/14/main/log/*.log`,
		JournaledLogGuide: `PostgreSQL with logging_collector=off writes logs to journald via systemd.

Enable logging_collector as described in the Logs Guide above, then restart PostgreSQL and update the log path in the platform.`,
		DetectionHints: []string{"postgres", "postgresql"},
		NVD:            &NVDProduct{Vendor: "postgresql", Product: "postgresql"},
	},

	"mysql": {
		Name:                   "mysql",
		DisplayName:            "MySQL / MariaDB",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/mysql/error.log", "/var/log/mysql/mysql.log", "/var/log/mysqld.log", "/var/log/mariadb/mariadb.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9104/metrics",
		ExporterName:           "mysqld_exporter",
		ExporterPort:           9104,
		MetricsGuide: `## Enable MySQL/MariaDB Metrics (mysqld_exporter)

**Step 1 — Create a monitoring user**

` + "```" + `bash
sudo mysql -u root <<SQL
CREATE USER 'monitoring'@'127.0.0.1' IDENTIFIED BY 'monitoring_password';
GRANT PROCESS, REPLICATION CLIENT, SELECT ON *.* TO 'monitoring'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL
` + "```" + `

**Step 2 — Install mysqld_exporter**

` + "```" + `bash
wget https://github.com/prometheus-community/mysqld_exporter/releases/latest/download/mysqld_exporter-linux-amd64.tar.gz
tar xzf mysqld_exporter-linux-amd64.tar.gz
sudo mv mysqld_exporter-linux-amd64/mysqld_exporter /usr/local/bin/
` + "```" + `

**Step 3 — Create credentials file**

` + "```" + `bash
sudo tee /etc/mysql/.mysqld_exporter.cnf > /dev/null <<EOF
[client]
user=monitoring
password=monitoring_password
host=127.0.0.1
EOF
sudo chmod 600 /etc/mysql/.mysqld_exporter.cnf
` + "```" + `

**Step 4 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/mysqld_exporter.service > /dev/null <<EOF
[Unit]
Description=MySQL Exporter
After=mysql.service

[Service]
User=mysql
ExecStart=/usr/local/bin/mysqld_exporter --config.my-cnf=/etc/mysql/.mysqld_exporter.cnf
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now mysqld_exporter
` + "```" + `

**Verify:** ` + "`curl http://127.0.0.1:9104/metrics | grep mysql_up`",
		LogsGuide: `## MySQL/MariaDB Log Collection

**Step 1 — Check current log configuration**

` + "```" + `bash
sudo mysql -u root -e "SELECT @@log_error, @@general_log_file, @@general_log;"
` + "```" + `

**Step 2 — Enable error logging in my.cnf**

Edit /etc/mysql/mysql.conf.d/mysqld.cnf (Debian) or /etc/my.cnf (RHEL):

` + "```" + `ini
[mysqld]
log_error = /var/log/mysql/error.log
general_log = 1
general_log_file = /var/log/mysql/mysql.log
slow_query_log = 1
slow_query_log_file = /var/log/mysql/slow.log
long_query_time = 2
` + "```" + `

**Step 3 — Restart MySQL**

` + "```" + `bash
sudo systemctl restart mysql
# or: sudo systemctl restart mariadb
` + "```" + ``,
		DetectionHints: []string{"mysql", "mysqld", "mariadb", "mariadbd"},
		NVD:            &NVDProduct{Vendor: "oracle", Product: "mysql"},
	},

	"redis": {
		Name:                   "redis",
		DisplayName:            "Redis",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/redis/redis-server.log", "/var/log/redis.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9121/metrics",
		ExporterName:           "redis_exporter",
		ExporterPort:           9121,
		MetricsGuide: `## Enable Redis Metrics (redis_exporter)

**Step 1 — Install redis_exporter**

` + "```" + `bash
wget https://github.com/oliver006/redis_exporter/releases/latest/download/redis_exporter-linux-amd64.tar.gz
tar xzf redis_exporter-linux-amd64.tar.gz
sudo mv redis_exporter /usr/local/bin/
` + "```" + `

**Step 2 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/redis_exporter.service > /dev/null <<EOF
[Unit]
Description=Redis Exporter
After=redis.service

[Service]
User=redis
ExecStart=/usr/local/bin/redis_exporter --redis.addr=redis://127.0.0.1:6379
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now redis_exporter
` + "```" + `

If Redis requires a password, add: ` + "`--redis.password=yourpassword`" + ` to ExecStart.

**Verify:** ` + "`curl http://127.0.0.1:9121/metrics | grep redis_up`",
		LogsGuide: `## Redis Log Collection

Redis logs to stdout by default. To write logs to a file, edit redis.conf.

**Step 1 — Find your redis.conf**

` + "```" + `bash
redis-cli CONFIG GET logfile
` + "```" + `

**Step 2 — Set a log file path**

Edit /etc/redis/redis.conf or /etc/redis.conf:

` + "```" + `
logfile /var/log/redis/redis-server.log
loglevel notice
` + "```" + `

**Step 3 — Create the directory and restart**

` + "```" + `bash
sudo mkdir -p /var/log/redis
sudo chown redis:redis /var/log/redis
sudo systemctl restart redis
` + "```" + `

**Verify:**

` + "```" + `bash
tail -f /var/log/redis/redis-server.log
` + "```" + ``,
		JournaledLogGuide: `Redis logs to stdout which is captured by journald when run via systemd. Configure a logfile in redis.conf as shown in the Logs Guide, then update the path in the platform.`,
		DetectionHints:    []string{"redis-server", "redis"},
		NVD:               &NVDProduct{Vendor: "redis", Product: "redis"},
	},

	"mongodb": {
		Name:                   "mongodb",
		DisplayName:            "MongoDB",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/mongodb/mongod.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9216/metrics",
		ExporterName:           "mongodb_exporter",
		ExporterPort:           9216,
		MetricsGuide: `## Enable MongoDB Metrics (mongodb_exporter)

We use the Percona mongodb_exporter.

**Step 1 — Create a monitoring user in MongoDB**

` + "```" + `bash
mongosh admin <<JS
db.createUser({
  user: "monitoring",
  pwd: "monitoring_password",
  roles: [
    { role: "clusterMonitor", db: "admin" },
    { role: "read", db: "local" }
  ]
});
JS
` + "```" + `

**Step 2 — Install mongodb_exporter**

` + "```" + `bash
wget https://github.com/percona/mongodb_exporter/releases/latest/download/mongodb_exporter-linux-amd64.tar.gz
tar xzf mongodb_exporter-linux-amd64.tar.gz
sudo mv mongodb_exporter /usr/local/bin/
` + "```" + `

**Step 3 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/mongodb_exporter.service > /dev/null <<EOF
[Unit]
Description=MongoDB Exporter
After=mongod.service

[Service]
User=mongodb
Environment=MONGODB_URI=mongodb://monitoring:monitoring_password@127.0.0.1:27017/admin
ExecStart=/usr/local/bin/mongodb_exporter --mongodb.uri=\${MONGODB_URI}
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now mongodb_exporter
` + "```" + ``,
		LogsGuide: `## MongoDB Log Collection

MongoDB writes logs to a file by default. Verify in mongod.conf:

` + "```" + `bash
grep -A5 "systemLog" /etc/mongod.conf
` + "```" + `

Expected output:

` + "```" + `yaml
systemLog:
  destination: file
  logAppend: true
  path: /var/log/mongodb/mongod.log
` + "```" + `

If destination is not "file", update mongod.conf:

` + "```" + `yaml
systemLog:
  destination: file
  logAppend: true
  path: /var/log/mongodb/mongod.log
  verbosity: 0
` + "```" + `

Then restart: ` + "`sudo systemctl restart mongod`",
		DetectionHints: []string{"mongod", "mongodb"},
		NVD:            &NVDProduct{Vendor: "mongodb", Product: "mongodb"},
	},

	"elasticsearch": {
		Name:                   "elasticsearch",
		DisplayName:            "Elasticsearch",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/elasticsearch/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9114/metrics",
		ExporterName:           "elasticsearch_exporter",
		ExporterPort:           9114,
		MetricsGuide: `## Enable Elasticsearch Metrics (elasticsearch_exporter)

**Step 1 — Install elasticsearch_exporter**

` + "```" + `bash
wget https://github.com/prometheus-community/elasticsearch_exporter/releases/latest/download/elasticsearch_exporter-linux-amd64.tar.gz
tar xzf elasticsearch_exporter-linux-amd64.tar.gz
sudo mv elasticsearch_exporter-linux-amd64/elasticsearch_exporter /usr/local/bin/
` + "```" + `

**Step 2 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/elasticsearch_exporter.service > /dev/null <<EOF
[Unit]
Description=Elasticsearch Exporter
After=elasticsearch.service

[Service]
User=elasticsearch
ExecStart=/usr/local/bin/elasticsearch_exporter \
  --es.uri=http://127.0.0.1:9200 \
  --es.all \
  --es.indices \
  --es.cluster_settings
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now elasticsearch_exporter
` + "```" + `

If Elasticsearch has security enabled, add: ` + "`--es.username=elastic --es.password=yourpassword`",
		LogsGuide: `## Elasticsearch Log Collection

Elasticsearch writes structured JSON logs to /var/log/elasticsearch/ by default.

**Verify:**

` + "```" + `bash
ls /var/log/elasticsearch/
` + "```" + `

You should see files like elasticsearch.log, elasticsearch_server.json, gc.log.

**If log path is different**, check elasticsearch.yml:

` + "```" + `bash
grep "path.logs" /etc/elasticsearch/elasticsearch.yml
` + "```" + `

Update the path in the platform if different from /var/log/elasticsearch/.`,
		DetectionHints: []string{"elasticsearch"},
		NVD:            &NVDProduct{Vendor: "elastic", Product: "elasticsearch"},
	},

	"clickhouse": {
		Name:                   "clickhouse",
		DisplayName:            "ClickHouse",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/clickhouse-server/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8123/metrics",
		MetricsGuide: `## ClickHouse Metrics

ClickHouse exposes Prometheus metrics natively at /metrics.

**Verify it is accessible:**

` + "```" + `bash
curl http://127.0.0.1:8123/metrics | head -20
` + "```" + `

If it returns metrics, you are ready — click "Install Integration".

**If the endpoint returns nothing**, check /etc/clickhouse-server/config.xml for:

` + "```" + `xml
<prometheus>
    <endpoint>/metrics</endpoint>
    <port>9363</port>
    <metrics>true</metrics>
    <events>true</events>
    <asynchronous_metrics>true</asynchronous_metrics>
</prometheus>
` + "```" + `

If this block is missing, add it and restart: ` + "`sudo systemctl restart clickhouse-server`",
		LogsGuide: `## ClickHouse Log Collection

ClickHouse writes logs to /var/log/clickhouse-server/ by default.

**Verify:**

` + "```" + `bash
ls /var/log/clickhouse-server/
` + "```" + `

You should see clickhouse-server.log and clickhouse-server.err.log.

If logs are in a different location, check /etc/clickhouse-server/config.xml:

` + "```" + `bash
grep -A2 "<log>" /etc/clickhouse-server/config.xml
` + "```" + ``,
		DetectionHints: []string{"clickhouse-server", "clickhouse"},
		NVD:            &NVDProduct{Vendor: "clickhouse", Product: "clickhouse"},
	},

	"influxdb": {
		Name:                   "influxdb",
		DisplayName:            "InfluxDB",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/influxdb/influxd.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8086/metrics",
		MetricsGuide: `## InfluxDB Metrics

InfluxDB exposes Prometheus metrics at /metrics natively.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:8086/metrics | head -20
` + "```" + `

No configuration is needed. If this works, click "Install Integration".`,
		LogsGuide: `## InfluxDB Log Collection

InfluxDB logs to /var/log/influxdb/ when run as a systemd service.

**Verify:**

` + "```" + `bash
ls /var/log/influxdb/
` + "```" + `

If the directory is empty, InfluxDB may be logging to journald. Check:

` + "```" + `bash
journalctl -u influxdb -n 20
` + "```" + ``,
		JournaledLogGuide: `InfluxDB may log to journald depending on your install method. To redirect to a file, set in /etc/influxdb/influxdb.conf:

` + "```" + `
[logging]
  format = "logfmt"
  level = "info"
  suppress-logo = false
` + "```" + `

And ensure the service writes stdout to a file. Update the path in the platform once logs are in a file.`,
		DetectionHints: []string{"influxd", "influxdb"},
		NVD:            &NVDProduct{Vendor: "influxdata", Product: "influxdb"},
	},

	"cockroachdb": {
		Name:                   "cockroachdb",
		DisplayName:            "CockroachDB",
		Category:               "database",
		DefaultLogPaths:        []string{"/var/log/cockroach/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8080/_status/vars",
		MetricsGuide: `## CockroachDB Metrics

CockroachDB exposes Prometheus metrics at /_status/vars natively.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:8080/_status/vars | head -20
` + "```" + `

No configuration needed. If this works, click "Install Integration".`,
		LogsGuide: `## CockroachDB Log Collection

CockroachDB writes structured logs to /var/log/cockroach/ by default.

**Verify:**

` + "```" + `bash
ls /var/log/cockroach/
` + "```" + `

If logs are elsewhere, check your cockroach start flags for --log-dir.`,
		DetectionHints: []string{"cockroach"},
		NVD:            &NVDProduct{Vendor: "cockroachlabs", Product: "cockroachdb"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// MESSAGE QUEUES
	// ═══════════════════════════════════════════════════════════════════

	"kafka": {
		Name:                   "kafka",
		DisplayName:            "Apache Kafka",
		Category:               "queue",
		DefaultLogPaths:        []string{"/var/log/kafka/*.log", "/opt/kafka/logs/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:7071/metrics",
		ExporterName:           "jmx_exporter",
		ExporterPort:           7071,
		MetricsGuide: `## Enable Kafka Metrics (JMX Exporter)

Kafka exposes metrics via JMX. We use jmx_exporter as a Java agent.

**Step 1 — Download jmx_exporter**

` + "```" + `bash
sudo mkdir -p /opt/jmx_exporter
cd /opt/jmx_exporter
sudo wget https://repo1.maven.org/maven2/io/prometheus/jmx/jmx_prometheus_javaagent/0.20.0/jmx_prometheus_javaagent-0.20.0.jar
` + "```" + `

**Step 2 — Create Kafka JMX config**

` + "```" + `bash
sudo tee /opt/jmx_exporter/kafka.yml > /dev/null <<EOF
startDelaySeconds: 0
ssl: false
lowercaseOutputName: false
lowercaseOutputLabelNames: false
EOF
` + "```" + `

**Step 3 — Add JMX agent to Kafka startup**

Edit /etc/kafka/kafka.env or your Kafka systemd service Environment:

` + "```" + `bash
KAFKA_OPTS="-javaagent:/opt/jmx_exporter/jmx_prometheus_javaagent-0.20.0.jar=7071:/opt/jmx_exporter/kafka.yml"
` + "```" + `

**Step 4 — Restart Kafka**

` + "```" + `bash
sudo systemctl restart kafka
` + "```" + `

**Verify:** ` + "`curl http://127.0.0.1:7071/metrics | head -20`",
		LogsGuide: `## Kafka Log Collection

Kafka writes logs to /var/log/kafka/ or $KAFKA_HOME/logs/.

**Verify:**

` + "```" + `bash
ls /var/log/kafka/
# or
ls /opt/kafka/logs/
` + "```" + `

Kafka uses Log4j. If logs are missing, check the log4j.properties for your Kafka install.`,
		DetectionHints: []string{"kafka"},
		NVD:            &NVDProduct{Vendor: "apache", Product: "kafka"},
	},

	"rabbitmq": {
		Name:                   "rabbitmq",
		DisplayName:            "RabbitMQ",
		Category:               "queue",
		DefaultLogPaths:        []string{"/var/log/rabbitmq/*.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:15692/metrics",
		MetricsGuide: `## Enable RabbitMQ Metrics (rabbitmq_prometheus plugin)

**Step 1 — Enable the prometheus plugin**

` + "```" + `bash
sudo rabbitmq-plugins enable rabbitmq_prometheus
` + "```" + `

**Step 2 — Restart RabbitMQ**

` + "```" + `bash
sudo systemctl restart rabbitmq-server
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:15692/metrics | head -20
` + "```" + `

The metrics endpoint runs on port 15692 by default, separate from the management UI (15672).`,
		LogsGuide: `## RabbitMQ Log Collection

RabbitMQ writes logs to /var/log/rabbitmq/ by default.

**Verify:**

` + "```" + `bash
ls /var/log/rabbitmq/
` + "```" + `

You should see files like rabbit@hostname.log.

If logs are missing, check:

` + "```" + `bash
rabbitmq-diagnostics log_location
` + "```" + ``,
		DetectionHints: []string{"rabbitmq-server", "rabbitmq", "beam.smp"},
		NVD:            &NVDProduct{Vendor: "vmware", Product: "rabbitmq"},
	},

	"nats": {
		Name:                   "nats",
		DisplayName:            "NATS",
		Category:               "queue",
		DefaultLogPaths:        []string{"/var/log/nats/nats.log"},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:7777/metrics",
		MetricsGuide: `## Enable NATS Prometheus Metrics

NATS has a built-in monitoring port (8222) and a Prometheus endpoint.

**Step 1 — Enable the Prometheus port in your NATS config**

Edit /etc/nats/nats.conf:

` + "```" + `
http_port: 8222

# Enable Prometheus metrics endpoint
prometheus_port: 7777
` + "```" + `

Or pass via CLI flags:

` + "```" + `bash
nats-server -m 8222 --prometheus_port 7777
` + "```" + `

**Step 2 — Restart NATS**

` + "```" + `bash
sudo systemctl restart nats
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:7777/metrics | head -20
` + "```" + ``,
		LogsGuide: `## NATS Log Collection

NATS logs to stdout (journald) by default. To write to a file, set in your config:

` + "```" + `
log_file: /var/log/nats/nats.log
logtime: true
` + "```" + `

Then create the directory and restart:

` + "```" + `bash
sudo mkdir -p /var/log/nats
sudo systemctl restart nats
` + "```" + ``,
		JournaledLogGuide: `NATS logs to stdout captured by journald. Configure log_file in nats.conf as shown in the Logs Guide, then update the path in the platform.`,
		DetectionHints:    []string{"nats-server", "nats"},
		NVD:               &NVDProduct{Vendor: "nats", Product: "nats_server"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// INFRASTRUCTURE
	// ═══════════════════════════════════════════════════════════════════

	"docker": {
		Name:                   "docker",
		DisplayName:            "Docker",
		Category:               "infrastructure",
		DefaultLogPaths:        []string{}, // handled separately by container log collection
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:9323/metrics",
		MetricsGuide: `## Enable Docker Daemon Metrics

Docker has an experimental Prometheus metrics endpoint.

**Step 1 — Edit /etc/docker/daemon.json**

` + "```" + `bash
sudo tee /etc/docker/daemon.json > /dev/null <<EOF
{
  "metrics-addr": "127.0.0.1:9323",
  "experimental": true
}
EOF
` + "```" + `

**Step 2 — Restart Docker**

` + "```" + `bash
sudo systemctl restart docker
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:9323/metrics | head -20
` + "```" + `

Note: Container-level metrics (CPU, memory, network per container) are already being collected by the platform agent automatically. This endpoint adds daemon-level metrics.`,
		LogsGuide: `Container logs are collected automatically by the platform agent via the Docker socket. No additional configuration is needed for container log collection.

For Docker daemon logs:

` + "```" + `bash
journalctl -u docker -f
` + "```" + ``,
		DetectionHints: []string{"dockerd", "docker"},
		NVD:            &NVDProduct{Vendor: "docker", Product: "docker"},
	},

	"etcd": {
		Name:                   "etcd",
		DisplayName:            "etcd",
		Category:               "infrastructure",
		DefaultLogPaths:        []string{},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:2379/metrics",
		MetricsGuide: `## etcd Metrics

etcd exposes Prometheus metrics natively at /metrics.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:2379/metrics | head -20
` + "```" + `

No configuration needed. If this works, click "Install Integration".`,
		LogsGuide: `## etcd Log Collection

etcd logs to stderr which is captured by journald. It does not write to a log file by default.

To redirect logs to a file, wrap the etcd process:

` + "```" + `bash
sudo tee /etc/systemd/system/etcd.service.d/logging.conf > /dev/null <<EOF
[Service]
StandardOutput=append:/var/log/etcd/etcd.log
StandardError=append:/var/log/etcd/etcd.log
EOF

sudo mkdir -p /var/log/etcd
sudo systemctl daemon-reload
sudo systemctl restart etcd
` + "```" + ``,
		JournaledLogGuide: `etcd logs to stderr/journald. Override the systemd unit to redirect to a file as shown in the Logs Guide, then update the path in the platform.`,
		DetectionHints:    []string{"etcd"},
		NVD:               &NVDProduct{Vendor: "etcd-io", Product: "etcd"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// SECRETS & SERVICE MESH
	// ═══════════════════════════════════════════════════════════════════

	"vault": {
		Name:                   "vault",
		DisplayName:            "HashiCorp Vault",
		Category:               "secrets",
		DefaultLogPaths:        []string{"/var/log/vault/vault.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8200/v1/sys/metrics?format=prometheus",
		MetricsGuide: `## Enable Vault Metrics

**Step 1 — Add telemetry block to vault.hcl**

` + "```" + `hcl
telemetry {
  prometheus_retention_time = "30s"
  disable_hostname = true
}
` + "```" + `

**Step 2 — Restart Vault**

` + "```" + `bash
sudo systemctl restart vault
` + "```" + `

**Step 3 — Verify (requires a Vault token)**

` + "```" + `bash
curl -H "X-Vault-Token: $VAULT_TOKEN" \
  http://127.0.0.1:8200/v1/sys/metrics?format=prometheus | head -20
` + "```" + `

Note: The metrics endpoint requires authentication. The platform will use the agent API key to authenticate with Vault. Ensure the agent has a read-only policy for sys/metrics.`,
		LogsGuide: `## Vault Log Collection

Vault logs to stdout (journald) by default. To write to a file, add to vault.hcl:

` + "```" + `hcl
log_file = "/var/log/vault/vault.log"
log_level = "info"
` + "```" + `

Then create the directory and restart:

` + "```" + `bash
sudo mkdir -p /var/log/vault
sudo chown vault:vault /var/log/vault
sudo systemctl restart vault
` + "```" + ``,
		JournaledLogGuide: `Vault logs to stdout/journald by default. Configure log_file in vault.hcl as shown in the Logs Guide, then update the path in the platform.`,
		DetectionHints:    []string{"vault"},
		NVD:               &NVDProduct{Vendor: "hashicorp", Product: "vault"},
	},

	"consul": {
		Name:                   "consul",
		DisplayName:            "HashiCorp Consul",
		Category:               "secrets",
		DefaultLogPaths:        []string{"/var/log/consul/consul.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8500/v1/agent/metrics?format=prometheus",
		MetricsGuide: `## Consul Metrics

Consul exposes Prometheus metrics natively.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:8500/v1/agent/metrics?format=prometheus | head -20
` + "```" + `

No configuration is needed — the metrics endpoint is always available.`,
		LogsGuide: `## Consul Log Collection

Consul logs to stdout by default. To write to a file, add to consul.hcl:

` + "```" + `hcl
log_file = "/var/log/consul/consul.log"
log_level = "INFO"
log_rotate_duration = "24h"
log_rotate_max_files = 7
` + "```" + `

Then:

` + "```" + `bash
sudo mkdir -p /var/log/consul
sudo chown consul:consul /var/log/consul
sudo systemctl restart consul
` + "```" + ``,
		JournaledLogGuide: `Consul logs to stdout/journald by default. Configure log_file in consul.hcl as shown in the Logs Guide.`,
		DetectionHints:    []string{"consul"},
		NVD:               &NVDProduct{Vendor: "hashicorp", Product: "consul"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// OBSERVABILITY STACK (meta — monitoring your monitoring)
	// ═══════════════════════════════════════════════════════════════════

	"prometheus": {
		Name:                   "prometheus",
		DisplayName:            "Prometheus",
		Category:               "observability",
		DefaultLogPaths:        []string{},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:9090/metrics",
		MetricsGuide: `## Prometheus Self-Monitoring

Prometheus exposes its own metrics at /metrics by default — no configuration needed.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:9090/metrics | head -20
` + "```" + ``,
		LogsGuide: `Prometheus logs to stderr which is captured by journald. No log file collection is typically needed.`,
		JournaledLogGuide: `Prometheus logs to stderr/journald. If you need file-based logging, override the systemd unit:

` + "```" + `bash
sudo mkdir -p /etc/systemd/system/prometheus.service.d
sudo tee /etc/systemd/system/prometheus.service.d/logging.conf <<EOF
[Service]
StandardOutput=append:/var/log/prometheus/prometheus.log
StandardError=append:/var/log/prometheus/prometheus.log
EOF
sudo mkdir -p /var/log/prometheus
sudo systemctl daemon-reload && sudo systemctl restart prometheus
` + "```" + ``,
		DetectionHints: []string{"prometheus"},
		NVD:            &NVDProduct{Vendor: "prometheus", Product: "prometheus"},
	},

	"grafana": {
		Name:                   "grafana",
		DisplayName:            "Grafana",
		Category:               "observability",
		DefaultLogPaths:        []string{"/var/log/grafana/grafana.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:3000/metrics",
		MetricsGuide: `## Grafana Metrics

Grafana exposes Prometheus metrics at /metrics with no configuration required.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:3000/metrics | head -20
` + "```" + `

Note: By default the /metrics endpoint is accessible without authentication. In production you should restrict it.`,
		LogsGuide: `## Grafana Log Collection

Grafana writes logs to /var/log/grafana/grafana.log by default.

**Verify:**

` + "```" + `bash
tail -20 /var/log/grafana/grafana.log
` + "```" + `

No configuration is needed.`,
		DetectionHints: []string{"grafana-server", "grafana"},
		NVD:            &NVDProduct{Vendor: "grafana", Product: "grafana"},
	},

	"loki": {
		Name:                   "loki",
		DisplayName:            "Grafana Loki",
		Category:               "observability",
		DefaultLogPaths:        []string{},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:3100/metrics",
		MetricsGuide: `## Loki Metrics

Loki exposes Prometheus metrics at /metrics natively.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:3100/metrics | head -20
` + "```" + ``,
		LogsGuide:         `Loki logs to stderr/stdout captured by journald. Log file collection is not typically needed for Loki itself.`,
		JournaledLogGuide: `Loki logs to stdout/journald. No action needed unless you require file-based logs.`,
		DetectionHints:    []string{"loki"},
		NVD:               &NVDProduct{Vendor: "grafana", Product: "loki"},
	},

	"jaeger": {
		Name:                   "jaeger",
		DisplayName:            "Jaeger",
		Category:               "observability",
		DefaultLogPaths:        []string{},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:14269/metrics",
		MetricsGuide: `## Jaeger Metrics

Jaeger exposes Prometheus metrics at /metrics on the admin port.

**Verify:**

` + "```" + `bash
curl http://127.0.0.1:14269/metrics | head -20
` + "```" + `

No configuration needed.`,
		LogsGuide:         `Jaeger logs to stdout/journald. No file-based log collection is typically needed.`,
		JournaledLogGuide: `Jaeger logs to stdout. No action needed unless you require file-based logs.`,
		DetectionHints:    []string{"jaeger"},
		NVD:               &NVDProduct{Vendor: "jaegertracing", Product: "jaeger"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// SECURITY
	// ═══════════════════════════════════════════════════════════════════

	"openssh": {
		Name:                   "openssh",
		DisplayName:            "OpenSSH",
		Category:               "security",
		DefaultLogPaths:        []string{"/var/log/auth.log", "/var/log/secure"},
		DefaultLoggingMode:     LoggingModeSyslog,
		MetricsPattern:         MetricsPatternNone,
		DefaultMetricsEndpoint: "",
		MetricsGuide:           `OpenSSH does not expose metrics. The platform collects SSH logs for security analysis (failed logins, brute force detection, unusual access patterns).`,
		LogsGuide: `## OpenSSH Log Collection

SSH logs authentication events to syslog.

**Debian/Ubuntu:** /var/log/auth.log
**RHEL/CentOS:** /var/log/secure

**Verify:**

` + "```" + `bash
# Debian/Ubuntu
tail -20 /var/log/auth.log | grep sshd

# RHEL/CentOS
tail -20 /var/log/secure | grep sshd
` + "```" + `

These logs power the brute force detection and SSH security rules in your security rules page.`,
		DetectionHints: []string{"sshd"},
		NVD:            &NVDProduct{Vendor: "openbsd", Product: "openssh"},
	},

	"fail2ban": {
		Name:                   "fail2ban",
		DisplayName:            "Fail2ban",
		Category:               "security",
		DefaultLogPaths:        []string{"/var/log/fail2ban.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9191/metrics",
		ExporterName:           "fail2ban_exporter",
		ExporterPort:           9191,
		MetricsGuide: `## Enable Fail2ban Metrics (fail2ban_exporter)

**Step 1 — Install fail2ban_exporter**

` + "```" + `bash
wget https://github.com/jangrewe/prometheus-fail2ban-exporter/releases/latest/download/fail2ban_exporter-linux-amd64.tar.gz
tar xzf fail2ban_exporter-linux-amd64.tar.gz
sudo mv fail2ban_exporter /usr/local/bin/
` + "```" + `

**Step 2 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/fail2ban_exporter.service > /dev/null <<EOF
[Unit]
Description=Fail2ban Exporter
After=fail2ban.service

[Service]
User=root
ExecStart=/usr/local/bin/fail2ban_exporter
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable --now fail2ban_exporter
` + "```" + `

**Verify:** ` + "`curl http://127.0.0.1:9191/metrics | grep fail2ban`",
		LogsGuide: `## Fail2ban Log Collection

Fail2ban writes to /var/log/fail2ban.log by default.

**Verify:**

` + "```" + `bash
tail -20 /var/log/fail2ban.log
` + "```" + `

No configuration is needed.`,
		DetectionHints: []string{"fail2ban"},
		NVD:            &NVDProduct{Vendor: "fail2ban", Product: "fail2ban"},
	},

	"ufw": {
		Name:                   "ufw",
		DisplayName:            "UFW Firewall",
		Category:               "security",
		DefaultLogPaths:        []string{"/var/log/ufw.log"},
		DefaultLoggingMode:     LoggingModeSyslog,
		MetricsPattern:         MetricsPatternNone,
		DefaultMetricsEndpoint: "",
		MetricsGuide:           `UFW does not expose metrics. The platform collects UFW logs for security analysis (blocked connections, port scans, unusual traffic patterns).`,
		LogsGuide: `## UFW Log Collection

**Step 1 — Enable UFW logging if not already enabled**

` + "```" + `bash
sudo ufw logging on
# or for more detail:
sudo ufw logging medium
` + "```" + `

**Step 2 — Verify logs are being written**

` + "```" + `bash
tail -20 /var/log/ufw.log
` + "```" + `

UFW log entries start with [UFW BLOCK] or [UFW ALLOW]. These power the port scan detection rules.`,
		DetectionHints: []string{"ufw"},
		NVD:            nil, // UFW itself has no CVEs to track
	},

	"auditd": {
		Name:                   "auditd",
		DisplayName:            "Linux Audit (auditd)",
		Category:               "security",
		DefaultLogPaths:        []string{"/var/log/audit/audit.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternNone,
		DefaultMetricsEndpoint: "",
		MetricsGuide:           `auditd does not expose metrics. The platform collects audit logs for security analysis (file access, privilege escalation, system calls, user authentication events).`,
		LogsGuide: `## auditd Log Collection

auditd writes to /var/log/audit/audit.log by default.

**Verify auditd is running:**

` + "```" + `bash
sudo systemctl status auditd
tail -5 /var/log/audit/audit.log
` + "```" + `

**Recommended audit rules for security monitoring**

Add to /etc/audit/rules.d/observability.rules:

` + "```" + `
# Track privilege escalation
-w /bin/su -p x -k privilege_escalation
-w /usr/bin/sudo -p x -k privilege_escalation

# Track user/group changes
-w /etc/passwd -p wa -k identity
-w /etc/shadow -p wa -k identity
-w /etc/group -p wa -k identity

# Track SSH config changes
-w /etc/ssh/sshd_config -p wa -k sshd_config
` + "```" + `

Then reload: ` + "`sudo augenrules --load`",
		DetectionHints: []string{"auditd"},
		NVD:            nil,
	},

	// ═══════════════════════════════════════════════════════════════════
	// APP SERVERS
	// ═══════════════════════════════════════════════════════════════════

	"phpfpm": {
		Name:                   "phpfpm",
		DisplayName:            "PHP-FPM",
		Category:               "appserver",
		DefaultLogPaths:        []string{"/var/log/php-fpm/error.log", "/var/log/php8.1-fpm.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9253/metrics",
		ExporterName:           "phpfpm_exporter",
		ExporterPort:           9253,
		MetricsGuide: `## Enable PHP-FPM Metrics (php-fpm_exporter)

**Step 1 — Enable the PHP-FPM status page**

Edit your PHP-FPM pool config (/etc/php/8.x/fpm/pool.d/www.conf):

` + "```" + `ini
pm.status_path = /php-fpm-status
` + "```" + `

Then expose it via nginx:

` + "```" + `nginx
location /php-fpm-status {
    fastcgi_pass unix:/run/php/php8.1-fpm.sock;
    fastcgi_param SCRIPT_FILENAME $fastcgi_script_name;
    include fastcgi_params;
    allow 127.0.0.1;
    deny all;
}
` + "```" + `

**Step 2 — Install php-fpm_exporter**

` + "```" + `bash
wget https://github.com/hipages/php-fpm_exporter/releases/latest/download/php-fpm_exporter_linux_amd64.tar.gz
tar xzf php-fpm_exporter_linux_amd64.tar.gz
sudo mv php-fpm_exporter /usr/local/bin/
` + "```" + `

**Step 3 — Start the exporter**

` + "```" + `bash
sudo tee /etc/systemd/system/phpfpm_exporter.service > /dev/null <<EOF
[Unit]
Description=PHP-FPM Exporter
[Service]
ExecStart=/usr/local/bin/php-fpm_exporter server --phpfpm.scrape-uri=tcp://127.0.0.1:9000/php-fpm-status
Restart=always
[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload && sudo systemctl enable --now phpfpm_exporter
` + "```" + ``,
		LogsGuide: `## PHP-FPM Log Collection

**Verify log path in pool config:**

` + "```" + `bash
grep -r "error_log\|access.log\|slowlog" /etc/php/
` + "```" + `

Common paths: /var/log/php-fpm/ (RHEL), /var/log/php8.x-fpm.log (Debian)

If no log path is set, add to your pool config:

` + "```" + `ini
access.log = /var/log/php-fpm/$pool.access.log
slowlog = /var/log/php-fpm/$pool.slow.log
` + "```" + `

Then restart PHP-FPM: ` + "`sudo systemctl restart php8.1-fpm`",
		DetectionHints: []string{"php-fpm", "php8", "php7"},
		NVD:            &NVDProduct{Vendor: "php", Product: "php"},
	},

	"tomcat": {
		Name:                   "tomcat",
		DisplayName:            "Apache Tomcat",
		Category:               "appserver",
		DefaultLogPaths:        []string{"/var/log/tomcat*/catalina.out", "/opt/tomcat/logs/catalina.out"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:7070/metrics",
		ExporterName:           "jmx_exporter",
		ExporterPort:           7070,
		MetricsGuide: `## Enable Tomcat Metrics (JMX Exporter)

**Step 1 — Download jmx_exporter**

` + "```" + `bash
sudo mkdir -p /opt/jmx_exporter
cd /opt/jmx_exporter
sudo wget https://repo1.maven.org/maven2/io/prometheus/jmx/jmx_prometheus_javaagent/0.20.0/jmx_prometheus_javaagent-0.20.0.jar
` + "```" + `

**Step 2 — Create Tomcat JMX config**

` + "```" + `bash
sudo tee /opt/jmx_exporter/tomcat.yml > /dev/null <<EOF
startDelaySeconds: 0
hostPort: localhost:1099
ssl: false
EOF
` + "```" + `

**Step 3 — Add to CATALINA_OPTS**

Edit /etc/tomcat/tomcat.conf or setenv.sh:

` + "```" + `bash
CATALINA_OPTS="-javaagent:/opt/jmx_exporter/jmx_prometheus_javaagent-0.20.0.jar=7070:/opt/jmx_exporter/tomcat.yml"
` + "```" + `

**Step 4 — Restart Tomcat and verify**

` + "```" + `bash
sudo systemctl restart tomcat
curl http://127.0.0.1:7070/metrics | head -20
` + "```" + ``,
		LogsGuide: `## Tomcat Log Collection

Tomcat writes to catalina.out and other log files in $CATALINA_HOME/logs/.

**Common log paths:**

` + "```" + `bash
ls /var/log/tomcat*/
# or
ls /opt/tomcat/logs/
` + "```" + `

The main log file is catalina.out. Other files include localhost_access_log, manager.log, host-manager.log.`,
		DetectionHints: []string{"catalina", "tomcat"},
		NVD:            &NVDProduct{Vendor: "apache", Product: "tomcat"},
	},

	"jenkins": {
		Name:                   "jenkins",
		DisplayName:            "Jenkins",
		Category:               "appserver",
		DefaultLogPaths:        []string{"/var/log/jenkins/jenkins.log"},
		DefaultLoggingMode:     LoggingModeFile,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:8080/prometheus",
		MetricsGuide: `## Enable Jenkins Metrics (Prometheus Plugin)

**Step 1 — Install the Prometheus metrics plugin**

In Jenkins: Manage Jenkins → Plugins → Available → search "Prometheus metrics" → Install.

Or via Jenkins CLI:

` + "```" + `bash
java -jar jenkins-cli.jar -s http://localhost:8080/ install-plugin prometheus
` + "```" + `

**Step 2 — Restart Jenkins**

After plugin install, restart Jenkins from the UI or:

` + "```" + `bash
sudo systemctl restart jenkins
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:8080/prometheus | head -20
` + "```" + `

Note: If Jenkins has authentication enabled, you may need to allow anonymous access to /prometheus or use an API token.`,
		LogsGuide: `## Jenkins Log Collection

Jenkins writes logs to /var/log/jenkins/jenkins.log when installed as a system service.

**Verify:**

` + "```" + `bash
tail -20 /var/log/jenkins/jenkins.log
` + "```" + ``,
		DetectionHints: []string{"jenkins"},
		NVD:            &NVDProduct{Vendor: "jenkins", Product: "jenkins"},
	},

	// ═══════════════════════════════════════════════════════════════════
	// MAIL & DNS
	// ═══════════════════════════════════════════════════════════════════

	"postfix": {
		Name:                   "postfix",
		DisplayName:            "Postfix",
		Category:               "mail",
		DefaultLogPaths:        []string{"/var/log/mail.log", "/var/log/maillog"},
		DefaultLoggingMode:     LoggingModeSyslog,
		MetricsPattern:         MetricsPatternExporter,
		DefaultMetricsEndpoint: "http://127.0.0.1:9154/metrics",
		ExporterName:           "postfix_exporter",
		ExporterPort:           9154,
		MetricsGuide: `## Enable Postfix Metrics (postfix_exporter)

postfix_exporter reads Postfix logs and exposes Prometheus metrics.

**Step 1 — Install postfix_exporter**

` + "```" + `bash
wget https://github.com/prometheus-community/postfix_exporter/releases/latest/download/postfix_exporter-linux-amd64.tar.gz
tar xzf postfix_exporter-linux-amd64.tar.gz
sudo mv postfix_exporter /usr/local/bin/
` + "```" + `

**Step 2 — Create systemd service**

` + "```" + `bash
sudo tee /etc/systemd/system/postfix_exporter.service > /dev/null <<EOF
[Unit]
Description=Postfix Exporter
After=postfix.service

[Service]
ExecStart=/usr/local/bin/postfix_exporter \
  --postfix.logfile_path=/var/log/mail.log
Restart=always

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload && sudo systemctl enable --now postfix_exporter
` + "```" + ``,
		LogsGuide: `## Postfix Log Collection

Postfix logs via syslog to /var/log/mail.log (Debian) or /var/log/maillog (RHEL).

**Verify:**

` + "```" + `bash
# Debian/Ubuntu
tail -20 /var/log/mail.log | grep postfix

# RHEL/CentOS
tail -20 /var/log/maillog | grep postfix
` + "```" + `

These logs are written by rsyslog (mail.* facility) — no Postfix configuration needed.`,
		DetectionHints: []string{"postfix", "master"},
		NVD:            &NVDProduct{Vendor: "postfix", Product: "postfix"},
	},

	"coredns": {
		Name:                   "coredns",
		DisplayName:            "CoreDNS",
		Category:               "dns",
		DefaultLogPaths:        []string{},
		DefaultLoggingMode:     LoggingModeStdout,
		MetricsPattern:         MetricsPatternBuiltIn,
		DefaultMetricsEndpoint: "http://127.0.0.1:9153/metrics",
		MetricsGuide: `## Enable CoreDNS Metrics

CoreDNS has a built-in Prometheus plugin.

**Step 1 — Add prometheus to your Corefile**

` + "```" + `
. {
    prometheus :9153
    forward . 8.8.8.8
    log
    errors
}
` + "```" + `

**Step 2 — Restart CoreDNS**

` + "```" + `bash
sudo systemctl restart coredns
` + "```" + `

**Step 3 — Verify**

` + "```" + `bash
curl http://127.0.0.1:9153/metrics | head -20
` + "```" + ``,
		LogsGuide: `## CoreDNS Log Collection

CoreDNS logs to stdout (journald). Add the log plugin to your Corefile to enable query logging:

` + "```" + `
. {
    log
    errors
    prometheus :9153
}
` + "```" + `

To write to a file, override the systemd unit:

` + "```" + `bash
sudo tee /etc/systemd/system/coredns.service.d/logging.conf > /dev/null <<EOF
[Service]
StandardOutput=append:/var/log/coredns/coredns.log
StandardError=append:/var/log/coredns/coredns.log
EOF
sudo mkdir -p /var/log/coredns
sudo systemctl daemon-reload && sudo systemctl restart coredns
` + "```" + ``,
		JournaledLogGuide: `CoreDNS logs to stdout/journald. Override the systemd unit to redirect to a file as shown in the Logs Guide.`,
		DetectionHints:    []string{"coredns"},
		NVD:               &NVDProduct{Vendor: "cncf", Product: "coredns"},
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

// List returns all catalog entries sorted by category then name.
func List() []ServiceDef {
	out := make([]ServiceDef, 0, len(Catalog))
	for _, v := range Catalog {
		out = append(out, v)
	}
	return out
}

// ByCategory returns all services in a given category.
func ByCategory(category string) []ServiceDef {
	var out []ServiceDef
	for _, v := range Catalog {
		if v.Category == category {
			out = append(out, v)
		}
	}
	return out
}

// NVDServices returns only services that have NVD CVE checking configured.
func NVDServices() []ServiceDef {
	var out []ServiceDef
	for _, v := range Catalog {
		if v.NVD != nil {
			out = append(out, v)
		}
	}
	return out
}
