package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type NotificationDispatcher struct {
	notifRepo     *repository.NotificationChannelRepository
	alertRuleRepo *repository.AlertRuleRepository
	emailSender   *EmailSender
	platformURL   string
}

func NewNotificationDispatcher(notifRepo *repository.NotificationChannelRepository, alertRuleRepo *repository.AlertRuleRepository, emailSender *EmailSender, platformURL string) *NotificationDispatcher {
	return &NotificationDispatcher{
		notifRepo:     notifRepo,
		alertRuleRepo: alertRuleRepo,
		emailSender:   emailSender,
		platformURL:   platformURL,
	}
}

func (d *NotificationDispatcher) DispatchAlertEvent(ctx context.Context, ruleID, tenantID uint, ruleName, serviceType, severity, description string) {
	if !d.emailSender.IsEnabled() {
		return
	}

	channels, err := d.notifRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		log.Printf("[notif] failed to list channels for tenant %d: %v", tenantID, err)
		return
	}

	dashboardURL := fmt.Sprintf("%s/alerts", d.platformURL)

	for _, ch := range channels {
		if ch.Type != "email" || !ch.IsVerified {
			continue
		}

		subject := fmt.Sprintf("[%s] Alert: %s", severity, ruleName)
		body := d.emailSender.AlertFiredHTML(ruleName, serviceType, severity, description, dashboardURL)

		if err := d.emailSender.Send(ch.Target, subject, body); err != nil {
			log.Printf("[notif] failed to send alert email to %s: %v", ch.Target, err)
		} else {
			log.Printf("[notif] alert email sent to %s for rule %d", ch.Target, ruleID)
		}
	}
}

func (d *NotificationDispatcher) StartNotificationWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.processPending(ctx)
		}
	}
}

func (d *NotificationDispatcher) processPending(ctx context.Context) {
	events, err := d.alertRuleRepo.FindUnnotifiedEventsCtx(ctx)
	if err != nil {
		log.Printf("[notif] failed to find unnotified events: %v", err)
		return
	}

	for _, ev := range events {
		ruleName := ev.Rule.TemplateName
		if ruleName == "" {
			ruleName = "Alert Rule"
		}
		serviceType := ev.Rule.ServiceType
		description := ev.Rule.Description

		d.DispatchAlertEvent(ctx, ev.RuleID, ev.TenantID, ruleName, serviceType, ev.Status, description)

		if err := d.alertRuleRepo.MarkEventNotifiedCtx(ctx, ev.ID); err != nil {
			log.Printf("[notif] failed to mark event %d notified: %v", ev.ID, err)
		}
	}
}
