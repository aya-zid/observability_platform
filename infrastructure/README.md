# Infrastructure

This folder contains the local observability backend stack used by the project.

## Purpose

Provides storage, routing, and visualization for:

- Metrics
- Logs
- Traces

## Components

- VictoriaMetrics cluster: `vmstorage`, `vminsert`, `vmselect`
- `vmauth`: multi-tenant auth/routing in front of VictoriaMetrics
- `loki`: log storage/query
- `tempo`: trace storage/query
- `grafana`: dashboards and visualization

Main compose file: `docker-compose.yml`

Config files are under `config/` (Grafana datasources, Loki, Tempo, vmauth).

## Run locally

From this folder:

```bash
docker compose up -d
```

Grafana is exposed on `http://localhost:3000` (default `admin/admin`).
