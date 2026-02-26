package main

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/handler"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
)

func main() {
	cfg := config.Load()

	infrastructure.InitJWT(cfg)
	infrastructure.Connect(cfg)

	// Auto-migrate
	if err := infrastructure.DB.AutoMigrate(&domain.Tenant{}, &domain.User{}); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	log.Println("[DB] AutoMigrate completed – tables created/updated")

	// Echo setup
	e := echo.New()
	e.HideBanner = true

	// Global middlewares
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Setup all routes
	handler.SetupRoutes(e)

	log.Println("🚀 Starting Observability SaaS Backend on :8080")
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
