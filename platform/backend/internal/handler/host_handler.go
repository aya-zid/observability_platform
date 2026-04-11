package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type HostHandler struct {
	hostRepo *repository.HostRepository
}

func NewHostHandler(hostRepo *repository.HostRepository) *HostHandler {
	return &HostHandler{hostRepo: hostRepo}
}

func (h *HostHandler) Register(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.HostRegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.hostRepo.UpsertByTenantMachineCtx(
		c.Request().Context(),
		tenant.ID,
		req.MachineID,
		req.Hostname,
		req.OSName,
		req.OSVersion,
		req.Arch,
		req.AgentVersion,
	); err != nil {
		c.Logger().Errorf("[hosts] register failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to register host"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *HostHandler) Heartbeat(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.HostHeartbeatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.hostRepo.TouchHeartbeatByTenantMachineCtx(
		c.Request().Context(),
		tenant.ID,
		req.MachineID,
		req.Hostname,
		req.AgentVersion,
	); err != nil {
		c.Logger().Errorf("[hosts] heartbeat failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update heartbeat"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

