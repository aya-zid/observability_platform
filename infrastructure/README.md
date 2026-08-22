# Medianet Observability — Infrastructure Layer

This folder contains the Docker Compose stack that provides the **data storage and query layer** for the Medianet Observability platform. It runs separately from the platform backend and frontend, but connects to them via a shared Docker network.

---

## Table of Contents

- [What This Is](#what-this-is)
- [Components](#components)
- [Network Topology](#network-topology)
- [Prerequisites](#prerequisites)
- [Deployment](#deployment)
- [Configuration](#configuration)
- [Volumes and Data](#volumes-and-data)
- [Grafana (Testing Only)](#grafana-testing-only)
- [Production Deployment Notes](#production-deployment-notes)
- [Common Operations](#common-operations)
- [Troubleshooting](#troubleshooting)

---

## What This Is

The Medianet Observability platform is split into two layers:

| Layer | Folder | What It Runs |
|---|---|---|
| **Infrastructure** | `infrastructure/` | VictoriaMetrics, Loki, Tempo (data storage) |
| **Platform** | `platform/` | Backend API, Frontend, PostgreSQL, Redis |

This folder is the **infrastructure layer**. It provides:

- **Metrics storage** — VictoriaMetrics cluster (receives, stores, and queries metrics)
- **Log storage** — Loki (receives, stores, and queries logs)
- **Trace storage** — Tempo (receives, stores, and queries distributed traces)

The platform backend (in `platform/`) connects to these services over the `obs-network` Docker network to proxy queries from the frontend and ingest telemetry from agents.

---

## Components

### VictoriaMetrics Cluster (Metrics)

| Container | Port | Role |
|---|---|---|
| `vmstorage` | 8482 (monitoring) | Stores metric data on disk. 30-day retention. |
| `vminsert` | 8480 | Receives metrics via Prometheus remote write. Routes to vmstorage. |
| `vmselect` | 8481 | Queries metrics from vmstorage. Serves PromQL. |

**Image:** `victoriametrics/vmstorage:v1.136.0-cluster`

### Loki (Logs)

| Container | Port | Role |
|---|---|---|
| `loki` | 3100 | Receives, stores, and queries logs. Multi-tenant enabled. |

**Image:** `grafana/loki:3.6.0`

Logs are stored locally on the filesystem at `/loki/chunks` with TSDB indexing. Retention is **unlimited** — logs are kept forever.

### Tempo (Traces) — Not Used Yet

| Container | Port | Role |
|---|---|---|
| `tempo` | 3200 (HTTP API), 4317 (OTLP gRPC internal), 4318 (OTLP HTTP internal) | Receives, stores, and queries distributed traces. |

**Image:** `grafana/tempo:2.3.1`

> **Note:** The APM/traces feature has **not been implemented** in the backend code yet. Tempo is included in the stack for future use but is **not actively used**. You can safely remove the `tempo` service from `docker-compose.yml` if you want to save resources. When the APM feature is developed, you can re-add it.

### Grafana (Testing Only)

| Container | Port | Role |
|---|---|---|
| `grafana` | 3000 | Visualization dashboards for testing. |

**Image:** `grafana/grafana:latest`

> **Important:** Grafana is included for development and testing only. It should **not** be deployed in production. The production frontend uses the Go backend proxy to query metrics, logs, and traces directly.

---

## Network Topology

```
+----------------------------------------------------------+
|  obs-network (external Docker network)                   |
|                                                          |
|  +-----------+     +----------+     +----------+         |
|  | vminsert  |     | vmselect |     | vmstorage|         |
|  | :8480     |     | :8481    |     | :8482    |         |
|  +-----+-----+     +----+-----+     +----+-----+        |
|        |                |                |                |
|        +----------------+----------------+                |
|                         |                                |
|  +-----------+     +----+-----+     +----------+         |
|  |   loki    |     |  tempo   |     | grafana  |         |
|  | :3100     |     | :3200    |     | :3000    |         |
|  +-----------+     +----------+     +----------+         |
|                                                          |
+---------------------------+------------------------------+
                            |
                            | (platform backend connects here)
                            |
              +-------------+-------------+
              |  obs-backend (platform)   |
              |  Connects to:             |
              |    vminsert:8480          |
              |    vmselect:8481          |
              |    loki:3100              |
              |    tempo:4318             |
              +---------------------------+
```

**Key points:**

- `obs-network` is an **external** network — it must be created once before starting either stack
- The platform backend connects to both `obs-network` (for infra) and `obs-platform-internal` (for PostgreSQL/Redis)
- Services are reachable by container name (e.g., `vminsert`, `loki`, `tempo`)
- Ports 8480, 8481, 3100, 3200 are exposed for **local debugging only** — they are not used by the agent or production frontend

---

## Prerequisites

- Docker and Docker Compose v2+
- The `obs-network` Docker network must exist before starting the stack

### Create the Network

```bash
docker network create obs-network
```

This only needs to be done once. Both the infrastructure stack and the platform stack share this network.

---

## Deployment

### 1. Create the Docker Network

```bash
docker network create obs-network
```

### 2. Create the Loki Data Directory

Loki requires its data directory to exist before the container starts:

```bash
mkdir -p data/loki
```

### 3. Start the Stack

```bash
cd infrastructure/
docker compose up -d
```

### 4. Verify All Services Are Running

```bash
docker compose ps
```

You should see `vmstorage`, `vminsert`, `vmselect`, `loki`, `tempo`, and `grafana` all in `Up` state.

### 5. Start the Platform Stack (Separately)

```bash
cd ../platform/
docker compose up -d
```

Both stacks must be running for the full platform to work. The platform backend connects to the infrastructure services over `obs-network`.

---

## Configuration

All configuration files are in `config/`:

| File | Purpose |
|---|---|
| `loki.yaml` | Loki server config — multi-tenancy, storage, retention, ingestion limits |
| `tempo.yaml` | Tempo server config — OTLP receivers, storage, per-tenant limits |
| `grafana-datasources.yaml` | Grafana auto-provisioned datasources (VM, Loki, Tempo) |

### Loki Highlights

- **Multi-tenancy:** `auth_enabled: true` — tenants are identified by the `X-Scope-OrgID` header
- **Storage:** Local filesystem with TSDB indexing
- **Retention:** Unlimited — logs are kept forever
- **Ingestion rate:** 256 MB/s per tenant, 512 MB burst
- **OTLP attributes indexed as labels:** `host_id`, `host_name`, `service_name`, `job`, `os_type`

### Tempo Highlights — Not Used Yet

- **Multi-tenancy:** Enabled
- **Receivers:** OTLP gRPC (4317) and HTTP (4318) — both internal only
- **Storage:** Local filesystem
- **Per-tenant limits:** 10 MB/s ingestion rate, 5 MB max per trace
- **Status:** The APM/traces feature is not implemented yet. Tempo can be removed from the stack to save resources.

---

## Volumes and Data

### Named Docker Volumes

| Volume | Used By | Purpose |
|---|---|---|
| `vmstorage-data` | vmstorage | Persistent metric data |
| `tempo-data` | tempo | Persistent trace data |
| `grafana-data` | grafana | Grafana state (dashboards, users) |

### Bind-Mounted Directories

| Host Path | Container Path | Purpose |
|---|---|---|
| `./config/loki.yaml` | `/etc/loki/local-config.yaml` | Loki configuration (read-only) |
| `./config/tempo.yaml` | `/etc/tempo.yaml` | Tempo configuration (read-only) |
| `./config/grafana-datasources.yaml` | `/etc/grafana/provisioning/datasources/datasources.yaml` | Grafana datasources (read-only) |
| `./data/loki/` | `/loki` | Loki's on-disk storage |

### Data Persistence

- All data persists across container restarts via Docker volumes and bind mounts
- To **wipe all data** and start fresh:
  ```bash
  docker compose down -v
  rm -rf data/loki/
  ```
- The `.gitignore` in this folder excludes `data/*` so local data is never committed

---

## Grafana (Testing Only)

Grafana is pre-configured with three datasources:

| Datasource | UID | Queries |
|---|---|---|
| VictoriaMetrics | `obs-victoriametrics` | PromQL via vmselect (`/select/1/prometheus`) |
| Loki | `obs-loki` | LogQL via loki:3100 |
| Tempo | `obs-tempo` | TraceQL via tempo:3200 |

**Access:** `http://localhost:3000`  
**Credentials:** `admin` / `admin`

> Grafana is for development/testing only. In production, the React frontend queries data through the Go backend proxy, which handles tenant isolation. Grafana does not support per-tenant switching with static provisioning.

### Multi-Tenant Note

The Grafana datasources are configured for a single tenant. To change the tenant:

1. Open `config/grafana-datasources.yaml`
2. For VictoriaMetrics: change the `1` in the URL path to your tenant's numeric DB ID
3. For Loki/Tempo: replace the UUID in `httpHeaderValue1` with your tenant's UUID

```sql
-- Find your tenant's ID and UUID:
SELECT id, uuid FROM tenants LIMIT 5;
```

---

## Production Deployment Notes

For production, consider the following changes:

### Remove Grafana

Grafana is for testing only. In docker-compose.yml, comment out or delete the entire `grafana` service block. The production frontend uses the Go backend proxy to query data.

### Remove Tempo (If APM Is Not Needed Yet)

Tempo is included for future APM/traces support, which has not been implemented yet. You can safely remove the `tempo` service from `docker-compose.yml` to save resources. When the APM feature is developed, you can re-add it.

### Remove Grafana Volumes

If you removed Grafana, also remove its volume from the `volumes:` section at the bottom of docker-compose.yml:

```yaml
volumes:
  vmstorage-data:
  tempo-data:
  # grafana-data:    # <-- remove this line
```

### Expose Only Required Ports

In production, only these ports need to be accessible from the host (for agent ingestion via the backend):

- `8080` — Platform backend API (from `platform/docker-compose.yml`)
- `5173` — Frontend (from `platform/docker-compose.yml`)

The infrastructure ports (8480, 8481, 3100, 3200) stay internal to the Docker network. Agents send telemetry to the backend, which proxies it to the correct storage service.

### Production Architecture

```
Agent (on customer host)
    |
    | HTTP (port 8080)
    v
Platform Backend (Go)
    |
    +---> vminsert:8480  (metrics)
    +---> loki:3100       (logs)
    +---> tempo:4318      (traces — not used yet)
    |
    v
Frontend (React)
    |
    | queries via backend proxy
    v
vmselect:8481 / loki:3100 / tempo:3200 (optional)
```

---

## Common Operations

### Start the Stack

```bash
docker compose up -d
```

### Stop the Stack

```bash
docker compose down
```

Stop and remove all volumes (wipes data):

```bash
docker compose down -v
```

### View Logs

```bash
# All services
docker compose logs -f

# Single service
docker compose logs -f loki
docker compose logs -f vmstorage
docker compose logs -f tempo
```

### Check Service Health

```bash
# VictoriaMetrics (query API)
curl http://localhost:8481/select/1/prometheus/api/v1/query?query=up

# Loki (ready check)
curl http://localhost:3100/ready

# Tempo (ready check)
curl http://localhost:3200/ready

# Grafana (health check)
curl http://localhost:3000/api/health
```

### Restart a Single Service

```bash
docker compose restart loki
docker compose restart vmstorage
```

---

## Troubleshooting

| Issue | Solution |
|---|---|
| `network obs-network not found` | Run `docker network create obs-network` before starting the stack |
| `loki` fails to start | Ensure `data/loki/` directory exists: `mkdir -p data/loki` |
| `vmstorage` crash loop | Check disk space — metrics data grows over time |
| `tempo` OOM (if deployed) | Increase memory limit or reduce `max_bytes_per_trace` in `config/tempo.yaml` |
| Agents can't send data | Verify the backend can reach `vminsert:8480`, `loki:3100` on `obs-network` |
| Grafana shows no data | Check that the tenant ID/UUID in `grafana-datasources.yaml` matches your tenant |
| Loki ingestion errors | Check `docker compose logs loki` for rate limit messages |

### Useful Commands

```bash
# Check if a container is on the network
docker network inspect obs-network | grep -A2 "Name"

# Test connectivity from backend to infra
docker exec obs-backend curl -s http://loki:3100/ready

# Check disk usage of volumes
docker system df -v
```
