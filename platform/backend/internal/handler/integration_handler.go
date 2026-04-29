package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type IntegrationHandler struct {
	integrationService *service.IntegrationService
}

func NewIntegrationHandler(integrationService *service.IntegrationService) *IntegrationHandler {
	return &IntegrationHandler{integrationService: integrationService}
}

func (h *IntegrationHandler) CreateInstallCommand(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	resp, err := h.integrationService.CreateInstallCommand(
		c.Request().Context(),
		claims.TenantID,
		uint(serviceID),
		c.Request().Host,
	)
	if err != nil {
		if errors.Is(err, service.ErrIntegrationConfigRequired) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if errors.Is(err, service.ErrTenantNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create integration install command"})
	}
	if resp == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *IntegrationHandler) NginxScript(c echo.Context) error {
	return h.GenericScript(c)
}

func (h *IntegrationHandler) GenericScript(c echo.Context) error {
	script, err := h.integrationService.RenderGenericScript(service.IntegrationScriptParams{
		ServiceID:       c.QueryParam("service_id"),
		MachineID:       c.QueryParam("machine_id"),
		ServiceName:     c.QueryParam("service_name"),
		APIKey:          c.QueryParam("api_key"),
		LogPaths:        c.QueryParam("log_paths"),
		MetricsEndpoint: c.QueryParam("metrics_endpoint"),
		PlatformHost:    c.QueryParam("platform_host"),
		PlatformPort:    c.QueryParam("platform_port"),
		RequestHost:     c.Request().Host,
	})
	if err != nil {
		if errors.Is(err, service.ErrMissingScriptParams) {
			return c.String(http.StatusBadRequest, err.Error())
		}
		return c.String(http.StatusInternalServerError, "failed to render integration script")
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/x-shellscript; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-store")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	return c.String(http.StatusOK, script)
}

func (h *IntegrationHandler) IngestStatus(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.IntegrationStatusIngestRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.integrationService.IngestStatus(c.Request().Context(), tenant.ID, req); err != nil {
		if service.IsNotFoundError(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update integration status"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
