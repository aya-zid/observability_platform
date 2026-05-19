// cmd/server/main.go
package main

import (
	"context"
	"log"

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
	infrastructure.ConnectRedis(cfg) // ← new

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
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("[DB] AutoMigrate completed")

	cveService := service.NewCVEService(cfg, repository.NewAlertsRepository(infrastructure.DB))
	cveService.StartScheduler(context.Background())

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
	handler.SetupRoutes(e, cfg)

	log.Printf("🚀 Observability Platform backend starting on :%s", cfg.ServerPort)
	if err := e.Start(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
