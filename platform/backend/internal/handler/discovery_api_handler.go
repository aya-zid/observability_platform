package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type DiscoveryAPIHandler struct {
	discoveryService *service.DiscoveryService
}

func NewDiscoveryAPIHandler(discoveryService *service.DiscoveryService) *DiscoveryAPIHandler {
	return &DiscoveryAPIHandler{discoveryService: discoveryService}
}

func (h *DiscoveryAPIHandler) claims(c echo.Context) (*infrastructure.Claims, bool) {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}

func (h *DiscoveryAPIHandler) ListCatalog(c echo.Context) error {
	return c.JSON(http.StatusOK, h.discoveryService.ListCatalog())
}

func (h *DiscoveryAPIHandler) ListHosts(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	hosts, err := h.discoveryService.ListHosts(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load hosts"})
	}
	return c.JSON(http.StatusOK, hosts)
}

func (h *DiscoveryAPIHandler) ListServices(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	services, err := h.discoveryService.ListServices(c.Request().Context(), claims.TenantID, c.QueryParam("machine_id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load services"})
	}
	return c.JSON(http.StatusOK, services)
}

func (h *DiscoveryAPIHandler) GetService(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	resp, err := h.discoveryService.GetServiceDetail(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load service"})
	}
	if resp == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *DiscoveryAPIHandler) UpdateCollection(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	var req dto.UpdateCollectionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.discoveryService.UpdateCollection(c.Request().Context(), claims.TenantID, uint(serviceID), req); err != nil {
		if service.IsNotFoundError(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update collection"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *DiscoveryAPIHandler) UpdateServiceConfig(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	var req dto.UpdateServiceConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.discoveryService.UpdateServiceConfig(c.Request().Context(), claims.TenantID, uint(serviceID), req); err != nil {
		if errors.Is(err, service.ErrMetricsEndpointRequired) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if service.IsNotFoundError(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update service config"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
