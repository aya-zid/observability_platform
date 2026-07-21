package handler

import (
	"net/http"
	"net"
	"os"
	"strings"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type InstallHandler struct {
	cfg *config.Config
}

func NewInstallHandler(cfg *config.Config) *InstallHandler {
	return &InstallHandler{cfg: cfg}
}

func (h *InstallHandler) injectPlatformHost(c echo.Context, path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	script := string(content)

	// Derive host and port from the incoming request.
	// This lets users curl the install script without manually setting PLATFORM_HOST.
	host, port, _ := net.SplitHostPort(c.Request().Host)
	if host == "" {
		host = c.Request().Host
	}
	if port == "" {
		port = "8080"
	}

	script = strings.ReplaceAll(script, "THE_INFRA_IP", host)
	script = strings.ReplaceAll(script, "PLATFORM_PORT=\"${PLATFORM_PORT:-8080}\"", "PLATFORM_PORT=\"${PLATFORM_PORT:-"+port+"}\"")
	return script, nil
}

// AgentInstallScript serves the agent installer script for curl | bash flows.
func (h *InstallHandler) AgentInstallScript(c echo.Context) error {
	path := h.cfg.AgentInstallScriptPath
	if path == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "installer path is not configured"})
	}

	script, err := h.injectPlatformHost(c, path)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "installer script not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/x-shellscript; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.String(http.StatusOK, script)
}

// AgentUninstallScript serves the agent uninstaller script for curl | bash flows.
func (h *InstallHandler) AgentUninstallScript(c echo.Context) error {
	path := h.cfg.AgentUninstallScriptPath
	if path == "" {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "uninstaller path is not configured"})
	}

	script, err := h.injectPlatformHost(c, path)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "uninstaller script not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/x-shellscript; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.String(http.StatusOK, script)
}

