// internal/handler/admin_auth_handler.go
package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type AdminAuthHandler struct {
	adminAuthService *service.AdminAuthService
	cfg              *config.Config
}

func NewAdminAuthHandler(adminAuthService *service.AdminAuthService, cfg *config.Config) *AdminAuthHandler {
	return &AdminAuthHandler{adminAuthService: adminAuthService, cfg: cfg}
}

func (h *AdminAuthHandler) Signup(c echo.Context) error {
	var req dto.AdminSignupRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	accessToken, refreshToken, err := h.adminAuthService.Signup(c.Request().Context(), req)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAdminAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusCreated, dto.AdminAuthResponse{OK: true, Role: domain.AdminRoleSuperAdmin})
}

func (h *AdminAuthHandler) Login(c echo.Context) error {
	var req dto.AdminLoginRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	accessToken, refreshToken, err := h.adminAuthService.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAdminAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusOK, dto.AdminAuthResponse{OK: true, Role: domain.AdminRoleSuperAdmin})
}

func (h *AdminAuthHandler) Refresh(c echo.Context) error {
	rt, err := c.Cookie("admin_refresh_token")
	if err != nil || rt == nil || rt.Value == "" {
		return writeAPIError(c, apierror.ErrUnauthorized, "")
	}

	accessToken, refreshToken, err := h.adminAuthService.Refresh(c.Request().Context(), rt.Value)
	if err != nil {
		return writeErr(c, err, "")
	}

	h.setAdminAuthCookies(c, accessToken, refreshToken)
	return c.JSON(http.StatusOK, dto.AdminAuthResponse{OK: true, Role: domain.AdminRoleSuperAdmin})
}

func (h *AdminAuthHandler) Logout(c echo.Context) error {
	if rt, err := c.Cookie("admin_refresh_token"); err == nil && rt != nil && rt.Value != "" {
		_ = h.adminAuthService.Logout(c.Request().Context(), rt.Value)
	}
	h.clearAdminAuthCookies(c)
	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminAuthHandler) GetMe(c echo.Context) error {
	adminClaims := c.Get("adminClaims").(*infrastructure.AdminClaims)
	admin, err := h.adminAuthService.GetProfile(c.Request().Context(), adminClaims.AdminID)
	if err != nil || admin == nil {
		return writeAPIError(c, apierror.ErrUnauthorized, "")
	}

	return c.JSON(http.StatusOK, dto.AdminProfileResponse{
		AdminID:  admin.ID,
		Email:    admin.Email,
		FullName: admin.FullName,
		Role:     adminClaims.Role,
	})
}

func (h *AdminAuthHandler) ChangePassword(c echo.Context) error {
	adminClaims := c.Get("adminClaims").(*infrastructure.AdminClaims)

	var req dto.AdminChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return writeAPIError(c, apierror.ErrBadRequest, "")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return writeAPIError(c, apierror.ErrValidationFailed, err.Error())
	}

	if err := h.adminAuthService.ChangePassword(c.Request().Context(), adminClaims.AdminID, req.CurrentPassword, req.NewPassword); err != nil {
		if apiErr, ok := apierror.As(err); ok {
			return writeAPIError(c, apiErr, "")
		}
		return writeErr(c, err, "")
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminAuthHandler) setAdminAuthCookies(c echo.Context, accessToken string, refreshToken string) {
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
		Name:     "admin_access_token",
		Value:    accessToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  accessExpires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: sameSite,
	})

	c.SetCookie(&http.Cookie{
		Name:     "admin_refresh_token",
		Value:    refreshToken,
		Path:     "/",
		Domain:   h.cfg.CookieDomain,
		Expires:  refreshExpires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: sameSite,
	})
}

func (h *AdminAuthHandler) clearAdminAuthCookies(c echo.Context) {
	sameSite := http.SameSiteLaxMode
	switch h.cfg.CookieSameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	for _, name := range []string{"admin_access_token", "admin_refresh_token"} {
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
