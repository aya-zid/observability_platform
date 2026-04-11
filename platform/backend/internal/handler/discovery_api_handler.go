package handler

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/infrastructure"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type DiscoveryAPIHandler struct {
	hostRepo      *repository.HostRepository
	discoveryRepo *repository.DiscoveryRepository
	alertsRepo    *repository.AlertsRepository
}

func NewDiscoveryAPIHandler(
	hostRepo *repository.HostRepository,
	discoveryRepo *repository.DiscoveryRepository,
	alertsRepo *repository.AlertsRepository,
) *DiscoveryAPIHandler {
	return &DiscoveryAPIHandler{
		hostRepo:      hostRepo,
		discoveryRepo: discoveryRepo,
		alertsRepo:    alertsRepo,
	}
}

func (h *DiscoveryAPIHandler) claims(c echo.Context) (*infrastructure.Claims, bool) {
	claimsAny := c.Get("claims")
	claims, ok := claimsAny.(*infrastructure.Claims)
	if !ok || claims == nil {
		return nil, false
	}
	return claims, true
}

func (h *DiscoveryAPIHandler) ListCatalog(c echo.Context) error {
	items := make([]catalog.ServiceDef, 0, len(catalog.Catalog))
	for _, def := range catalog.Catalog {
		items = append(items, def)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].DisplayName < items[j].DisplayName
	})
	return c.JSON(http.StatusOK, items)
}

func (h *DiscoveryAPIHandler) ListHosts(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	hosts, err := h.hostRepo.ListByTenantCtx(c.Request().Context(), claims.TenantID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load hosts"})
	}

	out := make([]dto.DiscoveryHostListItem, 0, len(hosts))
	for _, host := range hosts {
		out = append(out, dto.DiscoveryHostListItem{
			MachineID:    host.MachineID,
			Hostname:     host.Hostname,
			OSName:       host.OSName,
			Arch:         host.Arch,
			AgentVersion: host.AgentVersion,
			LastSeenAt:   host.LastSeenAt.UTC().Format(time.RFC3339),
		})
	}

	return c.JSON(http.StatusOK, out)
}

func (h *DiscoveryAPIHandler) ListServices(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	machineID := c.QueryParam("machine_id")
	var (
		rows []domain.DiscoveredService
		err  error
	)
	if machineID != "" {
		rows, err = h.discoveryRepo.ListServicesByTenantMachineCtx(c.Request().Context(), claims.TenantID, machineID)
	} else {
		rows, err = h.discoveryRepo.ListServicesByTenantCtx(c.Request().Context(), claims.TenantID)
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load services"})
	}

	out := make([]dto.DiscoveryServiceListItem, 0, len(rows))
	for _, row := range rows {
		def, supported := catalog.ResolveService(row.ServiceName)
		scannable := supported && def.NVD != nil
		category := "unknown"
		if supported {
			category = def.Category
		}
		metricsMode := strings.TrimSpace(row.MetricsMode)
		if metricsMode == "" {
			if supported {
				metricsMode = "catalog"
			} else {
				metricsMode = "unknown"
			}
		}
		logPathSource := strings.TrimSpace(row.LogPathSource)
		if logPathSource == "" {
			logPathSource = "detected"
		}
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
			MetricsMode:        metricsMode,
			LogCollectionOn:    row.LogCollectionOn,
			MetricCollectionOn: row.MetricCollectionOn,
			IntegrationStatus:  string(row.IntegrationStatus),
			LoggingMode:        row.LoggingMode,
			VulnScanSupported:  scannable,
			VulnScanStatus:     string(row.VulnerabilityScanStatus),
			LastVulnScanAt:     formatTimePtr(row.LastVulnerabilityScanAt),
		})
	}

	return c.JSON(http.StatusOK, out)
}

func (h *DiscoveryAPIHandler) GetService(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	service, err := h.discoveryRepo.FindServiceByIDTenantCtx(c.Request().Context(), claims.TenantID, uint(serviceID))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load service"})
	}
	if service == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
	}

	host, err := h.hostRepo.FindByTenantMachineCtx(c.Request().Context(), claims.TenantID, service.MachineID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load host"})
	}

	paths, err := h.discoveryRepo.ListServiceLogPathsCtx(c.Request().Context(), claims.TenantID, service.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load log paths"})
	}
	vulnerabilities, err := h.alertsRepo.ListServiceVulnerabilitiesCtx(c.Request().Context(), claims.TenantID, service.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to load vulnerabilities"})
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

	effectiveLogPaths := make([]string, 0, len(paths))
	if service.ConfirmedLogPath != "" {
		effectiveLogPaths = append(effectiveLogPaths, service.ConfirmedLogPath)
	} else {
		effectiveLogPaths = append(effectiveLogPaths, paths...)
	}
	if len(effectiveLogPaths) == 0 {
		if service.AccessLogPath != "" {
			effectiveLogPaths = append(effectiveLogPaths, service.AccessLogPath)
		}
		if service.ErrorLogPath != "" && service.ErrorLogPath != service.AccessLogPath {
			effectiveLogPaths = append(effectiveLogPaths, service.ErrorLogPath)
		}
	}
	suggestedLogPath := service.ConfirmedLogPath
	if suggestedLogPath == "" && len(paths) > 0 {
		suggestedLogPath = paths[0]
	}
	if suggestedLogPath == "" {
		suggestedLogPath = strings.TrimSpace(service.AccessLogPath)
	}
	if suggestedLogPath == "" {
		suggestedLogPath = strings.TrimSpace(service.ErrorLogPath)
	}
	category := "unknown"
	if def != nil {
		category = def.Category
	}
	metricsMode := strings.TrimSpace(service.MetricsMode)
	if metricsMode == "" {
		if supported {
			metricsMode = "catalog"
		} else {
			metricsMode = "unknown"
		}
	}
	logPathSource := strings.TrimSpace(service.LogPathSource)
	if logPathSource == "" {
		logPathSource = "detected"
	}
	suggestedMetricsEndpoint := strings.TrimSpace(service.UserMetricsEndpoint)
	if suggestedMetricsEndpoint == "" && def != nil {
		suggestedMetricsEndpoint = adaptEndpointToDetectedPort(def.DefaultMetricsEndpoint, service.Port)
	}
	vulnStatus := vulnerabilityStatusMessage(
		scannable,
		service.VulnerabilityScanStatus,
		service.LastVulnerabilityScanAt,
		len(vulnerabilities),
	)

	return c.JSON(http.StatusOK, dto.DiscoveryServiceDetailResponse{
		ID:                  service.ID,
		MachineID:           service.MachineID,
		Hostname:            hostname,
		ServiceName:         service.ServiceName,
		Port:                service.Port,
		Version:             service.Version,
		Category:            category,
		IsSupported:         supported,
		LoggingMode:         service.LoggingMode,
		UsesJournald:        service.UsesJournald,
		UsesSyslog:          service.UsesSyslog,
		AccessLogPath:       service.AccessLogPath,
		ErrorLogPath:        service.ErrorLogPath,
		ConfirmedLogPath:    service.ConfirmedLogPath,
		LogPathSource:       logPathSource,
		EffectiveLogPaths:   effectiveLogPaths,
		SuggestedLogPath:    suggestedLogPath,
		UserMetricsEndpoint: service.UserMetricsEndpoint,
		SuggestedMetricsEndpoint: suggestedMetricsEndpoint,
		MetricsMode:         metricsMode,
		IsJavaApp:           service.IsJavaApp,
		JMXExporterPort:     service.JMXExporterPort,
		DetectedLogPaths:    paths,
		LogCollectionOn:     service.LogCollectionOn,
		MetricCollectionOn:  service.MetricCollectionOn,
		IntegrationStatus:   string(service.IntegrationStatus),
		VulnScanSupported:   scannable,
		VulnScanStatus:      string(service.VulnerabilityScanStatus),
		LastVulnScanAt:      formatTimePtr(service.LastVulnerabilityScanAt),
		VulnStatusMessage:   vulnStatus,
		Vulnerabilities:     vulnerabilities,
		Catalog:             def,
	})
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

func (h *DiscoveryAPIHandler) UpdateCollection(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	var req dto.UpdateCollectionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	status := domain.IntegrationStatusNotInstalled
	if req.LogCollectionOn || req.MetricCollectionOn {
		status = domain.IntegrationStatusPending
	}

	if err := h.discoveryRepo.UpdateServiceCollectionCtx(
		c.Request().Context(),
		claims.TenantID,
		uint(serviceID),
		req.LogCollectionOn,
		req.MetricCollectionOn,
		status,
	); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update collection"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *DiscoveryAPIHandler) UpdateServiceConfig(c echo.Context) error {
	claims, ok := h.claims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	serviceID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service id"})
	}

	var req dto.UpdateServiceConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	req.MetricsMode = strings.TrimSpace(req.MetricsMode)
	switch req.MetricsMode {
	case "", "catalog", "prometheus", "jmx", "none", "unknown":
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid metrics mode"})
	}

	confirmedLogPath := strings.TrimSpace(req.ConfirmedLogPath)
	userMetricsEndpoint := strings.TrimSpace(req.UserMetricsEndpoint)
	logPathSource := "detected"
	if confirmedLogPath != "" {
		logPathSource = "user"
	}

	if req.MetricsMode == "prometheus" && userMetricsEndpoint == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "metrics endpoint is required for prometheus mode"})
	}
	if req.MetricsMode == "jmx" && req.JMXExporterPort == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "jmx exporter port is required for jmx mode"})
	}

	if err := h.discoveryRepo.UpdateServiceConfigCtx(
		c.Request().Context(),
		claims.TenantID,
		uint(serviceID),
		confirmedLogPath,
		logPathSource,
		userMetricsEndpoint,
		req.MetricsMode,
		req.IsJavaApp,
		req.JMXExporterPort,
	); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "service not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update service config"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
