package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type PasswordResetRepository struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) WithDB(db *gorm.DB) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}

func (r *PasswordResetRepository) Create(token *domain.PasswordResetToken) error {
	return r.db.Create(token).Error
}

func (r *PasswordResetRepository) FindByTokenCtx(ctx context.Context, token string) (*domain.PasswordResetToken, error) {
	var row domain.PasswordResetToken
	err := r.db.WithContext(ctx).Where("token = ?", token).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

// InvalidateForUserCtx revokes all active reset tokens for a user.
func (r *PasswordResetRepository) InvalidateForUserCtx(ctx context.Context, userID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.PasswordResetToken{}).Error
}

func (r *PasswordResetRepository) MarkUsedCtx(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).
		Model(&domain.PasswordResetToken{}).
		Where("id = ?", id).
		Update("used", true).Error
}