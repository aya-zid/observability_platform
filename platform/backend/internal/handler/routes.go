// internal/handler/routes.go
package handler

import (
	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/proxy"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

func SetupRoutes(e *echo.Echo, cfg *config.Config) {
	tenantRepo := repository.NewTenantRepository(infrastructure.DB)
	userRepo := repository.NewUserRepository(infrastructure.DB)

	// ── Auth routes (public) ─────────────────────────────────────────────────
	authService := service.NewAuthService(tenantRepo, userRepo)
	authHandler := NewAuthHandler(authService)

	e.POST("/signup", authHandler.Signup)
	e.POST("/login", authHandler.Login)

	// ── Health check ─────────────────────────────────────────────────────────
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"status":      "healthy",
			"db_status":   boolStatus(infrastructure.Ping()),
			"redis_status": boolStatus(infrastructure.PingRedis()),
		})
	})

	// ── Ingest routes (agent → backend → infra) ──────────────────────────────
	//
	// Authentication: Basic Auth where username = API key (password = API key too,
	// matching the otelcol prometheusremotewrite exporter convention).
	//
	// All three endpoints are protected by IngestMiddleware which:
	//   1. Validates the API key
	//   2. Resolves tenant (Redis → Postgres)
	//   3. Attaches CachedTenant to context for proxy handlers
	tenantCache := cache.NewTenantCache(infrastructure.Redis)
	ingestAuth := custommiddleware.IngestMiddleware(tenantCache, tenantRepo)

	ingest := e.Group("/ingest", ingestAuth)
	{
		// Prometheus remote_write → VictoriaMetrics vminsert (tenant isolated by URL path)
		ingest.POST("/metrics", proxy.MetricsProxy(cfg))

		// OTLP logs → Loki (tenant isolated by X-Scope-OrgID header)
		ingest.POST("/logs/*", proxy.LogsProxy(cfg))

		// OTLP traces → Tempo (tenant isolated by X-Scope-OrgID header)
		ingest.POST("/traces/*", proxy.TracesProxy(cfg))
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}