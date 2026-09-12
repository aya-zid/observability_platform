// internal/service/admin_pack_service.go
package service

import (
	"context"
	"errors"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type AdminPackService struct {
	db            *gorm.DB
	planRepo      *repository.PlanRepository
	subRepo       *repository.SubscriptionRepository
	dashboardRepo *repository.AdminDashboardRepository
}

func NewAdminPackService(
	db *gorm.DB,
	planRepo *repository.PlanRepository,
	subRepo *repository.SubscriptionRepository,
	dashboardRepo *repository.AdminDashboardRepository,
) *AdminPackService {
	return &AdminPackService{
		db:            db,
		planRepo:      planRepo,
		subRepo:       subRepo,
		dashboardRepo: dashboardRepo,
	}
}

func (s *AdminPackService) ListPacks(ctx context.Context) ([]dto.AdminPackResponse, error) {
	var plans []domain.Plan
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&plans).Error; err != nil {
		return nil, err
	}

	result := make([]dto.AdminPackResponse, 0, len(plans))
	for _, p := range plans {
		activeCount, err := s.getActiveSubscriptionsCount(ctx, p.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, dto.AdminPackResponse{
			ID:                   p.ID,
			Name:                 p.Name,
			Description:          p.Description,
			PricePerMonth:        p.PricePerMonth,
			MaxHosts:             p.MaxHosts,
			MaxServicesPerHost:   p.MaxServicesPerHost,
			LogRetentionDays:     p.LogRetentionDays,
			MetricsRetentionDays: p.MetricsRetentionDays,
			AIReportsEnabled:     p.AIReportsEnabled,
			DurationMonths:       p.DurationMonths,
			IsActive:             p.IsActive,
			IsTrial:              p.IsTrial,
			ActiveSubscriptions:  activeCount,
		})
	}
	return result, nil
}

func (s *AdminPackService) GetPack(ctx context.Context, packID uint) (*dto.AdminPackResponse, error) {
	var plan domain.Plan
	if err := s.db.WithContext(ctx).First(&plan, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("pack not found")
		}
		return nil, err
	}

	activeCount, err := s.getActiveSubscriptionsCount(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	return &dto.AdminPackResponse{
		ID:                   plan.ID,
		Name:                 plan.Name,
		Description:          plan.Description,
		PricePerMonth:        plan.PricePerMonth,
		MaxHosts:             plan.MaxHosts,
		MaxServicesPerHost:   plan.MaxServicesPerHost,
		LogRetentionDays:     plan.LogRetentionDays,
		MetricsRetentionDays: plan.MetricsRetentionDays,
		AIReportsEnabled:     plan.AIReportsEnabled,
		DurationMonths:       plan.DurationMonths,
		IsActive:             plan.IsActive,
		IsTrial:              plan.IsTrial,
		ActiveSubscriptions:  activeCount,
	}, nil
}

func (s *AdminPackService) CreatePack(ctx context.Context, req dto.AdminCreatePackRequest) (*dto.AdminPackResponse, error) {
	if err := s.ensureTrialPlanIsUnique(ctx, req.IsTrial, 0); err != nil {
		return nil, err
	}
	plan := &domain.Plan{
		Name:                 req.Name,
		Description:          req.Description,
		PricePerMonth:        req.PricePerMonth,
		MaxHosts:             req.MaxHosts,
		MaxServicesPerHost:   req.MaxServicesPerHost,
		LogRetentionDays:     req.LogRetentionDays,
		MetricsRetentionDays: req.MetricsRetentionDays,
		AIReportsEnabled:     req.AIReportsEnabled,
		DurationMonths:       req.DurationMonths,
		IsActive:             req.IsActive,
		IsTrial:              req.IsTrial,
	}

	if err := s.db.WithContext(ctx).Create(plan).Error; err != nil {
		return nil, err
	}

	return &dto.AdminPackResponse{
		ID:                   plan.ID,
		Name:                 plan.Name,
		Description:          plan.Description,
		PricePerMonth:        plan.PricePerMonth,
		MaxHosts:             plan.MaxHosts,
		MaxServicesPerHost:   plan.MaxServicesPerHost,
		LogRetentionDays:     plan.LogRetentionDays,
		MetricsRetentionDays: plan.MetricsRetentionDays,
		AIReportsEnabled:     plan.AIReportsEnabled,
		DurationMonths:       plan.DurationMonths,
		IsActive:             plan.IsActive,
		IsTrial:              plan.IsTrial,
	}, nil
}

func (s *AdminPackService) UpdatePack(ctx context.Context, packID uint, req dto.AdminUpdatePackRequest) (*dto.AdminPackResponse, error) {
	var plan domain.Plan
	if err := s.db.WithContext(ctx).First(&plan, packID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("pack not found")
		}
		return nil, err
	}
	if err := s.ensureTrialPlanIsUnique(ctx, req.IsTrial, packID); err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":                   req.Name,
		"description":            req.Description,
		"price_per_month":        req.PricePerMonth,
		"max_hosts":              req.MaxHosts,
		"max_services_per_host":  req.MaxServicesPerHost,
		"log_retention_days":     req.LogRetentionDays,
		"metrics_retention_days": req.MetricsRetentionDays,
		"ai_reports_enabled":     req.AIReportsEnabled,
		"duration_months":        req.DurationMonths,
		"is_active":              req.IsActive,
		"is_trial":               req.IsTrial,
	}

	if err := s.db.WithContext(ctx).Model(&domain.Plan{}).Where("id = ?", packID).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetPack(ctx, packID)
}

func (s *AdminPackService) DeletePack(ctx context.Context, packID uint) error {
	var subscriptions int64
	if err := s.db.WithContext(ctx).Model(&domain.TenantSubscription{}).Where("plan_id = ?", packID).Count(&subscriptions).Error; err != nil {
		return err
	}
	if subscriptions > 0 {
		return errors.New("cannot delete a pack that is assigned to a tenant")
	}
	result := s.db.WithContext(ctx).Delete(&domain.Plan{}, packID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("pack not found")
	}
	return nil
}

func (s *AdminPackService) getActiveSubscriptionsCount(ctx context.Context, planID uint) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.TenantSubscription{}).Where("plan_id = ?", planID).Count(&count).Error
	return count, err
}

// Only one active trial pack can exist: signup selects it by the is_trial flag.
func (s *AdminPackService) ensureTrialPlanIsUnique(ctx context.Context, isTrial bool, excludedID uint) error {
	if !isTrial {
		return nil
	}
	var count int64
	query := s.db.WithContext(ctx).Model(&domain.Plan{}).Where("is_trial = ? AND is_active = ?", true, true)
	if excludedID > 0 {
		query = query.Where("id <> ?", excludedID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("only one active trial pack is allowed")
	}
	return nil
}
