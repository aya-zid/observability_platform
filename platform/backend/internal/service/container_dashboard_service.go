package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

type ContainerDashboardSummary struct {
	Up              *float64 `json:"up,omitempty"`
	CPUPercent      *float64 `json:"cpu_percent,omitempty"`
	MemUsageBytes   *float64 `json:"mem_usage_bytes,omitempty"`
	MemLimitBytes   *float64 `json:"mem_limit_bytes,omitempty"`
	MemPercent      *float64 `json:"mem_percent,omitempty"`
	NetRxBytesTotal *float64 `json:"net_rx_bytes_total,omitempty"`
	NetTxBytesTotal *float64 `json:"net_tx_bytes_total,omitempty"`
	BlockRBytes     *float64 `json:"block_read_bytes_total,omitempty"`
	BlockWBytes     *float64 `json:"block_write_bytes_total,omitempty"`
	PIDs            *float64 `json:"pids,omitempty"`

	CPUTimeseries      []TimePoint `json:"cpu_timeseries"`
	MemTimeseries      []TimePoint `json:"mem_timeseries"`
	NetTimeseries      []TimePoint `json:"net_timeseries"`
	BlockTimeseries    []TimePoint `json:"block_timeseries"`
	PidsTimeseries     []TimePoint `json:"pids_timeseries"`
	MemPercentTimeseries []TimePoint `json:"mem_percent_timeseries"`
	MemLimitTimeseries   []TimePoint `json:"mem_limit_timeseries"`
}

type ContainerLogEvent struct {
	Timestamp string `json:"timestamp"`
	Line      string `json:"line"`
}

type ContainerLogsPage struct {
	Events   []ContainerLogEvent `json:"events"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	HasMore  bool                `json:"has_more"`
}

type ContainerDashboardService struct {
	base *NginxDashboardService
}

func NewContainerDashboardService(cfg *config.Config, redisClient *redis.Client) *ContainerDashboardService {
	return &ContainerDashboardService{
		base: NewNginxDashboardService(cfg, redisClient),
	}
}

func containerSafeName(name string) string {
	s := strings.ToLower(name)
	var result strings.Builder
	lastUnderscore := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
			lastUnderscore = false
		} else {
			if !lastUnderscore {
				result.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	trimmed := result.String()
	trimmed = strings.TrimRight(trimmed, "_")
	return strings.TrimLeft(trimmed, "_")
}

func (s *ContainerDashboardService) GetDashboardSummary(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	containerName string,
	startNs, endNs int64,
	step string,
	hostID string,
) (*ContainerDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)
	cacheKey := fmt.Sprintf("container:dash:%s:%s:%d:%d:%s", tenantUUID, containerName, startSec, endSec, step)

	if s.base.redis != nil {
		if cached, err := s.base.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result ContainerDashboardSummary
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	summary, err := s.fetchDashboard(ctx, tenantUUID, tenantID, containerName, startNs, endNs, step, hostID)
	if err != nil {
		return nil, err
	}

	if s.base.redis != nil {
		if b, err := json.Marshal(summary); err == nil {
			_ = s.base.redis.Set(ctx, cacheKey, b, dashboardCacheTTL).Err()
		}
	}

	return summary, nil
}

func (s *ContainerDashboardService) fetchDashboard(
	ctx context.Context,
	tenantUUID string,
	tenantID uint,
	containerName string,
	startNs, endNs int64,
	step string,
	hostID string,
) (*ContainerDashboardSummary, error) {
	startSec := startNs / int64(1e9)
	endSec := endNs / int64(1e9)

	safe := containerSafeName(containerName)
	prefix := fmt.Sprintf("container_%s", safe)

	type vR struct {
		data []VMSeries
		err  error
	}

	query := func(q string) vR {
		data, err := s.base.vmQueryRange(ctx, tenantID, q, startSec, endSec, step, hostID, "container")
		if err != nil {
			log.Printf("[container] vm query error: %v | query: %s", err, q)
		}
		return vR{data: data, err: err}
	}

	results := make(map[string]vR)
	var resultsMu sync.Mutex
	var wg sync.WaitGroup

	queries := map[string]string{
		"cpu":              prefix + "_cpu_percent",
		"mem_usage":        prefix + "_mem_usage_bytes",
		"mem_limit":        prefix + "_mem_limit_bytes",
		"mem_percent":      prefix + "_mem_percent",
		"net_rx":           "rate(" + prefix + "_net_rx_bytes_total[5m])",
		"net_tx":           "rate(" + prefix + "_net_tx_bytes_total[5m])",
		"block_read":       "rate(" + prefix + "_block_read_bytes_total[5m])",
		"block_write":      "rate(" + prefix + "_block_write_bytes_total[5m])",
		"pids":             prefix + "_pids",
		"net_rx_total":     prefix + "_net_rx_bytes_total",
		"net_tx_total":     prefix + "_net_tx_bytes_total",
		"block_read_total": prefix + "_block_read_bytes_total",
		"block_write_total": prefix + "_block_write_bytes_total",
	}

	for name, q := range queries {
		name, q := name, q
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := query(q)
			resultsMu.Lock()
			results[name] = result
			resultsMu.Unlock()
		}()
	}

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	summary := &ContainerDashboardSummary{
		CPUPercent:      latestVMValue(results["cpu"].data),
		MemUsageBytes:   latestVMValue(results["mem_usage"].data),
		MemLimitBytes:   latestVMValue(results["mem_limit"].data),
		MemPercent:      latestVMValue(results["mem_percent"].data),
		NetRxBytesTotal: latestVMValue(results["net_rx_total"].data),
		NetTxBytesTotal: latestVMValue(results["net_tx_total"].data),
		BlockRBytes:     latestVMValue(results["block_read_total"].data),
		BlockWBytes:     latestVMValue(results["block_write_total"].data),
		PIDs:            latestVMValue(results["pids"].data),
	}

	summary.CPUTimeseries = singleVMSeries(results["cpu"].data, "cpu_percent")
	summary.MemTimeseries = singleVMSeries(results["mem_usage"].data, "used")
	summary.MemPercentTimeseries = singleVMSeries(results["mem_percent"].data, "percent")
	summary.MemLimitTimeseries = singleVMSeries(results["mem_limit"].data, "limit")

	summary.NetTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"rx": results["net_rx"].data,
		"tx": results["net_tx"].data,
	})
	summary.BlockTimeseries = mergeNamedVMSeries(map[string][]VMSeries{
		"read":  results["block_read"].data,
		"write": results["block_write"].data,
	})
	summary.PidsTimeseries = singleVMSeries(results["pids"].data, "pids")

	return summary, nil
}

func (s *ContainerDashboardService) GetLogs(
	ctx context.Context,
	tenantUUID string,
	containerName string,
	startNs, endNs int64,
	search string,
	page int,
	pageSize int,
	hostID string,
) (*ContainerLogsPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 100
	}

	baseSelector := containerLokiSelector(containerName, hostID)
	lokiQuery := baseSelector
	if search != "" {
		lokiQuery += fmt.Sprintf(` |~ "(?i)%s"`, lokiLabelValue(search))
	}

	entries, err := s.lokiQueryAllLogEntries(ctx, tenantUUID, lokiQuery, startNs, endNs)
	if err != nil {
		return nil, fmt.Errorf("loki fetch: %w", err)
	}

	events := make([]ContainerLogEvent, 0, len(entries))
	for _, e := range entries {
		ts := e.TimestampNs
		if nanos, err2 := strconv.ParseInt(ts, 10, 64); err2 == nil {
			ts = time.Unix(0, nanos).Format(time.RFC3339)
		}
		events = append(events, ContainerLogEvent{
			Timestamp: ts,
			Line:      e.Line,
		})
	}

	total := len(events)
	offset := (page - 1) * pageSize
	if offset >= total {
		return &ContainerLogsPage{
			Events:   []ContainerLogEvent{},
			Total:    total,
			Page:     page,
			PageSize: pageSize,
			HasMore:  false,
		}, nil
	}

	end := offset + pageSize
	if end > total {
		end = total
	}

	return &ContainerLogsPage{
		Events:   events[offset:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  end < total,
	}, nil
}

func (s *ContainerDashboardService) lokiQueryAllLogEntries(
	ctx context.Context,
	tenantUUID string,
	query string,
	startNs, endNs int64,
) ([]LokiLogEntry, error) {
	const batchSize = 2000
	const maxTotal = 50000
	var all []LokiLogEntry
	seen := make(map[string]struct{}, 4096)

	cursorEnd := endNs
	for {
		batch, err := s.base.lokiQueryLogEntries(ctx, tenantUUID, query, startNs, cursorEnd, batchSize)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		minTs := cursorEnd
		for _, e := range batch {
			key := e.TimestampNs + "\x00" + e.Line
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			all = append(all, e)
			if len(all) >= maxTotal {
				break
			}
			if ts, err2 := strconv.ParseInt(e.TimestampNs, 10, 64); err2 == nil && ts < minTs {
				minTs = ts
			}
		}

		if len(all) >= maxTotal {
			break
		}
		if minTs <= startNs || minTs >= cursorEnd {
			break
		}
		cursorEnd = minTs - 1
		if len(batch) < batchSize {
			break
		}
	}

	sort.SliceStable(all, func(i, j int) bool {
		ti, _ := strconv.ParseInt(all[i].TimestampNs, 10, 64)
		tj, _ := strconv.ParseInt(all[j].TimestampNs, 10, 64)
		return ti < tj
	})

	sortLokiLogEntriesDesc(all)
	return all, nil
}

func containerLokiSelector(containerName, hostID string) string {
	serviceName := "container_" + strings.TrimSpace(containerName)
	selector := fmt.Sprintf(`{service_name="%s"}`, lokiLabelValue(serviceName))
	if hostID != "" {
		selector += fmt.Sprintf(` | host_id="%s"`, lokiLabelValue(hostID))
	}
	return selector
}
