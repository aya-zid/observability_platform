// internal/handler/admin_subscription_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type AdminSubscriptionHandler struct {
	subService *service.AdminSubscriptionService
}

func NewAdminSubscriptionHandler(subService *service.AdminSubscriptionService) *AdminSubscriptionHandler {
	return &AdminSubscriptionHandler{subService: subService}
}

func (h *AdminSubscriptionHandler) ListExpired(c echo.Context) error {
	subs, err := h.subService.ListExpired(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	type ExpiredSubscriptionItem struct {
		ID               uint    `json:"id"`
		TenantID         uint    `json:"tenant_id"`
		TenantName       string  `json:"tenant_name"`
		TenantEmail      string  `json:"tenant_email"`
		PlanName         string  `json:"plan_name"`
		Status           string  `json:"status"`
		CurrentPeriodEnd *string `json:"current_period_end"`
	}

	result := make([]ExpiredSubscriptionItem, 0, len(subs))
	for _, sub := range subs {
		periodEnd := ""
		if sub.CurrentPeriodEnd != nil {
			periodEnd = sub.CurrentPeriodEnd.Format("2006-01-02")
		}
		result = append(result, ExpiredSubscriptionItem{
			ID:               sub.ID,
			TenantID:         sub.TenantID,
			TenantName:       sub.Tenant.CompanyName,
			TenantEmail:      sub.Tenant.ContactEmail,
			PlanName:         sub.Plan.Name,
			Status:           sub.Status,
			CurrentPeriodEnd: ptrStr(periodEnd),
		})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *AdminSubscriptionHandler) AssignPack(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	var req dto.AdminAssignPackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.subService.AssignPack(c.Request().Context(), uint(tenantID), req.PackID); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *AdminSubscriptionHandler) RenewSubscription(c echo.Context) error {
	tenantID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid tenant id"})
	}

	var req dto.AdminRenewSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if err := h.subService.RenewSubscription(c.Request().Context(), uint(tenantID), req.DurationMonths); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
