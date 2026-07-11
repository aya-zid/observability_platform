package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/service"
)

type HostDashboardHandler struct {
	svc        *service.HostDashboardService
	logsSvc    *service.HostLogsService
	hostRepo   *repository.HostRepository
	tenantRepo *repository.TenantRepository
}

func NewHostDashboardHandler(
	svc *service.HostDashboardService,
	logsSvc *service.HostLogsService,
	hostRepo *repository.HostRepository,
	tenantRepo *repository.TenantRepository,
) *HostDashboardHandler {
	return &HostDashboardHandler{svc: svc, logsSvc: logsSvc, hostRepo: hostRepo, tenantRepo: tenantRepo}
}

func (h *HostDashboardHandler) resolveMachine(c echo.Context) (uint, string, bool) {
	claims, ok := getClaims(c)
	if !ok {
		return 0, "", false
	}
	machineID := c.Param("machineId")
	if machineID == "" {
		return 0, "", false
	}
	host, err := h.hostRepo.FindByTenantMachineCtx(c.Request().Context(), claims.TenantID, machineID)
	if err != nil || host == nil {
		return 0, "", false
	}
	return claims.TenantID, machineID, true
}

func parseRangeParams(c echo.Context) (startSec, endSec int64, step string, err error) {
	step = c.QueryParam("step")
	if step == "" {
		step = "30s"
	}
	startStr := c.QueryParam("start")
	endStr := c.QueryParam("end")
	if startStr == "" || endStr == "" {
		endSec = time.Now().Unix()
		startSec = endSec - 3600
		return
	}
	startSec, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		return
	}
	endSec, err = strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		return
	}
	return
}

func (h *HostDashboardHandler) Summary(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	timeSec := time.Now().Unix()
	cards, err := h.svc.GetSummaryCards(c.Request().Context(), tenantID, machineID, timeSec)
	if err != nil {
		c.Logger().Errorf("[host-summary] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, cards)
}

func (h *HostDashboardHandler) CPU(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	start, end, step, err := parseRangeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid params"})
	}
	data, err := h.svc.GetCPUMetrics(c.Request().Context(), tenantID, machineID, start, end, step)
	if err != nil {
		c.Logger().Errorf("[host-cpu] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) Memory(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	start, end, step, err := parseRangeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid params"})
	}
	data, err := h.svc.GetMemoryMetrics(c.Request().Context(), tenantID, machineID, start, end, step)
	if err != nil {
		c.Logger().Errorf("[host-memory] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) Disk(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	start, end, step, err := parseRangeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid params"})
	}
	data, err := h.svc.GetDiskMetrics(c.Request().Context(), tenantID, machineID, start, end, step)
	if err != nil {
		c.Logger().Errorf("[host-disk] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) Network(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	start, end, step, err := parseRangeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid params"})
	}
	data, err := h.svc.GetNetworkMetrics(c.Request().Context(), tenantID, machineID, start, end, step)
	if err != nil {
		c.Logger().Errorf("[host-network] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) Processes(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}
	start, end, step, err := parseRangeParams(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid params"})
	}
	data, err := h.svc.GetProcessMetrics(c.Request().Context(), tenantID, machineID, start, end, step)
	if err != nil {
		c.Logger().Errorf("[host-process] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed"})
	}
	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) LogsHealthSummary(c echo.Context) error {
	tenantID, machineID, ok := h.resolveMachine(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid host"})
	}

	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), tenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, err := strconv.ParseInt(c.QueryParam("start"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start param"})
	}
	endNs, err := strconv.ParseInt(c.QueryParam("end"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end param"})
	}

	step := c.QueryParam("step")
	if step == "" {
		step = "30s"
	}

	data, err := h.logsSvc.GetHealthSummary(
		c.Request().Context(),
		tenant.UUID,
		machineID,
		startNs,
		endNs,
		step,
	)
	if err != nil {
		c.Logger().Errorf("[host-logs-health] %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch log health summary"})
	}

	return c.JSON(http.StatusOK, data)
}

func (h *HostDashboardHandler) Logs(c echo.Context) error {
	claims, ok := getClaims(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthenticated"})
	}

	machineID := c.Param("machineId")
	if machineID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing machine id"})
	}
	host, err := h.hostRepo.FindByTenantMachineCtx(c.Request().Context(), claims.TenantID, machineID)
	if err != nil || host == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "host not found"})
	}
	tenant, err := h.tenantRepo.FindByIDCtx(c.Request().Context(), claims.TenantID)
	if err != nil || tenant == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "tenant not found"})
	}

	startNs, err := strconv.ParseInt(c.QueryParam("start"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start param"})
	}
	endNs, err := strconv.ParseInt(c.QueryParam("end"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end param"})
	}

	page := 1
	if p := c.QueryParam("page"); p != "" {
		if v, err2 := strconv.Atoi(p); err2 == nil && v > 0 {
			page = v
		}
	}
	pageSize := 100
	if ps := c.QueryParam("page_size"); ps != "" {
		if v, err2 := strconv.Atoi(ps); err2 == nil && v > 0 && v <= 500 {
			pageSize = v
		}
	}

	result, err := h.logsSvc.GetLogs(
		c.Request().Context(),
		tenant.UUID,
		machineID,
		startNs,
		endNs,
		c.QueryParam("priority"),
		c.QueryParam("unit"),
		c.QueryParam("identifier"),
		c.QueryParam("search"),
		page,
		pageSize,
	)
	if err != nil {
		c.Logger().Errorf("[host-logs] fetch failed: %v", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch logs"})
	}

	return c.JSON(http.StatusOK, result)
}
