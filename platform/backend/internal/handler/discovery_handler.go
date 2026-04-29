// internal/handler/discovery_handler.go
// Only the persistHostServices function changes — specifically the nginx fallback block.
// Everything else is identical to your current file.

package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type DiscoveryHandler struct {
	db *gorm.DB
}

func NewDiscoveryHandler(db *gorm.DB) *DiscoveryHandler {
	return &DiscoveryHandler{db: db}
}

func parseLogPaths(raw string) []string {
	if raw == "" || raw == "none" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == ','
	})
	paths := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == "none" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	return paths
}

func findAccessError(paths []string) (string, string) {
	var access, errp string
	for _, p := range paths {
		lp := strings.ToLower(p)
		if access == "" && strings.Contains(lp, "access") {
			access = p
		}
		if errp == "" && strings.Contains(lp, "error") {
			errp = p
		}
	}
	if access == "" && len(paths) > 0 {
		access = paths[0]
	}
	if errp == "" && len(paths) > 1 {
		errp = paths[1]
	}
	return access, errp
}

func parseDetectedFromNotes(notes string) (serviceName, image string) {
	serviceName = "unknown"
	image = ""
	for _, token := range strings.Fields(notes) {
		if strings.HasPrefix(token, "service=") {
			v := strings.TrimPrefix(token, "service=")
			if v != "" {
				serviceName = v
			}
		}
		if strings.HasPrefix(token, "image=") {
			v := strings.TrimPrefix(token, "image=")
			if v != "" {
				image = v
			}
		}
	}
	return serviceName, image
}

func deriveLoggingMode(hasFile bool, usesJournald bool, usesSyslog bool) string {
	if hasFile && (usesJournald || usesSyslog) {
		return "mixed"
	}
	if hasFile {
		return "file"
	}
	if usesJournald {
		return "journald"
	}
	if usesSyslog {
		return "syslog"
	}
	return "unknown"
}

func (h *DiscoveryHandler) runIngest(
	c echo.Context,
	req dto.InstallDiscoveryReportRequest,
	opName string,
	persistFn func(ctx context.Context, tenantID uint, machineID string, services []dto.InstallDiscoveryService, discoveryRepo *repository.DiscoveryRepository) error,
) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	if err := h.db.WithContext(c.Request().Context()).Transaction(func(tx *gorm.DB) error {
		hostRepo := repository.NewHostRepository(tx)
		discoveryRepo := repository.NewDiscoveryRepository(tx)

		if err := hostRepo.UpsertByTenantMachineCtx(
			c.Request().Context(),
			tenant.ID,
			req.MachineID,
			req.Host,
			req.OS,
			"",
			req.Arch,
			req.AgentVersion,
		); err != nil {
			return err
		}

		if err := persistFn(c.Request().Context(), tenant.ID, req.MachineID, req.Services, discoveryRepo); err != nil {
			return err
		}

		return nil
	}); err != nil {
		c.Logger().Errorf("[discovery] %s failed: %v", opName, err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to store discovered services"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func persistHostServices(ctx context.Context, tenantID uint, machineID string, services []dto.InstallDiscoveryService, discoveryRepo *repository.DiscoveryRepository) error {
	for _, svc := range services {
		svcName := strings.TrimSpace(svc.Name)
		if svcName == "" || strings.HasPrefix(svcName, "docker_") {
			continue
		}

		paths := parseLogPaths(svc.LogPaths)
		accessPath, errorPath := findAccessError(paths)
		notesLower := strings.ToLower(svc.Notes)
		usesJournald := strings.Contains(notesLower, "journald")
		usesSyslog := strings.Contains(notesLower, "syslog")
		loggingMode := deriveLoggingMode(len(paths) > 0, usesJournald, usesSyslog)

		rec := domain.DiscoveredService{
			TenantID:      tenantID,
			MachineID:     machineID,
			ServiceName:   svcName,
			Port:          svc.Port,
			Version:       strings.TrimSpace(svc.Version),
			AccessLogPath: accessPath,
			ErrorLogPath:  errorPath,
			LoggingMode:   loggingMode,
			UsesJournald:  usesJournald,
			UsesSyslog:    usesSyslog,
		}
		upserted, err := discoveryRepo.UpsertServiceCtx(ctx, rec)
		if err != nil {
			return err
		}
		if err := discoveryRepo.ReplaceServiceLogPathsCtx(ctx, tenantID, upserted.ID, paths); err != nil {
			return err
		}

		if svcName != "nginx" {
			continue
		}

		// ── Nginx virtual host persistence ────────────────────────────────────
		//
		// The agent sends virtual_hosts when it successfully parsed sites-enabled/.
		// We build per-domain rows from that structured data.
		//
		// Fallback: if the agent sent no virtual_hosts (old agent version, or
		// detection failed), we try to infer domains from the flat log path list.
		// We match "access" paths to "error" paths by their common prefix
		// (e.g. shop.access.log ↔ shop.error.log) instead of using findAccessError
		// which incorrectly picks a single access + a single error path.

		if len(svc.VirtualHosts) > 0 {
			// ── Happy path: agent sent structured vhost data ──────────────────
			detectedVhosts := make([]domain.NginxVirtualHost, 0, len(svc.VirtualHosts)*2)
			for _, vh := range svc.VirtualHosts {
				domainName := strings.TrimSpace(vh.Domain)
				if domainName == "" {
					domainName = "(default)"
				}
				if vh.AccessLog != "" {
					detectedVhosts = append(detectedVhosts, domain.NginxVirtualHost{
						Domain:    domainName,
						LogKind:   "access",
						LogPath:   strings.TrimSpace(vh.AccessLog),
						Source:    "detected",
						Enabled:   true,
						IsDefault: vh.IsDefault,
					})
				}
				if vh.ErrorLog != "" {
					detectedVhosts = append(detectedVhosts, domain.NginxVirtualHost{
						Domain:    domainName,
						LogKind:   "error",
						LogPath:   strings.TrimSpace(vh.ErrorLog),
						Source:    "detected",
						Enabled:   true,
						IsDefault: vh.IsDefault,
					})
				}
			}
			if len(detectedVhosts) > 0 {
				if err := discoveryRepo.UpsertDetectedVirtualHostsCtx(ctx, tenantID, upserted.ID, detectedVhosts); err != nil {
					return err
				}
			}
		} else if len(paths) > 0 {
			// ── Fallback: infer domains from the flat log path list ───────────
			//
			// Strategy: build a map from "stem" → {access, error} path.
			// Stem is the filename without the access/error suffix.
			// e.g. "/var/log/nginx/shop.access.log" → stem "shop"
			//      "/var/log/nginx/shop.error.log"  → same stem "shop"
			//
			// This correctly groups shop.access.log with shop.error.log,
			// and access.log with error.log (the default vhost), etc.

			type logPair struct {
				access    string
				errorPath string
				isDefault bool
			}
			// stem → pair, plus ordered keys to keep deterministic output
			pairs := map[string]*logPair{}
			order := []string{}

			for _, p := range paths {
				base := p
				// Take just the filename
				if idx := strings.LastIndex(p, "/"); idx >= 0 {
					base = p[idx+1:]
				}
				base = strings.TrimSuffix(base, ".log")

				var stem string
				var kind string
				if strings.HasSuffix(base, ".access") {
					stem = strings.TrimSuffix(base, ".access")
					kind = "access"
				} else if strings.HasSuffix(base, ".error") {
					stem = strings.TrimSuffix(base, ".error")
					kind = "error"
				} else if base == "access" {
					stem = "default"
					kind = "access"
				} else if base == "error" {
					stem = "default"
					kind = "error"
				} else {
					// Unrecognised pattern — put under its own stem
					stem = base
					kind = "access"
				}

				if _, ok := pairs[stem]; !ok {
					pairs[stem] = &logPair{}
					order = append(order, stem)
				}
				if kind == "access" {
					pairs[stem].access = p
				} else {
					pairs[stem].errorPath = p
				}
			}

			detectedVhosts := make([]domain.NginxVirtualHost, 0, len(pairs)*2)
			for _, stem := range order {
				pair := pairs[stem]
				// Build a human-readable domain name from the stem
				domainName := stem
				isDefault := false
				if stem == "default" {
					domainName = "(default)"
					isDefault = true
				}

				if pair.access != "" {
					detectedVhosts = append(detectedVhosts, domain.NginxVirtualHost{
						Domain:    domainName,
						LogKind:   "access",
						LogPath:   pair.access,
						Source:    "detected",
						Enabled:   true,
						IsDefault: isDefault,
					})
				}
				if pair.errorPath != "" {
					detectedVhosts = append(detectedVhosts, domain.NginxVirtualHost{
						Domain:    domainName,
						LogKind:   "error",
						LogPath:   pair.errorPath,
						Source:    "detected",
						Enabled:   true,
						IsDefault: isDefault,
					})
				}
			}

			if len(detectedVhosts) > 0 {
				if err := discoveryRepo.UpsertDetectedVirtualHostsCtx(ctx, tenantID, upserted.ID, detectedVhosts); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func persistContainerServices(ctx context.Context, tenantID uint, machineID string, services []dto.InstallDiscoveryService, discoveryRepo *repository.DiscoveryRepository) error {
	for _, svc := range services {
		containerName := strings.TrimSpace(svc.Name)
		if containerName == "" {
			continue
		}
		if after, ok := strings.CutPrefix(containerName, "docker_"); ok {
			containerName = after
		}

		detectedService, image := parseDetectedFromNotes(svc.Notes)
		rec := domain.DiscoveredContainerService{
			TenantID:       tenantID,
			MachineID:      machineID,
			ContainerName:  containerName,
			ServiceName:    detectedService,
			ContainerImage: image,
		}
		if err := discoveryRepo.UpsertContainerServiceCtx(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

func (h *DiscoveryHandler) bindDiscoveryRequest(c echo.Context) (dto.InstallDiscoveryReportRequest, error) {
	var req dto.InstallDiscoveryReportRequest
	if err := c.Bind(&req); err != nil {
		return dto.InstallDiscoveryReportRequest{}, err
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return dto.InstallDiscoveryReportRequest{}, err
	}
	return req, nil
}

func (h *DiscoveryHandler) IngestHostServices(c echo.Context) error {
	req, err := h.bindDiscoveryRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return h.runIngest(c, req, "ingest host services", persistHostServices)
}

func (h *DiscoveryHandler) IngestContainerServices(c echo.Context) error {
	req, err := h.bindDiscoveryRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return h.runIngest(c, req, "ingest container services", persistContainerServices)
}

func (h *DiscoveryHandler) IngestInstallReport(c echo.Context) error {
	req, err := h.bindDiscoveryRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return h.runIngest(
		c,
		req,
		"ingest combined discovery report",
		func(ctx context.Context, tenantID uint, machineID string, services []dto.InstallDiscoveryService, discoveryRepo *repository.DiscoveryRepository) error {
			if err := persistHostServices(ctx, tenantID, machineID, services, discoveryRepo); err != nil {
				return err
			}
			return persistContainerServices(ctx, tenantID, machineID, services, discoveryRepo)
		},
	)
}