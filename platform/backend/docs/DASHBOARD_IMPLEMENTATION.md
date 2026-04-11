# Dashboard Implementation Guide

This document provides a complete guide to implementing multi-tenant dashboards for your observability platform, following the architecture recommended in the implementation.

## Architecture Overview

The dashboard system is built with **three layers**:

```
┌─────────────────────────────────────────────────────────┐
│         React Frontend (NginxDashboardPage)              │
│  - Stat cards (React components)                        │
│  - Tables (React + useQuery)                            │
│  - Log viewer (React)                                   │
│  - Grafana iframes (for heavy charts)                   │
└───────────────────┬─────────────────────────────────────┘
                    │ HTTP calls to /api/query/*
                    │
┌───────────────────▼─────────────────────────────────────┐
│    Go Backend Query Proxy (QueryHandler)                │
│  - Validates JWT auth                                   │
│  - Extracts tenant ID from claims                       │
│  - Injects tenant headers/paths                         │
│  - Proxies to Loki/VictoriaMetrics                     │
│  - Returns standardized JSON responses                  │
└───────────────────┬─────────────────────────────────────┘
                    │ HTTP calls with tenant isolation
                    │
┌───────────────────▼─────────────────────────────────────┐
│  Infrastructure (Loki, VictoriaMetrics, Grafana)       │
│  - Loki: Tenant isolated by X-Scope-OrgID header       │
│  - VictoriaMetrics: Tenant isolated by URL path /select/{id}/
│  - Grafana: Embedded for UI-heavy panels only          │
└─────────────────────────────────────────────────────────┘
```

## What Was Implemented

### 1. Backend Query Proxy (`internal/handler/query_handler.go`)

Provides four HTTP endpoints for querying logs and metrics:

- `GET /api/query/metrics` — Query VictoriaMetrics (Prometheus queries)
- `GET /api/query/logs` — Query Loki (instant query for log lines)
- `GET /api/query/logs/range` — Query Loki (range query for aggregations)
- `GET /api/query/logs/labels` — Query Loki (get available labels)

**Example:**
```bash
# Query request rate over 1 hour
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/query/logs/range?query=sum(rate({job=\"nginx\"}[1m]))&start=$(date -d '1 hour ago' +%s)e9&end=$(date +%s)e9&step=60s"
```

All endpoints:
- Extract tenant ID from JWT claims
- Inject X-Scope-OrgID header (Loki) or tenant ID in URL path (VictoriaMetrics)
- Proxy the request upstream
- Stream response back unchanged

### 2. React Query Helpers (`src/lib/queryHelpers.ts`)

Utility functions for parsing Loki/Prometheus responses:

- `parseMatrixResponse()` — Parse time-series data
- `parseLogLines()` — Extract log text from streams
- `extractLatestValue()` — Get current value for stat cards
- `formatTimestamp()`, `formatNumber()`, `formatPercent()` — Format display values
- `getErrorRateStatus()` — Determine color (green/yellow/red) for values

**Example:**
```typescript
const { data } = useQuery({
  queryFn: async () => {
    const res = await apiClient.get('/api/query/logs/range', {
      params: { query: 'sum(rate({job="nginx"}[1m]))' }
    })
    return parseMatrixResponse(res.data)
  }
})
```

### 3. React Nginx Dashboard (`src/pages/integrations/NginxDashboardPage.tsx`)

Complete dashboard component with:

- **Time range selector** — 15m, 1h, 6h, 24h
- **Stat cards** — Total requests, error rates (5xx, 4xx)
- **Grafana iframes** — Request rate, active connections
- **Tables** — Top paths, top error paths
- **Log viewer** — Recent logs with click-to-expand

All components use `useQuery` for automatic refetching and loading states.

### 4. Grafana Dashboard Script (`infrastructure/scripts/create-grafana-dashboard.sh`)

Bash script that creates a Grafana dashboard via API with 4 panels:

1. Request Rate (Loki)
2. Active Connections (VictoriaMetrics)
3. HTTP Status Codes (Loki)
4. Request Duration p95 (Loki)

**Usage:**
```bash
bash create-grafana-dashboard.sh http://localhost:3000 admin admin
```

### 5. Grafana Datasources Update

Updated `infrastructure/config/grafana-datasources.yaml` to:
- Remove hardcoded tenant UUIDs
- Use default tenant ID 0
- Removed X-Scope-OrgID headers from datasource config (now handled by backend proxy)

## Multi-Tenant Isolation

Tenant isolation is enforced at **three levels**:

### Level 1: JWT Claims
```go
// In QueryHandler.QueryLogsRange():
claims, ok := getClaims(c)  // Extract from Authorization header
tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
```

### Level 2: Header & Path Injection
```go
// For Loki: inject X-Scope-OrgID header
headers := map[string]string{"X-Scope-OrgID": tenant.UUID}

// For VictoriaMetrics: inject tenant ID in URL path
upstream := fmt.Sprintf("http://vmselect:8481/select/%d/prometheus/...", claims.TenantID)
```

### Level 3: Infrastructure Config
```yaml
# Grafana datasources don't have magic headers
# Real isolation happens at the proxy (Level 2)
url: http://loki:3100  # No hardcoded tenant UUID
```

## QuickStart: Adding a New Dashboard

### Step 1: Create React Component

```typescript
// src/pages/integrations/MyServiceDashboard.tsx

export function MyServiceDashboard() {
  const [timeRange, setTimeRange] = useState(TIME_RANGES[1])
  const now = Math.floor(Date.now() / 1000)
  const start = now - timeRange.seconds

  const { data } = useQuery({
    queryKey: ['my-stat'],
    queryFn: async () => {
      const res = await apiClient.get('/api/query/logs/range', {
        params: {
          query: `{service_name="my-service"}`,
          start: start * 1e9,
          end: now * 1e9,
        }
      })
      return parseLogLines(res.data)
    }
  })

  return (
    <div className="space-y-6 p-6">
      <h1>My Service Dashboard</h1>
      {/* Add your components here */}
    </div>
  )
}
```

### Step 2: Add Route

```typescript
// src/router/index.tsx

import { MyServiceDashboard } from '@/pages/integrations/MyServiceDashboard'

export const routes = [
  {
    path: '/integrations/my-service',
    element: <MyServiceDashboard />
  }
]
```

### Step 3: Query Data

Use the backend proxy endpoints:

```typescript
// Logs aggregation (LogQL)
/api/query/logs/range?query={service_name="my-service"}&start=X&end=Y&step=Z

// Metrics (PromQL)
/api/query/metrics?query=my_metric&start=X&end=Y

// Raw log lines
/api/query/logs?query={service_name="my-service"}&limit=100
```

## Testing the Implementation

### Test Backend Proxy

```bash
# Get a valid token first
TOKEN=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"pass"}' | jq -r .access_token)

# Query logs
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/query/logs?query=%7Bservice_name%3D%22nginx%22%7D&limit=10"

# Query logs range (aggregation)
NOW=$(date +%s)
START=$((NOW - 3600))
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/query/logs/range?query=sum(rate(%7Bservice_name%3D%22nginx%22%7D%5B1m%5D))&start=${START}e9&end=${NOW}e9&step=60s"
```

### Test React Component

1. Navigate to the Nginx dashboard in your app
2. Check browser DevTools Network tab for `/api/query/*` requests
3. Verify response format matches expected Loki/Prometheus structure
4. Check that stat cards and tables render correctly

### Test Grafana Dashboard

```bash
# Create dashboard
bash infrastructure/scripts/create-grafana-dashboard.sh

# Visit Grafana
open http://localhost:3000/d/nginx-overview
```

## Common Queries

### Nginx Request Rate
```logql
sum(rate({service_name="nginx"}[1m]))
```

### Nginx Error Rate (5xx only)
```logql
sum(rate({service_name="nginx"} | pattern `<_> "<_>" <status> <_>` | status =~ "5.." [1m])) / sum(rate({service_name="nginx"}[1m])) * 100
```

### Request Duration p95
```logql
quantile_over_time(0.95, {service_name="nginx"} | pattern `<_> rt=<rt>` | unwrap rt [1m]) by ()
```

### Top Paths
```logql
topk(10, sum by (path) (rate({service_name="nginx"} | pattern `<_> "<method> <path> <_>" <_> <_>` [5m])))
```

### Recent Errors
```logql
{service_name="nginx"} | pattern `<_> "<_>" <status> <_>` | status =~ "5.."
```

## Performance Considerations

### Caching
- React Query automatically caches results and refetches on interval
- Adjust `refetchInterval` based on how often you need updates
- Use `useIsFetching()` to show loading states

### Query Optimization
- Always use time ranges appropriate to your use case
- Use appropriate step sizes (60s for most dashboards)
- Aggregate at the source (Loki) rather than in React when possible
- Limit result size (maxLines: 1000 in datasource config)

### Alarms
- Don't query too frequently (minimum 15-second intervals recommended)
- Use `refetchInterval` to space out queries
- Consider rate limiting on the backend if many users access dashboards

## Troubleshooting

### Queries return no data

1. Check tenant isolation:
   ```bash
   # Backend logs should show extracted tenant ID
   curl -v http://localhost:8080/api/query/logs/range?... | grep -i tenant
   ```

2. Verify Loki has data:
   ```bash
   curl -H "X-Scope-OrgID: YOUR_TENANT_UUID" \
     http://localhost:3100/loki/api/v1/query?query={service_name="nginx"}
   ```

3. Check that Integration script ran:
   ```bash
   cat /etc/otelcol/config.yaml | grep filelog
   ```

### 401 Unauthorized errors

- Ensure token is valid: `jwt.io` to decode and check expiry
- Pass token in Authorization header: `Authorization: Bearer $TOKEN`
- Backend extracts token in AuthMiddleware

### Response time queries return empty

- Verify nginx log format includes `rt=$request_time`
- Check that otelcol filelog parser regex includes `rt=` field
- Query with simple pattern first: `{service_name="nginx"} | pattern \`<_> rt=<rt>\``

## Next Steps

1. **Add more integrations** — Follow the same pattern for PostgreSQL, Redis, etc.
2. **Build alert rules** — Use Grafana's alert API to create threshold alerts
3. **Custom dashboards** — Create tenant-specific dashboard templates
4. **Export to customers** — Embed dashboards or provide public links
5. **Performance tuning** — Profile queries and optimize LogQL expressions

## Files Modified

```
backend/
├── internal/handler/query_handler.go       (NEW) Query proxy endpoints
├── internal/handler/routes.go              (MODIFIED) Add query routes
├── docs/NGINX_INTEGRATION.md               (NEW) Setup guide

frontend/
├── src/lib/queryHelpers.ts                 (NEW) Response parsing utilities
├── src/pages/integrations/NginxDashboardPage.tsx  (NEW) Dashboard component

infrastructure/
├── config/grafana-datasources.yaml         (MODIFIED) Remove hardcoded tenant IDs
├── scripts/create-grafana-dashboard.sh     (NEW) Dashboard provisioning script
```

## References

- [Loki LogQL Documentation](https://grafana.com/docs/loki/latest/logql/)
- [Prometheus/VictoriaMetrics PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana HTTP API](https://grafana.com/docs/grafana/latest/fundamentals/intro-to-grafana/)
- [OpenTelemetry Filelog Receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
