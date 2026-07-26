// cmd/server/main.go
package main

import (
	"context"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/handler"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

func main() {
	cfg := config.Load()

	// ── Infrastructure connections ────────────────────────────────────────────
	infrastructure.InitJWT(cfg)
	infrastructure.Connect(cfg)
	infrastructure.ConnectRedis(cfg)

	// ── DB migrations ─────────────────────────────────────────────────────────
	if err := infrastructure.DB.AutoMigrate(
		&domain.Tenant{},
		&domain.User{},
		&domain.Host{},
		&domain.DiscoveredService{},
		&domain.DiscoveredServiceLogPath{},
		&domain.DiscoveredContainerService{},
		&domain.ServiceVulnerability{},
		&domain.NginxVirtualHost{},
		&domain.ApacheVirtualHost{},
		&domain.Invitation{},
		&domain.NotificationChannel{},
		&domain.Plan{},
		&domain.TenantSubscription{},
		&domain.TenantUsageDaily{},
		&domain.AlertRule{},
		&domain.AlertRuleScope{},
		&domain.AlertThreshold{},
		&domain.AlertEvent{},
		&domain.Incident{},
		&domain.IncidentSignal{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("[DB] AutoMigrate completed")

	// ── Seed default plans ────────────────────────────────────────────────────
	seedPlans()

	// ── Start CVE scheduler ─────────────────────────────────────────────────
	cveService := service.NewCVEService(cfg, repository.NewAlertsRepository(infrastructure.DB))
	cveService.StartScheduler(context.Background())

	// ── Start usage metering flush goroutine ─────────────────────────────────
	usageRepo := repository.NewUsageRepository(infrastructure.DB)
	planRepo := repository.NewPlanRepository(infrastructure.DB)
	subscriptionRepo := repository.NewSubscriptionRepository(infrastructure.DB)
	usageMeteringService := service.NewUsageMeteringService(infrastructure.Redis, usageRepo, planRepo, subscriptionRepo)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			usageMeteringService.FlushToPostgres(context.Background())
		}
	}()

	// ── Email sender ──────────────────────────────────────────────────────────
	emailSender := service.NewEmailSender(
		cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass,
		cfg.SMTPFromAddress, cfg.SMTPFromName, cfg.PlatformURL,
	)
	if emailSender.IsEnabled() {
		log.Println("[email] SMTP configured — email delivery enabled")
	} else {
		log.Println("[email] SMTP not configured — email delivery disabled (set SMTP_HOST, SMTP_USER, SMTP_PASS, SMTP_FROM_ADDRESS)")
	}

	// ── Alert engine ─────────────────────────────────────────────────────────
	alertRuleRepo := repository.NewAlertRuleRepository(infrastructure.DB)
	tenantRepo := repository.NewTenantRepository(infrastructure.DB)
	discoveryRepo := repository.NewDiscoveryRepository(infrastructure.DB)
	queryBuilder := service.NewAlertQueryBuilder(cfg)
	alertEngine := service.NewAlertEngineService(alertRuleRepo, queryBuilder, tenantRepo, discoveryRepo)
	go alertEngine.StartEvaluator(context.Background())

	// ── Start notification dispatcher ─────────────────────────────────────────
	notifRepo := repository.NewNotificationChannelRepository(infrastructure.DB)
	notifDispatcher := service.NewNotificationDispatcher(notifRepo, alertRuleRepo, emailSender, cfg.PlatformURL)
	go notifDispatcher.StartNotificationWorker(context.Background())

	// ── Echo setup ────────────────────────────────────────────────────────────
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     cfg.CORSAllowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
	}))

	// cfg is now passed to SetupRoutes so proxy handlers can read infra addresses
	handler.SetupRoutes(e, cfg, emailSender)

	log.Printf("Observability Platform backend starting on :%s", cfg.ServerPort)
	if err := e.Start(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func seedPlans() {
	db := infrastructure.DB
	var count int64
	db.Model(&domain.Plan{}).Count(&count)
	if count > 0 {
		return
	}

	plans := []domain.Plan{
		{Name: "Free", MaxHosts: 2, MaxServicesPerHost: 20, LogRetentionDays: 3, MetricsRetentionDays: 7, MaxLogBytesPerDay: 524288000, PricePerMonth: 0, IsActive: true},
		{Name: "Pro", MaxHosts: 10, MaxServicesPerHost: 20, LogRetentionDays: 14, MetricsRetentionDays: 30, MaxLogBytesPerDay: 5368709120, PricePerMonth: 49, IsActive: true},
		{Name: "Enterprise", MaxHosts: 50, MaxServicesPerHost: 20, LogRetentionDays: 30, MetricsRetentionDays: 90, MaxLogBytesPerDay: 53687091200, PricePerMonth: 199, IsActive: true},
	}
	for _, p := range plans {
		if err := db.Create(&p).Error; err != nil {
			log.Printf("[seed] failed to create plan %s: %v", p.Name, err)
		}
	}
	log.Printf("[seed] created %d default plans", len(plans))
}

func boolStatus(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}
