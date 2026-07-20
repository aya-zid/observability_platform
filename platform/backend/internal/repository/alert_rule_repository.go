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

// Event methods

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
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}

func (r *AlertRuleRepository) FindFiringEventCtx(ctx context.Context, ruleID uint, hostMachineID string, serviceID *uint) (*domain.AlertEvent, error) {
	var event domain.AlertEvent
	query := r.db.WithContext(ctx).Where("rule_id = ? AND host_machine_id = ? AND status = 'firing'", ruleID, hostMachineID)
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
			"status":           "acknowledged",
			"acknowledged_by":  userID,
		}).Error
}

func (r *AlertRuleRepository) FindUnnotifiedEventsCtx(ctx context.Context) ([]domain.AlertEvent, error) {
	var events []domain.AlertEvent
	err := r.db.WithContext(ctx).
		Where("status = 'firing' AND is_notified = ?", false).
		Preload("Rule").
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
