// internal/handler/routes.go
package handler

import (
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/cache"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/proxy"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

func touchHostsOnIngest(hostRepo *repository.HostRepository, next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		err := next(c)
		tenant, ok := c.Get(custommiddleware.TenantContextKey).(*cache.CachedTenant)
		if ok && tenant != nil {
			_ = hostRepo.TouchHostsByTenant(c.Request().Context(), tenant.ID)
		}
		return err
	}
}

func SetupRoutes(e *echo.Echo, cfg *config.Config) {
	tenantRepo := repository.NewTenantRepository(infrastructure.DB)
	userRepo := repository.NewUserRepository(infrastructure.DB)
	hostRepo := repository.NewHostRepository(infrastructure.DB)
	alertsRepo := repository.NewAlertsRepository(infrastructure.DB)
	discoveryRepo := repository.NewDiscoveryRepository(infrastructure.DB)
	tenantCache := cache.NewTenantCache(infrastructure.Redis)

	// ── Auth routes (public) ─────────────────────────────────────────────────
	authService := service.NewAuthService(infrastructure.DB, infrastructure.Redis, cfg, tenantRepo, userRepo)
	authHandler := NewAuthHandler(authService, cfg)
	queryService := service.NewQueryService(cfg, tenantRepo)
	discoveryService := service.NewDiscoveryService(hostRepo, discoveryRepo, alertsRepo)
	integrationService := service.NewIntegrationService(cfg, tenantRepo, discoveryRepo)

	rlWindow := time.Duration(cfg.AuthRateLimitWindowSeconds) * time.Second
	e.POST("/signup", authHandler.Signup, custommiddleware.RateLimitByIP(infrastructure.Redis, "signup", cfg.AuthSignupRateLimitPerWindow, rlWindow))
	e.POST("/login", authHandler.Login, custommiddleware.RateLimitByIP(infrastructure.Redis, "login", cfg.AuthLoginRateLimitPerWindow, rlWindow))
	e.POST("/refresh", authHandler.Refresh, custommiddleware.RateLimitByIP(infrastructure.Redis, "refresh", cfg.AuthRefreshRateLimitPerWindow, rlWindow))
	e.POST("/logout", authHandler.Logout)
	installHandler := NewInstallHandler(cfg)
	integrationHandler := NewIntegrationHandler(integrationService)
	alertsHandler := NewAlertsHandler(alertsRepo)

	nginxDashboardService := service.NewNginxDashboardService(cfg, infrastructure.Redis)
	nginxDashboardHandler := NewNginxDashboardHandler(
		nginxDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	nginxPerformanceService := service.NewNginxPerformanceService(cfg, infrastructure.Redis)
	nginxPerformanceHandler := NewNginxPerformanceHandler(
		nginxPerformanceService,
		discoveryRepo,
		tenantRepo,
	)
	postgresDashboardService := service.NewPostgresDashboardService(cfg, infrastructure.Redis)
	postgresDashboardHandler := NewPostgresDashboardHandler(
		postgresDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	apacheDashboardService := service.NewApacheDashboardService(cfg, infrastructure.Redis)
	apacheDashboardHandler := NewApacheDashboardHandler(
		apacheDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	apachePerformanceService := service.NewApachePerformanceService(cfg, infrastructure.Redis)
	apachePerformanceHandler := NewApachePerformanceHandler(
		apachePerformanceService,
		discoveryRepo,
		tenantRepo,
	)

	phpfpmDashboardService := service.NewPHPFPMDashboardService(cfg, infrastructure.Redis)
	phpfpmDashboardHandler := NewPHPFPMDashboardHandler(
		phpfpmDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	redisDashboardService := service.NewRedisDashboardService(cfg, infrastructure.Redis)
	redisDashboardHandler := NewRedisDashboardHandler(
		redisDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	mariadbDashboardService := service.NewMariaDBDashboardService(cfg, infrastructure.Redis)
	mariadbDashboardHandler := NewMariaDBDashboardHandler(
		mariadbDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	hostDashboardService := service.NewHostDashboardService(cfg, infrastructure.Redis)
	hostLogsService := service.NewHostLogsService(cfg, infrastructure.Redis)
	hostDashboardHandler := NewHostDashboardHandler(
		hostDashboardService,
		hostLogsService,
		hostRepo,
		tenantRepo,
	)

	e.GET("/install.sh", installHandler.AgentInstallScript)
	e.GET("/integrations/nginx.sh", integrationHandler.NginxScript)
	e.GET("/integrations/generic.sh", integrationHandler.GenericScript)

	// ── Platform API (browser auth) ───────────────────────────────────────────
	tenantHandler := NewTenantHandler(tenantRepo, userRepo, tenantCache)
	discoveryAPIHandler := NewDiscoveryAPIHandler(discoveryService)
	queryHandler := NewQueryHandler(queryService)
	api := e.Group("/api", custommiddleware.AuthMiddleware(tenantRepo))
	{
		api.GET("/me", tenantHandler.GetMe)
		api.GET("/api-key", tenantHandler.GetAPIKey)
		api.POST("/api-key/rotate", tenantHandler.RotateAPIKey)
		api.GET("/alerts/vulnerabilities", alertsHandler.ListVulnerabilityAlerts)
		api.GET("/catalog", discoveryAPIHandler.ListCatalog)
		api.GET("/discovery/hosts", discoveryAPIHandler.ListHosts)
		api.GET("/discovery/services", discoveryAPIHandler.ListServices)
		api.GET("/discovery/services/:id", discoveryAPIHandler.GetService)
		api.PATCH("/discovery/services/:id/collection", discoveryAPIHandler.UpdateCollection)
		api.PATCH("/discovery/services/:id/config", discoveryAPIHandler.UpdateServiceConfig)
		api.POST("/discovery/services/:id/integration", integrationHandler.CreateInstallCommand)

		// Query routes for dashboards — tenant-isolated via JWT claims
		api.GET("/query/metrics", queryHandler.QueryMetrics)
		api.GET("/query/logs", queryHandler.QueryLogs)
		api.GET("/query/logs/range", queryHandler.QueryLogsRange)
		api.GET("/query/logs/labels", queryHandler.QueryLogsLabels)
		api.GET("/grafana/dashboard-url", queryHandler.GrafanaDashboardURL)

		// Nginx dashboard routes
		api.GET("/nginx/:id/domains", nginxDashboardHandler.Domains)
		api.GET("/nginx/:id/dashboard", nginxDashboardHandler.Dashboard)
		api.GET("/nginx/:id/explorer", nginxDashboardHandler.Explorer)
		api.GET("/nginx/:id/error-logs", nginxDashboardHandler.ErrorLogs)
		api.GET("/nginx/:id/ip-investigation", nginxDashboardHandler.IPInvestigation)
		api.GET("/nginx/:id/slow-requests", nginxDashboardHandler.SlowRequests)
		// Nginx performance chart routes (Chart 1, 2, 3)
		api.GET("/nginx/:id/performance/heatmap", nginxPerformanceHandler.Heatmap)
		api.GET("/nginx/:id/performance/scatter", nginxPerformanceHandler.Scatter)
		api.GET("/nginx/:id/performance/dist", nginxPerformanceHandler.Distribution)

		// PostgreSQL dashboard routes
		api.GET("/postgres/:id/dashboard", postgresDashboardHandler.Dashboard)
		api.GET("/postgres/:id/config", postgresDashboardHandler.Config)
		api.GET("/postgres/:id/logs", postgresDashboardHandler.Logs)

		// PHP-FPM dashboard routes
		api.GET("/phpfpm/:id/dashboard", phpfpmDashboardHandler.Dashboard)
		api.GET("/phpfpm/:id/logs", phpfpmDashboardHandler.Logs)
		api.GET("/phpfpm/:id/logs-meta", phpfpmDashboardHandler.LogsMeta)

		// Redis dashboard routes
		api.GET("/redis/:id/dashboard", redisDashboardHandler.Dashboard)
		api.GET("/redis/:id/logs", redisDashboardHandler.Logs)

		// MariaDB dashboard routes
		api.GET("/mariadb/:id/dashboard", mariadbDashboardHandler.Dashboard)
		api.GET("/mariadb/:id/config", mariadbDashboardHandler.Config)
		api.GET("/mariadb/:id/logs", mariadbDashboardHandler.Logs)
		api.GET("/mariadb/:id/error-logs", mariadbDashboardHandler.ErrorLogs)
		api.GET("/mariadb/:id/slow-query-logs", mariadbDashboardHandler.SlowQueryLogs)

		// Apache dashboard routes
		api.GET("/apache/:id/dashboard", apacheDashboardHandler.Dashboard)
		api.GET("/apache/:id/slow-requests", apacheDashboardHandler.SlowRequests)
		api.GET("/apache/:id/explorer", apacheDashboardHandler.Explorer)
		api.GET("/apache/:id/domains", apacheDashboardHandler.Domains)
		api.GET("/apache/:id/error-logs", apacheDashboardHandler.ErrorLogs)
		api.GET("/apache/:id/ip-investigation", apacheDashboardHandler.IPInvestigation)
		// Apache performance chart routes
		api.GET("/apache/:id/performance/heatmap", apachePerformanceHandler.Heatmap)
		api.GET("/apache/:id/performance/scatter", apachePerformanceHandler.Scatter)
		api.GET("/apache/:id/performance/dist", apachePerformanceHandler.Distribution)

		// Host dashboard routes (each section is an independent endpoint)
		api.GET("/hosts/:machineId/dashboard/summary", hostDashboardHandler.Summary)
		api.GET("/hosts/:machineId/dashboard/cpu", hostDashboardHandler.CPU)
		api.GET("/hosts/:machineId/dashboard/memory", hostDashboardHandler.Memory)
		api.GET("/hosts/:machineId/dashboard/disk", hostDashboardHandler.Disk)
		api.GET("/hosts/:machineId/dashboard/network", hostDashboardHandler.Network)
		api.GET("/hosts/:machineId/dashboard/processes", hostDashboardHandler.Processes)
		api.GET("/hosts/:machineId/logs", hostDashboardHandler.Logs)
		api.GET("/hosts/:machineId/logs/health-summary", hostDashboardHandler.LogsHealthSummary)
	}

	// ── Health check ─────────────────────────────────────────────────────────
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"status":       "healthy",
			"db_status":    boolStatus(infrastructure.Ping()),
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
	ingestAuth := custommiddleware.IngestMiddleware(tenantCache, tenantRepo)
	hostHandler := NewHostHandler(hostRepo)
	discoveryHandler := NewDiscoveryHandler(infrastructure.DB)

	ingest := e.Group("/ingest", ingestAuth)
	{
		// Host lifecycle (agent startup + periodic liveness heartbeat)
		ingest.POST("/hosts/register", hostHandler.Register)
		ingest.POST("/hosts/heartbeat", hostHandler.Heartbeat)
		// Discovery ingest: split host services and container services into two endpoints.
		ingest.POST("/discovery/host-services", discoveryHandler.IngestHostServices)
		ingest.POST("/discovery/container-services", discoveryHandler.IngestContainerServices)
		// Backward-compatible combined endpoint.
		ingest.POST("/discovery/report", discoveryHandler.IngestInstallReport)
		ingest.POST("/integrations/status", integrationHandler.IngestStatus)

		// Prometheus remote_write → VictoriaMetrics vminsert (tenant isolated by URL path)
		ingest.POST("/metrics", touchHostsOnIngest(hostRepo, proxy.MetricsProxy(cfg)))

		// OTLP logs → Loki (tenant isolated by X-Scope-OrgID header)
		ingest.POST("/logs/*", touchHostsOnIngest(hostRepo, proxy.LogsProxy(cfg)))

		// OTLP traces → Tempo (tenant isolated by X-Scope-OrgID header)
		ingest.POST("/traces/*", touchHostsOnIngest(hostRepo, proxy.TracesProxy(cfg)))
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}
