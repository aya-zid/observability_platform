package handler

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/dto"
	custommiddleware "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/middleware"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/validator"
)

type HostHandler struct {
	hostRepo         *repository.HostRepository
	subscriptionRepo *repository.SubscriptionRepository
	discoveryRepo    *repository.DiscoveryRepository
}

func NewHostHandler(hostRepo *repository.HostRepository, subscriptionRepo *repository.SubscriptionRepository, discoveryRepo *repository.DiscoveryRepository) *HostHandler {
	return &HostHandler{hostRepo: hostRepo, subscriptionRepo: subscriptionRepo, discoveryRepo: discoveryRepo}
}

func (h *HostHandler) Register(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.HostRegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Check if host already exists (upsert is fine)
	existing, _ := h.hostRepo.FindByTenantMachineCtx(c.Request().Context(), tenant.ID, req.MachineID)
	if existing == nil {
		// New host registration — check plan limit
		sub, err := h.subscriptionRepo.FindByTenantCtx(c.Request().Context(), tenant.ID)
		if err == nil && sub != nil {
			hosts, err := h.hostRepo.ListByTenantCtx(c.Request().Context(), tenant.ID)
			if err == nil && len(hosts) >= sub.Plan.MaxHosts {
				return c.JSON(http.StatusPaymentRequired, map[string]string{
					"error": "Host limit reached. Upgrade your plan.",
				})
			}
		}
	}

	if err := h.hostRepo.UpsertByTenantMachineCtx(
		c.Request().Context(),
		tenant.ID,
		req.MachineID,
		req.Hostname,
		req.OSName,
		req.OSVersion,
		req.Arch,
		req.AgentVersion,
	); err != nil {
		c.Logger().Errorf("[hosts] register failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to register host"})
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func (h *HostHandler) Heartbeat(c echo.Context) error {
	tenant := custommiddleware.GetTenantFromContext(c)
	if tenant == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	var req dto.HostHeartbeatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if err := h.hostRepo.TouchHeartbeatByTenantMachineCtx(
		c.Request().Context(),
		tenant.ID,
		req.MachineID,
		req.Hostname,
		req.AgentVersion,
	); err != nil {
		c.Logger().Errorf("[hosts] heartbeat failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update heartbeat"})
	}

	// Update LastSeenAt for running services and containers
	if len(req.RunningServices) > 0 {
		if err := h.discoveryRepo.TouchServiceHeartbeatCtx(c.Request().Context(), tenant.ID, req.MachineID, req.RunningServices); err != nil {
			c.Logger().Errorf("[hosts] service heartbeat failed: %v", err)
		}
	}
	if len(req.RunningContainers) > 0 {
		if err := h.discoveryRepo.TouchContainerHeartbeatCtx(c.Request().Context(), tenant.ID, req.MachineID, normalizeContainerHeartbeatNames(req.RunningContainers)); err != nil {
			c.Logger().Errorf("[hosts] container heartbeat failed: %v", err)
		}
	}

	return c.JSON(http.StatusOK, dto.OkResponse{OK: true})
}

func normalizeContainerHeartbeatNames(names []string) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		normalized := safeContainerName(name)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	return out
}

func safeContainerName(name string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}
