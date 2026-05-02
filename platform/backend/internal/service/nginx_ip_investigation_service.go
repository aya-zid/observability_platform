// internal/service/nginx_ip_investigation_service.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"sync"
	"time"
)

// ─── Response Types ───────────────────────────────────────────────────────────

type NginxIPPathStat struct {
	Path            string             `json:"path"`
	Method          string             `json:"method"`
	Count           float64            `json:"count"`
	Errors          float64            `json:"errors"`
	AvgRT           float64            `json:"avg_rt"`
	Status          string             `json:"status"`
	StatusBreakdown map[string]float64 `json:"status_breakdown"`
}

type NginxIPInvestigationResult struct {
	TotalRequests  float64 `json:"total_requests"`
	Total4xx       float64 `json:"total_4xx"`
	Total5xx       float64 `json:"total_5xx"`
	ErrorRate      float64 `json:"error_rate"`
	FirstSeenToday string  `json:"first_seen_today"`
	LastSeenToday  string  `json:"last_seen_today"`

	ReqRateTimeseries []TimePoint `json:"req_rate_timeseries"`
	StatusTimeseries  []TimePoint `json:"status_timeseries"`
	RTTimeseries      []TimePoint `json:"rt_timeseries"`

	Paths    []NginxIPPathStat `json:"paths"`
	Warnings []string          `json:"warnings,omitempty"`
}

// ─── Main Function ────────────────────────────────────────────────────────────

func (s *NginxDashboardService) GetIPInvestigation(
	ctx context.Context,
	tenantUUID string,
	serviceName string,
	ip string,
	domain string,
	startNs, endNs int64,
	step string,
) (*NginxIPInvestigationResult, error) {

	base := lokiSelector(serviceName, domain, "access")
	ipFilter := fmt.Sprintf(`%s %s | ip = "%s"`, base, parseExpr, lokiLabelValue(ip))
	ipFilterRT := fmt.Sprintf(`%s %s %s | ip = "%s"`, base, parseExpr, rtParseExpr, lokiLabelValue(ip))

	// All queries via metric aggregation — no raw log fetch, no line-count limit.
	const countInterval = "1m"

	type lR struct {
		data []LokiMatrixSeries
		err  error
	}

	lq := func(q string) lR {
		d, e := s.lokiQueryRange(ctx, tenantUUID, q, startNs, endNs, countInterval, 5000)
		return lR{d, e}
	}

	var wg sync.WaitGroup
	var (
		reqRate, st2xx, st3xx, st4xx, st5xx, avgRT lR
		pathCounts, pathErrors, pathAvgRT           lR
		pathMethodCounts, pathStatusCounts          lR
	)

	wg.Add(11)

	// Timeseries
	go func() {
		defer wg.Done()
		reqRate = lq(fmt.Sprintf(`sum(count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st2xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "2.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st3xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "3.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st4xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "4.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		st5xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "5.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		avgRT = lq(fmt.Sprintf(`avg(avg_over_time(%s | unwrap rt [%s]))`, ipFilterRT, countInterval))
	}()

	// Per-path aggregations — Loki groups by the parsed "path" label
	go func() {
		defer wg.Done()
		pathCounts = lq(fmt.Sprintf(`sum by (path) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathErrors = lq(fmt.Sprintf(`sum by (path) (count_over_time(%s | status =~ "4..|5.." [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathAvgRT = lq(fmt.Sprintf(`avg by (path) (avg_over_time(%s | unwrap rt [%s]))`, ipFilterRT, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathMethodCounts = lq(fmt.Sprintf(`sum by (path, method) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()
	go func() {
		defer wg.Done()
		pathStatusCounts = lq(fmt.Sprintf(`sum by (path, status) (count_over_time(%s [%s]))`, ipFilter, countInterval))
	}()

	wg.Wait()

	result := &NginxIPInvestigationResult{}

	// ── Scalar totals ────────────────────────────────────────────────────────
	result.TotalRequests = sumLokiSeries(reqRate.data)
	result.Total4xx = sumLokiSeries(st4xx.data)
	result.Total5xx = sumLokiSeries(st5xx.data)
	if result.TotalRequests > 0 {
		result.ErrorRate = ((result.Total4xx + result.Total5xx) / result.TotalRequests) * 100
	}

	// ── Timeseries ───────────────────────────────────────────────────────────
	if len(reqRate.data) > 0 {
		for _, pt := range reqRate.data[0].Values {
			result.ReqRateTimeseries = append(result.ReqRateTimeseries,
				TimePoint{"time": pt[0], "req": pt[1]})
		}
	}

	result.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": st2xx.data, "3xx": st3xx.data, "4xx": st4xx.data, "5xx": st5xx.data,
	})

	if len(avgRT.data) > 0 {
		for _, pt := range avgRT.data[0].Values {
			result.RTTimeseries = append(result.RTTimeseries,
				TimePoint{"time": pt[0], "avg_rt": pt[1] * 1000})
		}
	}

	// ── Path breakdown ───────────────────────────────────────────────────────
	result.Paths = buildPathStats(
		pathCounts.data,
		pathErrors.data,
		pathAvgRT.data,
		pathMethodCounts.data,
		pathStatusCounts.data,
	)

	// ── First / Last seen today ──────────────────────────────────────────────
	todayStartNs := ipInvTodayMidnightNs()
	nowNs := time.Now().UnixNano()

	var firstTs, lastTs int64
	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		defer wg2.Done()
		firstTs, _ = s.lokiQueryFirstEntryTs(ctx, tenantUUID, ipFilter, todayStartNs, nowNs, "forward")
	}()
	go func() {
		defer wg2.Done()
		lastTs, _ = s.lokiQueryFirstEntryTs(ctx, tenantUUID, ipFilter, todayStartNs, nowNs, "backward")
	}()
	wg2.Wait()

	if firstTs > 0 {
		result.FirstSeenToday = tsNsToHHMM(firstTs)
	}
	if lastTs > 0 {
		result.LastSeenToday = tsNsToHHMM(lastTs)
	}

	return result, nil
}

// ─── sumLokiSeries ────────────────────────────────────────────────────────────
// Values is [][2]float64 — pt[0] = timestamp, pt[1] = value.

func sumLokiSeries(series []LokiMatrixSeries) float64 {
	var total float64
	for _, s := range series {
		for _, pt := range s.Values {
			total += pt[1]
		}
	}
	return total
}

// ─── buildPathStats ───────────────────────────────────────────────────────────
// Reconstructs per-path stats from five separate metric query results.
// Uses s.Metric["path"] (and "method" / "status") to read the label values,
// matching the real LokiMatrixSeries struct that already works in the rest of
// the codebase (see nginx_performance_service.go: series.Metric["status"]).

func buildPathStats(
	countSeries []LokiMatrixSeries,
	errorSeries []LokiMatrixSeries,
	rtSeries []LokiMatrixSeries,
	methodSeries []LokiMatrixSeries,
	statusSeries []LokiMatrixSeries,
) []NginxIPPathStat {

	type acc struct {
		count    float64
		errors   float64
		rtSum    float64
		rtCount  float64
		methods  map[string]float64
		statuses map[string]float64
	}

	pathMap := make(map[string]*acc)
	get := func(path string) *acc {
		if a, ok := pathMap[path]; ok {
			return a
		}
		a := &acc{
			methods:  make(map[string]float64),
			statuses: make(map[string]float64),
		}
		pathMap[path] = a
		return a
	}

	// Total count per path
	for _, s := range countSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.count += pt[1]
		}
	}

	// Error count per path (4xx + 5xx)
	for _, s := range errorSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.errors += pt[1]
		}
	}

	// Average RT per path (avg of per-bucket avgs, good enough approximation)
	for _, s := range rtSeries {
		path := s.Metric["path"]
		if path == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.rtSum += pt[1]
			a.rtCount++
		}
	}

	// Dominant method per path
	for _, s := range methodSeries {
		path := s.Metric["path"]
		method := s.Metric["method"]
		if path == "" || method == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.methods[method] += pt[1]
		}
	}

	// Status breakdown per path
	for _, s := range statusSeries {
		path := s.Metric["path"]
		status := s.Metric["status"]
		if path == "" || status == "" {
			continue
		}
		a := get(path)
		for _, pt := range s.Values {
			a.statuses[status] += pt[1]
		}
	}

	// Sort by count desc, keep top 20
	type entry struct {
		path string
		acc  *acc
	}
	list := make([]entry, 0, len(pathMap))
	for p, a := range pathMap {
		list = append(list, entry{p, a})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].acc.count > list[j].acc.count
	})
	if len(list) > 20 {
		list = list[:20]
	}

	stats := make([]NginxIPPathStat, 0, len(list))
	for _, e := range list {
		avgRT := 0.0
		if e.acc.rtCount > 0 {
			avgRT = (e.acc.rtSum / e.acc.rtCount) * 1000 // seconds → milliseconds
		}
		stats = append(stats, NginxIPPathStat{
			Path:            e.path,
			Count:           e.acc.count,
			Errors:          e.acc.errors,
			Method:          dominantKey(e.acc.methods),
			Status:          dominantKey(e.acc.statuses),
			AvgRT:           avgRT,
			StatusBreakdown: e.acc.statuses,
		})
	}
	return stats
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func ipInvTodayMidnightNs() int64 {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.UnixNano()
}

func tsNsToHHMM(ns int64) string {
	local := time.Unix(0, ns).Local()
	local = local.Add(time.Hour)
	return fmt.Sprintf("%02d:%02d", local.Hour(), local.Minute())
}

func dominantKey(m map[string]float64) string {
	if len(m) == 0 {
		return ""
	}
	best := ""
	var bestVal float64
	for k, v := range m {
		if v > bestVal {
			bestVal = v
			best = k
		}
	}
	return best
}

// ─── Loki helpers (unchanged) ─────────────────────────────────────────────────

func (s *NginxDashboardService) lokiQueryFirstEntryTs(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
	direction string,
) (int64, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startNs, 10))
	params.Set("end", strconv.FormatInt(endNs, 10))
	params.Set("limit", "1")
	params.Set("direction", direction)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	return parseLokiFirstEntryTs(body)
}

func parseLokiFirstEntryTs(body []byte) (int64, error) {
	var envelope struct {
		Data struct {
			Result []struct {
				Values [][]string `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return 0, fmt.Errorf("parseLokiFirstEntryTs: %w", err)
	}

	for _, stream := range envelope.Data.Result {
		if len(stream.Values) == 0 {
			continue
		}
		val := stream.Values[0]
		if len(val) > 0 {
			tsNs, err := strconv.ParseInt(val[0], 10, 64)
			if err == nil && tsNs > 0 {
				return tsNs, nil
			}
		}
	}
	return 0, nil
}

func (s *NginxDashboardService) lokiQueryLogLinesDir(
	ctx context.Context,
	tenantUUID, query string,
	startNs, endNs int64,
	limit int,
	direction string,
) ([]string, error) {
	upstream := fmt.Sprintf("http://%s:%s/loki/api/v1/query_range", s.cfg.LokiHost, s.cfg.LokiPort)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startNs, 10))
	params.Set("end", strconv.FormatInt(endNs, 10))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("direction", direction)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upstream+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Scope-OrgID", tenantUUID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseLokiStreamLines(body), nil
}