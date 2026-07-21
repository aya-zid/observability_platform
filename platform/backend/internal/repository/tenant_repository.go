// internal/repository/tenant_repository.go
package repository

import (
	"context"
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

func (r *TenantRepository) WithDB(db *gorm.DB) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) Create(tenant *domain.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *TenantRepository) FindByEmail(email string) (*domain.Tenant, error) {
	return r.FindByEmailCtx(context.Background(), email)
}

func (r *TenantRepository) FindByEmailCtx(ctx context.Context, email string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).Where("contact_email = ?", email).First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}

func (r *TenantRepository) FindByIDCtx(ctx context.Context, id uint) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).First(&tenant, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}

func (r *TenantRepository) UpdateAPIKeyByIDCtx(ctx context.Context, id uint, apiKey string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Tenant{}).
		Where("id = ?", id).
		Update("api_key", apiKey).
		Error
}

func (r *TenantRepository) UpdateOrgSettingsCtx(ctx context.Context, tenantID uint, updates map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&domain.Tenant{}).Where("id = ?", tenantID).Updates(updates).Error
}

// FindByAPIKey resolves a raw API key to a full Tenant record.
func (r *TenantRepository) FindAllCtx(ctx context.Context) ([]domain.Tenant, error) {
	var tenants []domain.Tenant
	err := r.db.WithContext(ctx).Find(&tenants).Error
	return tenants, err
}

func (r *TenantRepository) FindByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).
		Where("api_key = ?", apiKey).
		First(&tenant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &tenant, err
}
