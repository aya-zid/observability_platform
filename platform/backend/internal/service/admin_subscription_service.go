// internal/service/admin_subscription_service.go
package service

import (
	"context"
	"errors"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type AdminSubscriptionService struct {
	db          *gorm.DB
	tenantRepo  *repository.TenantRepository
	subRepo     *repository.SubscriptionRepository
	planRepo    *repository.PlanRepository
	hostRepo    *repository.HostRepository
}

func NewAdminSubscriptionService(
	db *gorm.DB,
	tenantRepo *repository.TenantRepository,
	subRepo *repository.SubscriptionRepository,
	planRepo *repository.PlanRepository,
	hostRepo *repository.HostRepository,
) *AdminSubscriptionService {
	return &AdminSubscriptionService{
		db:         db,
		tenantRepo: tenantRepo,
		subRepo:    subRepo,
		planRepo:   planRepo,
		hostRepo:   hostRepo,
	}
}

func (s *AdminSubscriptionService) AssignPack(ctx context.Context, tenantID uint, planID uint) error {
	var plan domain.Plan
	if err := s.db.WithContext(ctx).First(&plan, planID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("pack not found")
		}
		return err
	}
	if !plan.IsActive {
		return errors.New("pack is not active")
	}

	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return err
	}
	if len(hosts) > plan.MaxHosts {
		return errors.New("tenant has more hosts than this pack allows")
	}

	now := time.Now()
	periodEnd := now.AddDate(0, plan.DurationMonths, 0)

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing domain.TenantSubscription
		if err := tx.Where("tenant_id = ?", tenantID).First(&existing).Error; err == nil {
			existing.PlanID = planID
			existing.Status = "active"
			existing.StartedAt = now
			existing.CurrentPeriodEnd = &periodEnd
			existing.BannedAt = nil
			existing.BanReason = ""
			existing.BannedByAdminID = nil
			existing.Notes = ""
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
		} else {
			sub := &domain.TenantSubscription{
				TenantID:         tenantID,
				PlanID:           planID,
				Status:           "active",
				StartedAt:        now,
				CurrentPeriodEnd: &periodEnd,
			}
			if err := tx.Create(sub).Error; err != nil {
				return err
			}
		}

		return s.tenantRepo.WithDB(tx).UpdateSubscriptionStatusCtx(context.Background(), tenantID, "active")
	})
}

func (s *AdminSubscriptionService) RenewSubscription(ctx context.Context, tenantID uint, durationMonths int) error {
	if durationMonths < 1 {
		return errors.New("duration must be at least 1 month")
	}

	var sub domain.TenantSubscription
	if err := s.db.WithContext(ctx).Where("tenant_id = ?", tenantID).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("subscription not found")
		}
		return err
	}

	now := time.Now()
	var newEnd time.Time
	if sub.CurrentPeriodEnd != nil && sub.CurrentPeriodEnd.After(now) {
		newEnd = sub.CurrentPeriodEnd.AddDate(0, durationMonths, 0)
	} else {
		newEnd = now.AddDate(0, durationMonths, 0)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.TenantSubscription{}).Where("tenant_id = ?", tenantID).Updates(map[string]interface{}{
			"status":             "active",
			"current_period_end": newEnd,
			"banned_at":          nil,
			"ban_reason":         "",
			"banned_by_admin_id": nil,
		}).Error; err != nil {
			return err
		}
		return s.tenantRepo.WithDB(tx).UpdateSubscriptionStatusCtx(context.Background(), tenantID, "active")
	})
}

func (s *AdminSubscriptionService) ListExpired(ctx context.Context) ([]domain.TenantSubscription, error) {
	var subs []domain.TenantSubscription
	now := time.Now()
	err := s.db.WithContext(ctx).
		Preload("Plan").
		Preload("Tenant").
		Where("status = ? AND current_period_end IS NOT NULL AND current_period_end < ?", "active", now).
		Find(&subs).Error
	return subs, err
}
