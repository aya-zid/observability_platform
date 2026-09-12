package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type BillingService struct {
	planRepo         *repository.PlanRepository
	subscriptionRepo *repository.SubscriptionRepository
	usageRepo        *repository.UsageRepository
	hostRepo         *repository.HostRepository
	tenantRepo       *repository.TenantRepository
	subReqRepo       *repository.SubscriptionRequestRepository
}

func NewBillingService(
	planRepo *repository.PlanRepository,
	subscriptionRepo *repository.SubscriptionRepository,
	usageRepo *repository.UsageRepository,
	hostRepo *repository.HostRepository,
	tenantRepo *repository.TenantRepository,
	subReqRepo *repository.SubscriptionRequestRepository,
) *BillingService {
	return &BillingService{
		planRepo:         planRepo,
		subscriptionRepo: subscriptionRepo,
		usageRepo:        usageRepo,
		hostRepo:         hostRepo,
		tenantRepo:       tenantRepo,
		subReqRepo:       subReqRepo,
	}
}

func (s *BillingService) GetBillingOverview(ctx context.Context, tenantID uint) (*dto.BillingOverview, error) {
	sub, err := s.subscriptionRepo.FindByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, errors.New("no subscription found")
	}

	plans, err := s.planRepo.ListActivePlansCtx(ctx)
	if err != nil {
		return nil, err
	}

	usage, _ := s.usageRepo.GetTodayUsageCtx(ctx, tenantID)
	hosts, _ := s.hostRepo.ListByTenantCtx(ctx, tenantID)

	planInfo := planToInfo(sub.Plan)
	availablePlans := make([]dto.PlanInfo, 0, len(plans))
	for _, p := range plans {
		availablePlans = append(availablePlans, planToInfo(p))
	}

	logBytesToday := int64(0)
	if usage != nil {
		logBytesToday = usage.LogBytesIngested
	}

	usageInfo := dto.UsageInfo{
		HostsUsed:        len(hosts),
		HostsLimit:       sub.Plan.MaxHosts,
		LogBytesToday:    logBytesToday,
		LogRetentionDays: sub.Plan.LogRetentionDays,
	}

	var pendingInfo *dto.PendingRequestInfo
	if s.subReqRepo != nil {
		pendingReq, _ := s.subReqRepo.GetPendingByTenantCtx(ctx, tenantID)
		if pendingReq != nil {
			pendingInfo = &dto.PendingRequestInfo{
				ID:        pendingReq.ID,
				PlanID:    pendingReq.PlanID,
				PlanName:  pendingReq.Plan.Name,
				PlanPrice: pendingReq.Plan.PricePerMonth,
				Status:    pendingReq.Status,
				Notes:     pendingReq.Notes,
				CreatedAt: pendingReq.CreatedAt.Format(time.RFC3339),
			}
		}
	}

	return &dto.BillingOverview{
		Plan:           planInfo,
		Usage:          usageInfo,
		AvailablePlans: availablePlans,
		PendingRequest: pendingInfo,
	}, nil
}

func (s *BillingService) ChangePlan(ctx context.Context, tenantID uint, newPlanID uint) error {
	newPlan, err := s.planRepo.FindByIDCtx(ctx, newPlanID)
	if err != nil {
		return err
	}
	if newPlan == nil {
		return errors.New("plan not found")
	}
	if !newPlan.IsActive {
		return errors.New("plan is not active")
	}

	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return err
	}
	if len(hosts) > newPlan.MaxHosts {
		return fmt.Errorf("cannot downgrade: you have %d hosts but the plan allows only %d", len(hosts), newPlan.MaxHosts)
	}

	if s.subReqRepo == nil {
		return errors.New("subscription request repository not initialized")
	}

	existingReq, err := s.subReqRepo.GetPendingByTenantCtx(ctx, tenantID)
	if err != nil {
		return err
	}

	if existingReq != nil {
		return s.subReqRepo.UpdatePlanCtx(ctx, existingReq.ID, newPlanID)
	}

	req := &domain.SubscriptionRequest{
		TenantID: tenantID,
		PlanID:   newPlanID,
		Status:   "pending",
	}
	return s.subReqRepo.Create(ctx, req)
}

func (s *BillingService) GetUsageHistory(ctx context.Context, tenantID uint, days int) ([]dto.DailyUsage, error) {
	to := time.Now()
	from := to.AddDate(0, 0, -days)
	rows, err := s.usageRepo.GetUsageRangeCtx(ctx, tenantID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	result := make([]dto.DailyUsage, 0, len(rows))
	for _, r := range rows {
		result = append(result, dto.DailyUsage{
			Date:     r.Date,
			LogBytes: r.LogBytesIngested,
			Metrics:  r.MetricSamplesIngested,
		})
	}
	return result, nil
}

func (s *BillingService) ListActivePlans(ctx context.Context) ([]dto.PlanInfo, error) {
	plans, err := s.planRepo.ListActivePlansCtx(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dto.PlanInfo, 0, len(plans))
	for _, p := range plans {
		result = append(result, planToInfo(p))
	}
	return result, nil
}

func planToInfo(plan domain.Plan) dto.PlanInfo {
	return dto.PlanInfo{
		ID:                   plan.ID,
		Name:                 plan.Name,
		MaxHosts:             plan.MaxHosts,
		MaxServicesPerHost:   plan.MaxServicesPerHost,
		LogRetentionDays:     plan.LogRetentionDays,
		MetricsRetentionDays: plan.MetricsRetentionDays,
		PricePerMonth:        plan.PricePerMonth,
	}
}
