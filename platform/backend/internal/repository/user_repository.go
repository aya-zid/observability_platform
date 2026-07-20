package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) WithDB(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *UserRepository) FindByEmail(email string) (*domain.User, error) {
	return r.FindByEmailCtx(context.Background(), email)
}

func (r *UserRepository) FindByEmailCtx(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByIDCtx(ctx context.Context, id uint) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) ListByTenantCtx(ctx context.Context, tenantID uint) ([]domain.User, error) {
	var rows []domain.User
	err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at ASC").Find(&rows).Error
	return rows, err
}

func (r *UserRepository) DeleteByIDCtx(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&domain.User{}, id).Error
}

func (r *UserRepository) UpdateRoleCtx(ctx context.Context, id uint, role string) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update("role", role).Error
}

func (r *UserRepository) UpdatePasswordCtx(ctx context.Context, id uint, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(map[string]interface{}{
		"password_hash":      passwordHash,
		"password_changed_at": time.Now(),
	}).Error
}

func (r *UserRepository) CountByTenantAndRoleCtx(ctx context.Context, tenantID uint, role string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.User{}).Where("tenant_id = ? AND role = ?", tenantID, role).Count(&count).Error
	return count, err
}
