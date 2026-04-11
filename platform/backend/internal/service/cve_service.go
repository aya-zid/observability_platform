package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/config"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/domain"
	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/repository"
)

type CVEService struct {
	cfg        *config.Config
	alertsRepo *repository.AlertsRepository
	httpClient *http.Client
}

func NewCVEService(cfg *config.Config, alertsRepo *repository.AlertsRepository) *CVEService {
	return &CVEService{
		cfg:        cfg,
		alertsRepo: alertsRepo,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

type nvdResponse struct {
	Vulnerabilities []struct {
		CVE nvdCVE `json:"cve"`
	} `json:"vulnerabilities"`
}

type nvdCVE struct {
	ID             string          `json:"id"`
	Descriptions   []nvdDesc       `json:"descriptions"`
	Metrics        nvdMetrics      `json:"metrics"`
	Configurations []nvdConfigNode `json:"configurations"`
}

type nvdDesc struct {
	Lang  string `json:"lang"`
	Value string `json:"value"`
}

type nvdMetrics struct {
	CvssMetricV31 []nvdCVSSMetric `json:"cvssMetricV31"`
	CvssMetricV30 []nvdCVSSMetric `json:"cvssMetricV30"`
}

type nvdCVSSMetric struct {
	CvssData struct {
		BaseScore    float32 `json:"baseScore"`
		BaseSeverity string  `json:"baseSeverity"`
	} `json:"cvssData"`
}

type nvdConfigNode struct {
	Nodes    []nvdNode `json:"nodes"`
	Operator string    `json:"operator"`
}

type nvdNode struct {
	CPEMatch []nvdCPEMatch `json:"cpeMatch"`
	Children []nvdNode     `json:"children"`
	Operator string        `json:"operator"`
}

type nvdCPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	VersionStartIncluding string `json:"versionStartIncluding"`
	VersionStartExcluding string `json:"versionStartExcluding"`
	VersionEndIncluding   string `json:"versionEndIncluding"`
	VersionEndExcluding   string `json:"versionEndExcluding"`
}

func (s *CVEService) StartScheduler(ctx context.Context) {
	if !s.cfg.VulnerabilityScanEnabled {
		log.Printf("[CVE] vulnerability scanning disabled")
		return
	}

	go func() {
		s.runOnce(context.Background())
		ticker := time.NewTicker(s.cfg.VulnerabilityScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runOnce(context.Background())
			}
		}
	}()
}

func (s *CVEService) runOnce(ctx context.Context) {
	rows, err := s.alertsRepo.ListAllDiscoveredServicesCtx(ctx)
	if err != nil {
		log.Printf("[CVE] list services failed: %v", err)
		return
	}

	for _, row := range rows {
		if err := s.processService(ctx, row); err != nil {
			log.Printf("[CVE] scan failed for %s %s (id=%d): %v", row.ServiceName, row.Version, row.DiscoveredServiceID, err)
		}
		time.Sleep(750 * time.Millisecond)
	}
}

func (s *CVEService) processService(ctx context.Context, row repository.ScannableService) error {
	now := time.Now().UTC()

	def, catalogFound := catalog.ResolveService(row.ServiceName)
	if !catalogFound || def.NVD == nil {
		return s.alertsRepo.SetScanStatusCtx(ctx, row.DiscoveredServiceID, false, domain.VulnScanStatusUnsupported, nil)
	}

	version := normalizeVersion(row.Version)
	if version == "" {
		return s.alertsRepo.SetScanStatusCtx(ctx, row.DiscoveredServiceID, true, domain.VulnScanStatusNoVersion, nil)
	}

	items, err := s.fetchVulnerabilities(ctx, def, version)
	if err != nil {
		_ = s.alertsRepo.SetScanStatusCtx(ctx, row.DiscoveredServiceID, true, domain.VulnScanStatusFailed, nil)
		return err
	}

	hostID, err := s.alertsRepo.GetHostIDForServiceCtx(ctx, row.DiscoveredServiceID)
	if err != nil {
		return fmt.Errorf("get host id: %w", err)
	}

	if err := s.alertsRepo.ReplaceServiceVulnerabilitiesCtx(
		ctx,
		row.TenantID,
		hostID,
		row.DiscoveredServiceID,
		row.ServiceName,
		row.Version,
		items,
	); err != nil {
		return err
	}

	return s.alertsRepo.SetScanStatusCtx(ctx, row.DiscoveredServiceID, true, domain.VulnScanStatusScanned, &now)
}

func (s *CVEService) fetchVulnerabilities(ctx context.Context, def catalog.ServiceDef, version string) ([]domain.ServiceVulnerability, error) {
	values := url.Values{}
	values.Set("cpeName", fmt.Sprintf("cpe:2.3:a:%s:%s:%s:*:*:*:*:*:*:*", def.NVD.Vendor, def.NVD.Product, version))
	values.Set("resultsPerPage", "200")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://services.nvd.nist.gov/rest/json/cves/2.0?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if s.cfg.NVDAPIKey != "" {
		req.Header.Set("apiKey", s.cfg.NVDAPIKey)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("nvd status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload nvdResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	out := make([]domain.ServiceVulnerability, 0)
	for _, vuln := range payload.Vulnerabilities {
		cve := vuln.CVE
		score, severity := extractScoreAndSeverity(cve)
		if !meetsThreshold(score, severity, s.cfg.VulnerabilitySeverityMinimum) {
			continue
		}
		if !cveAffectsVersion(cve, def, version) {
			continue
		}
		out = append(out, domain.ServiceVulnerability{
			CVEID:          cve.ID,
			CVSSScore:      score,
			Severity:       severity,
			Description:    englishDescription(cve),
			FixedInVersion: fixedInVersion(cve),
			PublishedAt:    time.Now().UTC(),
		})
	}
	return out, nil
}

func extractScoreAndSeverity(cve nvdCVE) (float32, string) {
	if len(cve.Metrics.CvssMetricV31) > 0 {
		return cve.Metrics.CvssMetricV31[0].CvssData.BaseScore, strings.ToUpper(cve.Metrics.CvssMetricV31[0].CvssData.BaseSeverity)
	}
	if len(cve.Metrics.CvssMetricV30) > 0 {
		return cve.Metrics.CvssMetricV30[0].CvssData.BaseScore, strings.ToUpper(cve.Metrics.CvssMetricV30[0].CvssData.BaseSeverity)
	}
	return 0, "UNKNOWN"
}

func englishDescription(cve nvdCVE) string {
	for _, d := range cve.Descriptions {
		if d.Lang == "en" {
			return d.Value
		}
	}
	return ""
}

func fixedInVersion(cve nvdCVE) string {
	for _, config := range cve.Configurations {
		for _, node := range config.Nodes {
			if value := fixedInVersionNode(node); value != "" {
				return value
			}
		}
	}
	return ""
}

func fixedInVersionNode(node nvdNode) string {
	for _, match := range node.CPEMatch {
		if match.VersionEndExcluding != "" {
			return "< " + match.VersionEndExcluding
		}
		if match.VersionEndIncluding != "" {
			return "<= " + match.VersionEndIncluding
		}
	}
	for _, child := range node.Children {
		if value := fixedInVersionNode(child); value != "" {
			return value
		}
	}
	return ""
}

func cveAffectsVersion(cve nvdCVE, def catalog.ServiceDef, version string) bool {
	for _, config := range cve.Configurations {
		for _, node := range config.Nodes {
			if nodeAffectsVersion(node, def, version) {
				return true
			}
		}
	}
	return false
}

func nodeAffectsVersion(node nvdNode, def catalog.ServiceDef, version string) bool {
	for _, match := range node.CPEMatch {
		if cpeMatchAffectsVersion(match, def, version) {
			return true
		}
	}
	for _, child := range node.Children {
		if nodeAffectsVersion(child, def, version) {
			return true
		}
	}
	return false
}

func cpeMatchAffectsVersion(match nvdCPEMatch, def catalog.ServiceDef, version string) bool {
	if !match.Vulnerable {
		return false
	}

	criteria := strings.ToLower(match.Criteria)
	needle := ":" + strings.ToLower(def.NVD.Vendor) + ":" + strings.ToLower(def.NVD.Product) + ":"
	if !strings.Contains(criteria, needle) {
		return false
	}

	startIn := normalizeVersion(match.VersionStartIncluding)
	startEx := normalizeVersion(match.VersionStartExcluding)
	endIn := normalizeVersion(match.VersionEndIncluding)
	endEx := normalizeVersion(match.VersionEndExcluding)

	if startIn != "" && compareVersions(version, startIn) < 0 {
		return false
	}
	if startEx != "" && compareVersions(version, startEx) <= 0 {
		return false
	}
	if endIn != "" && compareVersions(version, endIn) > 0 {
		return false
	}
	if endEx != "" && compareVersions(version, endEx) >= 0 {
		return false
	}

	if startIn == "" && startEx == "" && endIn == "" && endEx == "" {
		parts := strings.Split(criteria, ":")
		if len(parts) > 5 {
			cpeVersion := normalizeVersion(parts[5])
			if cpeVersion == "" {
				return true
			}
			return compareVersions(version, cpeVersion) == 0
		}
	}

	return true
}

func meetsThreshold(score float32, severity string, threshold string) bool {
	switch strings.ToUpper(threshold) {
	case "CRITICAL":
		return score >= 9.0 || strings.ToUpper(severity) == "CRITICAL"
	case "HIGH":
		return score >= 7.0 || strings.ToUpper(severity) == "HIGH" || strings.ToUpper(severity) == "CRITICAL"
	default:
		return score >= 4.0 || strings.ToUpper(severity) == "MEDIUM" || strings.ToUpper(severity) == "HIGH" || strings.ToUpper(severity) == "CRITICAL"
	}
}

var versionNumberPattern = regexp.MustCompile(`\d+`)

func normalizeVersion(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.TrimPrefix(v, "v")
	if v == "" || v == "*" {
		return ""
	}
	parts := versionNumberPattern.FindAllString(v, -1)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ".")
}

func compareVersions(a, b string) int {
	aa := splitVersionInts(a)
	bb := splitVersionInts(b)
	maxLen := len(aa)
	if len(bb) > maxLen {
		maxLen = len(bb)
	}
	for i := 0; i < maxLen; i++ {
		av, bv := 0, 0
		if i < len(aa) {
			av = aa[i]
		}
		if i < len(bb) {
			bv = bb[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func splitVersionInts(v string) []int {
	normalized := normalizeVersion(v)
	if normalized == "" {
		return nil
	}
	parts := strings.Split(normalized, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
