// internal/repository/admin_repository.go
package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type AdminRepository struct {
	db *gorm.DB
}

func NewAdminRepository(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) WithDB(db *gorm.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) Create(admin *domain.Admin) error {
	return r.db.Create(admin).Error
}

func (r *AdminRepository) FindByEmailCtx(ctx context.Context, email string) (*domain.Admin, error) {
	var admin domain.Admin
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &admin, err
}

func (r *AdminRepository) FindByIDCtx(ctx context.Context, id uint) (*domain.Admin, error) {
	var admin domain.Admin
	err := r.db.WithContext(ctx).First(&admin, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &admin, err
}

func (r *AdminRepository) UpdatePasswordCtx(ctx context.Context, adminID uint, passwordHash string) error {
	return r.db.WithContext(ctx).Model(&domain.Admin{}).Where("id = ?", adminID).Update("password_hash", passwordHash).Error
}
