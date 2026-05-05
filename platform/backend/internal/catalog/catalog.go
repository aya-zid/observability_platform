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
		MetricsGuide: `## Enable Nginx Metrics (stub_status)

Nginx exposes basic connection metrics via the built-in stub_status module.

**Step 1 — Add the status location block**

Edit your nginx server config and add inside a server block:

` + "```nginx" + `
location /nginx_status {
    stub_status on;
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
` + "```" + ``,
		LogsGuide: `## Nginx Log Collection

Nginx writes to files by default. No changes are usually needed.

**Verify log files exist:**

` + "```bash" + `
ls -la /var/log/nginx/
` + "```" + ``,
		DetectionHints: []string{"nginx", "openresty"},
		NVD:            &NVDProduct{Vendor: "f5", Product: "nginx_open_source"},
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