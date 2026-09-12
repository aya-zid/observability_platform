// internal/service/admin_dashboard_service.go
package service

import (
	"context"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type AdminDashboardService struct {
	db            *gorm.DB
	dashboardRepo *repository.AdminDashboardRepository
	planRepo      *repository.PlanRepository
	tenantRepo    *repository.TenantRepository
	hostRepo      *repository.HostRepository
	usageRepo     *repository.UsageRepository
	subRepo       *repository.SubscriptionRepository
}

func NewAdminDashboardService(
	db *gorm.DB,
	dashboardRepo *repository.AdminDashboardRepository,
	planRepo *repository.PlanRepository,
	tenantRepo *repository.TenantRepository,
	hostRepo *repository.HostRepository,
	usageRepo *repository.UsageRepository,
	subRepo *repository.SubscriptionRepository,
) *AdminDashboardService {
	return &AdminDashboardService{
		db:            db,
		dashboardRepo: dashboardRepo,
		planRepo:      planRepo,
		tenantRepo:    tenantRepo,
		hostRepo:      hostRepo,
		usageRepo:     usageRepo,
		subRepo:       subRepo,
	}
}

func (s *AdminDashboardService) GetStats(ctx context.Context) (*dto.AdminDashboardStatsResponse, error) {
	stats, err := s.dashboardRepo.GetTenantStats(ctx)
	if err != nil {
		return nil, err
	}

	planDist, err := s.dashboardRepo.GetPlanDistribution(ctx)
	if err != nil {
		return nil, err
	}

	trend, err := s.dashboardRepo.GetIngestionTrend(ctx, 7)
	if err != nil {
		return nil, err
	}

	trendDTO := make([]dto.DailyUsage, 0, len(trend))
	for _, t := range trend {
		trendDTO = append(trendDTO, dto.DailyUsage{
			Date:     t.Date,
			LogBytes: t.LogBytesIngested,
			Metrics:  t.MetricSamplesIngested,
		})
	}

	planDistDTO := make([]dto.PlanDistribution, 0, len(planDist))
	for _, p := range planDist {
		planDistDTO = append(planDistDTO, dto.PlanDistribution{
			PlanName: p.PlanName,
			Count:    p.Count,
		})
	}

	return &dto.AdminDashboardStatsResponse{
		TotalTenants:         stats.TotalTenants,
		ActiveTenants:        stats.ActiveTenants,
		BannedTenants:        stats.BannedTenants,
		TotalHosts:          stats.TotalHosts,
		TotalIngestToday:    stats.TotalIngestToday,
		RevenueEstimate:     stats.RevenueEstimate,
		PendingRequests:     stats.PendingRequests,
		ExpiredSubscriptions: stats.ExpiredSubscriptions,
		PlanDistribution:    planDistDTO,
		IngestionTrend:      trendDTO,
	}, nil
}

func (s *AdminDashboardService) ListTenants(ctx context.Context, limit int, offset int, search string) ([]dto.AdminTenantListItem, int64, error) {
	tenants, total, err := s.dashboardRepo.ListTenantsPaginated(ctx, limit, offset, search)
	if err != nil {
		return nil, 0, err
	}

	result := make([]dto.AdminTenantListItem, 0, len(tenants))
	for _, t := range tenants {
		sub, _ := s.subRepo.FindByTenantCtx(ctx, t.ID)
		hosts, _ := s.hostRepo.ListByTenantCtx(ctx, t.ID)

		planName := ""
		planPrice := 0.0
		currentPeriodEnd := ""
		if sub != nil && sub.Plan.ID > 0 {
			planName = sub.Plan.Name
			planPrice = sub.Plan.PricePerMonth
			if sub.CurrentPeriodEnd != nil {
				currentPeriodEnd = sub.CurrentPeriodEnd.Format("2006-01-02")
			}
		}

		bannedAt := ""
		if t.BannedAt != nil {
			bannedAt = t.BannedAt.Format("2006-01-02")
		}

		result = append(result, dto.AdminTenantListItem{
			ID:                 t.ID,
			CompanyName:        t.CompanyName,
			ContactEmail:       t.ContactEmail,
			IsActive:           t.IsActive,
			SubscriptionStatus: string(t.SubscriptionStatus),
			HostsUsed:          len(hosts),
			PlanName:           planName,
			PlanPrice:          planPrice,
			CurrentPeriodEnd:   ptrStr(currentPeriodEnd),
			BannedAt:           ptrStr(bannedAt),
			BanReason:          t.BanReason,
			CreatedAt:          t.CreatedAt.Format("2006-01-02"),
		})
	}

	return result, total, nil
}

func (s *AdminDashboardService) GetTenantDetail(ctx context.Context, tenantID uint) (*dto.AdminTenantDetailResponse, error) {
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
	aiReports := false
	currentPeriodEnd := ""
	startedAt := ""
	if sub != nil && sub.Plan.ID > 0 {
		planName = sub.Plan.Name
		planPrice = sub.Plan.PricePerMonth
		logRetention = sub.Plan.LogRetentionDays
		metricsRetention = sub.Plan.MetricsRetentionDays
		maxHosts = sub.Plan.MaxHosts
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
		CurrentPeriodEnd:     ptrStr(currentPeriodEnd),
		StartedAt:            startedAt,
		BannedAt:             ptrStr(bannedAt),
		BanReason:            tenant.BanReason,
		AIReportsEnabled:     aiReports,
		UsersCount:           int(usersCount),
		CreatedAt:            tenant.CreatedAt.Format("2006-01-02"),
	}, nil
}
