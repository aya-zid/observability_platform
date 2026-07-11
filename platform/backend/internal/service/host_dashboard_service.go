package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// ─── Response types ──────────────────────────────────────────────────────────

type HostSummaryCards struct {
	CPUUsagePercent   *float64 `json:"cpu_usage_percent,omitempty"`
	MemoryUsedPercent *float64 `json:"memory_used_percent,omitempty"`
	DiskUsedPercent   *float64 `json:"disk_used_percent,omitempty"`
	LoadAverage1m     *float64 `json:"load_average_1m,omitempty"`
	NetworkInRate     *float64 `json:"network_in_rate,omitempty"`
	NetworkOutRate    *float64 `json:"network_out_rate,omitempty"`
	SwapUsedPercent   *float64 `json:"swap_used_percent,omitempty"`
}

type HostCPUData struct {
	Overall   []TimePoint `json:"overall"`
	Breakdown []TimePoint `json:"breakdown"`
	PerCPU    []TimePoint `json:"per_cpu"`
	LoadAvg   []TimePoint `json:"load_avg"`
	Frequency []TimePoint `json:"frequency"`
}

type HostMemoryData struct {
	Usage     []TimePoint `json:"usage"`
	Percent   []TimePoint `json:"percent"`
	Avail     []TimePoint `json:"avail"`
	SwapUsage []TimePoint `json:"swap_usage"`
	Faults    []TimePoint `json:"faults"`
	SwapIO    []TimePoint `json:"swap_io"`
}

type HostDiskData struct {
	Utilization []TimePoint `json:"utilization"`
	Space       []TimePoint `json:"space"`
	IO          []TimePoint `json:"io"`
	Ops         []TimePoint `json:"ops"`
	Latency     []TimePoint `json:"latency"`
	QueueDepth  []TimePoint `json:"queue_depth"`
	InodeUsage  []TimePoint `json:"inode_usage"`
}

type HostNetworkData struct {
	Throughput []TimePoint `json:"throughput"`
	Packets    []TimePoint `json:"packets"`
	Errors     []TimePoint `json:"errors"`
	Dropped    []TimePoint `json:"dropped"`
	TCP        []TimePoint `json:"tcp"`
	Conntrack  []TimePoint `json:"conntrack"`
}

type HostProcessData struct {
	ByState      []TimePoint `json:"by_state"`
	CreationRate *float64    `json:"creation_rate,omitempty"`
	ZombieCount  *float64    `json:"zombie_count,omitempty"`
}

// ─── Service ─────────────────────────────────────────────────────────────────

type HostDashboardService struct {
	cfg        *config.Config
	redis      *redis.Client
	httpClient *http.Client
}

func NewHostDashboardService(cfg *config.Config, redisClient *redis.Client) *HostDashboardService {
	return &HostDashboardService{
		cfg:        cfg,
		redis:      redisClient,
		httpClient: &http.Client{Timeout: 45 * time.Second},
	}
}

func (s *HostDashboardService) GetSummaryCards(ctx context.Context, tenantID uint, machineID string, timeSec int64) (*HostSummaryCards, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)

	queries := map[string]string{
		"cpu":    fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{%s,state="idle"})) * 100`, hf),
		"mem":    fmt.Sprintf(`sum(system_memory_usage_bytes{%s,state="used"}) / sum(system_memory_usage_bytes{%s,state=~"used|free|buffered|cached"}) * 100`, hf, hf),
		"disk":   fmt.Sprintf(`system_filesystem_utilization_ratio{%s,mountpoint="/"} * 100`, hf),
		"load":   fmt.Sprintf(`system_cpu_load_average_1m{%s}`, hf),
		"netin":  fmt.Sprintf(`sum(rate(system_network_io_bytes_total{%s,direction="receive"}[5m]))`, hf),
		"netout": fmt.Sprintf(`sum(rate(system_network_io_bytes_total{%s,direction="transmit"}[5m]))`, hf),
		"swap":   fmt.Sprintf(`sum(system_paging_usage_bytes{%s,state="used"}) / clamp_min(sum(system_paging_usage_bytes{%s,state=~"used|free"}), 1) * 100`, hf, hf),
	}

	cacheKey := hostDashboardCacheKey("summary", tenantID, machineID, timeSec, timeSec, "")
	return cachedFetch(ctx, s.redis, cacheKey, 15*time.Second, func() (*HostSummaryCards, error) {
		results := s.runInstant(ctx, tenantID, timeSec, queries)
		cards := &HostSummaryCards{
			CPUUsagePercent:   vmInstValue(results["cpu"]),
			MemoryUsedPercent: vmInstValue(results["mem"]),
			DiskUsedPercent:   vmInstValue(results["disk"]),
			LoadAverage1m:     vmInstValue(results["load"]),
			NetworkInRate:     vmInstValue(results["netin"]),
			NetworkOutRate:    vmInstValue(results["netout"]),
			SwapUsedPercent:   vmInstValue(results["swap"]),
		}
		return cards, nil
	})
}

func (s *HostDashboardService) GetCPUMetrics(ctx context.Context, tenantID uint, machineID string, startSec, endSec int64, step string) (*HostCPUData, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)

	queries := map[string]string{
		"overall":   fmt.Sprintf(`(1 - avg(system_cpu_utilization_ratio{%s,state="idle"})) * 100`, hf),
		"breakdown": fmt.Sprintf(`avg by (state) (system_cpu_utilization_ratio{%s,state!="idle"}) * 100`, hf),
		"percpu":    fmt.Sprintf(`(1 - system_cpu_utilization_ratio{%s,state="idle"}) * 100`, hf),
		"load1m":    fmt.Sprintf(`system_cpu_load_average_1m{%s}`, hf),
		"load5m":    fmt.Sprintf(`system_cpu_load_average_5m{%s}`, hf),
		"load15m":   fmt.Sprintf(`system_cpu_load_average_15m{%s}`, hf),
		"freq":      fmt.Sprintf(`avg(system_cpu_frequency_hertz{%s})`, hf),
	}

	cacheKey := hostDashboardCacheKey("cpu", tenantID, machineID, startSec, endSec, step)
	return cachedFetch(ctx, s.redis, cacheKey, 30*time.Second, func() (*HostCPUData, error) {
		results := s.runRange(ctx, tenantID, startSec, endSec, step, queries)
		data := &HostCPUData{
			Overall:   singleToTimePoints(results["overall"]),
			Breakdown: hostMergeVMSeriesByLabel(results["breakdown"], "state"),
			PerCPU:    hostMergeVMSeriesByLabel(results["percpu"], "cpu"),
			Frequency: singleToTimePoints(results["freq"]),
		}
		data.LoadAvg = hostMergeLoadSeries(results["load1m"], results["load5m"], results["load15m"])
		return data, nil
	})
}

func (s *HostDashboardService) GetMemoryMetrics(ctx context.Context, tenantID uint, machineID string, startSec, endSec int64, step string) (*HostMemoryData, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)

	queries := map[string]string{
		"usage":  fmt.Sprintf(`system_memory_usage_bytes{%s}`, hf),
		"pct":    fmt.Sprintf(`sum(system_memory_usage_bytes{%s,state="used"}) / sum(system_memory_usage_bytes{%s,state=~"used|free|buffered|cached"}) * 100`, hf, hf),
		"avail":  fmt.Sprintf(`sum(system_memory_usage_bytes{%s,state=~"free|cached|buffered"})`, hf),
		"swap":   fmt.Sprintf(`system_paging_usage_bytes{%s}`, hf),
		"faults": fmt.Sprintf(`rate(system_paging_faults_total{%s}[5m])`, hf),
		"swapio": fmt.Sprintf(`rate(system_paging_operations_total{%s,type="major"}[5m])`, hf),
	}

	cacheKey := hostDashboardCacheKey("memory", tenantID, machineID, startSec, endSec, step)
	return cachedFetch(ctx, s.redis, cacheKey, 30*time.Second, func() (*HostMemoryData, error) {
		results := s.runRange(ctx, tenantID, startSec, endSec, step, queries)
		data := &HostMemoryData{
			Usage:     hostMergeVMSeriesByLabel(results["usage"], "state"),
			Percent:   singleToTimePoints(results["pct"]),
			Avail:     singleToTimePoints(results["avail"]),
			SwapUsage: hostMergeVMSeriesByLabel(results["swap"], "state"),
			Faults:    hostMergeVMSeriesByLabel(results["faults"], "type"),
			SwapIO:    hostMergeVMSeriesByLabel(results["swapio"], "direction"),
		}
		return data, nil
	})
}

func (s *HostDashboardService) GetDiskMetrics(ctx context.Context, tenantID uint, machineID string, startSec, endSec int64, step string) (*HostDiskData, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)
	parentDiskFilter := fmt.Sprintf(`%s,device=~"sd[a-z]+|vd[a-z]+|xvd[a-z]+|nvme[0-9]+n[0-9]+"`, hf)

	queries := map[string]string{
		"util":    fmt.Sprintf(`system_filesystem_utilization_ratio{%s,mountpoint!~"/boot.*|/run.*|/dev.*"} * 100`, hf),
		"space":   fmt.Sprintf(`system_filesystem_usage_bytes{%s,mountpoint!~"/boot.*|/run.*|/dev.*"}`, hf),
		"io":      fmt.Sprintf(`rate(system_disk_io_bytes_total{%s}[5m])`, parentDiskFilter),
		"ops":     fmt.Sprintf(`rate(system_disk_merged_total{%s}[5m])`, parentDiskFilter),
		"latency": fmt.Sprintf(`sum(rate(system_disk_operation_time_seconds_total{%s}[5m])) by (direction) / clamp_min(sum(rate(system_disk_merged_total{%s}[5m])) by (direction), 1) * 1000`, parentDiskFilter, parentDiskFilter),
		"queue":   fmt.Sprintf(`sum(system_disk_pending_operations{%s})`, parentDiskFilter),
		"inode":   fmt.Sprintf(`system_filesystem_inodes_usage{%s,state="used",mountpoint="/"} / (system_filesystem_inodes_usage{%s,state="used",mountpoint="/"} + system_filesystem_inodes_usage{%s,state="free",mountpoint="/"}) * 100`, hf, hf, hf),
	}

	cacheKey := hostDashboardCacheKey("disk", tenantID, machineID, startSec, endSec, step)
	return cachedFetch(ctx, s.redis, cacheKey, 60*time.Second, func() (*HostDiskData, error) {
		results := s.runRange(ctx, tenantID, startSec, endSec, step, queries)
		data := &HostDiskData{
			Utilization: hostMergeVMSeriesByLabel(results["util"], "mountpoint"),
			Space:       hostMergeMountpointState(results["space"]),
			IO:          hostSumByDirection(results["io"]),
			Ops:         hostSumByDirection(results["ops"]),
			Latency:     hostMergeVMSeriesByLabel(results["latency"], "direction"),
			QueueDepth:  singleToTimePoints(results["queue"]),
			InodeUsage:  singleToTimePoints(results["inode"]),
		}
		return data, nil
	})
}

func (s *HostDashboardService) GetNetworkMetrics(ctx context.Context, tenantID uint, machineID string, startSec, endSec int64, step string) (*HostNetworkData, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)

	queries := map[string]string{
		"throughput": fmt.Sprintf(`rate(system_network_io_bytes_total{%s,device!~"veth.*|lo"}[5m])`, hf),
		"packets":    fmt.Sprintf(`rate(system_network_packets_total{%s,device!~"veth.*|lo"}[5m])`, hf),
		"errors":     fmt.Sprintf(`rate(system_network_errors_total{%s,device!~"veth.*|lo"}[5m])`, hf),
		"dropped":    fmt.Sprintf(`rate(system_network_dropped_total{%s,device!~"veth.*|lo"}[5m])`, hf),
		"tcp":        fmt.Sprintf(`system_network_connections{%s}`, hf),
		"conntrack":  fmt.Sprintf(`system_network_conntrack_count{%s} / system_network_conntrack_max{%s} * 100`, hf, hf),
	}

	cacheKey := hostDashboardCacheKey("network", tenantID, machineID, startSec, endSec, step)
	return cachedFetch(ctx, s.redis, cacheKey, 30*time.Second, func() (*HostNetworkData, error) {
		results := s.runRange(ctx, tenantID, startSec, endSec, step, queries)
		data := &HostNetworkData{
			Throughput: hostSumByDirection(results["throughput"]),
			Packets:    hostSumByDirection(results["packets"]),
			Errors:     singleToTimePoints(results["errors"]),
			Dropped:    singleToTimePoints(results["dropped"]),
			TCP:        hostMergeVMSeriesByLabel(results["tcp"], "state"),
			Conntrack:  singleToTimePoints(results["conntrack"]),
		}
		return data, nil
	})
}

func (s *HostDashboardService) GetProcessMetrics(ctx context.Context, tenantID uint, machineID string, startSec, endSec int64, step string) (*HostProcessData, error) {
	hf := fmt.Sprintf(`host_id="%s"`, machineID)

	rangeQ := fmt.Sprintf(`system_processes_count{%s}`, hf)
	instZombie := fmt.Sprintf(`system_processes_count{%s,status="zombies"}`, hf)
	instCreate := fmt.Sprintf(`rate(system_processes_created_total{%s}[5m])`, hf)

	cacheKey := hostDashboardCacheKey("process", tenantID, machineID, startSec, endSec, step)
	return cachedFetch(ctx, s.redis, cacheKey, 30*time.Second, func() (*HostProcessData, error) {
		var wg sync.WaitGroup
		var byState []VMSeries
		var zombie, create []VMSeries
		var e1, e2, e3 error

		wg.Add(3)
		go func() { defer wg.Done(); byState, e1 = s.vmQueryRange(ctx, tenantID, rangeQ, startSec, endSec, step) }()
		go func() { defer wg.Done(); zombie, e2 = s.vmQueryInstant(ctx, tenantID, instZombie, endSec) }()
		go func() { defer wg.Done(); create, e3 = s.vmQueryInstant(ctx, tenantID, instCreate, endSec) }()
		wg.Wait()

		if e1 != nil || e2 != nil || e3 != nil {
			return nil, fmt.Errorf("process query failed: range=%v zombie=%v create=%v", e1, e2, e3)
		}

		data := &HostProcessData{
			ByState:      hostMergeVMSeriesByLabel(byState, "status"),
			ZombieCount:  vmInstValue(zombie),
			CreationRate: vmInstValue(create),
		}
		return data, nil
	})
}

// ─── Generic cached fetch ────────────────────────────────────────────────────

func hostDashboardCacheKey(kind string, tenantID uint, machineID string, startSec, endSec int64, step string) string {
	return fmt.Sprintf("host:%s:%d:%s:%d:%d:%s", kind, tenantID, machineID, startSec, endSec, step)
}

func cachedFetch[T any](ctx context.Context, redis *redis.Client, cacheKey string, ttl time.Duration, fetch func() (*T, error)) (*T, error) {
	if redis != nil {
		if cached, err := redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result T
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}
	result, err := fetch()
	if err != nil {
		return nil, err
	}
	if redis != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = redis.Set(ctx, cacheKey, b, ttl).Err()
		}
	}
	return result, nil
}

// ─── Parallel query helpers ──────────────────────────────────────────────────

type vmResult struct {
	data []VMSeries
	err  error
}

func (s *HostDashboardService) runInstant(ctx context.Context, tenantID uint, timeSec int64, queries map[string]string) map[string][]VMSeries {
	type namedResult struct {
		name string
		vmResult
	}
	ch := make(chan namedResult, len(queries))
	for name, q := range queries {
		go func(n, q string) {
			d, e := s.vmQueryInstant(ctx, tenantID, q, timeSec)
			ch <- namedResult{n, vmResult{d, e}}
		}(name, q)
	}
	results := map[string][]VMSeries{}
	for i := 0; i < len(queries); i++ {
		r := <-ch
		if r.err == nil {
			results[r.name] = r.data
		}
	}
	return results
}

func (s *HostDashboardService) runRange(ctx context.Context, tenantID uint, startSec, endSec int64, step string, queries map[string]string) map[string][]VMSeries {
	type namedResult struct {
		name string
		vmResult
	}
	ch := make(chan namedResult, len(queries))
	for name, q := range queries {
		go func(n, q string) {
			d, e := s.vmQueryRange(ctx, tenantID, q, startSec, endSec, step)
			ch <- namedResult{n, vmResult{d, e}}
		}(name, q)
	}
	results := map[string][]VMSeries{}
	for i := 0; i < len(queries); i++ {
		r := <-ch
		if r.err == nil {
			results[r.name] = r.data
		}
	}
	return results
}

// ─── VM query helpers ────────────────────────────────────────────────────────

func (s *HostDashboardService) vmQueryRange(ctx context.Context, tenantID uint, query string, startSec, endSec int64, step string) ([]VMSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query_range", s.cfg.VMSelectHost, s.cfg.VMSelectPort, tenantID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startSec, 10))
	params.Set("end", strconv.FormatInt(endSec, 10))
	params.Set("step", step)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseVMMatrixResponse(body)
}

func (s *HostDashboardService) vmQueryInstant(ctx context.Context, tenantID uint, query string, timeSec int64) ([]VMSeries, error) {
	upstream := fmt.Sprintf("http://%s:%s/select/%d/prometheus/api/v1/query", s.cfg.VMSelectHost, s.cfg.VMSelectPort, tenantID)
	params := url.Values{}
	params.Set("query", query)
	params.Set("time", strconv.FormatInt(timeSec, 10))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseVMInstantResponse(body)
}

// ─── Data transformation helpers ─────────────────────────────────────────────

func vmInstValue(series []VMSeries) *float64 {
	if len(series) == 0 || len(series[0].Values) == 0 {
		return nil
	}
	v := series[0].Values[0][1]
	return &v
}

func singleToTimePoints(series []VMSeries) []TimePoint {
	if len(series) == 0 || len(series[0].Values) == 0 {
		return nil
	}
	pts := make([]TimePoint, 0, len(series[0].Values))
	for _, v := range series[0].Values {
		pts = append(pts, TimePoint{"time": v[0], "value": v[1]})
	}
	sortTimePoints(pts)
	return pts
}

func hostMergeVMSeriesByLabel(series []VMSeries, labelKey string) []TimePoint {
	timeMap := map[float64]map[string]float64{}
	for _, s := range series {
		label := s.Metric[labelKey]
		if label == "" {
			label = "unknown"
		}
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]][label] = v[1]
		}
	}
	pts := make([]TimePoint, 0, len(timeMap))
	for t, vals := range timeMap {
		pt := TimePoint{"time": t}
		for k, v := range vals {
			pt[k] = v
		}
		pts = append(pts, pt)
	}
	sortTimePoints(pts)
	return pts
}

func hostMergeLoadSeries(load1, load5, load15 []VMSeries) []TimePoint {
	timeMap := map[float64]map[string]float64{}
	for _, s := range load1 {
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]]["1m"] = v[1]
		}
	}
	for _, s := range load5 {
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]]["5m"] = v[1]
		}
	}
	for _, s := range load15 {
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]]["15m"] = v[1]
		}
	}
	pts := make([]TimePoint, 0, len(timeMap))
	for t, vals := range timeMap {
		pt := TimePoint{"time": t}
		for k, v := range vals {
			pt[k] = v
		}
		pts = append(pts, pt)
	}
	sortTimePoints(pts)
	return pts
}

func hostSumByDirection(series []VMSeries) []TimePoint {
	timeMap := map[float64]map[string]float64{}
	for _, s := range series {
		dir := s.Metric["direction"]
		if dir == "" {
			dir = "unknown"
		}
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]][dir] = timeMap[v[0]][dir] + v[1]
		}
	}
	pts := make([]TimePoint, 0, len(timeMap))
	for t, vals := range timeMap {
		pt := TimePoint{"time": t}
		for k, v := range vals {
			pt[k] = v
		}
		pts = append(pts, pt)
	}
	sortTimePoints(pts)
	return pts
}

func hostMergeMountpointState(series []VMSeries) []TimePoint {
	timeMap := map[float64]map[string]float64{}
	for _, s := range series {
		mnt := s.Metric["mountpoint"]
		state := s.Metric["state"]
		if mnt == "" {
			mnt = "unknown"
		}
		key := mnt + "_" + state
		for _, v := range s.Values {
			if _, ok := timeMap[v[0]]; !ok {
				timeMap[v[0]] = map[string]float64{}
			}
			timeMap[v[0]][key] = v[1]
		}
	}
	pts := make([]TimePoint, 0, len(timeMap))
	for t, vals := range timeMap {
		pt := TimePoint{"time": t}
		for k, v := range vals {
			pt[k] = v
		}
		pts = append(pts, pt)
	}
	sortTimePoints(pts)
	return pts
}
