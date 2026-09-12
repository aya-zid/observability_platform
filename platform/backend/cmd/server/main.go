// cmd/server/main.go
package main

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/gorm"

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
		&domain.PasswordResetToken{},
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
		&domain.DismissedIncident{},
		&domain.Report{},
		&domain.Admin{},
		&domain.SubscriptionRequest{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("[DB] AutoMigrate completed")

	// This limit was retired in favor of retention and host limits. AutoMigrate
	// never drops columns, so remove it explicitly from databases created by
	// older versions as well.
	if infrastructure.DB.Migrator().HasColumn(&domain.Plan{}, "max_log_bytes_per_day") {
		if err := infrastructure.DB.Migrator().DropColumn(&domain.Plan{}, "max_log_bytes_per_day"); err != nil {
			log.Fatalf("[DB] failed to remove retired plan log-byte limit: %v", err)
		}
	}
	// Older releases stored a platform ban on the subscription. A platform ban
	// belongs to the tenant; repair those legacy rows so unbanning does not
	// accidentally leave the subscription suspended.
	if infrastructure.DB.Migrator().HasColumn(&domain.TenantSubscription{}, "banned_at") {
		if err := infrastructure.DB.Exec("UPDATE tenant_subscriptions SET status = 'active', banned_at = NULL, ban_reason = '', banned_by_admin_id = NULL WHERE banned_at IS NOT NULL AND status = 'suspended'").Error; err != nil {
			log.Fatalf("[DB] failed to repair legacy subscription bans: %v", err)
		}
	}

	// Backfill: make any pre-existing users active. GORM adds the new column
	// with a DEFAULT of 'active', but we also guard against NULL/empty rows
	// that may exist if a previous deployment added the column differently.
	if err := infrastructure.DB.Exec(`UPDATE users SET status = 'active' WHERE status IS NULL OR status = ''`).Error; err != nil {
		log.Fatalf("[DB] failed to backfill user status: %v", err)
	} else {
		log.Println("[DB] user status backfill completed (all existing users active)")
	}

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

	// ── Start subscription expiration checker ─────────────────────────────────
	subscriptionWorker := service.NewSubscriptionWorker(infrastructure.DB, subscriptionRepo)
	subscriptionWorker.StartExpirationChecker(context.Background(), 1*time.Hour)

	// Retention cleanup is deliberately disabled. The prior implementation used
	// Loki's delete API and could remove current logs, which is not acceptable
	// for an observability platform. Storage retention is controlled only by the
	// infrastructure configuration.

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
	plans := []domain.Plan{
		{Name: "Free", MaxHosts: 2, MaxServicesPerHost: 20, LogRetentionDays: 3, MetricsRetentionDays: 7, PricePerMonth: 0, IsActive: true, DurationMonths: 1},
		{Name: "Basic", MaxHosts: 5, MaxServicesPerHost: 20, LogRetentionDays: 7, MetricsRetentionDays: 14, PricePerMonth: 19, IsActive: true, DurationMonths: 1},
		{Name: "Pro", MaxHosts: 10, MaxServicesPerHost: 20, LogRetentionDays: 14, MetricsRetentionDays: 30, PricePerMonth: 49, IsActive: true, DurationMonths: 1},
		{Name: "Premium", MaxHosts: 25, MaxServicesPerHost: 20, LogRetentionDays: 30, MetricsRetentionDays: 60, PricePerMonth: 99, IsActive: true, DurationMonths: 1},
		{Name: "Enterprise", MaxHosts: 50, MaxServicesPerHost: 20, LogRetentionDays: 30, MetricsRetentionDays: 90, PricePerMonth: 199, IsActive: true, DurationMonths: 1},
		{Name: "Trial", MaxHosts: 25, MaxServicesPerHost: 20, LogRetentionDays: 30, MetricsRetentionDays: 60, PricePerMonth: 0, IsActive: true, DurationMonths: 1, IsTrial: true},
	}

	for _, p := range plans {
		var existing domain.Plan
		err := db.Where("name = ?", p.Name).First(&existing).Error
		switch {
		case err == nil:
			// Signup requires the system Trial plan to carry this flag. Older
			// databases may have the named plan without it.
			if p.IsTrial && !existing.IsTrial {
				if err := db.Model(&existing).Update("is_trial", true).Error; err != nil {
					log.Printf("[seed] failed to repair trial plan: %v", err)
				} else {
					log.Println("[seed] repaired trial plan")
				}
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := db.Create(&p).Error; err != nil {
				log.Printf("[seed] failed to create plan %s: %v", p.Name, err)
			}
		default:
			log.Printf("[seed] failed to look up plan %s: %v", p.Name, err)
		}
	}
}

func boolStatus(ok bool) string {
	if ok {
		return "connected"
	}
	return "disconnected"
}
