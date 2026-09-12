package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type SubscriptionRequestRepository struct {
	db *gorm.DB
}

func NewSubscriptionRequestRepository(db *gorm.DB) *SubscriptionRequestRepository {
	return &SubscriptionRequestRepository{db: db}
}

func (r *SubscriptionRequestRepository) Create(ctx context.Context, req *domain.SubscriptionRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

func (r *SubscriptionRequestRepository) FindByIDCtx(ctx context.Context, id uint) (*domain.SubscriptionRequest, error) {
	var req domain.SubscriptionRequest
	err := r.db.WithContext(ctx).Preload("Tenant").Preload("Plan").First(&req, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &req, err
}

func (r *SubscriptionRequestRepository) List(ctx context.Context, status string) ([]domain.SubscriptionRequest, error) {
	var requests []domain.SubscriptionRequest
	query := r.db.WithContext(ctx).Preload("Tenant").Preload("Plan").Order("created_at DESC")
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&requests).Error
	return requests, err
}

func (r *SubscriptionRequestRepository) ListByTenantCtx(ctx context.Context, tenantID uint) ([]domain.SubscriptionRequest, error) {
	var requests []domain.SubscriptionRequest
	err := r.db.WithContext(ctx).Preload("Plan").Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&requests).Error
	return requests, err
}

func (r *SubscriptionRequestRepository) UpdateStatus(ctx context.Context, id uint, status string, adminNotes string, adminID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.SubscriptionRequest{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      status,
			"admin_notes": adminNotes,
			"reviewed_at": &now,
			"reviewed_by": &adminID,
		}).Error
}

func (r *SubscriptionRequestRepository) CountPending(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.SubscriptionRequest{}).Where("status = ?", "pending").Count(&count).Error
	return count, err
}

func (r *SubscriptionRequestRepository) GetPendingByTenantCtx(ctx context.Context, tenantID uint) (*domain.SubscriptionRequest, error) {
	var req domain.SubscriptionRequest
	err := r.db.WithContext(ctx).Preload("Plan").Where("tenant_id = ? AND status = ?", tenantID, "pending").First(&req).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &req, err
}

func (r *SubscriptionRequestRepository) UpdatePlanCtx(ctx context.Context, id uint, planID uint) error {
	return r.db.WithContext(ctx).Model(&domain.SubscriptionRequest{}).
		Where("id = ?", id).
		Update("plan_id", planID).Error
}
