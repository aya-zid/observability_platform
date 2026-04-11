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

		// Ensure host row exists/updated when install report arrives.
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

// IngestHostServices stores host-level discovered services (non-docker).
func (h *DiscoveryHandler) IngestHostServices(c echo.Context) error {
	req, err := h.bindDiscoveryRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return h.runIngest(c, req, "ingest host services", persistHostServices)
}

// IngestContainerServices stores container-level discovered services.
func (h *DiscoveryHandler) IngestContainerServices(c echo.Context) error {
	req, err := h.bindDiscoveryRequest(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return h.runIngest(c, req, "ingest container services", persistContainerServices)
}

// IngestInstallReport keeps backward compatibility by ingesting both host + container services.
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
