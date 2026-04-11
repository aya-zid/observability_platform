# Nginx Integration Guide

This guide explains how to set up Nginx monitoring with the observability platform, including log format configuration for capture of request times and other metrics.

## Prerequisites

- Nginx installed on the target machine
- Agent (otelcol) installed and running on the target machine
- Integration script obtained from the platform

## Step 1: Update Nginx Log Format

To capture request duration and other metrics, update your Nginx configuration to include the `$request_time` variable.

Edit your Nginx config file (typically `/etc/nginx/nginx.conf` or `/etc/nginx/sites-available/default`):

```nginx
# Add or update the log_format directive:
log_format platform_fmt '$remote_addr - $remote_user [$time_local] '
    '"$request" $status $body_bytes_sent '
    '"$http_referer" "$http_user_agent" '
    'rt=$request_time';

# Update your access_log directive to use the new format:
access_log /var/log/nginx/access.log platform_fmt;
```

### Explanation of Fields

| Field | Description |
|-------|-------------|
| `$remote_addr` | Client IP address |
| `$remote_user` | Authenticated user |
| `$time_local` | Local timestamp |
| `$request` | HTTP method, path, and version |
| `$status` | HTTP response code |
| `$body_bytes_sent` | Response body size |
| `$http_referer` | Referrer header |
| `$http_user_agent` | User agent header |
| `rt=$request_time` | **Request time in seconds** (what we're adding) |

### Example Log Output

```
127.0.0.1 - - [27/Mar/2026:11:20:16 +0100] "GET /api/metrics HTTP/1.1" 200 4521 "-" "curl/7.68.0" rt=0.045
192.168.1.100 - - [27/Mar/2026:11:20:17 +0100] "POST /api/data HTTP/1.1" 201 256 "-" "Mozilla/5.0" rt=0.123
```

### Reload Nginx

After updating the configuration:

```bash
sudo nginx -t  # Test configuration
sudo systemctl reload nginx
```

## Step 2: Run the Integration Script

Once Nginx is configured, run the integration script provided by the platform:

```bash
sudo bash integration-script.sh
```

The script will:
1. Back up your current otelcol configuration
2. Add Nginx log file collector to otelcol
3. Configure parsing rules for the new log format
4. Reload the otelcol service

## Step 3: Verify Integration

Check that logs are being collected:

```bash
# Check otelcol status
sudo systemctl status otelcol

# Check logs (last 50 lines)
sudo journalctl -u otelcol -n 50 -f
```

Look for lines indicating successful Nginx log parsing.

## Step 4: Access the Dashboard

Once integration is confirmed, the Nginx dashboard will be available in the platform's integrations section, showing:

- Request rate (requests/sec)
- Error rates (4xx, 5xx)
- Top requested paths
- Request duration (p95, p99)
- Active connections
- Recent log entries

## Otelcol Configuration Details

The integration script configures the filelog receiver with these parsing rules:

```yaml
receivers:
  filelog/nginx_integration:
    include:
      - /var/log/nginx/access.log
    include_file_path: true
    start_at: end
    operators:
      - type: regex_parser
        regex: '^(?P<remote_addr>[\S]+) \S+ \S+ \[(?P<time_local>[^\]]+)\] "(?P<method>\S+) (?P<path>\S+) \S+" (?P<status>\d{3}) (?P<bytes>\d+|-) "[^"]*" "[^"]*"(?: rt=(?P<request_time>[\d.]+))?'
        on_error: send_quiet
      - type: resource
        attributes:
          service.name: nginx
          job: nginx
    resource:
      service.name: nginx
      job: nginx
```

### Parsed Fields

| Field | Type | Description |
|-------|------|-------------|
| `remote_addr` | string | Client IP |
| `time_local` | string | Timestamp |
| `method` | string | HTTP method (GET, POST, etc.) |
| `path` | string | Request path |
| `status` | int | HTTP status code |
| `bytes` | int | Response body size |
| `request_time` | float | Request duration in seconds |

## Querying Request Data in Dashboards

Once configured, you can query request times and other metrics in your platform dashboards:

### Request Rate Query
```
sum(rate({service_name="nginx"}[1m]))
```

### Error Rate Query
```
sum(rate({service_name="nginx"} | pattern `<_> "<_>" <status> <_>` | status =~ "5.." [1m]))
```

### p95 Response Time Query
```
quantile_over_time(0.95,
  {service_name="nginx"}
  | pattern `<_> rt=<rt>`
  | unwrap rt [1m]
) by ()
```

### Top Paths Query
```
topk(10,
  sum by (path) (
    rate({service_name="nginx"} | pattern `<_> "<method> <path> <_>" <_> <_>` [5m])
  )
)
```

## Troubleshooting

### Logs not appearing in dashboard

1. **Verify nginx log file permissions:**
   ```bash
   ls -la /var/log/nginx/access.log
   ```
   The otelcol service must have read permissions.

2. **Check otelcol config:**
   ```bash
   sudo otelcol validate --config file:/etc/otelcol/config.yaml
   ```

3. **Check if `request_time` is being parsed:**
   ```bash
   # Query without rt field
   {service_name="nginx"} | pattern `<_> "<method> <path>"`
   # Should return results even if rt parsing failed
   ```

### Request time not captured

1. **Verify nginx log format:**
   ```bash
   tail -f /var/log/nginx/access.log
   ```
   Look for `rt=` at the end of each line.

2. **Update regex if needed:**
   If your nginx log format is different, the regex in otelcol config needs to be adjusted to match.

## Advanced: Custom Nginx Log Format

If you're using a different nginx log format, you need to:

1. Update the `log_format` in nginx.conf to include `$request_time`
2. Contact support or manually update the regex in `/etc/otelcol/config.yaml` to match your format

The regex pattern is in the `filelog/nginx_integration` receiver section.

## Example: Querying in React Component

```typescript
const { data: responseTime } = useQuery({
  queryKey: ['nginx-p95-response-time'],
  queryFn: async () => {
    const res = await apiClient.get('/api/query/logs/range', {
      params: {
        query: `quantile_over_time(0.95, {service_name="nginx"} | pattern \`<_> rt=<rt>\` | unwrap rt [1m]) by ()`,
        start: start * 1e9,
        end: now * 1e9,
        step: '60s'
      }
    })
    return res.data
  }
})
```

## See Also

- [Platform Query API Documentation](../api/QUERIES.md)
- [Loki LogQL Reference](https://grafana.com/docs/loki/latest/logql/)
- [Nginx Log Format Documentation](https://nginx.org/en/docs/http/ngx_http_log_module.html)
