package service

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type IncidentCategory string

const (
	CategoryPerformance IncidentCategory = "performance"
	CategorySecurity     IncidentCategory = "security"
)

type IncidentRole string

const (
	RoleRoot       IncidentRole = "root"
	RoleSymptom    IncidentRole = "symptom"
	RoleStandalone IncidentRole = "standalone"
)

type IncidentMember struct {
	RuleID         uint         `json:"rule_id"`
	TemplateID     string       `json:"template_id"`
	RuleName       string       `json:"rule_name"`
	ServiceName    string       `json:"service_name,omitempty"`
	Domain         string       `json:"domain,omitempty"`
	Pool           string       `json:"pool,omitempty"`
	Role           IncidentRole `json:"role"`
	Severity       string       `json:"severity"`
	ActualValue    *float64     `json:"actual_value,omitempty"`
	Status         string       `json:"status"`
	Since          time.Time    `json:"since"`
	ResolvedAt     *time.Time   `json:"resolved_at,omitempty"`
	InvestigateURL string       `json:"investigate_url"`
}

type IncidentCard struct {
	ID            string           `json:"id"`
	HostMachineID string           `json:"host_machine_id"`
	Hostname      string           `json:"hostname"`
	Category      IncidentCategory `json:"category"`
	CaseID        int              `json:"case_id"`
	Status        string           `json:"status"`
	Title         string           `json:"title"`
	Narrative     string           `json:"narrative"`
	Severity      string           `json:"severity"`
	Members       []IncidentMember `json:"members"`
	FiringSince   time.Time        `json:"firing_since"`
	ResolvedAt    *time.Time       `json:"resolved_at,omitempty"`
	Enrichments   []string         `json:"enrichments,omitempty"`
}

type IncidentGroupingService struct {
	alertRuleRepo  *repository.AlertRuleRepository
	hostRepo       *repository.HostRepository
	incidentRepo   *repository.IncidentRepository
}

func NewIncidentGroupingService(
	alertRuleRepo *repository.AlertRuleRepository,
	hostRepo *repository.HostRepository,
	incidentRepo *repository.IncidentRepository,
) *IncidentGroupingService {
	return &IncidentGroupingService{
		alertRuleRepo: alertRuleRepo,
		hostRepo:      hostRepo,
		incidentRepo:  incidentRepo,
	}
}

// GroupIncidents fetches alert events for a tenant (firing + resolved within last 24h)
// and groups them into catalog incident cards. Dismissed cards are excluded.
func (s *IncidentGroupingService) GroupIncidents(ctx context.Context, tenantID uint) ([]IncidentCard, error) {
	events, err := s.alertRuleRepo.ListRecentEventsByTenantCtx(ctx, tenantID, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("list alert events: %w", err)
	}

	if len(events) == 0 {
		return []IncidentCard{}, nil
	}

	// Host lookup map: MachineID -> Hostname
	hosts, _ := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	hostNames := make(map[string]string)
	for _, h := range hosts {
		hostNames[h.MachineID] = h.Hostname
	}

	var securityEvents []domain.AlertEvent
	var perfEvents []domain.AlertEvent

	for _, ev := range events {
		templateID := getEventTemplateID(ev)
		tier := getTier(templateID)
		if tier == TierSecurity {
			securityEvents = append(securityEvents, ev)
		} else {
			perfEvents = append(perfEvents, ev)
		}
	}

	cards := make([]IncidentCard, 0)

	// 1. Group SECURITY events by host
	secByHost := groupByHost(securityEvents)
	for machineID, hostEvents := range secByHost {
		hostname := resolveHostname(machineID, hostNames)
		cards = append(cards, s.groupSecurityHost(machineID, hostname, hostEvents)...)
	}

	// 2. Group PERFORMANCE events by host
	perfByHost := groupByHost(perfEvents)
	for machineID, hostEvents := range perfByHost {
		hostname := resolveHostname(machineID, hostNames)
		cards = append(cards, s.groupPerformanceHost(machineID, hostname, hostEvents)...)
	}

	// 3. Determine card status based on member event statuses
	for i := range cards {
		hasFiring := false
		var latestResolved *time.Time
		for _, m := range cards[i].Members {
			if m.Status == "firing" {
				hasFiring = true
			}
			if m.ResolvedAt != nil {
				if latestResolved == nil || m.ResolvedAt.After(*latestResolved) {
					latestResolved = m.ResolvedAt
				}
			}
		}
		if hasFiring {
			cards[i].Status = "firing"
		} else {
			cards[i].Status = "resolved"
			cards[i].ResolvedAt = latestResolved
		}
	}

	// 4. Filter out dismissed cards
	if s.incidentRepo != nil {
		dismissed, err := s.incidentRepo.ListDismissedIncidents(ctx, tenantID)
		if err == nil {
			dismissedSet := make(map[string]bool, len(dismissed))
			for _, id := range dismissed {
				dismissedSet[id] = true
			}
			filtered := make([]IncidentCard, 0, len(cards))
			for _, c := range cards {
				if !dismissedSet[c.ID] {
					filtered = append(filtered, c)
				}
			}
			cards = filtered
		}
	}

	// 5. Sort cards: Security first, then Performance by severity desc, then by FiringSince asc
	sortCards(cards)

	return cards, nil
}

// ─── Security Grouping ────────────────────────────────────────────────────────

func (s *IncidentGroupingService) groupSecurityHost(machineID, hostname string, events []domain.AlertEvent) []IncidentCard {
	var cards []IncidentCard

	var sshBrute *domain.AlertEvent
	var authFailures []domain.AlertEvent
	otherSec := make([]domain.AlertEvent, 0)

	for i := range events {
		ev := events[i]
		tID := getEventTemplateID(ev)
		if tID == "host_ssh_brute_force" {
			sshBrute = &ev
		} else if strings.HasSuffix(tID, "_auth_failures") || tID == "redis_auth_or_security" {
			authFailures = append(authFailures, ev)
		} else {
			otherSec = append(otherSec, ev)
		}
	}

	// Case 42 check: SSH brute force AND any DB/service auth failures on same host
	if sshBrute != nil && len(authFailures) > 0 {
		members := make([]IncidentMember, 0)
		allEvs := append([]domain.AlertEvent{*sshBrute}, authFailures...)
		earliest, maxSev := summarizeEvents(allEvs, RoleStandalone, &members)

		entry := incidentCatalog[42]
		title := strings.ReplaceAll(entry.title, "{host}", hostname)

		cards = append(cards, IncidentCard{
			ID:            fmt.Sprintf("sec-%s-case42", machineID),
			HostMachineID: machineID,
			Hostname:      hostname,
			Category:      CategorySecurity,
			CaseID:        42,
			Title:         title,
			Narrative:     entry.narrative,
			Severity:      maxSev,
			Members:       members,
			FiringSince:   earliest,
		})
	} else {
		// Standalone SSH brute force
		if sshBrute != nil {
			var members []IncidentMember
			earliest, maxSev := summarizeEvents([]domain.AlertEvent{*sshBrute}, RoleStandalone, &members)
			entry := incidentCatalog[37]
			title := strings.ReplaceAll(entry.title, "{host}", hostname)
			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("sec-%s-case37", machineID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategorySecurity,
				CaseID:        37,
				Title:         title,
				Narrative:     entry.narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
			})
		}
		// Standalone DB auth failures
		for _, ev := range authFailures {
			tID := getEventTemplateID(ev)
			caseID := 38
			switch tID {
			case "nginx_auth_brute_force", "apache_auth_brute_force":
				caseID = 38
			case "postgres_auth_failures":
				caseID = 39
			case "mariadb_auth_failures":
				caseID = 40
			case "redis_auth_or_security":
				caseID = 41
			}
			var members []IncidentMember
			earliest, maxSev := summarizeEvents([]domain.AlertEvent{ev}, RoleStandalone, &members)
			entry := incidentCatalog[caseID]
			title := entry.title
			if ev.Domain != "" {
				title = strings.ReplaceAll(title, "{domain}", ev.Domain)
			}
			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("sec-%s-case%d-%d", machineID, caseID, ev.ID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategorySecurity,
				CaseID:        caseID,
				Title:         title,
				Narrative:     entry.narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
			})
		}
	}

	for _, ev := range otherSec {
		tID := getEventTemplateID(ev)
		caseID := 38
		if tID == "nginx_auth_brute_force" || tID == "apache_auth_brute_force" {
			caseID = 38
		}
		var members []IncidentMember
		earliest, maxSev := summarizeEvents([]domain.AlertEvent{ev}, RoleStandalone, &members)
		entry := incidentCatalog[caseID]
		title := entry.title
		if ev.Domain != "" {
			title = strings.ReplaceAll(title, "{domain}", ev.Domain)
		}
		cards = append(cards, IncidentCard{
			ID:            fmt.Sprintf("sec-%s-case%d-%d", machineID, caseID, ev.ID),
			HostMachineID: machineID,
			Hostname:      hostname,
			Category:      CategorySecurity,
			CaseID:        caseID,
			Title:         title,
			Narrative:     entry.narrative,
			Severity:      maxSev,
			Members:       members,
			FiringSince:   earliest,
		})
	}

	return cards
}

// ─── Performance Grouping ─────────────────────────────────────────────────────

func (s *IncidentGroupingService) groupPerformanceHost(machineID, hostname string, events []domain.AlertEvent) []IncidentCard {
	var cards []IncidentCard

	// Classify events by tier
	byTier := make(map[IncidentTier][]domain.AlertEvent)
	for _, ev := range events {
		tID := getEventTemplateID(ev)
		tier := getTier(tID)
		byTier[tier] = append(byTier[tier], ev)
	}

	// Check HOST_FAULT events for enrichments
	hostFaultEvs := byTier[TierHostFault]
	enrichments := make([]string, 0)
	consumedHostFaults := make(map[uint]bool)

	// Step a: HOST tier root
	if len(byTier[TierHost]) > 0 {
		hostEvs := byTier[TierHost]
		hostTemplates := make(map[string]bool)
		for _, ev := range hostEvs {
			hostTemplates[getEventTemplateID(ev)] = true
		}

		otherTiersFiring := len(byTier[TierData]) > 0 || len(byTier[TierApp]) > 0 || len(byTier[TierWeb]) > 0 || len(byTier[TierContainer]) > 0

		caseID := selectHostCase(hostTemplates, otherTiersFiring)

		// Collect host-fault enrichments
		for _, ev := range hostFaultEvs {
			tID := getEventTemplateID(ev)
			if note, ok := hostFaultEnrichment[tID]; ok {
				enrichments = append(enrichments, note)
				consumedHostFaults[ev.ID] = true
			}
		}

		members := make([]IncidentMember, 0)
		earliest, maxSev := summarizeEvents(hostEvs, RoleRoot, &members)

		// Absorb other firing tiers as symptoms
		for _, tier := range []IncidentTier{TierData, TierApp, TierWeb, TierContainer} {
			if len(byTier[tier]) > 0 {
				e, mSev := summarizeEvents(byTier[tier], RoleSymptom, &members)
				if e.Before(earliest) {
					earliest = e
				}
				if mSev == "critical" {
					maxSev = "critical"
				}
			}
		}

		entry := incidentCatalog[caseID]
		title := strings.ReplaceAll(entry.title, "{Host}", hostname)
		title = strings.ReplaceAll(title, "{host}", hostname)

		cards = append(cards, IncidentCard{
			ID:            fmt.Sprintf("perf-%s-case%d", machineID, caseID),
			HostMachineID: machineID,
			Hostname:      hostname,
			Category:      CategoryPerformance,
			CaseID:        caseID,
			Title:         title,
			Narrative:     entry.narrative,
			Severity:      maxSev,
			Members:       members,
			FiringSince:   earliest,
			Enrichments:   enrichments,
		})

		// Process unconsumed host faults as standalone
		cards = append(cards, s.standaloneHostFaultCards(machineID, hostname, hostFaultEvs, consumedHostFaults)...)
		return cards
	}

	// Consume host-fault enrichments if any host fault occurred
	for _, ev := range hostFaultEvs {
		tID := getEventTemplateID(ev)
		if note, ok := hostFaultEnrichment[tID]; ok {
			enrichments = append(enrichments, note)
			consumedHostFaults[ev.ID] = true
		}
	}

	// Step b: DATA tier root
	if len(byTier[TierData]) > 0 {
		dataEvs := byTier[TierData]

		// Group DATA events by service kind (postgres, mariadb, redis)
		dataByKind := make(map[string][]domain.AlertEvent)
		for _, ev := range dataEvs {
			kind := templateServiceKind(getEventTemplateID(ev))
			if kind != "" {
				dataByKind[kind] = append(dataByKind[kind], ev)
			}
		}

		// Determine symptom set
		appEvs := byTier[TierApp]
		webEvs := byTier[TierWeb]
		symptoms := symptomNone
		if len(appEvs) > 0 && len(webEvs) > 0 {
			symptoms = symptomAppWeb
		} else if len(appEvs) > 0 {
			symptoms = symptomApp
		} else if len(webEvs) > 0 {
			symptoms = symptomWeb
		}

		// Check for Case 45: Both Postgres and MariaDB firing
		if len(dataByKind["postgres"]) > 0 && len(dataByKind["mariadb"]) > 0 {
			members := make([]IncidentMember, 0)
			earliest, maxSev := summarizeEvents(dataEvs, RoleRoot, &members)
			if len(appEvs) > 0 {
				e, ms := summarizeEvents(appEvs, RoleSymptom, &members)
				if e.Before(earliest) {
					earliest = e
				}
				if ms == "critical" {
					maxSev = "critical"
				}
			}
			if len(webEvs) > 0 {
				e, ms := summarizeEvents(webEvs, RoleSymptom, &members)
				if e.Before(earliest) {
					earliest = e
				}
				if ms == "critical" {
					maxSev = "critical"
				}
			}

			entry := incidentCatalog[45]
			title := strings.ReplaceAll(entry.title, "{host}", hostname)
			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("perf-%s-case45", machineID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategoryPerformance,
				CaseID:        45,
				Title:         title,
				Narrative:     entry.narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
				Enrichments:   enrichments,
			})
		} else {
			// Check for Case 19 co-root: Redis cache_hit + any DB also firing on same host
			redisHasSlowIO := false
			if rEvs, ok := dataByKind["redis"]; ok {
				for _, ev := range rEvs {
					if getFlavor(getEventTemplateID(ev)) == FlavorSlowIO {
						redisHasSlowIO = true
						break
					}
				}
			}
			dbFiring := len(dataByKind["postgres"]) > 0 || len(dataByKind["mariadb"]) > 0
			isCoRoot := redisHasSlowIO && dbFiring

			if isCoRoot {
				// Single merged card: Redis + DB co-root
				dbName := "database"
				if len(dataByKind["postgres"]) > 0 {
					dbName = "Postgres"
				} else if len(dataByKind["mariadb"]) > 0 {
					dbName = "MariaDB"
				}

				allRootEvs := make([]domain.AlertEvent, 0)
				allRootEvs = append(allRootEvs, dataByKind["redis"]...)
				allRootEvs = append(allRootEvs, dataByKind["postgres"]...)
				allRootEvs = append(allRootEvs, dataByKind["mariadb"]...)

				members := make([]IncidentMember, 0)
				earliest, maxSev := summarizeEvents(allRootEvs, RoleRoot, &members)
				if len(appEvs) > 0 {
					e, ms := summarizeEvents(appEvs, RoleSymptom, &members)
					if e.Before(earliest) {
						earliest = e
					}
					if ms == "critical" {
						maxSev = "critical"
					}
				}
				if len(webEvs) > 0 {
					e, ms := summarizeEvents(webEvs, RoleSymptom, &members)
					if e.Before(earliest) {
						earliest = e
					}
					if ms == "critical" {
						maxSev = "critical"
					}
				}

				entry := incidentCatalog[19]
				title := strings.ReplaceAll(entry.title, "{DBName}", dbName)

				cards = append(cards, IncidentCard{
					ID:            fmt.Sprintf("perf-%s-coroot-case19", machineID),
					HostMachineID: machineID,
					Hostname:      hostname,
					Category:      CategoryPerformance,
					CaseID:        19,
					Title:         title,
					Narrative:     entry.narrative,
					Severity:      maxSev,
					Members:       members,
					FiringSince:   earliest,
					Enrichments:   enrichments,
				})
			} else {
				// Single DB or Redis — one card per service kind
				for kind, kEvs := range dataByKind {
					flavors := make(map[IncidentFlavor]bool)
					for _, ev := range kEvs {
						flavors[getFlavor(getEventTemplateID(ev))] = true
					}
					winningFlavor := pickWinningFlavor(kind, flavors)

					caseID := 0
					switch kind {
					case "postgres":
						caseID = selectPostgresCase(winningFlavor, symptoms)
					case "mariadb":
						caseID = selectMariaDBCase(winningFlavor, symptoms)
					case "redis":
						caseID = selectRedisCase(winningFlavor, symptoms)
					}

					members := make([]IncidentMember, 0)
					earliest, maxSev := summarizeEvents(kEvs, RoleRoot, &members)
					if len(appEvs) > 0 {
						e, ms := summarizeEvents(appEvs, RoleSymptom, &members)
						if e.Before(earliest) {
							earliest = e
						}
						if ms == "critical" {
							maxSev = "critical"
						}
					}
					if len(webEvs) > 0 {
						e, ms := summarizeEvents(webEvs, RoleSymptom, &members)
						if e.Before(earliest) {
							earliest = e
						}
						if ms == "critical" {
							maxSev = "critical"
						}
					}

					var title, narrative string
					if entry, ok := incidentCatalog[caseID]; ok && caseID != 0 {
						title = entry.title
						narrative = entry.narrative
					} else {
						// Fallback builder (§7)
						title, narrative = buildFallbackTitle(kind, winningFlavor, kEvs, appEvs, webEvs, hostname)
					}

					cards = append(cards, IncidentCard{
						ID:            fmt.Sprintf("perf-%s-%s-case%d", machineID, kind, caseID),
						HostMachineID: machineID,
						Hostname:      hostname,
						Category:      CategoryPerformance,
						CaseID:        caseID,
						Title:         title,
						Narrative:     narrative,
						Severity:      maxSev,
						Members:       members,
						FiringSince:   earliest,
						Enrichments:   enrichments,
					})
				}
			}
		}

		cards = append(cards, s.standaloneHostFaultCards(machineID, hostname, hostFaultEvs, consumedHostFaults)...)
		return cards
	}

	// Step c: APP tier root (PHP-FPM firing, no DATA tier)
	if len(byTier[TierApp]) > 0 {
		appEvs := byTier[TierApp]
		webEvs := byTier[TierWeb]
		webFiring := len(webEvs) > 0

		flavors := make(map[IncidentFlavor]bool)
		for _, ev := range appEvs {
			flavors[getFlavor(getEventTemplateID(ev))] = true
		}
		winningFlavor := pickWinningFlavor("phpfpm", flavors)
		caseID := selectPHPFPMCase(winningFlavor, webFiring)

		members := make([]IncidentMember, 0)
		earliest, maxSev := summarizeEvents(appEvs, RoleRoot, &members)
		if webFiring {
			e, ms := summarizeEvents(webEvs, RoleSymptom, &members)
			if e.Before(earliest) {
				earliest = e
			}
			if ms == "critical" {
				maxSev = "critical"
			}
		}

		title := incidentCatalog[caseID].title
		narrative := incidentCatalog[caseID].narrative
		if caseID == 0 {
			title = "PHP-FPM performance degradation detected"
			narrative = "PHP-FPM worker pools are under pressure on this host."
		}

		cards = append(cards, IncidentCard{
			ID:            fmt.Sprintf("perf-%s-phpfpm-case%d", machineID, caseID),
			HostMachineID: machineID,
			Hostname:      hostname,
			Category:      CategoryPerformance,
			CaseID:        caseID,
			Title:         title,
			Narrative:     narrative,
			Severity:      maxSev,
			Members:       members,
			FiringSince:   earliest,
			Enrichments:   enrichments,
		})

		cards = append(cards, s.standaloneHostFaultCards(machineID, hostname, hostFaultEvs, consumedHostFaults)...)
		return cards
	}

	// Step d: WEB tier root (only WEB firing, no DATA or APP)
	if len(byTier[TierWeb]) > 0 {
		webEvs := byTier[TierWeb]

		// Check Case 46: 2+ nginx_5xx/4xx for different domains
		domainSet := make(map[string]bool)
		for _, ev := range webEvs {
			if ev.Domain != "" {
				domainSet[ev.Domain] = true
			}
		}

		if len(domainSet) >= 2 {
			members := make([]IncidentMember, 0)
			earliest, maxSev := summarizeEvents(webEvs, RoleStandalone, &members)
			entry := incidentCatalog[46]
			domainList := make([]string, 0)
			for d := range domainSet {
				domainList = append(domainList, d)
			}
			narrative := strings.ReplaceAll(entry.narrative, "{domains}", strings.Join(domainList, ", "))
			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("perf-%s-case46", machineID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategoryPerformance,
				CaseID:        46,
				Title:         entry.title,
				Narrative:     narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
				Enrichments:   enrichments,
			})
		} else {
			flavors := make(map[IncidentFlavor]bool)
			for _, ev := range webEvs {
				flavors[getFlavor(getEventTemplateID(ev))] = true
			}
			winningFlavor := pickWinningFlavor("web", flavors)
			caseID := selectWebCase(winningFlavor)

			members := make([]IncidentMember, 0)
			earliest, maxSev := summarizeEvents(webEvs, RoleStandalone, &members)

			entry := incidentCatalog[caseID]
			domainName := "your web server"
			for _, ev := range webEvs {
				if ev.Domain != "" {
					domainName = ev.Domain
					break
				}
			}

			title := strings.ReplaceAll(entry.title, "{domain}", domainName)

			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("perf-%s-web-case%d", machineID, caseID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategoryPerformance,
				CaseID:        caseID,
				Title:         title,
				Narrative:     entry.narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
				Enrichments:   enrichments,
			})
		}

		cards = append(cards, s.standaloneHostFaultCards(machineID, hostname, hostFaultEvs, consumedHostFaults)...)
		return cards
	}

	// Step e: CONTAINER tier standalone
	if len(byTier[TierContainer]) > 0 {
		for _, ev := range byTier[TierContainer] {
			tID := getEventTemplateID(ev)
			caseID := 43
			if tID == "container_memory" {
				caseID = 44
			}

			containerName := ev.ContainerName
			if containerName == "" {
				containerName = "unknown"
			}

			var members []IncidentMember
			earliest, maxSev := summarizeEvents([]domain.AlertEvent{ev}, RoleStandalone, &members)

			entry := incidentCatalog[caseID]
			title := strings.ReplaceAll(entry.title, "{name}", containerName)
			title = strings.ReplaceAll(title, "{container}", containerName)

			cards = append(cards, IncidentCard{
				ID:            fmt.Sprintf("perf-%s-container-case%d-%d", machineID, caseID, ev.ID),
				HostMachineID: machineID,
				Hostname:      hostname,
				Category:      CategoryPerformance,
				CaseID:        caseID,
				Title:         title,
				Narrative:     entry.narrative,
				Severity:      maxSev,
				Members:       members,
				FiringSince:   earliest,
				Enrichments:   enrichments,
			})
		}
	}

	// Standalone host fault cards if any remain unconsumed
	cards = append(cards, s.standaloneHostFaultCards(machineID, hostname, hostFaultEvs, consumedHostFaults)...)

	return cards
}

func (s *IncidentGroupingService) standaloneHostFaultCards(machineID, hostname string, events []domain.AlertEvent, consumed map[uint]bool) []IncidentCard {
	var cards []IncidentCard
	for _, ev := range events {
		if consumed[ev.ID] {
			continue
		}
		tID := getEventTemplateID(ev)
		entry, ok := hostFaultStandaloneTitle[tID]
		if !ok {
			continue
		}

		var members []IncidentMember
		earliest, maxSev := summarizeEvents([]domain.AlertEvent{ev}, RoleStandalone, &members)
		title := strings.ReplaceAll(entry.title, "{host}", hostname)
		title = strings.ReplaceAll(title, "{service}", "service")

		cards = append(cards, IncidentCard{
			ID:            fmt.Sprintf("fault-%s-%s-%d", machineID, tID, ev.ID),
			HostMachineID: machineID,
			Hostname:      hostname,
			Category:      CategoryPerformance,
			CaseID:        0,
			Title:         title,
			Narrative:     entry.narrative,
			Severity:      maxSev,
			Members:       members,
			FiringSince:   earliest,
		})
	}
	return cards
}

// ─── Helper Functions ─────────────────────────────────────────────────────────

func getEventTemplateID(ev domain.AlertEvent) string {
	if ev.Rule != nil && ev.Rule.TemplateID != "" {
		return ev.Rule.TemplateID
	}
	return ""
}

func getEventSeverity(ev domain.AlertEvent) string {
	if ev.Threshold != nil && ev.Threshold.Severity != "" {
		return ev.Threshold.Severity
	}
	if ev.Rule != nil && ev.Rule.DefaultSeverity != "" {
		return ev.Rule.DefaultSeverity
	}
	return "warning"
}

func getEventRuleName(ev domain.AlertEvent) string {
	if ev.Rule != nil && ev.Rule.TemplateName != "" {
		return ev.Rule.TemplateName
	}
	return "Alert Rule"
}

func getEventServiceName(ev domain.AlertEvent) string {
	if ev.Rule != nil && ev.Rule.ServiceType != "" {
		return ev.Rule.ServiceType
	}
	return ""
}

func groupByHost(events []domain.AlertEvent) map[string][]domain.AlertEvent {
	m := make(map[string][]domain.AlertEvent)
	for _, ev := range events {
		m[ev.HostMachineID] = append(m[ev.HostMachineID], ev)
	}
	return m
}

func resolveHostname(machineID string, hostNames map[string]string) string {
	if name, ok := hostNames[machineID]; ok && name != "" {
		return name
	}
	if len(machineID) > 12 {
		return machineID[:12]
	}
	return machineID
}

func buildInvestigateURL(ev domain.AlertEvent) string {
	serviceID := 0
	if ev.ServiceID != nil {
		serviceID = int(*ev.ServiceID)
	}
	tID := getEventTemplateID(ev)
	svcKind := templateServiceKind(tID)

	switch svcKind {
	case "nginx":
		if serviceID > 0 {
			if ev.Domain != "" {
				return fmt.Sprintf("/services/%d/nginx?domain=%s", serviceID, url.QueryEscape(ev.Domain))
			}
			return fmt.Sprintf("/services/%d/nginx", serviceID)
		}
	case "apache":
		if serviceID > 0 {
			if ev.Domain != "" {
				return fmt.Sprintf("/services/%d/apache?domain=%s", serviceID, url.QueryEscape(ev.Domain))
			}
			return fmt.Sprintf("/services/%d/apache", serviceID)
		}
	case "postgres":
		if serviceID > 0 {
			return fmt.Sprintf("/services/%d/postgres", serviceID)
		}
	case "mariadb":
		if serviceID > 0 {
			return fmt.Sprintf("/services/%d/mariadb", serviceID)
		}
	case "redis":
		if serviceID > 0 {
			return fmt.Sprintf("/services/%d/redis", serviceID)
		}
	case "phpfpm":
		if serviceID > 0 {
			if ev.Pool != "" {
				return fmt.Sprintf("/services/%d/phpfpm?pool=%s", serviceID, url.QueryEscape(ev.Pool))
			}
			return fmt.Sprintf("/services/%d/phpfpm", serviceID)
		}
	}

	if getTier(tID) == TierHost || getTier(tID) == TierHostFault || getTier(tID) == TierSecurity {
		if ev.HostMachineID != "" {
			return fmt.Sprintf("/hosts/%s/dashboard/summary", url.QueryEscape(ev.HostMachineID))
		}
	}

	return "/alerts"
}

func summarizeEvents(events []domain.AlertEvent, role IncidentRole, members *[]IncidentMember) (time.Time, string) {
	earliest := time.Now().Add(24 * time.Hour)
	maxSev := "warning"

	for _, ev := range events {
		sev := getEventSeverity(ev)
		if sev == "critical" {
			maxSev = "critical"
		}
		if ev.TriggeredAt.Before(earliest) {
			earliest = ev.TriggeredAt
		}

		*members = append(*members, IncidentMember{
			RuleID:         ev.RuleID,
			TemplateID:     getEventTemplateID(ev),
			RuleName:       getEventRuleName(ev),
			ServiceName:    getEventServiceName(ev),
			Domain:         ev.Domain,
			Pool:           ev.Pool,
			Role:           role,
			Severity:       sev,
			ActualValue:    ev.ActualValue,
			Status:         ev.Status,
			Since:          ev.TriggeredAt,
			ResolvedAt:     ev.ResolvedAt,
			InvestigateURL: buildInvestigateURL(ev),
		})
	}

	if earliest.After(time.Now()) {
		earliest = time.Now()
	}

	return earliest, maxSev
}

func buildFallbackTitle(kind string, flavor IncidentFlavor, rootEvs, appEvs, webEvs []domain.AlertEvent, host string) (string, string) {
	rootNames := make([]string, 0)
	for _, ev := range rootEvs {
		rootNames = append(rootNames, getEventRuleName(ev))
	}

	fLabel := flavorLabel[flavor]
	if fLabel == "" {
		fLabel = "degradation"
	}

	rootSvcName := strings.ToUpper(kind[:1]) + kind[1:]

	symptomSvc := ""
	if len(appEvs) > 0 && len(webEvs) > 0 {
		symptomSvc = "PHP-FPM and Web tier"
	} else if len(appEvs) > 0 {
		symptomSvc = "PHP-FPM"
	} else if len(webEvs) > 0 {
		symptomSvc = "Web tier"
	}

	var title, narrative string
	if symptomSvc != "" {
		title = fmt.Sprintf("%s is showing %s, and %s is degraded at the same time on %s", rootSvcName, fLabel, symptomSvc, host)
		narrative = fmt.Sprintf("%s alerts (%s) are firing alongside downstream %s degradation.", rootSvcName, strings.Join(rootNames, ", "), symptomSvc)
	} else {
		title = fmt.Sprintf("%s is showing %s on %s", rootSvcName, fLabel, host)
		narrative = fmt.Sprintf("%s alerts (%s) are currently active.", rootSvcName, strings.Join(rootNames, ", "))
	}

	return title, narrative
}

func sortCards(cards []IncidentCard) {
	sort.Slice(cards, func(i, j int) bool {
		// 1. Security first
		if cards[i].Category != cards[j].Category {
			return cards[i].Category == CategorySecurity
		}
		// 2. Critical severity first
		if cards[i].Severity != cards[j].Severity {
			return cards[i].Severity == "critical"
		}
		// 3. FiringSince earliest first
		return cards[i].FiringSince.Before(cards[j].FiringSince)
	})
}
