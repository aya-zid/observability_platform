// internal/repository/tenant_repository.go
package repository

import (
	"context"

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

func (r *TenantRepository) FindByEmail(email string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.Where("contact_email = ?", email).First(&tenant).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tenant, err
}

// FindByAPIKey resolves an API key to a full Tenant record.
// Called by IngestMiddleware on cache miss — only hits DB once per 5-minute TTL window.
func (r *TenantRepository) FindByAPIKey(ctx context.Context, apiKey string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.WithContext(ctx).
		Where("api_key = ?", apiKey).
		First(&tenant).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return &tenant, err
}