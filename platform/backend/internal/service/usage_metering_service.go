package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type UsageMeteringService struct {
	redis     *redis.Client
	usageRepo *repository.UsageRepository
}

func NewUsageMeteringService(redis *redis.Client, usageRepo *repository.UsageRepository, _ *repository.PlanRepository, _ *repository.SubscriptionRepository) *UsageMeteringService {
	return &UsageMeteringService{
		redis:     redis,
		usageRepo: usageRepo,
	}
}

func (s *UsageMeteringService) RecordIngestion(ctx context.Context, tenantID uint, logBytes int64, metricSamples int64) {
	today := time.Now().Format("2006-01-02")
	key := fmt.Sprintf("usage:%d:%s", tenantID, today)

	pipe := s.redis.Pipeline()
	if logBytes > 0 {
		pipe.IncrBy(ctx, key+":log_bytes", logBytes)
	}
	if metricSamples > 0 {
		pipe.IncrBy(ctx, key+":metric_samples", metricSamples)
	}
	pipe.Expire(ctx, key+":log_bytes", 48*time.Hour)
	pipe.Expire(ctx, key+":metric_samples", 48*time.Hour)
	_, _ = pipe.Exec(ctx)
}

func (s *UsageMeteringService) FlushToPostgres(ctx context.Context) {
	iter := s.redis.Scan(ctx, 0, "usage:*:*:log_bytes", 100).Iterator()
	seen := map[string]bool{}

	for iter.Next(ctx) {
		key := iter.Val()
		parts := strings.Split(key, ":")
		if len(parts) < 4 {
			continue
		}
		tenantID := parts[1]
		date := parts[2]

		usageKey := fmt.Sprintf("usage:%s:%s", tenantID, date)
		if seen[usageKey] {
			continue
		}
		seen[usageKey] = true

		logBytes, _ := s.redis.Get(ctx, usageKey+":log_bytes").Int64()
		metricSamples, _ := s.redis.Get(ctx, usageKey+":metric_samples").Int64()

		if logBytes == 0 && metricSamples == 0 {
			continue
		}

		var tid uint
		fmt.Sscanf(tenantID, "%d", &tid)

		if err := s.usageRepo.UpsertDailyUsageCtx(ctx, &domain.TenantUsageDaily{
			TenantID:              tid,
			Date:                  date,
			LogBytesIngested:      logBytes,
			MetricSamplesIngested: metricSamples,
		}); err != nil {
			log.Printf("[usage] flush error for tenant=%s date=%s: %v", tenantID, date, err)
			continue
		}

		s.redis.Del(ctx, usageKey+":log_bytes", usageKey+":metric_samples")
	}
}
