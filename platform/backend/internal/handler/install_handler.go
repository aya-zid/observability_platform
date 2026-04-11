package handler

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type InstallHandler struct {
	cfg *config.Config
}

func NewInstallHandler(cfg *config.Config) *InstallHandler {
	return &InstallHandler{cfg: cfg}
}

// AgentInstallScript serves the agent installer script for curl | bash flows.
func (h *InstallHandler) AgentInstallScript(c echo.Context) error {
	path := h.cfg.AgentInstallScriptPath
	if path == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "installer path is not configured"})
	}
	if _, err := os.Stat(path); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "installer script not found"})
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/x-shellscript; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.File(path)
}

