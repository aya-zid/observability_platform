#!/bin/bash
# infrastructure/scripts/create-grafana-dashboard.sh
#
# This script creates the Nginx monitoring dashboard in Grafana via the API.
# The dashboard includes time-series panels for request rate, connections, and status codes.
#
# Usage:
#   bash create-grafana-dashboard.sh [GRAFANA_URL] [USERNAME] [PASSWORD]
#
# Defaults:
#   GRAFANA_URL=http://localhost:3000
#   USERNAME=admin
#   PASSWORD=admin

set -e

GRAFANA_URL="${1:-http://localhost:3000}"
USERNAME="${2:-admin}"
PASSWORD="${3:-admin}"

echo "📊 Creating Nginx Grafana dashboard..."
echo "   URL: $GRAFANA_URL"
echo "   User: $USERNAME"

# Read the dashboard JSON
DASHBOARD_JSON=$(cat <<'EOF'
{
  "dashboard": {
    "id": null,
    "uid": "nginx-overview",
    "title": "Nginx Overview",
    "tags": ["nginx", "web-server"],
    "timezone": "browser",
    "schemaVersion": 27,
    "version": 1,
    "refresh": "30s",
    "time": {
      "from": "now-1h",
      "to": "now"
    },
    "templating": {
      "list": []
    },
    "panels": [
      {
        "id": 1,
        "title": "Request Rate",
        "type": "timeseries",
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 0,
          "y": 0
        },
        "targets": [
          {
            "refId": "A",
            "datasource": {
              "type": "loki",
              "uid": "loki",
              "name": "Loki"
            },
            "expr": "sum(rate({service_name=\"nginx\"}[1m]))",
            "legendFormat": "requests/sec",
            "interval": "60s"
          }
        ],
        "options": {
          "tooltip": {
            "mode": "multi"
          },
          "legend": {
            "calcs": ["mean", "max"],
            "displayMode": "list",
            "placement": "bottom"
          }
        },
        "fieldConfig": {
          "defaults": {
            "unit": "reqps",
            "custom": {
              "lineWidth": 2,
              "fillOpacity": 10
            }
          },
          "overrides": []
        }
      },
      {
        "id": 2,
        "title": "Active Connections",
        "type": "timeseries",
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 12,
          "y": 0
        },
        "targets": [
          {
            "refId": "A",
            "datasource": {
              "type": "prometheus",
              "uid": "victoriametrics",
              "name": "VictoriaMetrics"
            },
            "expr": "nginx_connections_active",
            "legendFormat": "active connections",
            "interval": "30s"
          }
        ],
        "options": {
          "tooltip": {
            "mode": "multi"
          },
          "legend": {
            "calcs": ["mean", "max"],
            "displayMode": "list",
            "placement": "bottom"
          }
        },
        "fieldConfig": {
          "defaults": {
            "unit": "short",
            "custom": {
              "lineWidth": 2,
              "fillOpacity": 10
            }
          },
          "overrides": []
        }
      },
      {
        "id": 3,
        "title": "HTTP Status Codes (Rate)",
        "type": "timeseries",
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 0,
          "y": 8
        },
        "targets": [
          {
            "refId": "A",
            "datasource": {
              "type": "loki",
              "uid": "loki",
              "name": "Loki"
            },
            "expr": "sum(rate({service_name=\"nginx\"} | pattern `<_> \\\"<_>\\\" <status> <_>` | status =~ \\\"2..\\\" [1m])) by ()",
            "legendFormat": "2xx",
            "interval": "60s"
          },
          {
            "refId": "B",
            "datasource": {
              "type": "loki",
              "uid": "loki",
              "name": "Loki"
            },
            "expr": "sum(rate({service_name=\"nginx\"} | pattern `<_> \\\"<_>\\\" <status> <_>` | status =~ \\\"4..\\\" [1m])) by ()",
            "legendFormat": "4xx",
            "interval": "60s"
          },
          {
            "refId": "C",
            "datasource": {
              "type": "loki",
              "uid": "loki",
              "name": "Loki"
            },
            "expr": "sum(rate({service_name=\"nginx\"} | pattern `<_> \\\"<_>\\\" <status> <_>` | status =~ \\\"5..\\\" [1m])) by ()",
            "legendFormat": "5xx",
            "interval": "60s"
          }
        ],
        "options": {
          "tooltip": {
            "mode": "multi"
          },
          "legend": {
            "calcs": ["mean", "max"],
            "displayMode": "list",
            "placement": "bottom"
          }
        },
        "fieldConfig": {
          "defaults": {
            "unit": "reqps",
            "custom": {
              "lineWidth": 2,
              "fillOpacity": 10
            }
          },
          "overrides": [
            {
              "matcher": {
                "id": "byName",
                "options": "2xx"
              },
              "properties": [
                {
                  "id": "color",
                  "value": {
                    "mode": "fixed",
                    "fixedColor": "green"
                  }
                }
              ]
            },
            {
              "matcher": {
                "id": "byName",
                "options": "4xx"
              },
              "properties": [
                {
                  "id": "color",
                  "value": {
                    "mode": "fixed",
                    "fixedColor": "yellow"
                  }
                }
              ]
            },
            {
              "matcher": {
                "id": "byName",
                "options": "5xx"
              },
              "properties": [
                {
                  "id": "color",
                  "value": {
                    "mode": "fixed",
                    "fixedColor": "red"
                  }
                }
              ]
            }
          ]
        }
      },
      {
        "id": 4,
        "title": "Request Duration (p95)",
        "type": "timeseries",
        "gridPos": {
          "h": 8,
          "w": 12,
          "x": 12,
          "y": 8
        },
        "targets": [
          {
            "refId": "A",
            "datasource": {
              "type": "loki",
              "uid": "loki",
              "name": "Loki"
            },
            "expr": "quantile_over_time(0.95, {service_name=\"nginx\"} | pattern `<_> rt=<rt>` | unwrap rt [1m]) by ()",
            "legendFormat": "p95",
            "interval": "60s"
          }
        ],
        "options": {
          "tooltip": {
            "mode": "multi"
          },
          "legend": {
            "calcs": ["mean", "max"],
            "displayMode": "list",
            "placement": "bottom"
          }
        },
        "fieldConfig": {
          "defaults": {
            "unit": "ms",
            "custom": {
              "lineWidth": 2,
              "fillOpacity": 10
            }
          },
          "overrides": []
        }
      }
    ]
  },
  "overwrite": true
}
EOF
)

# Create the dashboard via Grafana API
echo "Sending dashboard to Grafana API..."

RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Basic $(echo -n "$USERNAME:$PASSWORD" | base64)" \
  "$GRAFANA_URL/api/dashboards/db" \
  -d "$DASHBOARD_JSON")

# Check for errors
if echo "$RESPONSE" | grep -q '"status":"success"\|"id":'; then
  echo "✅ Dashboard created successfully!"
  echo "$RESPONSE" | grep -oP '"url":"\K[^"]+' | head -1 | xargs -I {} echo "Dashboard URL: $GRAFANA_URL{}"
else
  echo "❌ Failed to create dashboard:"
  echo "$RESPONSE"
  exit 1
fi

echo ""
echo "✨ Nginx monitoring dashboard is ready!"
echo "   Access it at: $GRAFANA_URL/d/nginx-overview"
