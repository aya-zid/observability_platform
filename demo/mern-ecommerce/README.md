# MERN Ecommerce Mini Demo

A simplified MERN ecommerce app used mainly for local testing and trace validation.

## Why this demo exists

- It is a mini ecommerce app (signup/login, browse, cart, orders).
- It is intentionally simplified for local runs.
- It is useful as a realistic app workload to test traces end-to-end.

## Trace Collection Enablement

This demo was extended to emit OpenTelemetry traces from the backend server.

### What was added

- `server/tracing.js`:
  - Initializes OpenTelemetry Node SDK
  - Enables auto-instrumentation for HTTP, Express, and MongoDB
  - Exports traces via OTLP gRPC
  - Adds resource attributes such as service name/version, tenant, and environment
- `server/index.js`:
  - Boots tracing first via `require('./tracing')` before app initialization
- `server/package.json`:
  - Added OpenTelemetry dependencies:
    - `@opentelemetry/sdk-node`
    - `@opentelemetry/auto-instrumentations-node`
    - `@opentelemetry/exporter-trace-otlp-grpc`
    - `@opentelemetry/resources`
    - `@opentelemetry/semantic-conventions`

### Docker-compose updates for tracing

- Added `extra_hosts` for Linux Docker networking:
  - `host.docker.internal:host-gateway`
- Added tracing env vars on the `server` service:
  - `OTEL_EXPORTER_OTLP_ENDPOINT=http://host.docker.internal:4317`
  - `TENANT_ID=obs_test123`

This routes server traces to an OpenTelemetry Collector running on the host.

### MongoDB log access update

The Mongo container was updated to write logs to `/var/log/mongodb/mongod.log` and mount that path from the host for easier log collection.

One-time host setup:

```bash
sudo mkdir -p /var/log/mongodb
sudo chmod 777 /var/log/mongodb
```


## Run with Docker

From `demo/mern-ecommerce`:

```bash
docker compose up --build
```

Endpoints:

- Frontend: `http://localhost:8082`
- API: `http://localhost:3001`
- MongoDB: `mongodb://localhost:27017/mern_ecommerce`

Default seeded admin:

- Email: `admin@example.com`
- Password: `admin123`    

