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

type BillingHandler struct {
	billingService *service.BillingService
}

func NewBillingHandler(billingService *service.BillingService) *BillingHandler {
	return &BillingHandler{billingService: billingService}
}

func (h *BillingHandler) GetBilling(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	overview, err := h.billingService.GetBillingOverview(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, overview)
}

func (h *BillingHandler) ChangePlan(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req dto.ChangePlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.billingService.ChangePlan(c.Request().Context(), claims.TenantID, req.PlanID); err != nil {
		if apiErr, ok := apierror.As(err); ok {
			return c.JSON(apiErr.Status, map[string]string{"error": apiErr.Message})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *BillingHandler) GetUsageHistory(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	daysStr := c.QueryParam("days")
	days := 7
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	history, err := h.billingService.GetUsageHistory(c.Request().Context(), claims.TenantID, days)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, history)
}

func (h *BillingHandler) ListPlans(c echo.Context) error {
	plans, err := h.billingService.ListActivePlans(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, plans)
}
