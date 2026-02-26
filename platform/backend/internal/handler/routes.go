// internal/handler/routes.go
package handler

import (
	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

func SetupRoutes(e *echo.Echo) {
	// Create services once
	authService := service.NewAuthService(
		repository.NewTenantRepository(infrastructure.DB),
		repository.NewUserRepository(infrastructure.DB),
	)

	authHandler := NewAuthHandler(authService)

	// Public routes
	e.POST("/signup", authHandler.Signup)
	e.POST("/login", authHandler.Login)

	// Health check
	e.GET("/health", func(c echo.Context) error {
		dbStatus := "connected"
		if !infrastructure.Ping() {
			dbStatus = "disconnected"
		}

		return c.JSON(200, map[string]string{
			"status":    "healthy",
			"db_status": dbStatus,
		})
	})
}