package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type InvitationRepository struct {
	db *gorm.DB
}

func NewInvitationRepository(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

func (r *InvitationRepository) WithDB(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{db: db}
}

func (r *InvitationRepository) Create(invitation *domain.Invitation) error {
	return r.db.Create(invitation).Error
}

func (r *InvitationRepository) FindByTokenCtx(ctx context.Context, token string) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.WithContext(ctx).Where("token = ?", token).Preload("Tenant").First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &inv, err
}

func (r *InvitationRepository) ListByTenantCtx(ctx context.Context, tenantID uint) ([]domain.Invitation, error) {
	var rows []domain.Invitation
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = 'pending'", tenantID).
		Order("created_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *InvitationRepository) UpdateStatusCtx(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Invitation{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *InvitationRepository) FindPendingByEmailAndTenantCtx(ctx context.Context, email string, tenantID uint) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.WithContext(ctx).
		Where("email = ? AND tenant_id = ? AND status = 'pending'", email, tenantID).
		First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &inv, err
}

func (r *InvitationRepository) FindByIDAndTenantCtx(ctx context.Context, id uint, tenantID uint) (*domain.Invitation, error) {
	var inv domain.Invitation
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&inv).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &inv, err
}
