package repository

import (
	"context"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) WithDB(db *gorm.DB) *PlanRepository {
	return &PlanRepository{db: db}
}

func (r *PlanRepository) ListActivePlansCtx(ctx context.Context) ([]domain.Plan, error) {
	var rows []domain.Plan
	err := r.db.WithContext(ctx).Where("is_active = ?", true).Order("price_per_month ASC").Find(&rows).Error
	return rows, err
}

func (r *PlanRepository) FindByIDCtx(ctx context.Context, id uint) (*domain.Plan, error) {
	var plan domain.Plan
	err := r.db.WithContext(ctx).First(&plan, id).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}
