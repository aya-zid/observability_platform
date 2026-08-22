# Medianet Observability — Host Agent

The agent is a lightweight, auto-configuring daemon installed on your Linux servers. It automatically discovers running services, collects system metrics and logs, monitors Docker containers, and streams everything to the Medianet Observability platform in real time.

---

## Quick Start

```bash
curl -sSL http://<platform-host>:8080/install.sh | sudo API_KEY=<your-api-key> bash
```

The agent will scan your host, detect services and Docker containers, and start collecting metrics and logs immediately.

---

## Table of Contents

- [What the Agent Does](#what-the-agent-does)
- [Supported Services](#supported-services)
- [Architecture Overview](#architecture-overview)
- [Installation](#installation)
- [Agent Components](#agent-components)
- [Metrics Collected](#metrics-collected)
- [Logs Collected](#logs-collected)
- [Docker Support](#docker-support)
- [Custom Application Metrics](#custom-application-metrics)
- [Application Traces (OpenTelemetry SDK)](#application-traces-opentelemetry-sdk)
- [Uninstallation](#uninstallation)
- [Troubleshooting](#troubleshooting)

---

## What the Agent Does

Once installed, the agent runs as a background service and handles:

| Capability | Details |
|---|---|
| **System Metrics** | CPU, memory, disk I/O, network, filesystem usage, load averages, process counts — collected every 30 seconds |
| **Service Discovery** | Automatically detects running services and their log files |
| **Log Collection** | Parses and collects logs from host services and Docker containers |
| **Docker Monitoring** | Discovers containers, tails stdout/stderr in real time, collects per-container CPU/memory/network metrics |
| **Host Heartbeat** | Reports host status and running services to the platform every 5 minutes |
| **Daily Rediscovery** | Re-scans for new services daily at noon |
| **Trace Ingestion** | Accepts OpenTelemetry traces from your applications via OTLP (gRPC port 4317, HTTP port 4318) |
| **Custom Metrics** | Accepts application metrics via StatsD on port 8125 |

---

## Supported Services

The agent detects and configures log collection for these services when running as systemd units:

| Service | Log Format | What Gets Parsed |
|---|---|---|
| **Nginx** | `nginx_combined` | Access and error logs, per-virtual-host log paths |
| **Apache** | `apache_combined` | Access and error logs, per-virtual-host log paths |
| **PostgreSQL** | `pg_csv` | CSV logs with user, database, duration, query details |
| **MySQL** | `mysql_log` | Error log and slow query log |
| **MariaDB** | `mysql_log` | Error log and slow query log |
| **Redis** | `redis_log` | Application log with log levels |
| **PHP-FPM** | `phpfpm_log` | Error log, slow log, and access log per pool |
| **MongoDB** | `json` | JSON-formatted log entries |
| **Elasticsearch** | auto | Log files from standard paths |
| **RabbitMQ** | auto | Log files from `/var/log/rabbitmq/` |
| **Kafka** | auto | Log files from standard Kafka log directories |
| **HAProxy** | auto | Log file from HAProxy configuration |
| **Traefik** | auto | Access and error logs |
| **Consul** | auto | Log files from Consul configuration |
| **Vault** | auto | Log files from Vault configuration |
| **Memcached** | auto | Log files from open file descriptors |
| **Caddy** | auto | Log files from Caddyfile configuration |
| **ClickHouse** | auto | Log files from ClickHouse configuration |
| **Cassandra** | auto | Log files from `/var/log/cassandra/` |

> Even if a service isn't listed above, the agent will still attempt to discover it by inspecting open log file descriptors on the running process. Unknown services with discoverable logs are reported to the platform.

---

## Architecture Overview

```
+--------------------------------------------------+
|  Your Server (Linux host)                        |
|                                                  |
|  +-----------+  +------------+  +-------------+  |
|  |  otelcol  |  |  docker-   |  |  docker-    |  |
|  |  (main    |  |  log-      |  |  metrics    |  |
|  | collector)|  |  tailer    |  |  (every 30s)|  |
|  +-----+-----+  +-----+-----+  +------+------+  |
|        |               |                |         |
|        |               +----------------+         |
|        |                                          |
|  +-----+------+  +------------+  +-------------+  |
|  |  prom file |  |  docker-   |  |  heartbeat  |  |
|  |  server    |  |  watcher   |  |  (every 5m) |  |
|  |  (:19101)  |  |            |  |             |  |
|  +------------+  +------------+  +-------------+  |
|                                                  |
+-----------------------+--------------------------+
                        |
                        v
           +--------------------------+
           |  Medianet Observability  |
           |  Platform Backend        |
           |  (metrics, logs, traces) |
           +--------------------------+
```

---

## Installation

### Requirements

- **OS:** Linux (any distribution)
- **Architecture:** x86_64, aarch64, or armv7l
- **Privileges:** Root access (sudo)
- **Network:** Outbound HTTP to `<platform-host>:8080`
- **API Key:** Your tenant API key (found in the platform dashboard)

### Install Command

```bash
curl -sSL http://<platform-host>:8080/install.sh | sudo API_KEY=<your-api-key> bash
```

Or with explicit platform host:

```bash
sudo API_KEY=your-api-key PLATFORM_HOST=<platform-host> bash install.sh
```

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `API_KEY` | *(required)* | Your tenant API key for authentication |
| `PLATFORM_HOST` | *(required)* | The hostname or IP of the Medianet Observability platform |
| `PLATFORM_PORT` | `8080` | The port of the platform backend |

### What Happens During Installation

The installer runs in 7 phases:

1. **Service Discovery** — Scans all running systemd units, identifies known services, and finds their log files
2. **Docker Discovery** — If Docker is available, discovers running containers and sets up log collection
3. **Issue Fixing** — Interactively offers to fix common issues (e.g., enabling PostgreSQL `logging_collector`, Redis log file)
4. **Config Generation** — Builds the OpenTelemetry Collector configuration based on discovered services
5. **Binary Install** — Downloads and installs the OpenTelemetry Collector binary (v0.119.0)
6. **Helper Services** — Installs Docker log tailer, metrics collector, event watcher, and heartbeat services
7. **Host Registration** — Registers the host with the platform and sends the discovery report

---

## Agent Components

After installation, the following services run on your host:

| Service | Type | Description |
|---|---|---|
| `otelcol` | systemd service (always) | Main OpenTelemetry Collector — collects system metrics, logs, and forwards traces |
| `agent-docker-log-tailer` | systemd service (Docker only) | Tails stdout/stderr from all running containers in real time |
| `agent-docker-metrics` | systemd timer (Docker only) | Collects per-container CPU, memory, network, and disk metrics every 30 seconds |
| `agent-docker-watcher` | systemd service (Docker only) | Watches for container start/stop/rename events and reloads config |
| `agent-prom-file-server` | systemd service (always) | Serves Prometheus metrics files on `localhost:19101` |
| `agent-heartbeat` | systemd timer (always) | Sends host status and running service list to the platform every 5 minutes |
| `agent-rediscover` | cron job (always) | Runs daily at noon to detect newly installed services |

### Installed Files

| Path | Description |
|---|---|
| `/usr/local/bin/otelcol` | OpenTelemetry Collector binary |
| `/etc/otelcol/config.yaml` | Generated collector configuration |
| `/etc/otelcol/discovery-report.txt` | Last installation report |
| `/usr/local/bin/agent-docker-log-tailer` | Docker log tailing script |
| `/usr/local/bin/agent-docker-metrics` | Docker metrics collection script |
| `/usr/local/bin/agent-docker-update` | Docker event handler |
| `/usr/local/bin/agent-prom-file-server` | Prometheus file server (Python) |
| `/usr/local/bin/agent-heartbeat` | Heartbeat sender script |
| `/usr/local/bin/agent-rediscover` | Daily rediscovery script |
| `/var/lib/otelcol/` | Agent data directory (Docker logs, metrics) |
| `/etc/cron.d/observability-agent` | Daily rediscovery cron entry |

---

## Metrics Collected

### System Metrics (every 30 seconds)

| Category | Metrics |
|---|---|
| **CPU** | `system.cpu.time`, `system.cpu.utilization`, `system.cpu.logical.count`, `system.cpu.frequency` |
| **Memory** | `system.memory.usage`, `system.memory.utilization` |
| **Disk** | `system.disk.io`, `system.disk.operations`, `system.disk.io_time`, `system.disk.pending_operations` |
| **Filesystem** | `system.filesystem.usage`, `system.filesystem.utilization`, `system.filesystem.inodes.usage` |
| **Network** | `system.network.io`, `system.network.packets`, `system.network.errors`, `system.network.dropped`, `system.network.connections` |
| **Load** | `system.cpu.load_average.1m`, `system.cpu.load_average.5m`, `system.cpu.load_average.15m` |
| **Paging** | `system.paging.usage`, `system.paging.operations`, `system.paging.faults` |
| **Processes** | `system.processes.count`, `system.processes.created` |

### Docker Container Metrics (every 30 seconds)

Per container, named `container_<container_name>_<metric>`:

| Metric | Description |
|---|---|
| `container_<name>_cpu_percent` | CPU usage percentage |
| `container_<name>_mem_usage_bytes` | Memory usage in bytes |
| `container_<name>_mem_limit_bytes` | Memory limit in bytes |
| `container_<name>_mem_percent` | Memory usage percentage |
| `container_<name>_net_rx_bytes` | Network bytes received |
| `container_<name>_net_tx_bytes` | Network bytes transmitted |
| `container_<name>_block_read_bytes` | Block I/O read bytes |
| `container_<name>_block_write_bytes` | Block I/O write bytes |
| `container_<name>_pids` | Number of processes/threads |

---

## Logs Collected

### Host Logs

- **Journald** — System journal entries for all discovered services, SSH, and sudo
- **Service-specific logs** — Parsed and structured logs from supported services (see [Supported Services](#supported-services))
- **Syslog** — Fallback from `/var/log/syslog` or `/var/log/messages` (if present)

### Docker Container Logs

Two sources per container:

1. **Volume logs** (preferred) — If the container has bind-mounted log directories, those log files are collected directly
2. **Stdout/stderr** — Real-time tailing of `docker logs --follow` for all containers

All container logs are tagged with metadata:

- `container.id` — Full container ID
- `container.image` — Docker image name
- `container.service` — Detected service type (e.g., nginx, postgresql)
- `log.source` — Either `volume` or `stdout`

### Log Noise Filtering

The agent automatically filters out noisy or uninformative log entries:

- Health check and keepalive requests
- Empty or whitespace-only lines
- Info-level logs from containers (keeps warnings and errors)
- PostgreSQL "connection received" lines
- Monitoring user connections (agent's own DB connections)
- PHP-FPM `/status` and `/internal/opcache-collector` hits
- Excessive kernel/systemd-resolved/NetworkManager debug messages

---

## Docker Support

If Docker is installed on your host, the agent automatically:

1. **Discovers running containers** — Identifies the service type from the Docker image name, container command, or Compose labels
2. **Tails container logs** — Sets up real-time log tailing via `docker logs --follow` for every container, writing to `/var/lib/otelcol/docker-logs/<container>.log`
3. **Collects container metrics** — CPU, memory, network I/O, block I/O, and process count per container
4. **Watches for lifecycle events** — When containers start, stop, or are renamed, the agent automatically adjusts log collection
5. **Resumes on restart** — Uses per-container state files to resume log tailing from the last position, avoiding duplicate log entries

### Supported Container Services

The agent identifies containers running: Nginx, Apache, PostgreSQL, MySQL, MariaDB, Redis, MongoDB, RabbitMQ, Kafka, Elasticsearch, ClickHouse, Vault, Consul, Memcached, Traefik, HAProxy, Caddy, Cassandra, and generic Node.js/Python/Ruby/PHP/Java applications.

---

## Custom Application Metrics

You can send custom application metrics to the agent via **StatsD** on port `8125` (UDP):

```bash
# Example: send a metric
echo "myapp.requests.count:1|c" | nc -u -w0 localhost 8125
```

### By Language

- **Node.js:** `npm install node-statsd` — `const statsd = new StatsD('localhost', 8125)`
- **Python:** `pip install statsd` — `c = statsd.StatsClient('localhost', 8125)`
- **Go:** `github.com/DataDog/datadog-go/v5/statsd`

---

## Application Traces (OpenTelemetry SDK)

The agent accepts traces from your instrumented applications via **OTLP**:

| Protocol | Endpoint |
|---|---|
| gRPC | `localhost:4317` |
| HTTP | `localhost:4318` |

### Environment Variables for Your App

```bash
# On the same host
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# Inside Docker (pointing to host)
export OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4317
```

You can use any OpenTelemetry SDK (Node.js, Python, Go, Java, etc.) to instrument your application and send traces to the agent.

---

## Uninstallation

### Quick Uninstall

```bash
curl -sSL http://<platform-host>:8080/uninstall.sh | sudo bash
```

Or run locally:

```bash
sudo bash uninstall.sh
```

### Uninstall Options

| Flag | Description |
|---|---|
| `--force` / `-y` | Skip all confirmation prompts |
| `--purge` | Also remove `/var/lib/otelcol/` and rediscovery logs |
| `--no-journal` | Do not vacuum journald logs |

### What Gets Removed

- All systemd services and timers (`otelcol`, `agent-docker-*`, `agent-heartbeat`)
- Binary files from `/usr/local/bin/`
- Configuration from `/etc/otelcol/`
- Cron job from `/etc/cron.d/`
- Log files from `/var/log/otelcol/`

> **Note:** Uninstalling the agent does **not** delete your telemetry data (metrics, logs, traces) already stored in the platform. Only the agent on this machine is removed.

---

## Troubleshooting

### Check Agent Status

```bash
sudo systemctl status otelcol
```

### View Agent Logs

```bash
sudo journalctl -u otelcol -f
```

### View the Configuration

```bash
sudo cat /etc/otelcol/config.yaml
```

### Restart the Agent

```bash
sudo systemctl restart otelcol
```

### Check Helper Services

```bash
sudo systemctl status agent-docker-log-tailer
sudo systemctl status agent-docker-metrics.timer
sudo systemctl status agent-docker-watcher
sudo systemctl status agent-prom-file-server
sudo systemctl status agent-heartbeat.timer
```

### View the Discovery Report

```bash
cat /etc/otelcol/discovery-report.txt
```

### Validate the Configuration

```bash
sudo /usr/local/bin/otelcol validate --config file:/etc/otelcol/config.yaml
```

### Common Issues

| Issue | Solution |
|---|---|
| `API_KEY is required` | Make sure to pass `API_KEY=<your-key>` before the install command |
| `Please run as root` | Use `sudo` or run as root |
| `otelcol failed to start` | Check logs with `journalctl -u otelcol -n 50` |
| Docker containers not showing | Ensure Docker socket is at `/var/run/docker.sock` and the user has access |
| No log files found | The service may log to journald only; the agent will still collect via journald |
| PostgreSQL logs missing | The installer may offer to enable `logging_collector`; check the install output |
