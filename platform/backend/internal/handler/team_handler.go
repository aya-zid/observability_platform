package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type TeamHandler struct {
	teamService *service.TeamService
}

func NewTeamHandler(teamService *service.TeamService) *TeamHandler {
	return &TeamHandler{teamService: teamService}
}

func (h *TeamHandler) ListTeam(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	resp, err := h.teamService.ListTeam(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *TeamHandler) InviteTeammate(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req dto.InviteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	inv, err := h.teamService.InviteTeammate(c.Request().Context(), claims.TenantID, claims.UserID, req.Email, req.Role)
	if err != nil {
		if apiErr, ok := apierror.As(err); ok {
			return c.JSON(apiErr.Status, map[string]string{"error": apiErr.Message})
		}
		return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
	}

	inviteURL := c.Scheme() + "://" + c.Request().Host + "/invite/" + inv.Token
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"invitation_id": inv.ID,
		"invite_url":    inviteURL,
	})
}

func (h *TeamHandler) AcceptInvitation(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing token"})
	}

	var req dto.AcceptInviteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.teamService.AcceptInvitation(c.Request().Context(), token, req.FullName, req.Password); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *TeamHandler) ValidateInvitation(c echo.Context) error {
	token := c.Param("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing token"})
	}

	inv, err := h.teamService.ValidateInvitation(c.Request().Context(), token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":  true,
		"email":  inv.Email,
		"tenant": inv.Tenant.CompanyName,
	})
}

func (h *TeamHandler) RemoveMember(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	userIDStr := c.Param("userId")
	targetID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	if err := h.teamService.RemoveMember(c.Request().Context(), claims.TenantID, uint(targetID), claims.UserID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *TeamHandler) ChangeRole(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	userIDStr := c.Param("userId")
	targetID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	var req dto.ChangeRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.teamService.ChangeRole(c.Request().Context(), claims.TenantID, uint(targetID), req.Role); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *TeamHandler) ResendInvitation(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	invID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid invitation id"})
	}

	inv, err := h.teamService.ResendInvitation(c.Request().Context(), uint(invID), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	inviteURL := c.Scheme() + "://" + c.Request().Host + "/invite/" + inv.Token
	return c.JSON(http.StatusOK, map[string]interface{}{
		"invite_url": inviteURL,
		"expires_at": inv.ExpiresAt,
	})
}

func (h *TeamHandler) RevokeInvitation(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	invID, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid invitation id"})
	}

	if err := h.teamService.RevokeInvitation(c.Request().Context(), uint(invID), claims.TenantID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
