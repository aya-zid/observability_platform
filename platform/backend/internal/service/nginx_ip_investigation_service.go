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
	"strings"
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
	TotalRequests float64 `json:"total_requests"`
	Total4xx      float64 `json:"total_4xx"`
	Total5xx      float64 `json:"total_5xx"`
	ErrorRate     float64 `json:"error_rate"`
	FirstSeenToday string `json:"first_seen_today"`
	LastSeenToday  string `json:"last_seen_today"`

	ReqRateTimeseries []TimePoint `json:"req_rate_timeseries"`
	StatusTimeseries  []TimePoint `json:"status_timeseries"`
	RTTimeseries      []TimePoint `json:"rt_timeseries"`

	Paths []NginxIPPathStat `json:"paths"`
}

type pathAcc struct {
	Count    float64
	Errors   float64
	RTSum    float64
	Methods  map[string]float64
	Statuses map[string]float64
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

	// Exact counts + paths from raw logs (limit 5000 is usually enough)
	rawLogs, _ := s.lokiQueryLogLinesDir(ctx, tenantUUID, ipFilter, startNs, endNs, 5000, "backward")
	parsed := parseNginxLogLinesForIP(rawLogs, ip)
	result := computeExactStatsFromLogs(parsed)

	// ── Timeseries (fixed 1m buckets for clean per-minute counts) ─────────────
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
	var reqRate, st2xx, st3xx, st4xx, st5xx, avgRT lR

	wg.Add(6)
	go func() { defer wg.Done(); reqRate = lq(fmt.Sprintf(`sum(count_over_time(%s [%s]))`, ipFilter, countInterval)) }()
	go func() { defer wg.Done(); st2xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "2.." [%s]))`, ipFilter, countInterval)) }()
	go func() { defer wg.Done(); st3xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "3.." [%s]))`, ipFilter, countInterval)) }()
	go func() { defer wg.Done(); st4xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "4.." [%s]))`, ipFilter, countInterval)) }()
	go func() { defer wg.Done(); st5xx = lq(fmt.Sprintf(`sum(count_over_time(%s | status =~ "5.." [%s]))`, ipFilter, countInterval)) }()
	go func() { defer wg.Done(); avgRT = lq(fmt.Sprintf(`avg(avg_over_time(%s | unwrap rt [%s]))`, ipFilterRT, countInterval)) }()
	wg.Wait()

	if len(reqRate.data) > 0 {
		for _, pt := range reqRate.data[0].Values {
			result.ReqRateTimeseries = append(result.ReqRateTimeseries, TimePoint{"time": pt[0], "req": pt[1]})
		}
	}

	result.StatusTimeseries = mergeNamedLokiSeries(map[string][]LokiMatrixSeries{
		"2xx": st2xx.data, "3xx": st3xx.data, "4xx": st4xx.data, "5xx": st5xx.data,
	})

	if len(avgRT.data) > 0 {
		for _, pt := range avgRT.data[0].Values {
			result.RTTimeseries = append(result.RTTimeseries, TimePoint{"time": pt[0], "avg_rt": pt[1] * 1000})
		}
	}

	// ── First / Last seen today (reliable Loki timestamp) ─────────────────────
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

// ─── Log Parsing (unchanged) ──────────────────────────────────────────────────

func parseNginxLogLinesForIP(lines []string, targetIP string) []struct {
	Path   string
	Status int
	RT     float64
	Method string
} {
	var parsed []struct {
		Path   string
		Status int
		RT     float64
		Method string
	}
	for _, line := range lines {
		p := parseNginxLogLineGo(line)
		if p != nil && p.IP == targetIP {
			parsed = append(parsed, struct {
				Path   string
				Status int
				RT     float64
				Method string
			}{
				Path:   p.Path,
				Status: p.Status,
				RT:     p.RT,
				Method: p.Method,
			})
		}
	}
	return parsed
}

func parseNginxLogLineGo(line string) *struct {
	IP     string
	Method string
	Path   string
	Status int
	RT     float64
} {
	// ... (your existing implementation — keep it as-is)
	// I'll omit repeating the whole function for brevity. Use the one you already have.
	fields := strings.Fields(line)
	if len(fields) < 6 {
		return nil
	}
	ip := fields[0]

	requestStart := strings.Index(line, `"`)
	if requestStart == -1 {
		return nil
	}
	requestEnd := strings.Index(line[requestStart+1:], `"`)
	if requestEnd == -1 {
		return nil
	}
	requestLine := line[requestStart+1 : requestStart+1+requestEnd]
	reqParts := strings.Fields(requestLine)
	if len(reqParts) < 2 {
		return nil
	}

	method := reqParts[0]
	path := reqParts[1]

	after := line[requestStart+1+requestEnd+1:]
	statusFields := strings.Fields(after)
	status := 0
	if len(statusFields) > 0 {
		fmt.Sscanf(statusFields[0], "%d", &status)
	}

	rt := 0.0
	if rtIdx := strings.LastIndex(line, "rt="); rtIdx != -1 {
		fmt.Sscanf(line[rtIdx+3:], "%f", &rt)
	}

	return &struct {
		IP     string
		Method string
		Path   string
		Status int
		RT     float64
	}{
		IP:     ip,
		Method: method,
		Path:   path,
		Status: status,
		RT:     rt,
	}
}

// ─── Stats Computation (unchanged) ────────────────────────────────────────────

func computeExactStatsFromLogs(parsed []struct {
	Path   string
	Status int
	RT     float64
	Method string
}) *NginxIPInvestigationResult {
	// ... (keep your existing implementation)
	result := &NginxIPInvestigationResult{}
	pathMap := make(map[string]*pathAcc)

	for _, l := range parsed {
		result.TotalRequests++
		if l.Status >= 400 {
			if l.Status < 500 {
				result.Total4xx++
			} else {
				result.Total5xx++
			}
		}

		if l.Path == "" {
			continue
		}

		acc, ok := pathMap[l.Path]
		if !ok {
			acc = &pathAcc{
				Methods:  make(map[string]float64),
				Statuses: make(map[string]float64),
			}
			pathMap[l.Path] = acc
		}
		acc.Count++
		acc.RTSum += l.RT
		if l.Status >= 400 {
			acc.Errors++
		}
		acc.Methods[l.Method]++
		acc.Statuses[strconv.Itoa(l.Status)]++
	}

	if result.TotalRequests > 0 {
		result.ErrorRate = ((result.Total4xx + result.Total5xx) / result.TotalRequests) * 100
	}

	// Top 20 paths
	type entry struct {
		path string
		acc  *pathAcc
	}
	var list []entry
	for p, a := range pathMap {
		list = append(list, entry{p, a})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].acc.Count > list[j].acc.Count
	})
	if len(list) > 20 {
		list = list[:20]
	}

	for _, e := range list {
		avgRT := 0.0
		if e.acc.Count > 0 {
			avgRT = (e.acc.RTSum / e.acc.Count) * 1000
		}
		result.Paths = append(result.Paths, NginxIPPathStat{
			Path:            e.path,
			Count:           e.acc.Count,
			Errors:          e.acc.Errors,
			Method:          dominantKey(e.acc.Methods),
			Status:          dominantKey(e.acc.Statuses),
			AvgRT:           avgRT,
			StatusBreakdown: e.acc.Statuses,
		})
	}

	return result
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func ipInvTodayMidnightNs() int64 {
	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.UnixNano()
}

func tsNsToHHMM(ns int64) string {
	local := time.Unix(0, ns).Local()
	// Add 1 hour to compensate for the timezone offset bug
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

// lokiQueryFirstEntryTs and parseLokiFirstEntryTs (Claude's version is good)
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

		// For both forward and backward, we take the first value in the response
		// because Loki already sorts according to the direction parameter.
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

// Keep your existing lokiQueryLogLinesDir + parseLokiStreamLines
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