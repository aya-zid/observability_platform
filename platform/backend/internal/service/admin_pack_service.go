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
	db           *gorm.DB
	planRepo     *repository.PlanRepository
	subRepo      *repository.SubscriptionRepository
	dashboardRepo *repository.AdminDashboardRepository
}

func NewAdminPackService(
	db *gorm.DB,
	planRepo *repository.PlanRepository,
	subRepo *repository.SubscriptionRepository,
	dashboardRepo *repository.AdminDashboardRepository,
) *AdminPackService {
	return &AdminPackService{
		db:           db,
		planRepo:     planRepo,
		subRepo:      subRepo,
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
		activeCount, _ := s.getActiveSubscriptionsCount(ctx, p.ID)
		result = append(result, dto.AdminPackResponse{
			ID:                   p.ID,
			Name:                 p.Name,
			Description:          p.Description,
			PricePerMonth:        p.PricePerMonth,
			MaxHosts:             p.MaxHosts,
			MaxServicesPerHost:   p.MaxServicesPerHost,
			LogRetentionDays:     p.LogRetentionDays,
			MetricsRetentionDays: p.MetricsRetentionDays,
			MaxLogBytesPerDay:    p.MaxLogBytesPerDay,
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

	activeCount, _ := s.getActiveSubscriptionsCount(ctx, plan.ID)
	return &dto.AdminPackResponse{
		ID:                   plan.ID,
		Name:                 plan.Name,
		Description:          plan.Description,
		PricePerMonth:        plan.PricePerMonth,
		MaxHosts:             plan.MaxHosts,
		MaxServicesPerHost:   plan.MaxServicesPerHost,
		LogRetentionDays:     plan.LogRetentionDays,
		MetricsRetentionDays: plan.MetricsRetentionDays,
		MaxLogBytesPerDay:    plan.MaxLogBytesPerDay,
		AIReportsEnabled:     plan.AIReportsEnabled,
		DurationMonths:       plan.DurationMonths,
		IsActive:             plan.IsActive,
		IsTrial:              plan.IsTrial,
		ActiveSubscriptions:  activeCount,
	}, nil
}

func (s *AdminPackService) CreatePack(ctx context.Context, req dto.AdminCreatePackRequest) (*dto.AdminPackResponse, error) {
	plan := &domain.Plan{
		Name:                 req.Name,
		Description:          req.Description,
		PricePerMonth:        req.PricePerMonth,
		MaxHosts:             req.MaxHosts,
		MaxServicesPerHost:   req.MaxServicesPerHost,
		LogRetentionDays:     req.LogRetentionDays,
		MetricsRetentionDays: req.MetricsRetentionDays,
		MaxLogBytesPerDay:    req.MaxLogBytesPerDay,
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
		MaxLogBytesPerDay:    plan.MaxLogBytesPerDay,
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

	updates := map[string]interface{}{
		"name":                  req.Name,
		"description":           req.Description,
		"price_per_month":       req.PricePerMonth,
		"max_hosts":             req.MaxHosts,
		"max_services_per_host": req.MaxServicesPerHost,
		"log_retention_days":    req.LogRetentionDays,
		"metrics_retention_days": req.MetricsRetentionDays,
		"max_log_bytes_per_day": req.MaxLogBytesPerDay,
		"ai_reports_enabled":    req.AIReportsEnabled,
		"duration_months":       req.DurationMonths,
		"is_active":             req.IsActive,
		"is_trial":              req.IsTrial,
	}

	if err := s.db.WithContext(ctx).Model(&domain.Plan{}).Where("id = ?", packID).Updates(updates).Error; err != nil {
		return nil, err
	}

	return s.GetPack(ctx, packID)
}

func (s *AdminPackService) DeletePack(ctx context.Context, packID uint) error {
	activeCount, _ := s.getActiveSubscriptionsCount(ctx, packID)
	if activeCount > 0 {
		return errors.New("cannot delete pack with active subscriptions")
	}
	return s.db.WithContext(ctx).Model(&domain.Plan{}).Where("id = ?", packID).Update("is_active", false).Error
}

func (s *AdminPackService) getActiveSubscriptionsCount(ctx context.Context, planID uint) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&domain.TenantSubscription{}).Where("plan_id = ? AND status = ?", planID, "active").Count(&count).Error
	return count, err
}
