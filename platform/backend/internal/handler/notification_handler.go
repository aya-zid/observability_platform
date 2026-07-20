package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type NotificationHandler struct {
	notifRepo *repository.NotificationChannelRepository
}

func NewNotificationHandler(notifRepo *repository.NotificationChannelRepository) *NotificationHandler {
	return &NotificationHandler{notifRepo: notifRepo}
}

func (h *NotificationHandler) ListChannels(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	channels, err := h.notifRepo.ListByTenantCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list channels"})
	}
	return c.JSON(http.StatusOK, channels)
}

type addChannelRequest struct {
	Type   string `json:"type" validate:"required"`
	Target string `json:"target" validate:"required,email"`
}

func (h *NotificationHandler) AddChannel(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req addChannelRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	channel := &domain.NotificationChannel{
		TenantID:   claims.TenantID,
		Type:       req.Type,
		Target:     req.Target,
		IsVerified: true,
	}

	if err := h.notifRepo.Create(channel); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create channel"})
	}

	return c.JSON(http.StatusCreated, channel)
}

func (h *NotificationHandler) DeleteChannel(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	if err := h.notifRepo.DeleteByIDAndTenantCtx(c.Request().Context(), uint(id), claims.TenantID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete channel"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
