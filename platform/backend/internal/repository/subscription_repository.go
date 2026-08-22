package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type SubscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) WithDB(db *gorm.DB) *SubscriptionRepository {
	return &SubscriptionRepository{db: db}
}

func (r *SubscriptionRepository) FindByTenantCtx(ctx context.Context, tenantID uint) (*domain.TenantSubscription, error) {
	var sub domain.TenantSubscription
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Preload("Plan").First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &sub, err
}

func (r *SubscriptionRepository) Create(sub *domain.TenantSubscription) error {
	return r.db.Create(sub).Error
}

func (r *SubscriptionRepository) UpdatePlanCtx(ctx context.Context, tenantID uint, planID uint, periodEnd time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.TenantSubscription{}).
		Where("tenant_id = ?", tenantID).
		Updates(map[string]interface{}{
			"plan_id":            planID,
			"status":             "active",
			"started_at":         time.Now(),
			"current_period_end": periodEnd,
			"banned_at":          nil,
			"ban_reason":         "",
			"banned_by_admin_id": nil,
		}).Error
}
