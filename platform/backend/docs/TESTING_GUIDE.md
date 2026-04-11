# Dashboard Implementation Testing Guide

This guide provides step-by-step instructions to test all components of the dashboard implementation.

## Prerequisites

- Backend running on `http://localhost:8080`
- Frontend running on `http://localhost:5173`
- Grafana running on `http://localhost:3000`
- Nginx integration script applied to at least one machine
- Valid user account and JWT token

## Phase 1: Backend Proxy Testing

### 1.1 Verify Endpoints Are Registered

Check that routes are properly registered:

```bash
# Backend should be running
curl http://localhost:8080/health

# Expected output:
# {
#   "status": "healthy",
#   "db_status": "connected",
#   "redis_status": "connected"
# }
```

### 1.2 Authenticate and Get Token

```bash
# Sign up or login
USER_EMAIL="test@example.com"
PASSWORD="Test1234!"

# Login
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${USER_EMAIL}\",\"password\":\"${PASSWORD}\"}")

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.access_token')
TENANT_ID=$(echo $LOGIN_RESPONSE | jq -r '.tenant_id')

echo "Token: $TOKEN"
echo "Tenant ID: $TENANT_ID"

# Save to environment for next tests
export AUTH_TOKEN=$TOKEN
export TENANT_ID=$TENANT_ID
```

### 1.3 Test Query Logs Endpoint

```bash
# Query recent nginx logs
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/logs?query=%7Bservice_name%3D%22nginx%22%7D&limit=10" | jq .

# Expected response format:
# {
#   "status": "success",
#   "data": {
#     "resultType": "streams",
#     "result": [
#       {
#         "stream": { "service_name": "nginx", "job": "nginx", ... },
#         "values": [
#           [ "1711234567123456789", "127.0.0.1 - - [27/Mar/2026:...] ..." ],
#           ...
#         ]
#       }
#     ]
#   }
# }
```

**Validation Checklist:**
- [ ] Response status is "success"
- [ ] Result type is "streams"
- [ ] Streams have `stream` (labels) and `values` (log lines)
- [ ] Log lines are readable text
- [ ] No 401/403 errors

### 1.4 Test Query Logs Range (Aggregation)

```bash
# Get current time in seconds
NOW=$(date +%s)
START=$((NOW - 3600))  # 1 hour ago

# Query: sum of nginx request rate over 1 hour
QUERY_URL_ENCODED=$(echo 'sum(rate({service_name="nginx"}[1m]))' | jq -sRr @uri)

curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/logs/range?query=${QUERY_URL_ENCODED}&start=${START}e9&end=${NOW}e9&step=60s" | jq .

# Expected response format:
# {
#   "status": "success",
#   "data": {
#     "resultType": "matrix",
#     "result": [
#       {
#         "metric": { ... },
#         "values": [
#           [ 1711234567, "150.5" ],  # [timestamp, value]
#           [ 1711234627, "152.3" ],
#           ...
#         ]
#       }
#     ]
#   }
# }
```

**Validation Checklist:**
- [ ] Response status is "success"
- [ ] Result type is "matrix"
- [ ] Each value is [timestamp, string_number]
- [ ] Timestamps are in seconds (Unix time)
- [ ] Values are numeric strings

### 1.5 Test Query Metrics (VictoriaMetrics)

```bash
# Query nginx connections
QUERY_URL_ENCODED=$(echo 'nginx_connections_active' | jq -sRr @uri)

curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/metrics?query=${QUERY_URL_ENCODED}&start=${START}e9&end=${NOW}e9&step=60s" | jq .

# Expected response format (same as logs/range):
# {
#   "status": "success",
#   "data": {
#     "resultType": "matrix",
#     "result": [ ... ]
#   }
# }
```

**Validation Checklist:**
- [ ] Response received (VictoriaMetrics accessible)
- [ ] Response format matches expected Prometheus format
- [ ] Values are numeric

### 1.6 Test Query Labels

```bash
# Get available label names
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/logs/labels" | jq .

# Expected response:
# {
#   "status": "success",
#   "data": {
#     "names": [ "service_name", "job", "host_id", "path", "status", ... ]
#   }
# }

# Get label values for a specific label
LABEL_NAME="service_name"
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/logs/labels?name=${LABEL_NAME}" | jq .

# Expected response:
# {
#   "status": "success",
#   "data": [ "nginx", "postgres", "redis", ... ]
# }
```

**Validation Checklist:**
- [ ] Labels endpoint returns label names
- [ ] Label values endpoint returns correct values for a label
- [ ] Responses are properly formatted

### 1.7 Test Tenant Isolation

```bash
# Create two test accounts (if needed)
# Account 1: tenant A
# Account 2: tenant B

# Log in with account 1, get token
TOKEN_A=$AUTH_TOKEN  # From 1.2

# Logout and log in with account 2
curl -s -X POST http://localhost:8080/logout \
  -H "Authorization: Bearer $TOKEN_A"

# Login as account 2
TOKEN_B=$(curl -s -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"other@example.com","password":"password"}' | jq -r '.access_token')

# Query with each token
RESULT_A=$(curl -s -H "Authorization: Bearer $TOKEN_A" \
  "http://localhost:8080/api/query/logs?query=%7Bservice_name%3D%22nginx%22%7D&limit=1" | jq '.data.result[0].stream.nginx_tenant_id // empty')

RESULT_B=$(curl -s -H "Authorization: Bearer $TOKEN_B" \
  "http://localhost:8080/api/query/logs?query=%7Bservice_name%3D%22nginx%22%7D&limit=1" | jq '.data.result[0].stream.nginx_tenant_id // empty')

# Results should be different (different tenant data)
if [ "$RESULT_A" != "$RESULT_B" ]; then
  echo "✅ Tenant isolation verified"
else
  echo "⚠️ Warning: Might be same tenant or check label names"
fi
```

## Phase 2: React Frontend Testing

### 2.1 Navigate to Nginx Dashboard

1. Open frontend: `http://localhost:5173`
2. Log in with your test account
3. Navigate to integrations section
4. Click "Nginx Dashboard" or similar

### 2.2 Verify Stat Cards Render

```
Expected to see:
- "Total Requests" card with a number
- "5xx Error Rate" card with a percentage
- "4xx Error Rate" card with a percentage
```

**Validation Checklist:**
- [ ] Cards display immediately (or show loading skeleton)
- [ ] "Total Requests" shows a number > 0
- [ ] Error rates show percentages (e.g., "1.23%")
- [ ] Cards have appropriate status colors (green/yellow/red)

### 2.3 Verify Tables Render

**Top Paths by Traffic:**
```
Expected table columns:
- Path (e.g., "/api/v1/users", "/health")
- Requests/sec (numeric value)
```

**Top Error Paths:**
```
Expected table columns:
- Path (e.g., "/admin" showing 404s)
- Requests/sec (error requests/sec)
```

**Validation Checklist:**
- [ ] Tables display top 10 items
- [ ] Paths are realistic URLs
- [ ] Numbers are positive and make sense
- [ ] Tables update on time range change

### 2.4 Verify Log Viewer

```
Expected to see:
- List of 50 recent log lines
- Each line truncated to ~120 chars
- Clickable to expand full log text
```

**Validation Checklist:**
- [ ] Log entries are displayed
- [ ] Entries are truncated with "..."
- [ ] Click to expand shows full log line
- [ ] Log lines contain nginx log format

### 2.5 Verify Grafana Iframes

```
Expected to see two embedded Grafana panels:
- Request Rate (line chart showing requests over time)
- Active Connections (line chart)
```

**Validation Checklist:**
- [ ] Grafana panels load (might take 5-10 seconds)
- [ ] Charts show time-series data
- [ ] X-axis shows time, Y-axis shows metric value
- [ ] Legend shows metric names

### 2.6 Verify Time Range Selector

```
Click each time range button:
- 15m
- 1h
- 6h
- 24h
```

**Validation Checklist:**
- [ ] Active button highlights (blue/primary color)
- [ ] Dashboard updates when time range changes
- [ ] Data fetches within 1-2 seconds
- [ ] Charts update to show new time range

### 2.7 Browser Console Check

Press `F12` to open DevTools Console and check:

```javascript
// Should see no errors (red messages)
// Check Network tab for:
// - GET /api/query/logs (200 status)
// - GET /api/query/logs/range (200 status)
// - GET /api/query/metrics (200 status)

// Check response sizes (should be < 1MB):
// - logs query: ~ 50-100 KB
// - logs/range: ~ 20-50 KB
```

**Validation Checklist:**
- [ ] No 401/403 errors in Network tab
- [ ] All /api/query/* requests return 200
- [ ] Response times < 2 seconds
- [ ] Network tab shows requests being made

## Phase 3: Grafana Dashboard Testing

### 3.1 Create Dashboard via API

```bash
bash infrastructure/scripts/create-grafana-dashboard.sh
```

Expected output:
```
📊 Creating Nginx Grafana dashboard...
✅ Dashboard created successfully!
Dashboard URL: http://localhost:3000/d/nginx-overview
```

### 3.2 Access Dashboard

1. Open `http://localhost:3000/d/nginx-overview`
2. Log in if prompted (admin/admin)

Expected to see 4 panels:
1. **Request Rate** — Line chart showing requests/sec over time
2. **Active Connections** — Line chart of active connections
3. **HTTP Status Codes (Rate)** — Stacked line chart with 2xx/4xx/5xx
4. **Request Duration (p95)** — Line chart of p95 response time

### 3.3 Verify Panel Data

For each panel:

1. Hover over chart to see data points
2. Click legend items to toggle series
3. Change time range (top-right: "Last 1 hour", etc.)
4. Check that data updates

**Validation Checklist:**
- [ ] All 4 panels display data
- [ ] Data is reasonable (no huge spikes or flat zeros)
- [ ] Time range selector works
- [ ] Legend shows metric names
- [ ] Charts are interactive (hover shows tooltip)

### 3.4 Verify Datasource Configuration

1. Click settings icon (gear) → Data sources
2. Verify three datasources exist:
   - **VictoriaMetrics** → `http://vmselect:8481/select/0/prometheus`
   - **Loki** → `http://loki:3100`
   - **Tempo** → `http://tempo:3200`

**Validation Checklist:**
- [ ] All 3 datasources are listed
- [ ] No hardcoded tenant UUIDs in datasource config
- [ ] URLs match infrastructure setup

## Phase 4: End-to-End Integration Testing

### 4.1 Nginx Generates Logs

On the target machine with nginx:

```bash
# Generate some HTTP traffic
for i in {1..10}; do
  curl http://localhost/api/v1/users
  curl http://localhost/nonexistent  # Generate 404
  sleep 1
done

# Check nginx logs
tail -f /var/log/nginx/access.log

# Should see logs with rt= at end
# Example: ... rt=0.045
```

### 4.2 Agent Collects Logs

```bash
# Check otelcol status on target machine
sudo systemctl status otelcol

# Check otelcol logs
sudo journalctl -u otelcol -n 50 -f

# Look for lines indicating nginx log parsing
```

### 4.3 Dashboard Shows Live Data

1. Open React dashboard
2. Watch stat cards update in real-time
3. Generate traffic on nginx (curl commands)
4. Verify numbers change within 30 seconds

### 4.4 Grafana Shows Live Data

1. Open Grafana dashboard
2. Generate traffic
3. Verify panels update

## Performance & Load Testing

### 5.1 Measure Query Response Times

```bash
# Measure /api/query/logs/range response time
time curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "http://localhost:8080/api/query/logs/range?query=sum(rate(%7Bservice_name%3D%22nginx%22%7D%5B1m%5D))&start=${START}e9&end=${NOW}e9&step=60s" > /dev/null

# Expected: < 2 seconds for 1 hour of data
```

### 5.2 Check System Resources

```bash
# Check backend memory/CPU
docker stats observability-platform-backend

# Check Loki memory/CPU
docker stats loki

# Check VictoriaMetrics memory/CPU
docker stats vmselect
```

**Expected baseline:**
- Backend: ~100-200 MB RAM
- Loki: ~200-500 MB RAM
- VictoriaMetrics: ~300-700 MB RAM

### 5.3 Monitor for Memory Leaks

Leave dashboard open for 1 hour while refreshing every 30 seconds:

```bash
# Monitor browser memory (DevTools → Memory tab)
# Expected: stable, not growing unbounded
```

## Troubleshooting

### Issue: 401 Unauthorized from Query Endpoints

**Solution:**
1. Verify token is fresh (not expired — 15 min default)
2. Check Authorization header format: `Authorization: Bearer $TOKEN`
3. Verify backend sees token: `curl -v http://localhost:8080/api/query/logs/range 2>&1 | grep Authorization`

### Issue: Query Returns Empty Data

**Solution:**
1. Verify Loki has data: `curl http://localhost:3100/loki/api/v1/query?query={service_name="nginx"}`
2. Check Loki auth: `curl -H "X-Scope-OrgID: YOUR_UUID" http://localhost:3100/loki/api/v1/query_range`
3. Verify otelcol is running on target: `sudo systemctl status otelcol`
4. Check otelcol logs: `sudo journalctl -u otelcol -f`

### Issue: Grafana Panels Show "No Data"

**Solution:**
1. Verify datasources are accessible: Settings → Data sources → Test
2. Check datasource URL: Should be `http://loki:3100` (not localhost:3100)
3. For VictoriaMetrics, check tenant in URL path: `/select/0/prometheus`

### Issue: React Component Doesn't Update

**Solution:**
1. Open DevTools Console for errors
2. Check Network tab for failed requests
3. Verify token is in request headers
4. Check `refetchInterval` setting (default 30s)

### Issue: Stat Cards Show "—" (No Data)

**Solution:**
1. Query function might be failing — check browser console
2. Parse helper might be breaking — add `console.log(data)` to debug
3. No data in time range — try expanding time range to 24h

## Sign-Off Checklist

- [ ] Backend proxy returns correct response formats
- [ ] Tenant isolation verified (different tenants see different data)
- [ ] Frontend stat cards show numbers > 0
- [ ] Frontend tables display top paths
- [ ] Frontend log viewer shows recent logs
- [ ] Frontend time range selector works
- [ ] Grafana iframes load and show data
- [ ] Grafana standalone dashboard shows all 4 panels
- [ ] Live data updates within 30 seconds
- [ ] No console errors
- [ ] No 401/403 errors
- [ ] Network requests complete within 2 seconds
- [ ] Memory usage stable over 1 hour
- [ ] Multi-tenant isolation working correctly

## Next Steps After Verification

1. **Production Deployment**
   - Set up SSL certificates for HTTPS
   - Configure firewall rules
   - Set up monitoring/alerting for dashboards

2. **Additional Dashboards**
   - PostgreSQL dashboard
   - Redis dashboard
   - Application metrics dashboard

3. **Custom Queries**
   - Create tenant-specific dashboard templates
   - Add drill-down capabilities
   - Export dashboard definitions

4. **Performance Tuning**
   - Profile slow queries
   - Optimize LogQL expressions
   - Consider caching strategies
