#!/usr/bin/env python3
"""
Send both METRICS and LOGS via OTLP/HTTP to OTEL Collector
Metrics → VictoriaMetrics Cluster (Tenant 1)
Logs → Loki
"""
import json
import time
import random
import http.client
import logging
from datetime import datetime, timezone

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(message)s')
logger = logging.getLogger(__name__)

OTEL_HOST = "localhost"
OTEL_PORT = 4318          # OTLP/HTTP port
SERVICE_NAME = "test-service"
TENANT_ID = "default-tenant"

def iso_timestamp():
    return datetime.now(timezone.utc).isoformat(timespec='milliseconds') + 'Z'

def send_log(message, severity_text="INFO", severity_number=9, extra_attrs=None):
    """Send logs via OTLP HTTP"""
    if extra_attrs is None:
        extra_attrs = {}

    timestamp_ns = int(time.time() * 1e9)

    payload = {
        "resourceLogs": [{
            "resource": {
                "attributes": [
                    {"key": "service.name", "value": {"stringValue": SERVICE_NAME}},
                    {"key": "tenant.id", "value": {"stringValue": TENANT_ID}}
                ]
            },
            "scopeLogs": [{
                "scope": {"name": "test-logger", "version": "1.0"},
                "logRecords": [{
                    "timeUnixNano": str(timestamp_ns),
                    "observedTimeUnixNano": str(timestamp_ns),
                    "severityNumber": severity_number,
                    "severityText": severity_text,
                    "body": {"stringValue": message},
                    "attributes": [
                        {"key": k, "value": {"stringValue": str(v)}}
                        for k, v in extra_attrs.items()
                    ],
                    "flags": 0
                }]
            }]
        }]
    }

    try:
        conn = http.client.HTTPConnection(OTEL_HOST, OTEL_PORT, timeout=10)
        headers = {"Content-Type": "application/json"}
        conn.request("POST", "/v1/logs", body=json.dumps(payload), headers=headers)
        response = conn.getresponse()
        body = response.read().decode()
        if response.status in (200, 202):
            logger.info(f"✓ Log sent: [{severity_text}] {message[:60]}...")
        else:
            logger.warning(f"✗ Log failed: HTTP {response.status}")
        conn.close()
    except Exception as e:
        logger.error(f"✗ Log connection error: {e}")

def send_metric(metric_name, value, attributes=None):
    """Send metrics via OTLP HTTP to VictoriaMetrics"""
    if attributes is None:
        attributes = {}
    
    timestamp_ns = int(time.time() * 1e9)
    
    # Build attribute list
    attr_list = [
        {"key": "service.name", "value": {"stringValue": SERVICE_NAME}},
        {"key": "tenant.id", "value": {"stringValue": TENANT_ID}}
    ]
    for k, v in attributes.items():
        attr_list.append({"key": k, "value": {"stringValue": str(v)}})
    
    # OTLP Metrics payload
    payload = {
        "resourceMetrics": [{
            "resource": {
                "attributes": attr_list
            },
            "scopeMetrics": [{
                "scope": {"name": "test-meter", "version": "1.0"},
                "metrics": [{
                    "name": metric_name,
                    "gauge": {
                        "dataPoints": [{
                            "timeUnixNano": str(timestamp_ns),
                            "asDouble": float(value),
                            "attributes": [
                                {"key": k, "value": {"stringValue": str(v)}}
                                for k, v in attributes.items()
                            ]
                        }]
                    }
                }]
            }]
        }]
    }
    
    try:
        conn = http.client.HTTPConnection(OTEL_HOST, OTEL_PORT, timeout=10)
        headers = {"Content-Type": "application/json"}
        conn.request("POST", "/v1/metrics", body=json.dumps(payload), headers=headers)
        response = conn.getresponse()
        body = response.read().decode()
        if response.status in (200, 202):
            logger.info(f"✓ Metric sent: {metric_name}={value}")
        else:
            logger.warning(f"✗ Metric failed: HTTP {response.status} - {body[:100]}")
        conn.close()
    except Exception as e:
        logger.error(f"✗ Metric connection error: {e}")

def main():
    print("🚀 Sending METRICS + LOGS to OTEL Collector")
    print(f"Metrics → VictoriaMetrics Cluster (Tenant 1)")
    print(f"Logs → Loki")
    print(f"Endpoint: http://{OTEL_HOST}:{OTEL_PORT}")
    print("\nGrafana queries:")
    print("  Metrics: http_requests_total, response_time_seconds")
    print("  Logs: {service_name=\"test-service\"}")
    print("Press Ctrl+C to stop\n")

    counter = 0
    
    try:
        while True:
            counter += 1
            
            # Generate random data
            response_time = random.uniform(0.05, 0.5)
            is_error = random.random() < 0.15
            status = "500" if is_error else "200"
            
            # ========== SEND METRICS ==========
            # Counter: total requests
            send_metric(
                "http_requests_total",
                1,
                {
                    "status": status,
                    "method": "GET",
                    "endpoint": "/api/test"
                }
            )
            
            # Gauge: response time
            send_metric(
                "response_time_seconds",
                response_time,
                {
                    "status": status,
                    "endpoint": "/api/test"
                }
            )
            
            # ========== SEND LOGS ==========
            if is_error:
                msg = f"Request {counter} failed: timeout after {response_time:.3f}s"
                send_log(msg, "ERROR", 21, {
                    "level": "ERROR",
                    "request_id": f"req_{counter}",
                    "status": status,
                    "response_time": f"{response_time:.3f}"
                })
            else:
                msg = f"Request {counter} processed in {response_time:.3f}s"
                send_log(msg, "INFO", 9, {
                    "level": "INFO",
                    "request_id": f"req_{counter}",
                    "status": status,
                    "response_time": f"{response_time:.3f}"
                })
            
            time.sleep(2)  # Send every 2 seconds
            
    except KeyboardInterrupt:
        print("\n\nStopped. Check Grafana for metrics and logs!")

if __name__ == "__main__":
    main()
