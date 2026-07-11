// internal/service/discovery_service.go
// CHANGE: GetServiceDetail — fix NginxVirtualHost grouping.
// The DB stores one row per (domain, log_kind, log_path).
// The previous code created one response object per DB row, so a domain
// with both access + error logs produced two separate table entries instead
// of being merged into one.  Fixed below in GetServiceDetail.

package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

var ErrMetricsEndpointRequired = errors.New("metrics endpoint is required")

type DiscoveryService struct {
	hostRepo      *repository.HostRepository
	discoveryRepo *repository.DiscoveryRepository
	alertsRepo    *repository.AlertsRepository
}

func NewDiscoveryService(
	hostRepo *repository.HostRepository,
	discoveryRepo *repository.DiscoveryRepository,
	alertsRepo *repository.AlertsRepository,
) *DiscoveryService {
	return &DiscoveryService{
		hostRepo:      hostRepo,
		discoveryRepo: discoveryRepo,
		alertsRepo:    alertsRepo,
	}
}

func (s *DiscoveryService) ListCatalog() []catalog.ServiceDef {
	items := make([]catalog.ServiceDef, 0, len(catalog.Catalog))
	for _, def := range catalog.Catalog {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].DisplayName < items[j].DisplayName
	})
	return items
}

func (s *DiscoveryService) ListHosts(ctx context.Context, tenantID uint) ([]dto.DiscoveryHostListItem, error) {
	hosts, err := s.hostRepo.ListByTenantCtx(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	out := make([]dto.DiscoveryHostListItem, 0, len(hosts))
	for _, host := range hosts {
		elapsed := now.Sub(host.LastSeenAt)
		status := "offline"
		switch {
		case elapsed <= 7*time.Minute:
			status = "online"
		case elapsed <= 15*time.Minute:
			status = "warning"
		}

		out = append(out, dto.DiscoveryHostListItem{
			MachineID:    host.MachineID,
			Hostname:     host.Hostname,
			OSName:       host.OSName,
			Arch:         host.Arch,
			AgentVersion: host.AgentVersion,
			LastSeenAt:   host.LastSeenAt.UTC().Format(time.RFC3339),
			Status:       status,
		})
	}
	return out, nil
}

func (s *DiscoveryService) ListServices(ctx context.Context, tenantID uint, machineID string) ([]dto.DiscoveryServiceListItem, error) {
	var (
		rows []domain.DiscoveredService
		err  error
	)
	if machineID != "" {
		rows, err = s.discoveryRepo.ListServicesByTenantMachineCtx(ctx, tenantID, machineID)
	} else {
		rows, err = s.discoveryRepo.ListServicesByTenantCtx(ctx, tenantID)
	}
	if err != nil {
		return nil, err
	}

	out := make([]dto.DiscoveryServiceListItem, 0, len(rows))
	for _, row := range rows {
		def, supported := catalog.ResolveService(row.ServiceName)
		scannable := supported && def.NVD != nil
		category := "unknown"
		if supported {
			category = def.Category
		}
		logPathSource := normalizedLogPathSource(row.LogPathSource)

		out = append(out, dto.DiscoveryServiceListItem{
			ID:                 row.ID,
			MachineID:          row.MachineID,
			ServiceName:        row.ServiceName,
			Port:               row.Port,
			Version:            row.Version,
			Category:           category,
			IsSupported:        supported,
			ConfirmedLogPath:   row.ConfirmedLogPath,
			LogPathSource:      logPathSource,
			LogCollectionOn:    row.LogCollectionOn,
			MetricCollectionOn: row.MetricCollectionOn,
			LoggingMode:        row.LoggingMode,
			VulnScanSupported:  scannable,
			VulnScanStatus:     string(row.VulnerabilityScanStatus),
			LastVulnScanAt:     formatTimePtr(row.LastVulnerabilityScanAt),
		})
	}

	return out, nil
}

func (s *DiscoveryService) GetServiceDetail(ctx context.Context, tenantID uint, serviceID uint) (*dto.DiscoveryServiceDetailResponse, error) {
	service, err := s.discoveryRepo.FindServiceByIDTenantCtx(ctx, tenantID, serviceID)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, nil
	}

	host, err := s.hostRepo.FindByTenantMachineCtx(ctx, tenantID, service.MachineID)
	if err != nil {
		return nil, err
	}
	paths, err := s.discoveryRepo.ListServiceLogPathsCtx(ctx, tenantID, service.ID)
	if err != nil {
		return nil, err
	}
	vulnerabilities, err := s.alertsRepo.ListServiceVulnerabilitiesCtx(ctx, tenantID, service.ID)
	if err != nil {
		return nil, err
	}

	var def *catalog.ServiceDef
	supported := false
	scannable := false
	if d, ok := catalog.ResolveService(service.ServiceName); ok {
		copy := d
		def = &copy
		supported = true
		scannable = d.NVD != nil
	}

	hostname := ""
	if host != nil {
		hostname = host.Hostname
	}

	effectiveLogPaths := effectiveLogPaths(service, paths)
	suggestedLogPath := suggestedLogPath(service, paths)
	category := "unknown"
	if def != nil {
		category = def.Category
	}
	logPathSource := normalizedLogPathSource(service.LogPathSource)
	suggestedMetricsEndpoint := strings.TrimSpace(service.UserMetricsEndpoint)
	if suggestedMetricsEndpoint == "" && def != nil {
		suggestedMetricsEndpoint = adaptEndpointToDetectedPort(def.DefaultMetricsEndpoint, service.Port)
	}

	// ── Nginx virtual hosts ──────────────────────────────────────────────────
	// The DB stores one row per (domain, log_kind, log_path).
	// We group them by domain so the frontend gets one object per domain with
	// both access_log and error_log fields populated.
	var nginxVirtualHosts []dto.WebServerVirtualHostResponse
	if service.ServiceName == "nginx" {
		vhosts, err := s.discoveryRepo.ListVirtualHostsByServiceCtx(ctx, tenantID, service.ID)
		if err != nil {
			return nil, err
		}
		nginxVirtualHosts = groupNginxVirtualHosts(vhosts)
	}
	var apacheVirtualHosts []dto.WebServerVirtualHostResponse
	if service.ServiceName == "apache" {
		vhosts, err := s.discoveryRepo.ListApacheVirtualHostsByServiceCtx(ctx, tenantID, service.ID)
		if err != nil {
			return nil, err
		}
		apacheVirtualHosts = groupApacheVirtualHosts(vhosts)
	}

	resp := &dto.DiscoveryServiceDetailResponse{
		ID:                       service.ID,
		MachineID:                service.MachineID,
		Hostname:                 hostname,
		ServiceName:              service.ServiceName,
		Port:                     service.Port,
		Version:                  service.Version,
		Category:                 category,
		IsSupported:              supported,
		LoggingMode:              service.LoggingMode,
		UsesJournald:             service.UsesJournald,
		UsesSyslog:               service.UsesSyslog,
		AccessLogPath:            service.AccessLogPath,
		ErrorLogPath:             service.ErrorLogPath,
		ConfirmedLogPath:         service.ConfirmedLogPath,
		LogPathSource:            logPathSource,
		EffectiveLogPaths:        effectiveLogPaths,
		SuggestedLogPath:         suggestedLogPath,
		UserMetricsEndpoint:      service.UserMetricsEndpoint,
		SuggestedMetricsEndpoint: suggestedMetricsEndpoint,
		DetectedLogPaths:         paths,
		LogCollectionOn:          service.LogCollectionOn,
		MetricCollectionOn:       service.MetricCollectionOn,
		VulnScanSupported:        scannable,
		VulnScanStatus:           string(service.VulnerabilityScanStatus),
		LastVulnScanAt:           formatTimePtr(service.LastVulnerabilityScanAt),
		VulnStatusMessage: vulnerabilityStatusMessage(
			scannable,
			service.VulnerabilityScanStatus,
			service.LastVulnerabilityScanAt,
			len(vulnerabilities),
		),
		Vulnerabilities:    vulnerabilities,
		Catalog:            def,
		NginxVirtualHosts:  nginxVirtualHosts,
		ApacheVirtualHosts: apacheVirtualHosts,
	}

	return resp, nil
}

type webServerVirtualHostRow struct {
	ID          uint
	Domain      string
	LogKind     string
	LogPath     string
	Source      string
	Enabled     bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// groupWebServerVirtualHosts merges DB rows (one per log_kind) into one response
// object per domain. The key is domain.
// We use the earliest row's ID and timestamps for the merged entry.
func groupWebServerVirtualHosts(rows []webServerVirtualHostRow) []dto.WebServerVirtualHostResponse {
	type groupKey struct {
		domain      string
	}

	// Preserve insertion order using a slice of keys
	order := []groupKey{}
	groups := map[groupKey]*dto.WebServerVirtualHostResponse{}

	for _, vh := range rows {
		key := groupKey{
			domain:      vh.Domain,
		}

		entry, exists := groups[key]
		if !exists {
			entry = &dto.WebServerVirtualHostResponse{
				ID:          vh.ID,
				Domain:      vh.Domain,
				Enabled:     vh.Enabled,
				Source:      vh.Source,
				CreatedAt:   vh.CreatedAt.Format(time.RFC3339),
				UpdatedAt:   vh.UpdatedAt.Format(time.RFC3339),
			}
			groups[key] = entry
			order = append(order, key)
		}

		// Merge log path into the right field
		switch vh.LogKind {
		case "access":
			entry.AccessLog = vh.LogPath
		case "error":
			entry.ErrorLog = vh.LogPath
		}

		// Use the most recent UpdatedAt
		if vh.UpdatedAt.After(entry.UpdatedAt_t) {
			entry.UpdatedAt = vh.UpdatedAt.Format(time.RFC3339)
			entry.UpdatedAt_t = vh.UpdatedAt
		}
	}

	result := make([]dto.WebServerVirtualHostResponse, 0, len(order))
	for _, key := range order {
		result = append(result, *groups[key])
	}
	return result
}

func groupNginxVirtualHosts(rows []domain.NginxVirtualHost) []dto.WebServerVirtualHostResponse {
	webServerRows := make([]webServerVirtualHostRow, 0, len(rows))
	for _, row := range rows {
		webServerRows = append(webServerRows, webServerVirtualHostRow{
			ID:          row.ID,
			Domain:      row.Domain,
			LogKind:     row.LogKind,
			LogPath:     row.LogPath,
			Source:      row.Source,
			Enabled:     row.Enabled,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return groupWebServerVirtualHosts(webServerRows)
}

func groupApacheVirtualHosts(rows []domain.ApacheVirtualHost) []dto.WebServerVirtualHostResponse {
	webServerRows := make([]webServerVirtualHostRow, 0, len(rows))
	for _, row := range rows {
		webServerRows = append(webServerRows, webServerVirtualHostRow{
			ID:          row.ID,
			Domain:      row.Domain,
			LogKind:     row.LogKind,
			LogPath:     row.LogPath,
			Source:      row.Source,
			Enabled:     row.Enabled,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return groupWebServerVirtualHosts(webServerRows)
}

func (s *DiscoveryService) UpdateCollection(ctx context.Context, tenantID uint, serviceID uint, req dto.UpdateCollectionRequest) error {
	return s.discoveryRepo.UpdateServiceCollectionCtx(
		ctx,
		tenantID,
		serviceID,
		req.LogCollectionOn,
		req.MetricCollectionOn,
	)
}

func (s *DiscoveryService) UpdateServiceConfig(ctx context.Context, tenantID uint, serviceID uint, req dto.UpdateServiceConfigRequest) error {
	confirmedLogPath := strings.TrimSpace(req.ConfirmedLogPath)
	userMetricsEndpoint := strings.TrimSpace(req.UserMetricsEndpoint)
	logPathSource := "detected"
	if confirmedLogPath != "" {
		logPathSource = "user"
	}

	// Handle web-server virtual hosts update
	if len(req.NginxVirtualHosts) > 0 || len(req.ApacheVirtualHosts) > 0 {
		svc, err := s.discoveryRepo.FindServiceByIDTenantCtx(ctx, tenantID, serviceID)
		if err != nil {
			return err
		}
		if svc == nil {
			return gorm.ErrRecordNotFound
		}

		if svc.ServiceName == "nginx" {
			// Each frontend row (one per domain) maps to up to two DB rows
			// (one for access log, one for error log).
			vhosts := make([]domain.NginxVirtualHost, 0, len(req.NginxVirtualHosts)*2)
			for _, vhReq := range req.NginxVirtualHosts {
				domainName := strings.TrimSpace(vhReq.Domain)
				if domainName == "" {
					domainName = "(default)"
				}

				if vhReq.AccessLog != "" {
					vhosts = append(vhosts, domain.NginxVirtualHost{
						Domain:  domainName,
						LogKind: "access",
						LogPath: strings.TrimSpace(vhReq.AccessLog),
						Source:  "user",
						Enabled: vhReq.Enabled,
					})
				}

				if vhReq.ErrorLog != "" {
					vhosts = append(vhosts, domain.NginxVirtualHost{
						Domain:  domainName,
						LogKind: "error",
						LogPath: strings.TrimSpace(vhReq.ErrorLog),
						Source:  "user",
						Enabled: vhReq.Enabled,
					})
				}
			}

			if err := s.discoveryRepo.ReplaceVirtualHostsCtx(ctx, tenantID, serviceID, vhosts); err != nil {
				return err
			}
		}
		if svc.ServiceName == "apache" {
			vhosts := make([]domain.ApacheVirtualHost, 0, len(req.ApacheVirtualHosts)*2)
			for _, vhReq := range req.ApacheVirtualHosts {
				domainName := strings.TrimSpace(vhReq.Domain)
				if domainName == "" {
					domainName = "(default)"
				}

				if vhReq.AccessLog != "" {
					vhosts = append(vhosts, domain.ApacheVirtualHost{
						Domain:  domainName,
						LogKind: "access",
						LogPath: strings.TrimSpace(vhReq.AccessLog),
						Source:  "user",
						Enabled: vhReq.Enabled,
					})
				}

				if vhReq.ErrorLog != "" {
					vhosts = append(vhosts, domain.ApacheVirtualHost{
						Domain:  domainName,
						LogKind: "error",
						LogPath: strings.TrimSpace(vhReq.ErrorLog),
						Source:  "user",
						Enabled: vhReq.Enabled,
					})
				}
			}

			if err := s.discoveryRepo.ReplaceApacheVirtualHostsCtx(ctx, tenantID, serviceID, vhosts); err != nil {
				return err
			}
		}
	}

	return s.discoveryRepo.UpdateServiceConfigCtx(
		ctx,
		tenantID,
		serviceID,
		confirmedLogPath,
		logPathSource,
		userMetricsEndpoint,
	)
}

func IsNotFoundError(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func normalizedLogPathSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "detected"
	}
	return source
}

func effectiveLogPaths(service *domain.DiscoveredService, paths []string) []string {
	out := make([]string, 0, len(paths))
	if service.ConfirmedLogPath != "" {
		var parts []string
		for _, part := range strings.Split(service.ConfirmedLogPath, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}

	out = append(out, paths...)
	if len(out) > 0 {
		return out
	}

	if service.AccessLogPath != "" {
		out = append(out, service.AccessLogPath)
	}
	if service.ErrorLogPath != "" && service.ErrorLogPath != service.AccessLogPath {
		out = append(out, service.ErrorLogPath)
	}
	return out
}

func suggestedLogPath(service *domain.DiscoveredService, paths []string) string {
	if service.ConfirmedLogPath != "" {
		return service.ConfirmedLogPath
	}
	if len(paths) > 0 {
		return paths[0]
	}
	if trimmed := strings.TrimSpace(service.AccessLogPath); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(service.ErrorLogPath)
}

func adaptEndpointToDetectedPort(raw string, detectedPort *int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || detectedPort == nil || *detectedPort <= 0 {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Host == "" {
		return raw
	}

	host := parsed.Hostname()
	if host == "" {
		return raw
	}

	parsed.Host = net.JoinHostPort(host, strconv.Itoa(*detectedPort))
	return parsed.String()
}

func vulnerabilityStatusMessage(supported bool, status domain.VulnScanStatus, lastScanAt *time.Time, findingCount int) string {
	if !supported {
		return "Vulnerability scanning is unavailable for this service because it is not mapped in the catalog."
	}

	switch status {
	case domain.VulnScanStatusUnsupported:
		return "Vulnerability scanning is unavailable for this service because it is not mapped in the catalog."
	case domain.VulnScanStatusNoVersion:
		return "Vulnerability scanning is waiting for a detected version. The agent will retry on the next discovery run."
	case domain.VulnScanStatusNotScanned:
		return "Vulnerability scan has not run yet. The scanner runs on backend startup and then every 24 hours."
	case domain.VulnScanStatusFailed:
		return "The last vulnerability scan failed. It will retry automatically on the next scheduled run."
	case domain.VulnScanStatusScanned:
		if findingCount == 0 {
			if lastScanAt == nil {
				return "No medium-or-higher vulnerability findings are stored for this service."
			}
			return "No known medium-or-higher vulnerabilities were found in the latest scan."
		}
		if lastScanAt == nil {
			return fmt.Sprintf("%d vulnerability finding(s) detected for this service version.", findingCount)
		}
		return fmt.Sprintf("%d vulnerability finding(s) detected in the latest scan.", findingCount)
	default:
		return "Vulnerability scan status is unknown."
	}
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
