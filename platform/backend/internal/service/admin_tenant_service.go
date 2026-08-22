// internal/service/admin_tenant_service.go
package service

import (
	"context"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type AdminTenantService struct {
	db            *gorm.DB
	tenantRepo    *repository.TenantRepository
	subRepo       *repository.SubscriptionRepository
	planRepo      *repository.PlanRepository
	hostRepo      *repository.HostRepository
	userRepo      *repository.UserRepository
	dashboardRepo *repository.AdminDashboardRepository
}

func NewAdminTenantService(
	db *gorm.DB,
	tenantRepo *repository.TenantRepository,
	subRepo *repository.SubscriptionRepository,
	planRepo *repository.PlanRepository,
	hostRepo *repository.HostRepository,
	userRepo *repository.UserRepository,
	dashboardRepo *repository.AdminDashboardRepository,
) *AdminTenantService {
	return &AdminTenantService{
		db:            db,
		tenantRepo:    tenantRepo,
		subRepo:       subRepo,
		planRepo:      planRepo,
		hostRepo:      hostRepo,
		userRepo:      userRepo,
		dashboardRepo: dashboardRepo,
	}
}

func (s *AdminTenantService) BanTenant(ctx context.Context, tenantID uint, adminID uint, reason string) error {
	now := time.Now()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Tenant{}).Where("id = ?", tenantID).Updates(map[string]interface{}{
			"is_active":           false,
			"subscription_status": domain.SubscriptionStatusSuspended,
			"banned_at":           now,
			"ban_reason":          reason,
			"banned_by_admin_id":  adminID,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.TenantSubscription{}).Where("tenant_id = ?", tenantID).Updates(map[string]interface{}{
			"status":             "suspended",
			"banned_at":          now,
			"ban_reason":         reason,
			"banned_by_admin_id": adminID,
		}).Error
	})
}

func (s *AdminTenantService) UnbanTenant(ctx context.Context, tenantID uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Tenant{}).Where("id = ?", tenantID).Updates(map[string]interface{}{
			"is_active":           true,
			"subscription_status": domain.SubscriptionStatusActive,
			"banned_at":           nil,
			"ban_reason":          "",
			"banned_by_admin_id":  nil,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.TenantSubscription{}).Where("tenant_id = ?", tenantID).Updates(map[string]interface{}{
			"status":             "active",
			"banned_at":          nil,
			"ban_reason":         "",
			"banned_by_admin_id": nil,
		}).Error
	})
}

func (s *AdminTenantService) GetTenantDetail(ctx context.Context, tenantID uint) (*dto.AdminTenantDetailResponse, error) {
	tenant, err := s.tenantRepo.FindByIDCtx(ctx, tenantID)
	if err != nil || tenant == nil {
		return nil, err
	}

	sub, _ := s.subRepo.FindByTenantCtx(ctx, tenantID)
	hosts, _ := s.hostRepo.ListByTenantCtx(ctx, tenantID)

	var usersCount int64
	s.db.WithContext(ctx).Model(&domain.User{}).Where("tenant_id = ?", tenantID).Count(&usersCount)

	planName := ""
	planPrice := 0.0
	logRetention := 0
	metricsRetention := 0
	maxHosts := 0
	maxLogBytes := int64(0)
	aiReports := false
	currentPeriodEnd := ""
	startedAt := ""
	if sub != nil && sub.Plan.ID > 0 {
		planName = sub.Plan.Name
		planPrice = sub.Plan.PricePerMonth
		logRetention = sub.Plan.LogRetentionDays
		metricsRetention = sub.Plan.MetricsRetentionDays
		maxHosts = sub.Plan.MaxHosts
		maxLogBytes = sub.Plan.MaxLogBytesPerDay
		aiReports = sub.Plan.AIReportsEnabled
		if sub.CurrentPeriodEnd != nil {
			currentPeriodEnd = sub.CurrentPeriodEnd.Format("2006-01-02")
		}
		startedAt = sub.StartedAt.Format("2006-01-02")
	}

	bannedAt := ""
	if tenant.BannedAt != nil {
		bannedAt = tenant.BannedAt.Format("2006-01-02")
	}

	return &dto.AdminTenantDetailResponse{
		ID:                   tenant.ID,
		CompanyName:          tenant.CompanyName,
		ContactEmail:         tenant.ContactEmail,
		ContactPhone:         tenant.ContactPhone,
		IsActive:             tenant.IsActive,
		SubscriptionStatus:   string(tenant.SubscriptionStatus),
		Timezone:             tenant.Timezone,
		HostsUsed:            len(hosts),
		PlanName:             planName,
		PlanPrice:            planPrice,
		LogRetentionDays:     logRetention,
		MetricsRetentionDays: metricsRetention,
		MaxHosts:             maxHosts,
		MaxLogBytesPerDay:    maxLogBytes,
		CurrentPeriodEnd:     ptrStr(currentPeriodEnd),
		StartedAt:            startedAt,
		BannedAt:             ptrStr(bannedAt),
		BanReason:            tenant.BanReason,
		AIReportsEnabled:     aiReports,
		UsersCount:           int(usersCount),
		CreatedAt:            tenant.CreatedAt.Format("2006-01-02"),
	}, nil
}

func (s *AdminTenantService) AssignPack(ctx context.Context, tenantID uint, planID uint) error {
	subSvc := NewAdminSubscriptionService(s.db, s.tenantRepo, s.subRepo, s.planRepo, s.hostRepo)
	return subSvc.AssignPack(ctx, tenantID, planID)
}

func (s *AdminTenantService) RenewSubscription(ctx context.Context, tenantID uint, durationMonths int) error {
	subSvc := NewAdminSubscriptionService(s.db, s.tenantRepo, s.subRepo, s.planRepo, s.hostRepo)
	return subSvc.RenewSubscription(ctx, tenantID, durationMonths)
}

func ptrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
