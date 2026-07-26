package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

// evalContext holds a resolved (host, service, domain, pool, containerName) tuple for a single evaluation.
type evalContext struct {
	hostMachineID string
	hostname      string
	serviceID     *uint
	serviceName   string
	domain        string
	pool          string
	containerName string
}

// breachKey uniquely identifies a firing condition for dedup & resolution.
type breachKey struct {
	ruleID        uint
	thresholdID   uint
	hostMachineID string
	serviceID     uint // 0 means nil
	domain        string
	pool          string
}

var domainScopedTemplates = map[string]bool{
	"nginx_5xx":             true,
	"nginx_4xx":             true,
	"nginx_p99":             true,
	"nginx_auth_brute_force": true,
	"nginx_traffic_drop":    true,
	"apache_5xx":            true,
	"apache_4xx":            true,
	"apache_p99":            true,
}

var poolScopedTemplates = map[string]bool{
	"phpfpm_processes":           true,
	"phpfpm_queue_saturation":    true,
	"phpfpm_slow_requests":       true,
	"phpfpm_max_children_reached": true,
}

type AlertEngineService struct {
	alertRuleRepo *repository.AlertRuleRepository
	queryBuilder  *AlertQueryBuilder
	tenantRepo    *repository.TenantRepository
	discoveryRepo *repository.DiscoveryRepository
}

func NewAlertEngineService(
	alertRuleRepo *repository.AlertRuleRepository,
	queryBuilder *AlertQueryBuilder,
	tenantRepo *repository.TenantRepository,
	discoveryRepo *repository.DiscoveryRepository,
) *AlertEngineService {
	return &AlertEngineService{
		alertRuleRepo: alertRuleRepo,
		queryBuilder:  queryBuilder,
		tenantRepo:    tenantRepo,
		discoveryRepo: discoveryRepo,
	}
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

// evalCandidate is a (rule, threshold, host) tuple ready for evaluation.
type evalCandidate struct {
	rule      *domain.AlertRule
	threshold domain.AlertThreshold
	ec        evalContext
}

func (s *AlertEngineService) evaluate(ctx context.Context) {
	rules, err := s.alertRuleRepo.ListEnabledRulesCtx(ctx)
	if err != nil {
		log.Printf("[alert-engine] failed to list enabled rules: %v", err)
		return
	}

	activeBreaches := make(map[breachKey]bool)

	// Group rules by (template_id, service_type) — same metric identity.
	// Within a group, only the highest breached threshold fires per host.
	groups := make(map[string][]domain.AlertRule)
	for _, rule := range rules {
		key := rule.TemplateID + ":" + rule.ServiceType
		groups[key] = append(groups[key], rule)
	}

	for _, group := range groups {
		s.evaluateRuleGroup(ctx, group, activeBreaches)
	}

	s.resolveRecovered(ctx, activeBreaches)
}

// evaluateRuleGroup evaluates all rules that share the same metric
// (same template_id + service_type). For each host, only the highest
// breached threshold across all rules in the group creates an event.
func (s *AlertEngineService) evaluateRuleGroup(ctx context.Context, rules []domain.AlertRule, activeBreaches map[breachKey]bool) {
	// Collect all (rule, threshold, context) candidates.
	var candidates []evalCandidate
	for i := range rules {
		rule := &rules[i]
		contexts := s.resolveContexts(ctx, rule)
		if len(contexts) == 0 {
			continue
		}
		for _, ec := range contexts {
			for _, threshold := range rule.Thresholds {
				candidates = append(candidates, evalCandidate{
					rule:      rule,
					threshold: threshold,
					ec:        ec,
				})
			}
		}
	}
	if len(candidates) == 0 {
		return
	}

	// Group candidates by (host_machine_id, service_id, domain, pool) so we fetch the
	// metric once per host and pick a single winner across all rules.
	// Domain is critical for nginx/apache rules where each virtual host has its own metric.
	// Pool is critical for PHP-FPM rules where each pool has its own metric.
	type hostKey struct {
		machineID string
		serviceID uint
		domain    string
		pool      string
	}
	hostGroups := make(map[hostKey][]evalCandidate)
	for _, c := range candidates {
		svcID := uint(0)
		if c.ec.serviceID != nil {
			svcID = *c.ec.serviceID
		}
		key := hostKey{machineID: c.ec.hostMachineID, serviceID: svcID, domain: c.ec.domain, pool: c.ec.pool}
		hostGroups[key] = append(hostGroups[key], c)
	}

	for _, group := range hostGroups {
		first := group[0]

		// Pick the duration from the highest threshold — they should all
		// be the same metric so one fetch suffices.
		var bestDuration string
		var bestVal float64
		for _, c := range group {
			if c.threshold.Threshold > bestVal {
				bestVal = c.threshold.Threshold
				bestDuration = c.threshold.ForDuration
			}
		}
		if bestDuration == "" {
			bestDuration = first.threshold.ForDuration
		}

		currentValue, metricString := s.fetchMetric(ctx, first.rule, &first.ec, bestDuration)
		if currentValue == nil {
			continue
		}

		// Find the candidate (from any rule in the group) with the highest
		// breached threshold value.
		var winner *evalCandidate
		for _, c := range group {
			if s.isBreached(*currentValue, c.threshold.Threshold, c.rule.ComparisonOp) {
				if winner == nil || c.threshold.Threshold > winner.threshold.Threshold {
					winner = &c
				}
			}
		}

		svcIDVal := uint(0)
		if first.ec.serviceID != nil {
			svcIDVal = *first.ec.serviceID
		}

		// ── Resolve all firing events on this host for non-winning rules ──
		// We iterate over every RULE in the group that overlaps this host,
		// query its firing events, and resolve any that aren't the winner.
		evaluatedRuleIDs := make(map[uint]bool)
		for _, c := range group {
			if evaluatedRuleIDs[c.rule.ID] {
				continue
			}
			evaluatedRuleIDs[c.rule.ID] = true

			// If this rule is the winner's rule and the threshold matches,
			// don't resolve it. Otherwise, resolve all its host events.
			isWinningRule := winner != nil && c.rule.ID == winner.rule.ID

			firingEvents, err := s.alertRuleRepo.ListFiringEventsByRuleCtx(ctx, c.rule.ID)
			if err != nil {
				log.Printf("[alert-engine] list firing events for rule %d: %v", c.rule.ID, err)
				continue
			}
			for _, fe := range firingEvents {
				if fe.HostMachineID != first.ec.hostMachineID || fe.Status != "firing" {
					continue
				}
				matchesService := (first.ec.serviceID == nil && fe.ServiceID == nil) ||
					(first.ec.serviceID != nil && fe.ServiceID != nil && *first.ec.serviceID == *fe.ServiceID)
				if !matchesService {
					continue
				}
				matchesDomain := fe.Domain == first.ec.domain
				if !matchesDomain {
					continue
				}
				matchesPool := fe.Pool == first.ec.pool
				if !matchesPool {
					continue
				}

				if isWinningRule && fe.ThresholdID != nil && *fe.ThresholdID == winner.threshold.ID {
					// This IS the winning event — keep it alive.
					continue
				}

				// Every other event on this host for this metric gets resolved.
				if err := s.alertRuleRepo.ResolveEventCtx(ctx, fe.ID); err != nil {
					log.Printf("[alert-engine] resolve cross-rule event %d: %v", fe.ID, err)
				} else {
					log.Printf("[alert-engine] CROSS-RULE RESOLVED event %d for rule %d (superseded by rule %d)",
						fe.ID, c.rule.ID, winner.rule.ID)
				}
			}
		}

		// ── Record the winning breach ──
		if winner != nil {
			key := breachKey{
				ruleID:        winner.rule.ID,
				thresholdID:   winner.threshold.ID,
				hostMachineID: first.ec.hostMachineID,
				serviceID:     svcIDVal,
				domain:        first.ec.domain,
				pool:          first.ec.pool,
			}
			activeBreaches[key] = true

			// Create event for winner if not already firing.
			alreadyFiring := false
			firingW, fErr := s.alertRuleRepo.ListFiringEventsByRuleCtx(ctx, winner.rule.ID)
			if fErr == nil {
				for _, fe := range firingW {
					if fe.HostMachineID == first.ec.hostMachineID && fe.Status == "firing" {
						matchesService := (first.ec.serviceID == nil && fe.ServiceID == nil) ||
							(first.ec.serviceID != nil && fe.ServiceID != nil && *first.ec.serviceID == *fe.ServiceID)
						if !matchesService {
							continue
						}
						if fe.Domain != first.ec.domain {
							continue
						}
						if fe.Pool != first.ec.pool {
							continue
						}
						if fe.ThresholdID != nil && *fe.ThresholdID == winner.threshold.ID {
							alreadyFiring = true
							break
						}
					}
				}
			}

			if !alreadyFiring {
				eventMetricString := metricString + fmt.Sprintf(" threshold=%.2f%s", winner.threshold.Threshold, winner.rule.Unit)
				event := &domain.AlertEvent{
					RuleID:        winner.rule.ID,
					ThresholdID:   &winner.threshold.ID,
					TenantID:      winner.rule.TenantID,
					HostMachineID: first.ec.hostMachineID,
					ServiceID:     first.ec.serviceID,
					Domain:        first.ec.domain,
					Pool:          first.ec.pool,
					Status:        "firing",
					TriggeredAt:   time.Now(),
					ActualValue:   currentValue,
					MetricString:  eventMetricString,
				}
				if err := s.alertRuleRepo.CreateAlertEventCtx(ctx, event); err != nil {
					log.Printf("[alert-engine] failed to create event for rule %d: %v", winner.rule.ID, err)
				} else {
					log.Printf("[alert-engine] FIRED rule=%q template=%q threshold=%.2f value=%.2f host=%s service=%s domain=%s pool=%s",
						winner.rule.TemplateName, winner.rule.TemplateID, winner.threshold.Threshold,
						*currentValue, first.ec.hostname, first.ec.serviceName, first.ec.domain, first.ec.pool)
				}
			}
		}
		// If winner == nil, no threshold breached on this host.
		// All firing events for this host were already resolved above.
	}
}

// expandEvalContexts returns the evalContexts for a service,
// expanding by domain or pool when the rule has no specific value.
func (s *AlertEngineService) expandEvalContexts(ctx context.Context, rule *domain.AlertRule, svc *domain.DiscoveredService) []evalContext {
	host, _ := s.alertRuleRepo.FindHostByMachineIDCtx(ctx, svc.MachineID, rule.TenantID)
	hostname := ""
	if host != nil {
		hostname = host.Hostname
	}
	svcID := svc.ID

	// Base context: domain & pool come from rule (could be "" meaning "all")
	base := evalContext{
		hostMachineID: svc.MachineID,
		hostname:      hostname,
		serviceID:     &svcID,
		serviceName:   svc.ServiceName,
	}

	if domainScopedTemplates[rule.TemplateID] && rule.Domain == "" {
		// "All domains" — enumerate enabled vhosts from DB.
		var domains []string
		if svc.ServiceName == "nginx" {
			rows, err := s.discoveryRepo.ListVirtualHostsByServiceCtx(ctx, rule.TenantID, svc.ID)
			if err == nil {
				seen := map[string]bool{}
				for _, row := range rows {
					if row.Enabled {
						seen[row.Domain] = true
					}
				}
				for d := range seen {
					domains = append(domains, d)
				}
			}
		} else if svc.ServiceName == "apache" {
			rows, err := s.discoveryRepo.ListApacheVirtualHostsByServiceCtx(ctx, rule.TenantID, svc.ID)
			if err == nil {
				seen := map[string]bool{}
				for _, row := range rows {
					if row.Enabled {
						seen[row.Domain] = true
					}
				}
				for d := range seen {
					domains = append(domains, d)
				}
			}
		}
		if len(domains) == 0 {
			log.Printf("[alert-engine] no enabled vhosts for rule %d service %q — skipping domain expansion",
				rule.ID, svc.ServiceName)
			return nil
		}
		result := make([]evalContext, 0, len(domains))
		for _, d := range domains {
			ec := base
			ec.domain = d
			result = append(result, ec)
		}
		return result
	}

	if poolScopedTemplates[rule.TemplateID] && rule.Pool == "" {
		// "All pools" — discover pools from VM metrics.
		pools, err := s.queryBuilder.DiscoverPHPFPMPools(ctx, rule.TenantID, svc.MachineID)
		if err != nil || len(pools) == 0 {
			log.Printf("[alert-engine] no pools discovered for rule %d service %q (err=%v) — skipping pool expansion",
				rule.ID, svc.ServiceName, err)
			return nil
		}
		result := make([]evalContext, 0, len(pools))
		for _, p := range pools {
			ec := base
			ec.pool = p
			result = append(result, ec)
		}
		return result
	}

	// No expansion needed — use rule.Domain/rule.Pool directly.
	base.domain = rule.Domain
	base.pool = rule.Pool
	return []evalContext{base}
}

// resolveContexts expands the rule's scopes into concrete evalContexts.
func (s *AlertEngineService) resolveContexts(ctx context.Context, rule *domain.AlertRule) []evalContext {
	isHostTemplate := rule.ServiceType == "host"
	var contexts []evalContext

	for _, scope := range rule.Scopes {
		switch scope.ScopeType {
		case "all_hosts":
			if !isHostTemplate {
				continue
			}
			hosts, err := s.alertRuleRepo.ListHostsByTenantCtx(ctx, rule.TenantID)
			if err != nil {
				log.Printf("[alert-engine] list hosts for tenant %d: %v", rule.TenantID, err)
				continue
			}
			for _, h := range hosts {
				contexts = append(contexts, evalContext{
					hostMachineID: h.MachineID,
					hostname:      h.Hostname,
				})
			}

		case "host":
			if !isHostTemplate {
				continue
			}
			if scope.ScopeID == nil {
				continue
			}
			host, err := s.alertRuleRepo.FindHostByIDCtx(ctx, *scope.ScopeID, rule.TenantID)
			if err != nil {
				log.Printf("[alert-engine] find host %d: %v", *scope.ScopeID, err)
				continue
			}
			if host == nil {
				continue
			}
			contexts = append(contexts, evalContext{
				hostMachineID: host.MachineID,
				hostname:      host.Hostname,
			})

		case "all_services_of_type":
			if isHostTemplate {
				continue
			}
			services, err := s.alertRuleRepo.ListServicesByTypeCtx(ctx, rule.TenantID, rule.ServiceType)
			if err != nil {
				log.Printf("[alert-engine] list services type %q: %v", rule.ServiceType, err)
				continue
			}
			for _, svc := range services {
				contexts = append(contexts, s.expandEvalContexts(ctx, rule, &svc)...)
			}

		case "service":
			if isHostTemplate {
				continue
			}
			if scope.ScopeID == nil {
				continue
			}
			svc, err := s.alertRuleRepo.FindServiceByIDCtx(ctx, *scope.ScopeID, rule.TenantID)
			if err != nil {
				log.Printf("[alert-engine] find service %d: %v", *scope.ScopeID, err)
				continue
			}
			if svc == nil {
				continue
			}
			contexts = append(contexts, s.expandEvalContexts(ctx, rule, svc)...)

		case "all_containers":
			if isHostTemplate {
				continue
			}
			containers, err := s.discoveryRepo.ListContainerServicesByTenantCtx(ctx, rule.TenantID)
			if err != nil {
				log.Printf("[alert-engine] list containers for tenant %d: %v", rule.TenantID, err)
				continue
			}
			for _, c := range containers {
				host, _ := s.alertRuleRepo.FindHostByMachineIDCtx(ctx, c.MachineID, rule.TenantID)
				hostname := ""
				if host != nil {
					hostname = host.Hostname
				}
				contexts = append(contexts, evalContext{
					hostMachineID: c.MachineID,
					hostname:      hostname,
					containerName: c.ContainerName,
				})
			}

		case "container":
			if isHostTemplate {
				continue
			}
			if scope.ScopeID == nil {
				continue
			}
			container, err := s.discoveryRepo.FindContainerServiceByIDTenantCtx(ctx, rule.TenantID, *scope.ScopeID)
			if err != nil {
				log.Printf("[alert-engine] find container %d: %v", *scope.ScopeID, err)
				continue
			}
			if container == nil {
				continue
			}
			host, _ := s.alertRuleRepo.FindHostByMachineIDCtx(ctx, container.MachineID, rule.TenantID)
			hostname := ""
			if host != nil {
				hostname = host.Hostname
			}
			contexts = append(contexts, evalContext{
				hostMachineID: container.MachineID,
				hostname:      hostname,
				containerName: container.ContainerName,
			})
		}
	}
	return contexts
}

// fetchMetric builds the appropriate query and executes it, returning the
// current metric value and a human-readable description string.
func (s *AlertEngineService) fetchMetric(ctx context.Context, rule *domain.AlertRule, ec *evalContext, forDuration string) (*float64, string) {
	bq, err := s.queryBuilder.BuildQuery(rule.TemplateID, ec.hostMachineID, ec.serviceName, ec.domain, ec.pool, ec.containerName, forDuration)
	if err != nil {
		log.Printf("[alert-engine] build query for rule %d: %v", rule.ID, err)
		return nil, ""
	}

	var value *float64
	switch bq.QueryType {
	case QueryTypePromQL:
		value, err = s.queryBuilder.ExecutePromQL(ctx, rule.TenantID, bq.Query)
	case QueryTypeLogQL:
		tenant, tErr := s.tenantRepo.FindByIDCtx(ctx, rule.TenantID)
		if tErr != nil || tenant == nil {
			log.Printf("[alert-engine] tenant lookup for %d: %v", rule.TenantID, tErr)
			return nil, ""
		}
		value, err = s.queryBuilder.ExecuteLogQL(ctx, tenant.UUID, bq.Query)
	}

	if err != nil {
		log.Printf("[alert-engine] query exec for rule %d: %v", rule.ID, err)
		return nil, ""
	}

	if value == nil {
		return nil, ""
	}

	metricString := fmt.Sprintf("host=%s", ec.hostname)
	if ec.serviceName != "" {
		metricString += fmt.Sprintf(" service=%s", ec.serviceName)
	}
	if ec.domain != "" {
		metricString += fmt.Sprintf(" domain=%s", ec.domain)
	}
	if ec.pool != "" {
		metricString += fmt.Sprintf(" pool=%s", ec.pool)
	}
	if ec.containerName != "" {
		metricString += fmt.Sprintf(" container=%s", ec.containerName)
	}
	metricString += fmt.Sprintf(" value=%.2f%s", *value, rule.Unit)

	return value, metricString
}

func (s *AlertEngineService) isBreached(currentValue, threshold float64, comparisonOp string) bool {
	switch comparisonOp {
	case ">":
		return currentValue > threshold
	case ">=":
		return currentValue >= threshold
	case "<":
		return currentValue < threshold
	case "<=":
		return currentValue <= threshold
	case "==":
		return currentValue == threshold
	default:
		return currentValue > threshold
	}
}

// resolveRecovered resolves firing events for conditions that are no longer breached.
func (s *AlertEngineService) resolveRecovered(ctx context.Context, activeBreaches map[breachKey]bool) {
	ruleIDs := make(map[uint]bool)
	for key := range activeBreaches {
		ruleIDs[key.ruleID] = true
	}

	for ruleID := range ruleIDs {
		events, err := s.alertRuleRepo.ListFiringEventsByRuleCtx(ctx, ruleID)
		if err != nil {
			log.Printf("[alert-engine] list firing events for rule %d: %v", ruleID, err)
			continue
		}
		for _, ev := range events {
			svcID := uint(0)
			if ev.ServiceID != nil {
				svcID = *ev.ServiceID
			}
			threshID := uint(0)
			if ev.ThresholdID != nil {
				threshID = *ev.ThresholdID
			}
			key := breachKey{
				ruleID:        ev.RuleID,
				thresholdID:   threshID,
				hostMachineID: ev.HostMachineID,
				serviceID:     svcID,
				domain:        ev.Domain,
				pool:          ev.Pool,
			}
			if !activeBreaches[key] {
				if err := s.alertRuleRepo.ResolveEventCtx(ctx, ev.ID); err != nil {
					log.Printf("[alert-engine] resolve recovered event %d: %v", ev.ID, err)
				} else {
					log.Printf("[alert-engine] AUTO-RESOLVED event %d for rule %d (recovered)", ev.ID, ruleID)
				}
			}
		}
	}
}
