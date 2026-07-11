package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
)

// ─── Response types ───────────────────────────────────────────────────────────

type RTIntensityHeatmap struct {
	Paths     []string    `json:"paths"`
	Buckets   []string    `json:"buckets"`
	Matrix    [][]float64 `json:"matrix"`     // matrix[pathIdx][bucketIdx] = P95 RT in ms
	Counts    [][]int     `json:"counts"`     // counts[pathIdx][bucketIdx] = total request count in bucket
	P95Counts [][]int     `json:"p95_counts"` // p95_counts[pathIdx][bucketIdx] = requests at or below P95
}

type RTDistributionByStatus struct {
	TwoXX   RTStatusMetrics `json:"2xx"`
	ThreeXX RTStatusMetrics `json:"3xx"`
	FourXX  RTStatusMetrics `json:"4xx"`
	FiveXX  RTStatusMetrics `json:"5xx"`
}

type RTStatusMetrics struct {
	Avg             float64        `json:"avg"`
	P50             float64        `json:"p50"`
	P95             float64        `json:"p95"`
	P99             float64        `json:"p99"`
	Count           int            `json:"count"`
	P50Count        int            `json:"p50_count"`
	P95Count        int            `json:"p95_count"`
	P99Count        int            `json:"p99_count"`
	StatusBreakdown map[string]int `json:"status_breakdown,omitempty"`
}

// ─── Scatter types ────────────────────────────────────────────────────────────

type RTScatterPoint struct {
	Time           int64   `json:"time"`                   // unix milliseconds
	RT             float64 `json:"rt"`                     // response time in ms
	Status         int     `json:"status"`                 // HTTP status code
	Path           string  `json:"path"`                   // raw request path (empty for aggregate dots)
	Method         string  `json:"method"`                 // HTTP method (empty for aggregate dots)
	NormalizedPath string  `json:"normalized_path"`        // normalized path (empty for aggregate dots)
	IP             string  `json:"ip,omitempty"`           // client IP (individual dots only)
	IsAggregate    bool    `json:"is_aggregate"`           // true = aggregate dot
	Count          int     `json:"count,omitempty"`        // for aggregate dots: number of requests averaged
	WindowLabel    string  `json:"window_label,omitempty"` // e.g. "10:00–10:03"
}

// RTScatter is the top-level response for Chart 2.
// TotalRequests = sum of all individual points + sum of Count across all aggregate dots.
// This lets the UI verify no requests are silently dropped.
type RTScatter struct {
	Points        []RTScatterPoint `json:"points"`
	TotalRequests int              `json:"total_requests"`
}

// ─── Service ──────────────────────────────────────────────────────────────────

type NginxPerformanceService struct {
	cfg       *config.Config
	redis     *redis.Client
	dashboard *NginxDashboardService
}

func NewNginxPerformanceService(cfg *config.Config, redisClient *redis.Client) *NginxPerformanceService {
	return &NginxPerformanceService{
		cfg:       cfg,
		redis:     redisClient,
		dashboard: NewNginxDashboardService(cfg, redisClient),
	}
}

const (
	performanceCacheTTL = 30 * time.Second
	perfBucketSize      = 5 * time.Minute
	perfMaxHeatmapPaths = 20
)

// ─── Chart 1: RT Intensity Heatmap ───────────────────────────────────────────

func (s *NginxPerformanceService) GetRTIntensityHeatmap(
	ctx context.Context,
	tenantUUID, serviceName, domain string,
	startNs, endNs int64,
	hostID string,
) (*RTIntensityHeatmap, error) {
	cacheKey := fmt.Sprintf(
		"nginx:perf:heatmap:%s:%s:%s:%d:%d",
		tenantUUID, serviceName, domain,
		startNs/int64(time.Second),
		endNs/int64(time.Second),
	)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result RTIntensityHeatmap
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := s.fetchRTIntensityHeatmap(ctx, tenantUUID, serviceName, domain, startNs, endNs, hostID)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, performanceCacheTTL).Err()
		}
	}

	return result, nil
}

func (s *NginxPerformanceService) fetchRTIntensityHeatmap(
	ctx context.Context,
	tenantUUID, serviceName, domain string,
	startNs, endNs int64,
	hostID string,
) (*RTIntensityHeatmap, error) {
	if endNs <= startNs {
		return &RTIntensityHeatmap{}, nil
	}

	base := lokiSelector(serviceName, domain, "access", hostID)
	rangeSec := (endNs - startNs) / int64(time.Second)
	if rangeSec <= 0 {
		return &RTIntensityHeatmap{}, nil
	}

	// Step 1: find top-N normalized paths
	topPathsQ := fmt.Sprintf(
		`topk(%d, sum by (normalized_path) (count_over_time(%s | normalized_path!="" [%ds])))`,
		perfMaxHeatmapPaths,
		base,
		rangeSec,
	)

	topData, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, topPathsQ, endNs, 5000)
	if err != nil {
		return nil, fmt.Errorf("heatmap top paths query failed: %w", err)
	}

	paths := make([]string, 0, perfMaxHeatmapPaths)
	for _, series := range topData {
		if np := series.Metric["normalized_path"]; np != "" {
			paths = append(paths, np)
		}
	}
	if len(paths) == 0 {
		return &RTIntensityHeatmap{}, nil
	}

	// Step 2: build bucket labels using Tunis local time
	bucketCount := int((endNs - startNs + int64(perfBucketSize) - 1) / int64(perfBucketSize))
	if bucketCount <= 0 {
		bucketCount = 1
	}

	loc, _ := time.LoadLocation("Africa/Tunis")
	buckets := make([]string, bucketCount)
	for i := 0; i < bucketCount; i++ {
		bucketStartSec := startNs/int64(time.Second) + int64(i)*int64(perfBucketSize/time.Second)
		buckets[i] = time.Unix(bucketStartSec, 0).In(loc).Format("15:04")
	}

	// Step 3: P95 and request count per path
	matrix := make([][]float64, len(paths))
	counts := make([][]int, len(paths))
	p95Counts := make([][]int, len(paths))
	for i := range matrix {
		matrix[i] = make([]float64, bucketCount)
		counts[i] = make([]int, bucketCount)
		p95Counts[i] = make([]int, bucketCount)
	}

	type pathResult struct {
		index       int
		p95Values   [][2]float64
		countValues [][2]float64
		err         error
	}

	resultsCh := make(chan pathResult, len(paths))
	var wg sync.WaitGroup

	for i, path := range paths {
		wg.Add(1)
		go func(idx int, normalizedPath string) {
			defer wg.Done()
			q := fmt.Sprintf(
				`quantile_over_time(0.95, %s | normalized_path="%s" %s | unwrap rt [5m]) by ()`,
				base,
				lokiLabelValue(normalizedPath),
				rtParseExpr,
			)
			countQ := fmt.Sprintf(
				`sum(count_over_time(%s | normalized_path="%s" [5m]))`,
				base,
				lokiLabelValue(normalizedPath),
			)
			series, queryErr := s.dashboard.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, "5m", 5000)
			if queryErr != nil {
				resultsCh <- pathResult{index: idx, err: queryErr}
				return
			}
			countSeries, countErr := s.dashboard.lokiQueryRange(ctx, tenantUUID, countQ, startNs, endNs, "5m", 5000)
			if countErr != nil {
				resultsCh <- pathResult{index: idx, err: countErr}
				return
			}
			var p95Vals [][2]float64
			for _, s := range series {
				p95Vals = append(p95Vals, s.Values...)
			}
			var countVals [][2]float64
			for _, s := range countSeries {
				countVals = append(countVals, s.Values...)
			}
			resultsCh <- pathResult{index: idx, p95Values: p95Vals, countValues: countVals}
		}(i, path)
	}

	wg.Wait()
	close(resultsCh)

	for res := range resultsCh {
		if res.err != nil {
			return nil, fmt.Errorf("heatmap p95 path query failed: %w", res.err)
		}
		for _, pt := range res.p95Values {
			sampleEndNs := int64(pt[0]) * int64(time.Second)
			bucketStartNs := sampleEndNs - int64(perfBucketSize)
			offsetNs := bucketStartNs - startNs
			bucketIdx := int(offsetNs / int64(perfBucketSize))
			if bucketIdx < 0 || bucketIdx >= bucketCount {
				continue
			}
			matrix[res.index][bucketIdx] = pt[1] * 1000
		}
		for _, pt := range res.countValues {
			sampleEndNs := int64(pt[0]) * int64(time.Second)
			bucketStartNs := sampleEndNs - int64(perfBucketSize)
			offsetNs := bucketStartNs - startNs
			bucketIdx := int(offsetNs / int64(perfBucketSize))
			if bucketIdx < 0 || bucketIdx >= bucketCount {
				continue
			}
			counts[res.index][bucketIdx] = int(pt[1])
			p95Counts[res.index][bucketIdx] = (counts[res.index][bucketIdx]*95 + 99) / 100
		}
	}

	return &RTIntensityHeatmap{
		Paths:     paths,
		Buckets:   buckets,
		Matrix:    matrix,
		Counts:    counts,
		P95Counts: p95Counts,
	}, nil
}

// ─── Chart 3: RT Distribution by Status ──────────────────────────────────────

func (s *NginxPerformanceService) GetRTDistributionByStatus(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	domain string,
	startNs, endNs int64,
	hostID string,
) (*RTDistributionByStatus, error) {
	cacheKey := fmt.Sprintf(
		"nginx:perf:dist:%s:%s:%s:%d:%d",
		tenantUUID, serviceName, domain,
		startNs/int64(time.Second),
		endNs/int64(time.Second),
	)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result RTDistributionByStatus
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := s.fetchRTDistributionByStatus(ctx, tenantUUID, serviceName, domain, startNs, endNs, hostID)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, performanceCacheTTL).Err()
		}
	}

	return result, nil
}

func (s *NginxPerformanceService) fetchRTDistributionByStatus(
	ctx context.Context,
	tenantUUID, serviceName, domain string,
	startNs, endNs int64,
	hostID string,
) (*RTDistributionByStatus, error) {
	if endNs <= startNs {
		return &RTDistributionByStatus{}, nil
	}

	base := lokiSelector(serviceName, domain, "access", hostID)
	rangeSec := (endNs - startNs) / int64(time.Second)
	if rangeSec <= 0 {
		return &RTDistributionByStatus{}, nil
	}

	type classConfig struct {
		key    string
		filter string
	}

	classes := []classConfig{
		{key: "2xx", filter: `| status =~ "2.."`},
		{key: "3xx", filter: `| status =~ "3.."`},
		{key: "4xx", filter: `| status =~ "4.."`},
		{key: "5xx", filter: `| status =~ "5.."`},
	}

	type classResult struct {
		key     string
		metrics RTStatusMetrics
		err     error
	}

	resultsCh := make(chan classResult, len(classes))
	var wg sync.WaitGroup

	for _, cls := range classes {
		wg.Add(1)
		go func(cfg classConfig) {
			defer wg.Done()
			metrics, err := s.fetchStatusMetrics(ctx, tenantUUID, base, cfg.filter, endNs, rangeSec)
			resultsCh <- classResult{key: cfg.key, metrics: metrics, err: err}
		}(cls)
	}

	wg.Wait()
	close(resultsCh)

	result := &RTDistributionByStatus{}
	for res := range resultsCh {
		if res.err != nil {
			return nil, fmt.Errorf("dist metrics for %s: %w", res.key, res.err)
		}

		switch res.key {
		case "2xx":
			result.TwoXX = res.metrics
		case "3xx":
			result.ThreeXX = res.metrics
		case "4xx":
			result.FourXX = res.metrics
		case "5xx":
			result.FiveXX = res.metrics
		}
	}

	return result, nil
}

func (s *NginxPerformanceService) fetchStatusMetrics(
	ctx context.Context,
	tenantUUID string,
	base string,
	statusFilter string,
	endNs int64,
	rangeSec int64,
) (RTStatusMetrics, error) {
	rtBase := fmt.Sprintf(
		`%s %s %s %s | unwrap rt`,
		base,
		parseExpr,
		statusFilter,
		rtParseExpr,
	)
	countBase := fmt.Sprintf(`%s %s %s`, base, parseExpr, statusFilter)

	sumQ := fmt.Sprintf(`sum(sum_over_time(%s [%ds]))`, rtBase, rangeSec)
	countQ := fmt.Sprintf(`sum(count_over_time(%s [%ds]))`, countBase, rangeSec)
	p50Q := fmt.Sprintf(`quantile_over_time(0.50, %s [%ds]) by ()`, rtBase, rangeSec)
	p95Q := fmt.Sprintf(`quantile_over_time(0.95, %s [%ds]) by ()`, rtBase, rangeSec)
	p99Q := fmt.Sprintf(`quantile_over_time(0.99, %s [%ds]) by ()`, rtBase, rangeSec)
	breakdownQ := fmt.Sprintf(`sum by (status) (count_over_time(%s [%ds]))`, countBase, rangeSec)

	type scalarResult struct {
		val float64
		err error
	}
	type breakdownResult struct {
		val map[string]int
		err error
	}

	var (
		wg           sync.WaitGroup
		sumRes       scalarResult
		countRes     scalarResult
		p50Res       scalarResult
		p95Res       scalarResult
		p99Res       scalarResult
		breakdownRes breakdownResult
	)

	wg.Add(6)
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, sumQ, endNs, 100)
		sumRes = scalarResult{val: firstInstantValue(data), err: err}
	}()
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, countQ, endNs, 100)
		countRes = scalarResult{val: firstInstantValue(data), err: err}
	}()
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, p50Q, endNs, 100)
		p50Res = scalarResult{val: firstInstantValue(data), err: err}
	}()
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, p95Q, endNs, 100)
		p95Res = scalarResult{val: firstInstantValue(data), err: err}
	}()
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, p99Q, endNs, 100)
		p99Res = scalarResult{val: firstInstantValue(data), err: err}
	}()
	go func() {
		defer wg.Done()
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, breakdownQ, endNs, 200)
		if err != nil {
			breakdownRes = breakdownResult{err: err}
			return
		}
		breakdown := make(map[string]int, len(data))
		for _, series := range data {
			code := series.Metric["status"]
			if code == "" || len(series.Values) == 0 {
				continue
			}
			breakdown[code] = int(series.Values[0][1])
		}
		breakdownRes = breakdownResult{val: breakdown}
	}()
	wg.Wait()

	for _, res := range []scalarResult{sumRes, countRes, p50Res, p95Res, p99Res} {
		if res.err != nil {
			return RTStatusMetrics{}, res.err
		}
	}

	avg := 0.0
	if countRes.val > 0 {
		avg = (sumRes.val / countRes.val) * 1000
	}

	percentileCount := func(thresholdSec float64) (int, error) {
		if countRes.val <= 0 || thresholdSec <= 0 {
			return 0, nil
		}
		q := fmt.Sprintf(`sum(count_over_time(%s %s | rt <= %.6f [%ds]))`, countBase, rtParseExpr, thresholdSec, rangeSec)
		data, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, q, endNs, 100)
		if err != nil {
			return 0, err
		}
		return int(firstInstantValue(data)), nil
	}

	p50Count, err := percentileCount(p50Res.val)
	if err != nil {
		return RTStatusMetrics{}, err
	}
	p95Count, err := percentileCount(p95Res.val)
	if err != nil {
		return RTStatusMetrics{}, err
	}
	p99Count, err := percentileCount(p99Res.val)
	if err != nil {
		return RTStatusMetrics{}, err
	}

	return RTStatusMetrics{
		Avg:             avg,
		P50:             p50Res.val * 1000,
		P95:             p95Res.val * 1000,
		P99:             p99Res.val * 1000,
		Count:           int(countRes.val),
		P50Count:        p50Count,
		P95Count:        p95Count,
		P99Count:        p99Count,
		StatusBreakdown: breakdownRes.val,
	}, nil
}

func firstInstantValue(series []LokiMatrixSeries) float64 {
	if len(series) == 0 || len(series[0].Values) == 0 {
		return 0
	}
	return series[0].Values[0][1]
}

// ─── Chart 2: Raw RT Scatter ──────────────────────────────────────────────────

const (
	scatterBucketSize     = 3 * time.Minute
	scatterBucketCount    = 20
	scatterErrorBudget    = 150
	scatterTotalDotBudget = 3000
)

func (s *NginxPerformanceService) lokiQueryLogEntriesAll(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
) ([]LokiLogEntry, error) {
	const pageSize = 5000
	var all []LokiLogEntry
	curStart := startNs

	for {
		upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range",
			s.cfg.LokiHost, s.cfg.LokiPort)
		params := url.Values{}
		params.Set("query", query)
		params.Set("start", strconv.FormatInt(curStart, 10))
		params.Set("end", strconv.FormatInt(endNs, 10))
		params.Set("limit", strconv.Itoa(pageSize))
		params.Set("direction", "forward")

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Scope-OrgID", tenantUUID)

		resp, err := s.dashboard.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		// Parse inline — v[0] is the timestamp string, v[1] is the log line.
		// parseLokiStreamEntries does NOT store v[0], so the cursor would
		// never advance if we used it. We store v[0] ourselves.
		var raw struct {
			Data struct {
				Result []struct {
					Values [][]interface{} `json:"values"`
				} `json:"result"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, err
		}

		var page []LokiLogEntry
		var latestNs int64
		for _, stream := range raw.Data.Result {
			for _, v := range stream.Values {
				if len(v) < 2 {
					continue
				}
				tsStr := fmt.Sprint(v[0])
				entry := LokiLogEntry{
					Line:     fmt.Sprint(v[1]),
					Metadata: map[string]string{"__timestamp_ns__": tsStr},
				}
				// Also copy any structured metadata from v[2] if present
				if len(v) >= 3 {
					if meta, ok := v[2].(map[string]interface{}); ok {
						for k, val := range meta {
							entry.Metadata[k] = fmt.Sprint(val)
						}
					}
				}
				page = append(page, entry)

				// Track the latest timestamp for cursor advancement
				if ts, err2 := strconv.ParseInt(tsStr, 10, 64); err2 == nil && ts > latestNs {
					latestNs = ts
				}
			}
		}

		if len(page) == 0 {
			break
		}
		all = append(all, page...)
		if len(page) < pageSize {
			break // last page
		}
		if latestNs <= curStart {
			break // no progress — safety guard
		}
		curStart = latestNs + 1 // exclude last entry from next page
	}

	return all, nil
}
func (s *NginxPerformanceService) GetRTScatter(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	domain string,
	startNs, endNs int64,
	hostID string,
) (*RTScatter, error) {
	cacheKey := fmt.Sprintf(
		"nginx:perf:scatter:%s:%s:%s:%d:%d",
		tenantUUID, serviceName, domain,
		startNs/int64(time.Second),
		endNs/int64(time.Second),
	)

	if s.redis != nil {
		if cached, err := s.redis.Get(ctx, cacheKey).Bytes(); err == nil {
			var result RTScatter
			if json.Unmarshal(cached, &result) == nil {
				return &result, nil
			}
		}
	}

	result, err := s.fetchRTScatter(ctx, tenantUUID, serviceName, domain, startNs, endNs, hostID)
	if err != nil {
		return nil, err
	}

	if s.redis != nil {
		if b, err := json.Marshal(result); err == nil {
			_ = s.redis.Set(ctx, cacheKey, b, performanceCacheTTL).Err()
		}
	}

	return result, nil
}

type scatterRawPoint struct {
	timestampMs    int64
	rtMs           float64
	status         int
	path           string
	method         string
	normalizedPath string
	ip             string
}

func (s *NginxPerformanceService) fetchRTScatter(
	ctx context.Context,
	tenantUUID, serviceName, domain string,
	startNs, endNs int64,
	hostID string,
) (*RTScatter, error) {
	if endNs <= startNs {
		return &RTScatter{Points: []RTScatterPoint{}}, nil
	}

	base := lokiSelector(serviceName, domain, "access", hostID)
	rangeSec := (endNs - startNs) / int64(time.Second)
	if rangeSec <= 0 {
		return &RTScatter{Points: []RTScatterPoint{}}, nil
	}

	// Baseline for outliers (P95 per normalized path over the full window)
	baselineQ := fmt.Sprintf(
		`quantile_over_time(0.95, %s %s | normalized_path!="" %s | unwrap rt [%ds]) by (normalized_path)`,
		base, parseExpr, rtParseExpr, rangeSec,
	)
	baselineSeries, err := s.dashboard.lokiQueryInstant(ctx, tenantUUID, baselineQ, endNs, 5000)
	if err != nil {
		baselineSeries = nil
	}

	pathBaseline := make(map[string]float64, len(baselineSeries))
	for _, series := range baselineSeries {
		if np := series.Metric["normalized_path"]; np != "" && len(series.Values) > 0 {
			pathBaseline[np] = series.Values[0][1] * 1000
		}
	}

	// Divide the hour into fixed windows
	bucketSizeNs := int64(scatterBucketSize)
	totalRange := endNs - startNs

	numBuckets := int(totalRange / bucketSizeNs)
	if numBuckets > scatterBucketCount {
		numBuckets = scatterBucketCount
	}
	if numBuckets <= 0 {
		numBuckets = 1
	}

	type windowResult struct {
		bucketIdx int
		raw       []scatterRawPoint
		err       error
	}

	windowCh := make(chan windowResult, numBuckets)
	var wg sync.WaitGroup

	loc, _ := time.LoadLocation("Africa/Tunis")

	for i := 0; i < numBuckets; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			winStart := startNs + int64(idx)*bucketSizeNs
			winEnd := winStart + bucketSizeNs
			if winEnd > endNs {
				winEnd = endNs
			}

			// FIX 1: use {{.ip}} — the parseExpr regexp captures the remote address
			// into the label named "ip" ((?P<ip>[^ ]+)), not "remote_addr".
			logQ := fmt.Sprintf(
				`%s %s | status!="" %s | rt!="" | line_format "{{.status}}\t{{.rt}}\t{{.method}}\t{{.path}}\t{{.normalized_path}}\t{{.ip}}"`,
				base, parseExpr, rtParseExpr,
			)

			// FIX 2: no per-window limit — fetch all log lines so every request is
			// either shown as an individual dot or folded into an aggregate dot.
			// Requests beyond the dot budget are averaged (never silently dropped),
			// so TotalRequests = Σ individual + Σ aggregate.Count = true total.
			lines, queryErr := s.lokiQueryLogEntriesAll(ctx, tenantUUID, logQ, winStart, winEnd)
			if queryErr != nil {
				windowCh <- windowResult{bucketIdx: idx, err: queryErr}
				return
			}

			raw := make([]scatterRawPoint, 0, len(lines))
			for _, line := range lines {
				// 6 fields: status, rt, method, path, normalized_path, ip
				parts := strings.SplitN(line.Line, "\t", 6)
				if len(parts) < 4 {
					continue
				}
				statusInt, errStatus := strconv.Atoi(parts[0])
				if errStatus != nil {
					continue
				}
				rtSec, errRT := strconv.ParseFloat(parts[1], 64)
				if errRT != nil || rtSec <= 0 {
					continue
				}
				method := parts[2]
				path := parts[3]
				normalizedPath := ""
				if len(parts) >= 5 {
					normalizedPath = parts[4]
				}
				ip := ""
				if len(parts) >= 6 {
					ip = strings.TrimSpace(parts[5])
				}

				tsMs := winStart/int64(time.Millisecond) + int64(scatterBucketSize/time.Millisecond)/2
				if tsStr, ok := line.Metadata["__timestamp_ns__"]; ok {
					if tsNs, err := strconv.ParseInt(tsStr, 10, 64); err == nil {
						tsMs = tsNs / int64(time.Millisecond)
					}
				}

				raw = append(raw, scatterRawPoint{
					timestampMs:    tsMs,
					rtMs:           rtSec * 1000,
					status:         statusInt,
					path:           path,
					method:         method,
					normalizedPath: normalizedPath,
					ip:             ip,
				})
			}

			windowCh <- windowResult{bucketIdx: idx, raw: raw}
		}(i)
	}

	wg.Wait()
	close(windowCh)

	bucketRaw := make([][]scatterRawPoint, numBuckets)
	for res := range windowCh {
		if res.err != nil {
			bucketRaw[res.bucketIdx] = nil
			continue
		}
		bucketRaw[res.bucketIdx] = res.raw
	}

	var allPoints []RTScatterPoint

	for idx, raw := range bucketRaw {
		winStartMs := (startNs + int64(idx)*bucketSizeNs) / int64(time.Millisecond)
		winEndMs := winStartMs + int64(scatterBucketSize/time.Millisecond)

		startT := time.UnixMilli(winStartMs).In(loc)
		endT := time.UnixMilli(winEndMs).In(loc)
		winLabel := fmt.Sprintf("%s–%s",
			startT.Format("15:04"),
			endT.Format("15:04"),
		)

		if len(raw) == 0 {
			continue
		}

		var errors4xx, errors5xx, outliers []scatterRawPoint
		var normal2xx, normal3xx []scatterRawPoint

		for _, pt := range raw {
			switch {
			case pt.status >= 500:
				errors5xx = append(errors5xx, pt)
			case pt.status >= 400:
				errors4xx = append(errors4xx, pt)
			case pt.status >= 300:
				baseline := pathBaseline[pt.normalizedPath]
				if baseline > 0 && pt.rtMs > baseline*2 {
					outliers = append(outliers, pt)
				} else {
					normal3xx = append(normal3xx, pt)
				}
			default:
				baseline := pathBaseline[pt.normalizedPath]
				if baseline > 0 && pt.rtMs > baseline*2 {
					outliers = append(outliers, pt)
				} else {
					normal2xx = append(normal2xx, pt)
				}
			}
		}

		sortDescByRT := func(pts []scatterRawPoint) {
			for i := 1; i < len(pts); i++ {
				for j := i; j > 0 && pts[j].rtMs > pts[j-1].rtMs; j-- {
					pts[j], pts[j-1] = pts[j-1], pts[j]
				}
			}
		}
		sortDescByRT(errors4xx)
		sortDescByRT(errors5xx)
		sortDescByRT(outliers)

		// addSampled shows top-budget individual dots and collapses the remainder.
		// For 4xx and 5xx the overflow is aggregated PER EXACT STATUS CODE so that
		// e.g. 401 and 404 get separate avg dots (not one blended dot).
		addSampled := func(pts []scatterRawPoint, budget int) {
			if len(pts) == 0 {
				return
			}
			indiv := pts
			var aggregated []scatterRawPoint
			if len(pts) > budget {
				indiv = pts[:budget-1]
				aggregated = pts[budget-1:]
			}
			for _, p := range indiv {
				allPoints = append(allPoints, RTScatterPoint{
					Time:           p.timestampMs,
					RT:             p.rtMs,
					Status:         p.status,
					Path:           p.path,
					Method:         p.method,
					NormalizedPath: p.normalizedPath,
					IP:             p.ip,
					IsAggregate:    false,
				})
			}
			if len(aggregated) > 0 {
				// Group overflow by exact status code for per-code aggregate dots.
				byCode := make(map[int][]scatterRawPoint)
				for _, p := range aggregated {
					byCode[p.status] = append(byCode[p.status], p)
				}
				midMs := winStartMs + int64(scatterBucketSize/time.Millisecond)/2
				for code, group := range byCode {
					var sumRT float64
					for _, p := range group {
						sumRT += p.rtMs
					}
					allPoints = append(allPoints, RTScatterPoint{
						Time:        midMs,
						RT:          sumRT / float64(len(group)),
						Status:      code,
						IsAggregate: true,
						Count:       len(group),
						WindowLabel: winLabel,
					})
				}
			}
		}

		addSampled(errors5xx, scatterErrorBudget)
		addSampled(errors4xx, scatterErrorBudget)
		addSampled(outliers, scatterErrorBudget)

		// 2xx and 3xx normals: single avg dot per window (all folded — count preserved)
		if len(normal2xx) > 0 {
			var sumRT float64
			for _, p := range normal2xx {
				sumRT += p.rtMs
			}
			midMs := winStartMs + int64(scatterBucketSize/time.Millisecond)/2
			allPoints = append(allPoints, RTScatterPoint{
				Time:        midMs,
				RT:          sumRT / float64(len(normal2xx)),
				Status:      200,
				IsAggregate: true,
				Count:       len(normal2xx),
				WindowLabel: winLabel,
			})
		}

		if len(normal3xx) > 0 {
			var sumRT float64
			for _, p := range normal3xx {
				sumRT += p.rtMs
			}
			midMs := winStartMs + int64(scatterBucketSize/time.Millisecond)/2
			allPoints = append(allPoints, RTScatterPoint{
				Time:        midMs,
				RT:          sumRT / float64(len(normal3xx)),
				Status:      301,
				IsAggregate: true,
				Count:       len(normal3xx),
				WindowLabel: winLabel,
			})
		}
	}

	// FIX 3: global budget — keep top-RT individual dots, convert overflow into
	// per-window per-status aggregate dots so no request count is ever lost.
	if len(allPoints) > scatterTotalDotBudget {
		var aggDots, indivDots []RTScatterPoint
		for _, pt := range allPoints {
			if pt.IsAggregate {
				aggDots = append(aggDots, pt)
			} else {
				indivDots = append(indivDots, pt)
			}
		}
		remaining := scatterTotalDotBudget - len(aggDots)
		if remaining < 0 {
			remaining = 0
		}
		if len(indivDots) > remaining {
			// Sort individual dots desc by RT — keep the most interesting ones.
			for i := 1; i < len(indivDots); i++ {
				for j := i; j > 0 && indivDots[j].RT > indivDots[j-1].RT; j-- {
					indivDots[j], indivDots[j-1] = indivDots[j-1], indivDots[j]
				}
			}
			overflow := indivDots[remaining:]
			indivDots = indivDots[:remaining]

			// Convert overflow into per-window per-status aggregate dots so their
			// counts are preserved and TotalRequests stays accurate.
			type aggKey struct {
				windowMs int64
				status   int
			}
			byKey := make(map[aggKey][]RTScatterPoint)
			startMs := startNs / int64(time.Millisecond)
			bucketMs := int64(scatterBucketSize / time.Millisecond)
			for _, pt := range overflow {
				windowIdx := (pt.Time - startMs) / bucketMs
				if windowIdx < 0 {
					windowIdx = 0
				}
				midMs := startMs + windowIdx*bucketMs + bucketMs/2
				key := aggKey{windowMs: midMs, status: pt.Status} // pt.Status — exported field
				byKey[key] = append(byKey[key], pt)
			}
			for key, group := range byKey {
				var sumRT float64
				for _, p := range group {
					sumRT += p.RT
				}
				aggDots = append(aggDots, RTScatterPoint{
					Time:        key.windowMs,
					RT:          sumRT / float64(len(group)),
					Status:      key.status,
					IsAggregate: true,
					Count:       len(group),
				})
			}
		}
		allPoints = append(aggDots, indivDots...)
	}

	if allPoints == nil {
		allPoints = []RTScatterPoint{}
	}

	// TotalRequests: every individual dot = 1, every aggregate dot = its Count.
	// With the three fixes above this now equals the true number of requests fetched.
	totalRequests := 0
	for _, pt := range allPoints {
		if pt.IsAggregate {
			totalRequests += pt.Count
		} else {
			totalRequests++
		}
	}

	return &RTScatter{
		Points:        allPoints,
		TotalRequests: totalRequests,
	}, nil
}
