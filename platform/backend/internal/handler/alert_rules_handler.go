package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type AlertRulesHandler struct {
	alertRuleRepo *repository.AlertRuleRepository
}

func NewAlertRulesHandler(alertRuleRepo *repository.AlertRuleRepository) *AlertRulesHandler {
	return &AlertRulesHandler{alertRuleRepo: alertRuleRepo}
}

// templateDTO is returned from the code-defined templates
type templateDTO struct {
	ID                 string  `json:"id"`
	ServiceType        string  `json:"service_type"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	Explanation        string  `json:"explanation"`
	Unit               string  `json:"unit"`
	DefaultThreshold   float64 `json:"default_threshold"`
	ComparisonOp       string  `json:"comparison_op"`
	DefaultSeverity    string  `json:"default_severity"`
	DefaultForDuration string  `json:"default_for_duration"`
}

func toTemplateDTO(t domain.AlertTemplate) templateDTO {
	return templateDTO{
		ID:                 t.ID,
		ServiceType:        t.ServiceType,
		Name:               t.Name,
		Description:        t.Description,
		Explanation:        t.Explanation,
		Unit:               t.Unit,
		DefaultThreshold:   t.DefaultThreshold,
		ComparisonOp:       t.ComparisonOp,
		DefaultSeverity:    t.DefaultSeverity,
		DefaultForDuration: t.DefaultForDuration,
	}
}

func (h *AlertRulesHandler) ListTemplates(c echo.Context) error {
	result := make([]templateDTO, 0, len(domain.AlertTemplates))
	for _, t := range domain.AlertTemplates {
		result = append(result, toTemplateDTO(t))
	}
	return c.JSON(http.StatusOK, result)
}

func (h *AlertRulesHandler) ListTemplatesByService(c echo.Context) error {
	serviceType := c.Param("serviceType")
	templates := domain.TemplatesByServiceType(serviceType)
	result := make([]templateDTO, 0, len(templates))
	for _, t := range templates {
		result = append(result, toTemplateDTO(t))
	}
	return c.JSON(http.StatusOK, result)
}

type scopeRequest struct {
	ScopeType string `json:"scope_type" validate:"required"`
	ScopeID   *uint  `json:"scope_id"`
}

type thresholdRequest struct {
	Label       string  `json:"label"`
	Threshold   float64 `json:"threshold" validate:"required"`
	Severity    string  `json:"severity"`
	ForDuration string  `json:"for_duration"`
}

type createRuleRequest struct {
	TemplateID string `json:"template_id" validate:"required"`

	// Scope can be:
	// - {scope_type: "all_hosts"}
	// - {scope_type: "host", scope_id: <id>}
	// - {scope_type: "all_services_of_type"}
	// - {scope_type: "service", scope_id: <id>}
	Scopes     []scopeRequest `json:"scopes" validate:"required,min=1"`
	Thresholds []thresholdRequest `json:"thresholds" validate:"required,min=1"`

	ForDuration string `json:"for_duration"`
	Domain      string `json:"domain"`
	Pool        string `json:"pool"`
}

func (h *AlertRulesHandler) CreateRule(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)

	var req createRuleRequest
	if err := c.Bind(&req); err != nil {
		log.Printf("[alerts] CreateRule bind error: %v", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request: " + err.Error()})
	}

	tpl := domain.FindTemplateByID(req.TemplateID)
	if tpl == nil {
		log.Printf("[alerts] CreateRule unknown template: %q", req.TemplateID)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown template_id"})
	}

	scopes := make([]domain.AlertRuleScope, 0, len(req.Scopes))
	for _, s := range req.Scopes {
		scopes = append(scopes, domain.AlertRuleScope{
			ScopeType: s.ScopeType,
			ScopeID:   s.ScopeID,
		})
	}

	thresholds := make([]domain.AlertThreshold, 0, len(req.Thresholds))
	for _, t := range req.Thresholds {
		sev := t.Severity
		if sev == "" {
			sev = tpl.DefaultSeverity
		}
		dur := t.ForDuration
		if dur == "" {
			dur = req.ForDuration
		}
		if dur == "" {
			dur = tpl.DefaultForDuration
		}
		thresholds = append(thresholds, domain.AlertThreshold{
			Label:       t.Label,
			Threshold:   t.Threshold,
			Severity:    sev,
			ForDuration: dur,
		})
	}

	rule := &domain.AlertRule{
		TenantID:        claims.TenantID,
		TemplateID:      tpl.ID,
		ServiceType:     tpl.ServiceType,
		TemplateName:    tpl.Name,
		Description:     tpl.Description,
		Explanation:     tpl.Explanation,
		Unit:            tpl.Unit,
		ComparisonOp:    tpl.ComparisonOp,
		DefaultSeverity: tpl.DefaultSeverity,
		ForDuration:     req.ForDuration,
		IsEnabled:       true,
		Domain:          req.Domain,
		Pool:            req.Pool,
		Scopes:          scopes,
		Thresholds:      thresholds,
	}
	if rule.ForDuration == "" {
		rule.ForDuration = tpl.DefaultForDuration
	}

	if err := h.alertRuleRepo.CreateRuleCtx(c.Request().Context(), rule); err != nil {
		log.Printf("[alerts] CreateRule db error: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create rule: " + err.Error()})
	}

	// Reload with preloaded associations
	created, _ := h.alertRuleRepo.FindByIDCtx(c.Request().Context(), rule.ID)
	if created != nil {
		rule = created
	}
	return c.JSON(http.StatusCreated, rule)
}

func (h *AlertRulesHandler) ListRules(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	rules, err := h.alertRuleRepo.ListRulesByTenantCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list rules"})
	}
	return c.JSON(http.StatusOK, rules)
}

type updateRuleRequest struct {
	Scopes      *[]scopeRequest      `json:"scopes"`
	Thresholds  *[]thresholdRequest  `json:"thresholds"`
	IsEnabled   *bool                `json:"is_enabled"`
	ForDuration *string              `json:"for_duration"`
	Domain      *string              `json:"domain"`
	Pool        *string              `json:"pool"`
}

func (h *AlertRulesHandler) UpdateRule(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	var req updateRuleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	updates := map[string]interface{}{}
	if req.IsEnabled != nil {
		updates["is_enabled"] = *req.IsEnabled
	}
	if req.ForDuration != nil {
		updates["for_duration"] = *req.ForDuration
	}
	if req.Domain != nil {
		updates["domain"] = *req.Domain
	}
	if req.Pool != nil {
		updates["pool"] = *req.Pool
	}

	if len(updates) > 0 {
		if err := h.alertRuleRepo.UpdateRuleCtx(c.Request().Context(), uint(id), claims.TenantID, updates); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update rule"})
		}
	}

	if req.Scopes != nil {
		scopeModels := make([]domain.AlertRuleScope, 0, len(*req.Scopes))
		for _, s := range *req.Scopes {
			scopeModels = append(scopeModels, domain.AlertRuleScope{
				ScopeType: s.ScopeType,
				ScopeID:   s.ScopeID,
			})
		}
		if err := h.alertRuleRepo.ReplaceScopesCtx(c.Request().Context(), uint(id), scopeModels); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update scopes"})
		}
	}

	if req.Thresholds != nil {
		threshModels := make([]domain.AlertThreshold, 0, len(*req.Thresholds))
		for _, t := range *req.Thresholds {
			sev := t.Severity
			if sev == "" {
				sev = "warning"
			}
			dur := t.ForDuration
			if dur == "" {
				dur = "5m"
			}
			threshModels = append(threshModels, domain.AlertThreshold{
				Label:       t.Label,
				Threshold:   t.Threshold,
				Severity:    sev,
				ForDuration: dur,
			})
		}
		if err := h.alertRuleRepo.ReplaceThresholdsCtx(c.Request().Context(), uint(id), threshModels); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update thresholds"})
		}
	}

	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (h *AlertRulesHandler) DeleteRule(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	if err := h.alertRuleRepo.DeleteRuleCtx(c.Request().Context(), uint(id), claims.TenantID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete rule"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}

func (h *AlertRulesHandler) ListAlertEvents(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	limitStr := c.QueryParam("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	events, err := h.alertRuleRepo.ListAlertEventsByTenantCtx(c.Request().Context(), claims.TenantID, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list events"})
	}
	return c.JSON(http.StatusOK, events)
}

func (h *AlertRulesHandler) AcknowledgeEvent(c echo.Context) error {
	claims := c.Get("claims").(*infrastructure.Claims)
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	if err := h.alertRuleRepo.AcknowledgeEventCtx(c.Request().Context(), uint(id), claims.UserID, claims.TenantID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to acknowledge event"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"ok": true})
}
