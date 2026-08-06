package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type AlertRuleRepository struct {
	db *gorm.DB
}

func NewAlertRuleRepository(db *gorm.DB) *AlertRuleRepository {
	return &AlertRuleRepository{db: db}
}

func (r *AlertRuleRepository) CreateRuleCtx(ctx context.Context, rule *domain.AlertRule) error {
	return r.db.WithContext(ctx).Create(rule).Error
}

func (r *AlertRuleRepository) ListRulesByTenantCtx(ctx context.Context, tenantID uint) ([]domain.AlertRule, error) {
	var rows []domain.AlertRule
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Scopes").
		Preload("Thresholds").
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) UpdateRuleCtx(ctx context.Context, ruleID uint, tenantID uint, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&domain.AlertRule{}).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Updates(updates).Error
}

func (r *AlertRuleRepository) DeleteRuleCtx(ctx context.Context, ruleID uint, tenantID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", ruleID, tenantID).
		Delete(&domain.AlertRule{}).Error
}

func (r *AlertRuleRepository) ListEnabledRulesCtx(ctx context.Context) ([]domain.AlertRule, error) {
	var rows []domain.AlertRule
	err := r.db.WithContext(ctx).
		Where("is_enabled = ?", true).
		Preload("Scopes").
		Preload("Thresholds").
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) FindByIDCtx(ctx context.Context, ruleID uint) (*domain.AlertRule, error) {
	var rule domain.AlertRule
	err := r.db.WithContext(ctx).Preload("Scopes").Preload("Thresholds").First(&rule, ruleID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rule, nil
}

// Scope methods

func (r *AlertRuleRepository) ReplaceScopesCtx(ctx context.Context, ruleID uint, scopes []domain.AlertRuleScope) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", ruleID).Delete(&domain.AlertRuleScope{}).Error; err != nil {
			return err
		}
		for i := range scopes {
			scopes[i].RuleID = ruleID
		}
		if len(scopes) > 0 {
			return tx.Create(&scopes).Error
		}
		return nil
	})
}

func (r *AlertRuleRepository) ReplaceThresholdsCtx(ctx context.Context, ruleID uint, thresholds []domain.AlertThreshold) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("rule_id = ?", ruleID).Delete(&domain.AlertThreshold{}).Error; err != nil {
			return err
		}
		for i := range thresholds {
			thresholds[i].RuleID = ruleID
		}
		if len(thresholds) > 0 {
			return tx.Create(&thresholds).Error
		}
		return nil
	})
}

// ── Scope resolution helpers ─────────────────────────────────────

func (r *AlertRuleRepository) FindHostByIDCtx(ctx context.Context, hostID uint, tenantID uint) (*domain.Host, error) {
	var host domain.Host
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", hostID, tenantID).First(&host).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &host, err
}

func (r *AlertRuleRepository) FindHostByMachineIDCtx(ctx context.Context, machineID string, tenantID uint) (*domain.Host, error) {
	var host domain.Host
	err := r.db.WithContext(ctx).Where("machine_id = ? AND tenant_id = ?", machineID, tenantID).First(&host).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &host, err
}

func (r *AlertRuleRepository) FindServiceByIDCtx(ctx context.Context, serviceID uint, tenantID uint) (*domain.DiscoveredService, error) {
	var svc domain.DiscoveredService
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", serviceID, tenantID).First(&svc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &svc, err
}

func (r *AlertRuleRepository) ListHostsByTenantCtx(ctx context.Context, tenantID uint) ([]domain.Host, error) {
	var rows []domain.Host
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("hostname ASC").Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) ListServicesByTypeCtx(ctx context.Context, tenantID uint, serviceType string) ([]domain.DiscoveredService, error) {
	var rows []domain.DiscoveredService
	// serviceType maps to ServiceName: "nginx", "apache", "postgres", "mariadb", "redis", "phpfpm"
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND service_name = ?", tenantID, serviceType).
		Order("machine_id ASC").
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) ListNginxVHostsByServiceCtx(ctx context.Context, tenantID uint, serviceID uint) ([]domain.NginxVirtualHost, error) {
	var rows []domain.NginxVirtualHost
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Order("domain ASC").
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) ListApacheVHostsByServiceCtx(ctx context.Context, tenantID uint, serviceID uint) ([]domain.ApacheVirtualHost, error) {
	var rows []domain.ApacheVirtualHost
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND discovered_service_id = ?", tenantID, serviceID).
		Order("domain ASC").
		Find(&rows).Error
	return rows, err
}

// ── Event methods ────────────────────────────────────────────────

func (r *AlertRuleRepository) CreateAlertEventCtx(ctx context.Context, event *domain.AlertEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *AlertRuleRepository) ListAlertEventsByTenantCtx(ctx context.Context, tenantID uint, limit int) ([]domain.AlertEvent, error) {
	var rows []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Preload("Rule").
		Preload("Rule.Scopes").
		Preload("Rule.Thresholds").
		Preload("Threshold").
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) ListFiringEventsByTenantCtx(ctx context.Context, tenantID uint) ([]domain.AlertEvent, error) {
	var rows []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 'firing'", tenantID).
		Preload("Rule").
		Preload("Threshold").
		Order("triggered_at ASC").
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) ListRecentEventsByTenantCtx(ctx context.Context, tenantID uint, since time.Duration) ([]domain.AlertEvent, error) {
	var rows []domain.AlertEvent
	cutoff := time.Now().Add(-since)
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND (status = 'firing' OR resolved_at >= ?)", tenantID, cutoff).
		Preload("Rule").
		Preload("Threshold").
		Order("triggered_at ASC").
		Find(&rows).Error
	return rows, err
}

// ListEventsByTenantInRange returns alert events whose trigger time falls within
// [start, end). Used by reports to count/list alerts fired during a day window.
func (r *AlertRuleRepository) ListEventsByTenantInRange(ctx context.Context, tenantID uint, start, end time.Time) ([]domain.AlertEvent, error) {
	var rows []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND triggered_at >= ? AND triggered_at < ?", tenantID, start, end).
		Preload("Rule").
		Preload("Threshold").
		Order("triggered_at ASC").
		Find(&rows).Error
	return rows, err
}


func (r *AlertRuleRepository) FindFiringEventByThresholdCtx(ctx context.Context, ruleID uint, thresholdID uint, hostMachineID string, serviceID *uint) (*domain.AlertEvent, error) {
	var event domain.AlertEvent
	query := r.db.WithContext(ctx).
		Where("rule_id = ? AND threshold_id = ? AND host_machine_id = ? AND status = 'firing'", ruleID, thresholdID, hostMachineID)
	if serviceID != nil {
		query = query.Where("service_id = ?", *serviceID)
	} else {
		query = query.Where("service_id IS NULL")
	}
	err := query.First(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &event, err
}

// ListFiringEventsByRule returns all firing events for a given rule.
// Used during resolution detection.
func (r *AlertRuleRepository) ListFiringEventsByRuleCtx(ctx context.Context, ruleID uint) ([]domain.AlertEvent, error) {
	var events []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("rule_id = ? AND status = 'firing'", ruleID).
		Preload("Threshold").
		Find(&events).Error
	return events, err
}

type FiringAlertCounts struct {
	Total    int
	Critical int
	Warning  int
}

func (r *AlertRuleRepository) CountFiringEventsByTenantCtx(ctx context.Context, tenantID uint) (FiringAlertCounts, error) {
	var rows []struct {
		Severity string
		Count    int
	}
	err := r.db.WithContext(ctx).
		Table("alert_events ae").
		Select("coalesce(at.severity, ar.default_severity, 'warning') as severity, count(*) as count").
		Joins("left join alert_thresholds at on at.id = ae.threshold_id").
		Joins("left join alert_rules ar on ar.id = ae.rule_id").
		Where("ae.tenant_id = ? AND ae.status = 'firing'", tenantID).
		Group("coalesce(at.severity, ar.default_severity, 'warning')").
		Scan(&rows).Error
	if err != nil {
		return FiringAlertCounts{}, err
	}

	var out FiringAlertCounts
	for _, row := range rows {
		out.Total += row.Count
		switch row.Severity {
		case "critical":
			out.Critical += row.Count
		default:
			out.Warning += row.Count
		}
	}
	return out, nil
}

func (r *AlertRuleRepository) ResolveEventCtx(ctx context.Context, eventID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.AlertEvent{}).
		Where("id = ?", eventID).
		Updates(map[string]interface{}{
			"status":      "resolved",
			"resolved_at": &now,
		}).Error
}

func (r *AlertRuleRepository) AcknowledgeEventCtx(ctx context.Context, eventID uint, userID uint, tenantID uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.AlertEvent{}).
		Where("id = ? AND tenant_id = ?", eventID, tenantID).
		Updates(map[string]interface{}{
			"status":          "acknowledged",
			"acknowledged_by": userID,
		}).Error
}

func (r *AlertRuleRepository) FindUnnotifiedEventsCtx(ctx context.Context) ([]domain.AlertEvent, error) {
	var events []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("status = 'firing' AND is_notified = ?", false).
		Preload("Rule").
		Preload("Threshold").
		Order("created_at ASC").
		Find(&events).Error
	return events, err
}

func (r *AlertRuleRepository) MarkEventNotifiedCtx(ctx context.Context, eventID uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.AlertEvent{}).
		Where("id = ?", eventID).
		Update("is_notified", true).Error
}
