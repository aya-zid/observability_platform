package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(profileService *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: profileService}
}

func (h *ProfileHandler) GetProfile(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":   claims.UserID,
		"tenant_id": claims.TenantID,
		"email":     claims.Email,
		"role":      claims.Role,
	})
}

func (h *ProfileHandler) UpdateProfile(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req dto.UpdateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.profileService.UpdateProfile(c.Request().Context(), claims.UserID, req.FullName); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update profile"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *ProfileHandler) ChangePassword(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req dto.ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.profileService.ChangePassword(c.Request().Context(), claims.UserID, req.CurrentPassword, req.NewPassword); err != nil {
		if apiErr, ok := apierror.As(err); ok {
			return c.JSON(apiErr.Status, map[string]string{"error": apiErr.Message})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
