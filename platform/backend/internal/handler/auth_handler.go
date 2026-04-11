// internal/handler/auth_handler.go
package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type AuthHandler struct {
	authService *service.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService *service.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{authService: authService, cfg: cfg}
}

func (h *AuthHandler) Signup(c echo.Context) error {
	var req dto.SignupRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}

	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	accessToken, refreshToken, err := h.authService.Signup(c.Request().Context(), req)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusCreated, dto.OkResponse{OK: true})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req dto.LoginRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}

	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	accessToken, refreshToken, err := h.authService.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AuthHandler) Refresh(c echo.Context) error {
	rt, err := c.Cookie("refresh_token")
	if err != nil || rt == nil || rt.Value == "" {
		return writeAPIError(c, apierror.ErrUnauthorized, "")
	}

	accessToken, refreshToken, err := h.authService.Refresh(c.Request().Context(), rt.Value)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AuthHandler) Logout(c echo.Context) error {
	if rt, err := c.Cookie("refresh_token"); err == nil && rt != nil && rt.Value != "" {
		_ = h.authService.Logout(c.Request().Context(), rt.Value)
	}
	h.clearAuthCookies(c)
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func writeErr(c echo.Context, err error, detail string) error {
	if apiErr, ok := apierror.As(err); ok {
		return writeAPIError(c, apiErr, detail)
	}
	c.Logger().Errorf("[auth] untyped error: %v", err)
	return writeAPIError(c, apierror.ErrInternal, detail)
}

func writeAPIError(c echo.Context, apiErr *apierror.APIError, detail string) error {
	body := map[string]interface{}{
		"error": apiErr.Message, // backward compatible with old string-based clients
		"code":  apiErr.Code,
	}
	if detail != "" {
		body["detail"] = detail
	}
	return c.JSON(apiErr.Status, body)
}

func (h *AuthHandler) setAuthCookies(c echo.Context, accessToken string, refreshToken string) {
	now := time.Now()
	accessExpires := now.Add(time.Duration(h.cfg.AccessTokenTTLMinutes) * time.Minute)
	refreshExpires := now.Add(time.Duration(h.cfg.RefreshTokenTTLDays) * 24 * time.Hour)

	sameSite := http.SameSiteLaxMode
	switch h.cfg.CookieSameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    accessToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  accessExpires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: sameSite,
	})

	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  refreshExpires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: sameSite,
	})
}

func (h *AuthHandler) clearAuthCookies(c echo.Context) {
	sameSite := http.SameSiteLaxMode
	switch h.cfg.CookieSameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	for _, name := range []string{"access_token", "refresh_token"} {
		c.SetCookie(&http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Domain:   h.cfg.CookieDomain,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.cfg.CookieSecure,
			SameSite: sameSite,
		})
	}
}
