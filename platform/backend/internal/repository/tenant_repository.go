package repository

import (
	"errors"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type TenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) Create(tenant *domain.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *TenantRepository) FindByUUID(uuid string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.Where("uuid = ?", uuid).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // not found → return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *TenantRepository) FindByEmail(email string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.Where("contact_email = ?", email).First(&tenant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &tenant, nil
}