// internal/proxy/handlers.go
package proxy

import (
	"fmt"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// MetricsProxy → vminsert:8480/insert/{tenant.ID}/prometheus/api/v1/write
// tenant.ID (uint) is the VictoriaMetrics account ID — internal only, never exposed to agents
func MetricsProxy(cfg *config.Config) echo.HandlerFunc {
	return NewReverseProxy(ProxyConfig{
		TargetURL: fmt.Sprintf("http://%s:%s", cfg.VMInsertHost, cfg.VMInsertPort),
		RewritePath: func(tenantID uint, _ string) string {
			return fmt.Sprintf("/insert/%d/prometheus/api/v1/write", tenantID)
		},
		InjectHeaders: nil,
	})
}

// LogsProxy → loki:3100/otlp/v1/logs with X-Scope-OrgID: tenant.UUID
func LogsProxy(cfg *config.Config) echo.HandlerFunc {
	return NewReverseProxy(ProxyConfig{
		TargetURL: fmt.Sprintf("http://%s:%s", cfg.LokiHost, cfg.LokiPort),
		RewritePath: func(_ uint, _ string) string {
			return "/otlp/v1/logs"
		},
		InjectHeaders: func(_ uint, tenantUUID string) map[string]string {
			return map[string]string{"X-Scope-OrgID": tenantUUID}
		},
	})
}

// TracesProxy → tempo:4318/v1/traces with X-Scope-OrgID: tenant.UUID
func TracesProxy(cfg *config.Config) echo.HandlerFunc {
	return NewReverseProxy(ProxyConfig{
		TargetURL: fmt.Sprintf("http://%s:%s", cfg.TempoHost, cfg.TempoPort),
		RewritePath: func(_ uint, _ string) string {
			return "/v1/traces"
		},
		InjectHeaders: func(_ uint, tenantUUID string) map[string]string {
			return map[string]string{"X-Scope-OrgID": tenantUUID}
		},
	})
}