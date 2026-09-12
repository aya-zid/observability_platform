// internal/handler/admin_subscription_handler.go
package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
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

func (h *AdminSubscriptionHandler) ListAll(c echo.Context) error {
	status := c.QueryParam("status")
	subs, err := h.subService.ListAllSubscriptions(c.Request().Context(), status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := make([]dto.AdminSubscriptionListItem, 0, len(subs))
	for _, sub := range subs {
		periodEnd := ""
		if sub.CurrentPeriodEnd != nil {
			periodEnd = sub.CurrentPeriodEnd.Format("2006-01-02")
		}
		result = append(result, dto.AdminSubscriptionListItem{
			ID:               sub.ID,
			TenantID:         sub.TenantID,
			TenantName:       sub.Tenant.CompanyName,
			TenantEmail:      sub.Tenant.ContactEmail,
			PlanName:         sub.Plan.Name,
			PlanPrice:        sub.Plan.PricePerMonth,
			Status:           sub.Status,
			StartedAt:        sub.StartedAt.Format("2006-01-02"),
			CurrentPeriodEnd: ptrStr(periodEnd),
			Notes:            sub.Notes,
		})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *AdminSubscriptionHandler) ListRequests(c echo.Context) error {
	status := c.QueryParam("status")
	reqs, err := h.subService.ListSubscriptionRequests(c.Request().Context(), status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	result := make([]dto.AdminSubscriptionRequestItem, 0, len(reqs))
	for _, r := range reqs {
		var reviewedAt *string
		if r.ReviewedAt != nil {
			formatted := r.ReviewedAt.Format("2006-01-02 15:04")
			reviewedAt = &formatted
		}

		result = append(result, dto.AdminSubscriptionRequestItem{
			ID:          r.ID,
			TenantID:    r.TenantID,
			TenantName:  r.Tenant.CompanyName,
			TenantEmail: r.Tenant.ContactEmail,
			PlanID:      r.PlanID,
			PlanName:    r.Plan.Name,
			PlanPrice:   r.Plan.PricePerMonth,
			Status:      r.Status,
			Notes:       r.Notes,
			AdminNotes:  r.AdminNotes,
			ReviewedAt:  reviewedAt,
			CreatedAt:   r.CreatedAt.Format("2006-01-02 15:04"),
		})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *AdminSubscriptionHandler) ReviewRequest(c echo.Context) error {
	adminClaims, ok := c.Get("adminClaims").(*infrastructure.AdminClaims)
	adminID := uint(0)
	if ok && adminClaims != nil {
		adminID = adminClaims.AdminID
	}

	reqID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request id"})
	}

	var req dto.AdminReviewSubscriptionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request payload"})
	}

	if err := h.subService.ReviewSubscriptionRequest(c.Request().Context(), uint(reqID), req.Status, req.AdminNotes, adminID); err != nil {
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
