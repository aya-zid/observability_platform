// internal/handler/admin_pack_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/apierror"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type AdminPackHandler struct {
	packService *service.AdminPackService
}

func NewAdminPackHandler(packService *service.AdminPackService) *AdminPackHandler {
	return &AdminPackHandler{packService: packService}
}

func (h *AdminPackHandler) ListPacks(c echo.Context) error {
	packs, err := h.packService.ListPacks(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, packs)
}

func (h *AdminPackHandler) GetPack(c echo.Context) error {
	packID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid pack id"})
	}

	pack, err := h.packService.GetPack(c.Request().Context(), uint(packID))
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, pack)
}

func (h *AdminPackHandler) CreatePack(c echo.Context) error {
	var req dto.AdminCreatePackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	pack, err := h.packService.CreatePack(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, pack)
}

func (h *AdminPackHandler) UpdatePack(c echo.Context) error {
	packID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid pack id"})
	}

	var req dto.AdminUpdatePackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	pack, err := h.packService.UpdatePack(c.Request().Context(), uint(packID), req)
	if err != nil {
		if err.Error() == "pack not found" {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, pack)
}

func (h *AdminPackHandler) DeletePack(c echo.Context) error {
	packID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid pack id"})
	}

	if err := h.packService.DeletePack(c.Request().Context(), uint(packID)); err != nil {
		if apiErr, ok := apierror.As(err); ok {
			return c.JSON(apiErr.Status, map[string]string{"error": apiErr.Message})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}
