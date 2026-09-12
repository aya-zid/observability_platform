// internal/repository/admin_dashboard_repository.go
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
)

type AdminDashboardRepository struct {
	db *gorm.DB
}

func NewAdminDashboardRepository(db *gorm.DB) *AdminDashboardRepository {
	return &AdminDashboardRepository{db: db}
}

type TenantStats struct {
	TotalTenants         int64
	ActiveTenants        int64
	BannedTenants        int64
	TotalHosts          int64
	TotalIngestToday    int64
	RevenueEstimate     float64
	PendingRequests      int64
	ExpiredSubscriptions int64
}

type PlanDistribution struct {
	PlanName string
	Count    int64
}

func (r *AdminDashboardRepository) GetTenantStats(ctx context.Context) (*TenantStats, error) {
	var stats TenantStats

	if err := r.db.WithContext(ctx).Model(&domain.Tenant{}).Count(&stats.TotalTenants).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&domain.Tenant{}).Where("is_active = ? AND subscription_status = ?", true, domain.SubscriptionStatusActive).Count(&stats.ActiveTenants).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&domain.Tenant{}).Where("banned_at IS NOT NULL").Count(&stats.BannedTenants).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Model(&domain.Host{}).Count(&stats.TotalHosts).Error; err != nil {
		return nil, err
	}

	today := time.Now().Format("2006-01-02")
	var totalIngest int64
	if err := r.db.WithContext(ctx).Model(&domain.TenantUsageDaily{}).Where("date = ?", today).Select("COALESCE(SUM(log_bytes_ingested), 0)").Scan(&totalIngest).Error; err != nil {
		return nil, err
	}
	stats.TotalIngestToday = totalIngest

	var subs []domain.TenantSubscription
	if err := r.db.WithContext(ctx).Preload("Plan").Where("status = ?", "active").Find(&subs).Error; err != nil {
		return nil, err
	}
	for _, sub := range subs {
		if sub.Plan.ID > 0 {
			stats.RevenueEstimate += sub.Plan.PricePerMonth
		}
	}

	r.db.WithContext(ctx).Model(&domain.SubscriptionRequest{}).Where("status = ?", "pending").Count(&stats.PendingRequests)

	now := time.Now()
	r.db.WithContext(ctx).Model(&domain.TenantSubscription{}).Where("status = ? AND current_period_end IS NOT NULL AND current_period_end < ?", "active", now).Count(&stats.ExpiredSubscriptions)

	return &stats, nil
}

func (r *AdminDashboardRepository) GetPlanDistribution(ctx context.Context) ([]PlanDistribution, error) {
	var results []PlanDistribution
	err := r.db.WithContext(ctx).
		Table("tenant_subscriptions AS ts").
		Select("p.name as plan_name, COUNT(ts.id) as count").
		Joins("JOIN plans p ON p.id = ts.plan_id").
		Where("ts.status = ?", "active").
		Group("p.name").
		Order("count DESC").
		Scan(&results).Error
	return results, err
}

func (r *AdminDashboardRepository) GetIngestionTrend(ctx context.Context, days int) ([]domain.TenantUsageDaily, error) {
	var rows []domain.TenantUsageDaily
	from := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	err := r.db.WithContext(ctx).
		Where("date >= ?", from).
		Order("date ASC").
		Find(&rows).Error
	return rows, err
}

func (r *AdminDashboardRepository) ListTenantsPaginated(ctx context.Context, limit int, offset int, search string) ([]domain.Tenant, int64, error) {
	var total int64
	query := r.db.WithContext(ctx).Model(&domain.Tenant{})
	if search != "" {
		query = query.Where("company_name ILIKE ? OR contact_email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var tenants []domain.Tenant
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&tenants).Error
	return tenants, total, err
}
