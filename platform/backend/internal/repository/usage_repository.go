package repository

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type UsageRepository struct {
	db *gorm.DB
}

func NewUsageRepository(db *gorm.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

func (r *UsageRepository) GetTodayUsageCtx(ctx context.Context, tenantID uint) (*domain.TenantUsageDaily, error) {
	today := time.Now().Format("2006-01-02")
	return r.GetUsageByDateCtx(ctx, tenantID, today)
}

func (r *UsageRepository) GetUsageByDateCtx(ctx context.Context, tenantID uint, date string) (*domain.TenantUsageDaily, error) {
	var usage domain.TenantUsageDaily
	err := r.db.WithContext(ctx).Where("tenant_id = ? AND date = ?", tenantID, date).First(&usage).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &usage, nil
}

func (r *UsageRepository) GetUsageRangeCtx(ctx context.Context, tenantID uint, fromDate string, toDate string) ([]domain.TenantUsageDaily, error) {
	var rows []domain.TenantUsageDaily
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND date >= ? AND date <= ?", tenantID, fromDate, toDate).
		Order("date ASC").
		Find(&rows).Error
	return rows, err
}

func (r *UsageRepository) UpsertDailyUsageCtx(ctx context.Context, usage *domain.TenantUsageDaily) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "date"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"log_bytes_ingested":       gorm.Expr("tenant_usage_dailies.log_bytes_ingested + ?", usage.LogBytesIngested),
			"metric_samples_ingested":  gorm.Expr("tenant_usage_dailies.metric_samples_ingested + ?", usage.MetricSamplesIngested),
		}),
	}).Create(usage).Error
}
