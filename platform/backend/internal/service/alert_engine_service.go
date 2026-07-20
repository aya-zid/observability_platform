package service

import (
	"context"
	"log"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type AlertEngineService struct {
	alertRuleRepo *repository.AlertRuleRepository
}

func NewAlertEngineService(alertRuleRepo *repository.AlertRuleRepository) *AlertEngineService {
	return &AlertEngineService{alertRuleRepo: alertRuleRepo}
}

func (s *AlertEngineService) StartEvaluator(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.evaluate(ctx)
		}
	}
}

func (s *AlertEngineService) evaluate(ctx context.Context) {
	rules, err := s.alertRuleRepo.ListEnabledRulesCtx(ctx)
	if err != nil {
		log.Printf("[alert-engine] failed to list enabled rules: %v", err)
		return
	}

	for _, rule := range rules {
		for _, scope := range rule.Scopes {
			hostMachineID := ""
			var serviceID *uint

			switch scope.ScopeType {
			case "host":
				if scope.ScopeID != nil {
					// In production: resolve host machine ID from scope.ScopeID
					// For PFE: skip host-specific hostMachineID resolution
				}
			case "service":
				if scope.ScopeID != nil {
					serviceID = scope.ScopeID
				}
			}

			for _, threshold := range rule.Thresholds {
				firing, err := s.alertRuleRepo.FindFiringEventCtx(ctx, rule.ID, hostMachineID, serviceID)
				if err != nil {
					log.Printf("[alert-engine] dedup check failed for rule %d scope %d: %v", rule.ID, scope.ID, err)
					continue
				}
				if firing != nil {
					continue
				}

				event := &domain.AlertEvent{
					RuleID:        rule.ID,
					ThresholdID:   &threshold.ID,
					TenantID:      rule.TenantID,
					HostMachineID: hostMachineID,
					ServiceID:     serviceID,
					Status:        "firing",
					TriggeredAt:   time.Now(),
				}
				if err := s.alertRuleRepo.CreateAlertEventCtx(ctx, event); err != nil {
					log.Printf("[alert-engine] failed to create event for rule %d: %v", rule.ID, err)
				}
			}
		}
	}
}
