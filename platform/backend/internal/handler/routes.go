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

func SetupRoutes(e *echo.Echo, cfg *config.Config, emailSender *service.EmailSender) {
	tenantRepo := repository.NewTenantRepository(infrastructure.DB)
	userRepo := repository.NewUserRepository(infrastructure.DB)
	hostRepo := repository.NewHostRepository(infrastructure.DB)
	alertsRepo := repository.NewAlertsRepository(infrastructure.DB)
	discoveryRepo := repository.NewDiscoveryRepository(infrastructure.DB)
	tenantCache := cache.NewTenantCache(infrastructure.Redis)
	invitationRepo := repository.NewInvitationRepository(infrastructure.DB)
	notifRepo := repository.NewNotificationChannelRepository(infrastructure.DB)
	planRepo := repository.NewPlanRepository(infrastructure.DB)
	subscriptionRepo := repository.NewSubscriptionRepository(infrastructure.DB)
	usageRepo := repository.NewUsageRepository(infrastructure.DB)
	alertRuleRepo := repository.NewAlertRuleRepository(infrastructure.DB)

	// ── Auth routes (public) ─────────────────────────────────────────────────
	authService := service.NewAuthService(infrastructure.DB, infrastructure.Redis, cfg, tenantRepo, userRepo)
	authHandler := NewAuthHandler(authService, cfg)
	queryService := service.NewQueryService(cfg, tenantRepo)
	discoveryService := service.NewDiscoveryService(hostRepo, discoveryRepo, alertsRepo)
	integrationService := service.NewIntegrationService(cfg, tenantRepo, discoveryRepo)

	// ── Team / Profile / Settings / Billing services ─────────────────────────
	teamService := service.NewTeamService(infrastructure.DB, userRepo, invitationRepo, tenantRepo, emailSender, cfg.PlatformURL)
	profileService := service.NewProfileService(infrastructure.DB, userRepo)
	billingService := service.NewBillingService(planRepo, subscriptionRepo, usageRepo, hostRepo)

	rlWindow := time.Duration(cfg.AuthRateLimitWindowSeconds) * time.Second
	e.POST("/signup", authHandler.Signup, custommiddleware.RateLimitByIP(infrastructure.Redis, "signup", cfg.AuthSignupRateLimitPerWindow, rlWindow))
	e.POST("/login", authHandler.Login, custommiddleware.RateLimitByIP(infrastructure.Redis, "login", cfg.AuthLoginRateLimitPerWindow, rlWindow))
	e.POST("/refresh", authHandler.Refresh, custommiddleware.RateLimitByIP(infrastructure.Redis, "refresh", cfg.AuthRefreshRateLimitPerWindow, rlWindow))
	e.POST("/logout", authHandler.Logout)
	installHandler := NewInstallHandler(cfg)
	integrationHandler := NewIntegrationHandler(integrationService)
	alertsHandler := NewAlertsHandler(alertsRepo)
	teamHandler := NewTeamHandler(teamService)
	settingsHandler := NewSettingsHandler(tenantRepo)
	notificationHandler := NewNotificationHandler(notifRepo)
	profileHandler := NewProfileHandler(profileService)
	billingHandler := NewBillingHandler(billingService)
	alertRulesHandler := NewAlertRulesHandler(alertRuleRepo)

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

	overviewService := service.NewOverviewService(cfg, infrastructure.Redis, hostRepo, discoveryRepo)
	overviewHandler := NewOverviewHandler(overviewService, tenantRepo)

	hostDashboardService := service.NewHostDashboardService(cfg, infrastructure.Redis)
	hostLogsService := service.NewHostLogsService(cfg, infrastructure.Redis)
	hostDashboardHandler := NewHostDashboardHandler(
		hostDashboardService,
		hostLogsService,
		hostRepo,
		tenantRepo,
	)

	containerDashboardService := service.NewContainerDashboardService(cfg, infrastructure.Redis)
	containerDashboardHandler := NewContainerDashboardHandler(
		containerDashboardService,
		discoveryRepo,
		tenantRepo,
	)

	e.GET("/install.sh", installHandler.AgentInstallScript)
	e.GET("/uninstall.sh", installHandler.AgentUninstallScript)
	e.GET("/integrations/nginx.sh", integrationHandler.NginxScript)
	e.GET("/integrations/generic.sh", integrationHandler.GenericScript)

	// Invite routes (public)
	e.GET("/invite/:token", teamHandler.ValidateInvitation)
	e.POST("/invite/:token/accept", teamHandler.AcceptInvitation)

	// ── Platform API (browser auth) ───────────────────────────────────────────
	tenantHandler := NewTenantHandler(tenantRepo, userRepo, tenantCache)
	discoveryAPIHandler := NewDiscoveryAPIHandler(discoveryService)
	queryHandler := NewQueryHandler(queryService)
	api := e.Group("/api", custommiddleware.AuthMiddleware(tenantRepo))
	{
		// ── Member-accessible mutations ─────────────────────────
		// Profile (own name / password)
		api.GET("/profile", profileHandler.GetProfile)
		api.PATCH("/profile", profileHandler.UpdateProfile)
		api.POST("/profile/change-password", profileHandler.ChangePassword)

		// Org settings — readable by all (company name, timezone), writable only by admin
		api.GET("/settings/org", settingsHandler.GetOrgSettings)

		// Service config — members can enable/disable collection, save config, generate commands
		api.PATCH("/discovery/services/:id/collection", discoveryAPIHandler.UpdateCollection)
		api.PATCH("/discovery/services/:id/config", discoveryAPIHandler.UpdateServiceConfig)
		api.POST("/discovery/services/:id/integration", integrationHandler.CreateInstallCommand)

		// ── Admin-only mutations (members can still GET) ─────────
		r := api.Group("", custommiddleware.RequireAdminForMutation())
		{
			r.GET("/me", tenantHandler.GetMe)
			r.GET("/api-key", tenantHandler.GetAPIKey)
			r.POST("/api-key/rotate", tenantHandler.RotateAPIKey)

			// Settings — fully admin-only
			settings := r.Group("/settings", custommiddleware.RequireRole("admin"))
			settings.PATCH("/org", settingsHandler.UpdateOrgSettings)
			settings.GET("/team", teamHandler.ListTeam)
			settings.POST("/team/invite", teamHandler.InviteTeammate)
			settings.DELETE("/team/:userId", teamHandler.RemoveMember)
			settings.PATCH("/team/:userId/role", teamHandler.ChangeRole)
			settings.POST("/team/invitations/:id/resend", teamHandler.ResendInvitation)
			settings.DELETE("/team/invitations/:id", teamHandler.RevokeInvitation)
			settings.GET("/notifications", notificationHandler.ListChannels)
			settings.POST("/notifications", notificationHandler.AddChannel)
			settings.DELETE("/notifications/:id", notificationHandler.DeleteChannel)
			settings.GET("/billing", billingHandler.GetBilling)
			settings.POST("/billing/change-plan", billingHandler.ChangePlan)
			settings.GET("/billing/usage", billingHandler.GetUsageHistory)

			// Alert rules / events
			r.GET("/alerts/templates", alertRulesHandler.ListTemplates)
			r.GET("/alerts/templates/:serviceType", alertRulesHandler.ListTemplatesByService)
			r.POST("/alerts/rules", alertRulesHandler.CreateRule)
			r.GET("/alerts/rules", alertRulesHandler.ListRules)
			r.PATCH("/alerts/rules/:id", alertRulesHandler.UpdateRule)
			r.DELETE("/alerts/rules/:id", alertRulesHandler.DeleteRule)
			r.GET("/alerts/events", alertRulesHandler.ListAlertEvents)
			r.POST("/alerts/events/:id/acknowledge", alertRulesHandler.AcknowledgeEvent)
			r.GET("/overview", overviewHandler.GetOverview)
			r.GET("/alerts/vulnerabilities", alertsHandler.ListVulnerabilityAlerts)
			r.GET("/catalog", discoveryAPIHandler.ListCatalog)
			r.GET("/discovery/hosts", discoveryAPIHandler.ListHosts)
			r.GET("/discovery/services", discoveryAPIHandler.ListServices)
			r.GET("/discovery/services/:id", discoveryAPIHandler.GetService)
			r.GET("/discovery/containers", discoveryAPIHandler.ListContainers)
			r.GET("/discovery/containers/:id", discoveryAPIHandler.GetContainer)

			// Query routes for dashboards
			r.GET("/query/metrics", queryHandler.QueryMetrics)
			r.GET("/query/logs", queryHandler.QueryLogs)
			r.GET("/query/logs/range", queryHandler.QueryLogsRange)
			r.GET("/query/logs/labels", queryHandler.QueryLogsLabels)
			r.GET("/grafana/dashboard-url", queryHandler.GrafanaDashboardURL)

			// Nginx dashboard routes
			r.GET("/nginx/:id/domains", nginxDashboardHandler.Domains)
			r.GET("/nginx/:id/dashboard", nginxDashboardHandler.Dashboard)
			r.GET("/nginx/:id/explorer", nginxDashboardHandler.Explorer)
			r.GET("/nginx/:id/error-logs", nginxDashboardHandler.ErrorLogs)
			r.GET("/nginx/:id/ip-investigation", nginxDashboardHandler.IPInvestigation)
			r.GET("/nginx/:id/slow-requests", nginxDashboardHandler.SlowRequests)
			r.GET("/nginx/:id/performance/heatmap", nginxPerformanceHandler.Heatmap)
			r.GET("/nginx/:id/performance/scatter", nginxPerformanceHandler.Scatter)
			r.GET("/nginx/:id/performance/dist", nginxPerformanceHandler.Distribution)

			// PostgreSQL dashboard routes
			r.GET("/postgres/:id/dashboard", postgresDashboardHandler.Dashboard)
			r.GET("/postgres/:id/config", postgresDashboardHandler.Config)
			r.GET("/postgres/:id/logs", postgresDashboardHandler.Logs)

			// PHP-FPM dashboard routes
			r.GET("/phpfpm/:id/dashboard", phpfpmDashboardHandler.Dashboard)
			r.GET("/phpfpm/:id/logs", phpfpmDashboardHandler.Logs)
			r.GET("/phpfpm/:id/logs-meta", phpfpmDashboardHandler.LogsMeta)

			// Redis dashboard routes
			r.GET("/redis/:id/dashboard", redisDashboardHandler.Dashboard)
			r.GET("/redis/:id/logs", redisDashboardHandler.Logs)

			// MariaDB dashboard routes
			r.GET("/mariadb/:id/dashboard", mariadbDashboardHandler.Dashboard)
			r.GET("/mariadb/:id/config", mariadbDashboardHandler.Config)
			r.GET("/mariadb/:id/logs", mariadbDashboardHandler.Logs)
			r.GET("/mariadb/:id/error-logs", mariadbDashboardHandler.ErrorLogs)
			r.GET("/mariadb/:id/slow-query-logs", mariadbDashboardHandler.SlowQueryLogs)

			// Apache dashboard routes
			r.GET("/apache/:id/dashboard", apacheDashboardHandler.Dashboard)
			r.GET("/apache/:id/slow-requests", apacheDashboardHandler.SlowRequests)
			r.GET("/apache/:id/explorer", apacheDashboardHandler.Explorer)
			r.GET("/apache/:id/domains", apacheDashboardHandler.Domains)
			r.GET("/apache/:id/error-logs", apacheDashboardHandler.ErrorLogs)
			r.GET("/apache/:id/ip-investigation", apacheDashboardHandler.IPInvestigation)
			r.GET("/apache/:id/performance/heatmap", apachePerformanceHandler.Heatmap)
			r.GET("/apache/:id/performance/scatter", apachePerformanceHandler.Scatter)
			r.GET("/apache/:id/performance/dist", apachePerformanceHandler.Distribution)

			// Host dashboard routes
			r.GET("/hosts/:machineId/dashboard/summary", hostDashboardHandler.Summary)
			r.GET("/hosts/:machineId/dashboard/cpu", hostDashboardHandler.CPU)
			r.GET("/hosts/:machineId/dashboard/memory", hostDashboardHandler.Memory)
			r.GET("/hosts/:machineId/dashboard/disk", hostDashboardHandler.Disk)
			r.GET("/hosts/:machineId/dashboard/network", hostDashboardHandler.Network)
			r.GET("/hosts/:machineId/dashboard/processes", hostDashboardHandler.Processes)
			r.GET("/hosts/:machineId/logs", hostDashboardHandler.Logs)
			r.GET("/hosts/:machineId/logs/health-summary", hostDashboardHandler.LogsHealthSummary)

			// Container dashboard routes
			r.GET("/containers/:id/dashboard", containerDashboardHandler.Dashboard)
			r.GET("/containers/:id/logs", containerDashboardHandler.Logs)
		}
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
	hostHandler := NewHostHandler(hostRepo, subscriptionRepo)
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


