// internal/handler/password_reset_handler.go
package handler

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type PasswordResetHandler struct {
	service *service.PasswordResetService
}

func NewPasswordResetHandler(service *service.PasswordResetService) *PasswordResetHandler {
	return &PasswordResetHandler{service: service}
}

// ForgotPassword always answers 200 OK so it can't be used to enumerate accounts.
func (h *PasswordResetHandler) ForgotPassword(c echo.Context) error {
	var req dto.ForgotPasswordRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	if err := h.service.RequestPasswordReset(c.Request().Context(), req.Email); err != nil {
		c.Logger().Errorf("[password_reset] forgot-password: %v", err)
		return writeAPIError(c, apierror.ErrInternal, "")
	}
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *PasswordResetHandler) ValidateResetToken(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}

	user, err := h.service.ValidateResetToken(c.Request().Context(), token)
	if err != nil {
		return writeErr(c, err, "")
	}
	if user == nil {
		return writeAPIError(c, apierror.ErrInvalidResetToken, "")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
		"email": user.Email,
	})
}

func (h *PasswordResetHandler) ResetPassword(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}

	var req dto.ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	if err := h.service.ResetPassword(c.Request().Context(), token, req.Password); err != nil {
		return writeErr(c, err, "")
	}
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}