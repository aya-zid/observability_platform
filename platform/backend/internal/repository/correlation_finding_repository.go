package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type CorrelationFindingRepository struct {
	db *gorm.DB
}

func NewCorrelationFindingRepository(db *gorm.DB) *CorrelationFindingRepository {
	return &CorrelationFindingRepository{db: db}
}

func (r *CorrelationFindingRepository) ListActiveByTenantCtx(ctx context.Context, tenantID uint) ([]domain.CorrelationFinding, error) {
	var findings []domain.CorrelationFinding
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND resolved_at IS NULL", tenantID).
		Order("fired_at DESC").
		Find(&findings).Error
	return findings, err
}

func (r *CorrelationFindingRepository) UpsertFinding(ctx context.Context, f *domain.CorrelationFinding) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "machine_id"}, {Name: "rule_id"}},
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "resolved_at IS NULL"}}},
			DoUpdates: clause.AssignmentColumns([]string{"severity", "title", "message", "evidence", "recommended_action", "dashboard_link", "hostname", "service_name", "fired_at", "updated_at"}),
		}).
		Create(f).Error
}

func (r *CorrelationFindingRepository) ResolveFinding(ctx context.Context, tenantID uint, machineID, ruleID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.CorrelationFinding{}).
		Where("tenant_id = ? AND machine_id = ? AND rule_id = ? AND resolved_at IS NULL", tenantID, machineID, ruleID).
		Update("resolved_at", &now).Error
}

func (r *CorrelationFindingRepository) ResolveAllByHost(ctx context.Context, tenantID uint, machineID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&domain.CorrelationFinding{}).
		Where("tenant_id = ? AND machine_id = ? AND resolved_at IS NULL", tenantID, machineID).
		Update("resolved_at", &now).Error
}

func (r *CorrelationFindingRepository) ResolveStale(ctx context.Context, tenantID uint, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	res := r.db.WithContext(ctx).
		Model(&domain.CorrelationFinding{}).
		Where("tenant_id = ? AND resolved_at IS NULL AND updated_at < ?", tenantID, cutoff).
		Update("resolved_at", time.Now())
	return res.RowsAffected, res.Error
}
