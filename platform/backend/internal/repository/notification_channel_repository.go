package repository

import (
	"context"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type NotificationChannelRepository struct {
	db *gorm.DB
}

func NewNotificationChannelRepository(db *gorm.DB) *NotificationChannelRepository {
	return &NotificationChannelRepository{db: db}
}

func (r *NotificationChannelRepository) Create(channel *domain.NotificationChannel) error {
	return r.db.Create(channel).Error
}

func (r *NotificationChannelRepository) ListByTenantCtx(ctx context.Context, tenantID uint) ([]domain.NotificationChannel, error) {
	var rows []domain.NotificationChannel
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}

func (r *NotificationChannelRepository) DeleteByIDAndTenantCtx(ctx context.Context, id uint, tenantID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&domain.NotificationChannel{}).Error
}

func (r *NotificationChannelRepository) MarkVerifiedCtx(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Model(&domain.NotificationChannel{}).Where("id = ?", id).Update("is_verified", true).Error
}
