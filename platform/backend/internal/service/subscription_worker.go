// internal/service/subscription_worker.go
package service

import (
	"context"
	"log"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type SubscriptionWorker struct {
	db             *gorm.DB
	subscriptionRepo *repository.SubscriptionRepository
}

func NewSubscriptionWorker(db *gorm.DB, subscriptionRepo *repository.SubscriptionRepository) *SubscriptionWorker {
	return &SubscriptionWorker{
		db:             db,
		subscriptionRepo: subscriptionRepo,
	}
}

func (w *SubscriptionWorker) StartExpirationChecker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.checkExpiredSubscriptions(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (w *SubscriptionWorker) checkExpiredSubscriptions(ctx context.Context) {
	now := time.Now()
	result := w.db.WithContext(ctx).Model(&domain.TenantSubscription{}).
		Where("status = ? AND current_period_end IS NOT NULL AND current_period_end < ?", "active", now).
		Updates(map[string]interface{}{"status": "expired"})

	if result.Error != nil {
		log.Printf("[subscription-worker] error checking expired subscriptions: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		log.Printf("[subscription-worker] marked %d expired subscriptions", result.RowsAffected)
	}

	var trialSubs []domain.TenantSubscription
	if err := w.db.WithContext(ctx).
		Preload("Plan").
		Where("status = ? AND current_period_end IS NULL AND started_at < ?", "active", now.AddDate(0, 0, -14)).
		Find(&trialSubs).Error; err != nil {
		log.Printf("[subscription-worker] error loading trial subscriptions: %v", err)
		return
	}

	for _, sub := range trialSubs {
		if sub.Plan.IsTrial && sub.StartedAt.AddDate(0, 0, 14).Before(now) {
			if err := w.db.WithContext(ctx).Model(&sub).Update("status", "expired").Error; err != nil {
				log.Printf("[subscription-worker] error expiring trial subscription %d: %v", sub.ID, err)
			} else {
				log.Printf("[subscription-worker] marked trial subscription %d as expired", sub.ID)
			}
		}
	}
}
