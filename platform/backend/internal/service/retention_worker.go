// internal/service/retention_worker.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
	"gorm.io/gorm"
)

type RetentionWorker struct {
	cfg             *config.Config
	db              *gorm.DB
	subscriptionRepo *repository.SubscriptionRepository
	tenantRepo      *repository.TenantRepository
	httpClient      *http.Client
}

func NewRetentionWorker(cfg *config.Config, db *gorm.DB, subscriptionRepo *repository.SubscriptionRepository, tenantRepo *repository.TenantRepository) *RetentionWorker {
	return &RetentionWorker{
		cfg:             cfg,
		db:              db,
		subscriptionRepo: subscriptionRepo,
		tenantRepo:      tenantRepo,
		httpClient:      &http.Client{Timeout: 30 * time.Second},
	}
}

func (w *RetentionWorker) StartRetentionCleaner(ctx context.Context, interval time.Duration) {
	log.Printf("[retention-worker] starting retention cleaner, interval=%v", interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				w.cleanupRetention(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (w *RetentionWorker) cleanupRetention(ctx context.Context) {
	var subs []domain.TenantSubscription
	if err := w.db.WithContext(ctx).Preload("Plan").Preload("Tenant").Where("status = ?", "active").Find(&subs).Error; err != nil {
		log.Printf("[retention-worker] error fetching subscriptions: %v", err)
		return
	}

	for _, sub := range subs {
		if sub.Plan.ID == 0 || sub.Tenant.UUID == "" {
			continue
		}

		logCutoff := time.Now().Add(-time.Duration(sub.Plan.LogRetentionDays) * 24 * time.Hour)
		metricsCutoff := time.Now().Add(-time.Duration(sub.Plan.MetricsRetentionDays) * 24 * time.Hour)

		w.deleteOldLogs(ctx, sub.Tenant.UUID, logCutoff)
		w.deleteOldMetrics(ctx, sub.Tenant.ID, metricsCutoff)
	}
}

func (w *RetentionWorker) deleteOldLogs(ctx context.Context, tenantUUID string, cutoff time.Time) {
	if tenantUUID == "" {
		return
	}

	apiKey := w.lookupAPIKeyByUUID(ctx, tenantUUID)

	w.deleteOldLogsForScope(ctx, tenantUUID, apiKey, cutoff)
}

func (w *RetentionWorker) lookupAPIKeyByUUID(ctx context.Context, tenantUUID string) string {
	var apiKey string
	if err := w.db.WithContext(ctx).Model(&domain.Tenant{}).Where("uuid = ?", tenantUUID).Pluck("api_key", &apiKey).Error; err != nil {
		log.Printf("[retention-worker] api key lookup error for uuid=%s: %v", tenantUUID, err)
	}
	return apiKey
}

func (w *RetentionWorker) deleteOldLogsForScope(ctx context.Context, tenantScopeUUID, tenantAPIKey string, cutoff time.Time) {
	if tenantScopeUUID == "" {
		return
	}

	// Tenant isolation in Loki is enforced by the X-Scope-OrgID header (the
	// tenant UUID), NOT by any log label. The OTLP pipeline only indexes a fixed
	// set of labels (job/service_name/host_id/host_name/os_type — see loki.yaml
	// otlp_config.resource_attributes). `tenant_id` is structured metadata, so a
	// selector like {tenant_id="..."} matches ZERO streams and Loki silently
	// no-ops the delete (it still returns 204). We therefore match all of the
	// tenant's streams inside its own org scope. Optionally refine to the exact
	// tenant by structured-metadata filter so we never touch another org's rows.
	deleteURL := "http://" + w.cfg.LokiHost + ":" + w.cfg.LokiPort + "/loki/api/v1/delete"
	q := url.Values{}
	query := `{job=~".+"}`
	if tenantAPIKey != "" {
		query = fmt.Sprintf(`{job=~".+"} | tenant_id="%s"`, tenantAPIKey)
	}
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(cutoff.Unix(), 10))
	q.Set("end", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deleteURL+"?"+q.Encode(), nil)
	if err != nil {
		log.Printf("[retention-worker] loki delete request build error for tenant %s: %v", tenantScopeUUID, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Scope-OrgID", tenantScopeUUID)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		log.Printf("[retention-worker] loki delete request error for tenant %s: %v", tenantScopeUUID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[retention-worker] loki delete unexpected status %d for tenant %s: %s", resp.StatusCode, tenantScopeUUID, string(body))
	} else {
		log.Printf("[retention-worker] loki delete executed for tenant %s before %s", tenantScopeUUID, cutoff.Format(time.RFC3339))
	}
}

// deleteOldMetrics removes metrics for a tenant whose entire series is older
// than the retention cutoff. VictoriaMetrics' delete_series API does NOT accept
// start/end time ranges (it rejects them with "args aren't supported"), so
// firing delete_series with match[]={__name__=~".+"} would wipe every sample of
// the tenant (including recent, in-retention data) — that's what emptied all
// tenants before. We therefore:
//   1. list the series that have data BEFORE the cutoff (old window)
//   2. list the series that have data AFTER the cutoff (recent window)
//   3. delete only the old-window series that have NO recent data
func (w *RetentionWorker) deleteOldMetrics(ctx context.Context, tenantID uint, cutoff time.Time) {
	baseURL := "http://" + w.cfg.VMSelectHost + ":" + w.cfg.VMSelectPort
	selectURL := baseURL + "/select/" + strconv.FormatUint(uint64(tenantID), 10) + "/prometheus/api/v1/series"
	deleteURL := baseURL + "/delete/" + strconv.FormatUint(uint64(tenantID), 10) + "/prometheus/api/v1/admin/tsdb/delete_series"

	oldSeries, err := w.vmSeriesInRange(ctx, selectURL, 0, cutoff.Unix())
	if err != nil {
		log.Printf("[retention-worker] vm old-window series query error for tenant %d: %v", tenantID, err)
		return
	}
	recentSeries, err := w.vmSeriesInRange(ctx, selectURL, cutoff.Unix(), time.Now().Unix())
	if err != nil {
		log.Printf("[retention-worker] vm recent-window series query error for tenant %d: %v", tenantID, err)
		return
	}

	recentSet := make(map[string]struct{}, len(recentSeries))
	for _, s := range recentSeries {
		recentSet[s] = struct{}{}
	}
	var stale []string
	for _, s := range oldSeries {
		if _, ok := recentSet[s]; !ok {
			stale = append(stale, s)
		}
	}

	if len(stale) == 0 {
		log.Printf("[retention-worker] vm no stale series older than %s for tenant %d", cutoff.Format(time.RFC3339), tenantID)
		return
	}

	for _, matcher := range stale {
		form := url.Values{}
		form.Set("match[]", matcher)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, deleteURL, strings.NewReader(form.Encode()))
		if err != nil {
			log.Printf("[retention-worker] vm delete request build error for tenant %d: %v", tenantID, err)
			return
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := w.httpClient.Do(req)
		if err != nil {
			log.Printf("[retention-worker] vm delete request error for tenant %d: %v", tenantID, err)
			return
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("[retention-worker] vm delete unexpected status %d for tenant %d (%s): %s", resp.StatusCode, tenantID, matcher, string(body))
		}
	}
	log.Printf("[retention-worker] vm delete executed for %d stale series older than %s for tenant %d", len(stale), cutoff.Format(time.RFC3339), tenantID)
}

// vmSeriesInRange returns the distinct JSON label-sets (marshal-stable) for
// series holding data within [start, end] (unix seconds).
func (w *RetentionWorker) vmSeriesInRange(ctx context.Context, selectURL string, start, end int64) ([]string, error) {
	q := url.Values{}
	q.Set("match[]", `{__name__=~".+"}`)
	if start == 0 {
		q.Set("start", strconv.FormatInt((time.Now().Unix()-365*24*3600), 10))
	} else {
		q.Set("start", strconv.FormatInt(start, 10))
	}
	q.Set("end", strconv.FormatInt(end, 10))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, selectURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		Status string              `json:"status"`
		Data   []map[string]string `json:"data"`
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.Data))
	for _, labels := range payload.Data {
		b, err := json.Marshal(labels)
		if err != nil {
			continue
		}
		out = append(out, `{`+string(b[1:len(b)-1])+`}`)
	}
	return out, nil
}
