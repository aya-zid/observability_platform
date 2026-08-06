// internal/handler/report_handler.go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type ReportHandler struct {
	svc        *service.ReportService
	tenantRepo *repository.TenantRepository
}

func NewReportHandler(svc *service.ReportService, tenantRepo *repository.TenantRepository) *ReportHandler {
	return &ReportHandler{svc: svc, tenantRepo: tenantRepo}
}

type generateReportRequest struct {
	Type  string `json:"type"`
	DateA string `json:"date_a"`
	DateB string `json:"date_b"`
}

type reportResponse struct {
	ID          uint            `json:"id"`
	Type        string          `json:"type"`
	DateA       string          `json:"date_a"`
	DateB       string          `json:"date_b"`
	Status      string          `json:"status"`
	Error       string          `json:"error,omitempty"`
	GeneratedBy uint            `json:"generated_by"`
	Numbers     json.RawMessage `json:"numbers,omitempty"`
	AiReport    json.RawMessage `json:"ai_report,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at,omitempty"`
}

func toReportResponse(r *domain.Report) reportResponse {
	resp := reportResponse{
		ID:          r.ID,
		Type:        r.Type,
		DateA:       r.DateA,
		DateB:       r.DateB,
		Status:      r.Status,
		Error:       r.Error,
		GeneratedBy: r.GeneratedBy,
		CreatedAt:   r.CreatedAt,
		FinishedAt:  r.FinishedAt,
	}
	if len(r.Numbers) > 0 {
		resp.Numbers = r.Numbers
	}
	if len(r.AiReport) > 0 {
		resp.AiReport = r.AiReport
	}
	return resp
}

// tenantUUIDFor resolves the tenant UUID used as the Loki tenant header. It is
// needed for any operation that triggers telemetry gathering.
func (h *ReportHandler) tenantUUIDFor(c echo.Context) (string, error) {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return "", errors.New("unauthenticated")
	}
	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return "", errors.New("tenant not found")
	}
	return tenant.UUID, nil
}

// parseReportDate validates a YYYY-MM-DD string and returns the UTC midnight.
func parseReportDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// normalizeReportDates applies the generate defaults/validation: for "today"
// date_a defaults to today (UTC) and date_b is dropped; for "compare" date_a is
// required, date_b defaults to the day before date_a, and both must be valid
// distinct YYYY-MM-DD dates.
func normalizeReportDates(typ, dateA, dateB string, now time.Time) (a, b string, err error) {
	switch typ {
	case "today":
		if dateA == "" {
			dateA = now.UTC().Format("2006-01-02")
		}
		if _, err := parseReportDate(dateA); err != nil {
			return "", "", errors.New("date_a must be YYYY-MM-DD")
		}
		return dateA, "", nil
	case "compare":
		if dateA == "" {
			return "", "", errors.New("compare reports require date_a")
		}
		dayA, err := parseReportDate(dateA)
		if err != nil {
			return "", "", errors.New("date_a must be YYYY-MM-DD")
		}
		if dateB == "" {
			dateB = dayA.AddDate(0, 0, -1).Format("2006-01-02")
		} else if _, err := parseReportDate(dateB); err != nil {
			return "", "", errors.New("date_b must be YYYY-MM-DD")
		}
		if dateA == dateB {
			return "", "", errors.New("date_a and date_b must differ")
		}
		return dateA, dateB, nil
	default:
		return "", "", errors.New("type must be 'today' or 'compare'")
	}
}

// Generate handles POST /api/reports/generate.
func (h *ReportHandler) Generate(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req generateReportRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}
	if req.Type != "today" && req.Type != "compare" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be 'today' or 'compare'"})
	}

	dateA, dateB, err := normalizeReportDates(req.Type, req.DateA, req.DateB, time.Now())
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	tenantUUID, err := h.tenantUUIDFor(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	report, created, err := h.svc.Generate(c.Request().Context(), claims.TenantID, claims.UserID, tenantUUID, req.Type, dateA, dateB, false)
	if err != nil {
		c.Logger().Errorf("[reports] generate: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create report"})
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	return c.JSON(status, toReportResponse(report))
}

// List handles GET /api/reports.
func (h *ReportHandler) List(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	typ := c.QueryParam("type")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	reports, err := h.svc.ListByTenant(c.Request().Context(), claims.TenantID, typ, limit, offset)
	if err != nil {
		c.Logger().Errorf("[reports] list: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list reports"})
	}
	out := make([]reportResponse, 0, len(reports))
	for i := range reports {
		out = append(out, toReportResponse(&reports[i]))
	}
	return c.JSON(http.StatusOK, out)
}

// Get handles GET /api/reports/:id.
func (h *ReportHandler) Get(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid report id"})
	}

	report, err := h.svc.GetByIDTenant(c.Request().Context(), uint(id), claims.TenantID)
	if err != nil {
		if errors.Is(err, service.ErrReportNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "report not found"})
		}
		c.Logger().Errorf("[reports] get: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch report"})
	}
	return c.JSON(http.StatusOK, toReportResponse(report))
}

// Regenerate handles POST /api/reports/:id/regenerate.
func (h *ReportHandler) Regenerate(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid report id"})
	}

	tenantUUID, err := h.tenantUUIDFor(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
	}

	report, err := h.svc.Regenerate(c.Request().Context(), uint(id), claims.TenantID, tenantUUID)
	if err != nil {
		if errors.Is(err, service.ErrReportNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "report not found"})
		}
		c.Logger().Errorf("[reports] regenerate: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to regenerate report"})
	}
	return c.JSON(http.StatusOK, toReportResponse(report))
}

// Delete handles DELETE /api/reports/:id.
func (h *ReportHandler) Delete(c echo.Context) error {
	claims, ok := c.Get("claims").(*infrastructure.Claims)
	if !ok || claims == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid report id"})
	}

	if err := h.svc.Delete(c.Request().Context(), uint(id), claims.TenantID); err != nil {
		c.Logger().Errorf("[reports] delete: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete report"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
