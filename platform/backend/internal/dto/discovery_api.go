// internal/dto/discovery_api.go
package dto

import (
	"time"

	"gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"
)

type DiscoveryHostListItem struct {
	MachineID    string `json:"machine_id"`
	Hostname     string `json:"hostname"`
	OSName       string `json:"os_name"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
	LastSeenAt   string `json:"last_seen_at"`
	Status       string `json:"status"`
}

type DiscoveryServiceListItem struct {
	ID                 uint   `json:"id"`
	MachineID          string `json:"machine_id"`
	ServiceName        string `json:"service_name"`
	Port               *int   `json:"port,omitempty"`
	Version            string `json:"version,omitempty"`
	Category           string `json:"category"`
	IsSupported        bool   `json:"is_supported"`
	ConfirmedLogPath   string `json:"confirmed_log_path,omitempty"`
	LogPathSource      string `json:"log_path_source"`
	LogCollectionOn    bool   `json:"log_collection_on"`
	MetricCollectionOn bool   `json:"metric_collection_on"`
	LoggingMode        string `json:"logging_mode"`
	VulnScanSupported  bool   `json:"vuln_scan_supported"`
	VulnScanStatus     string `json:"vuln_scan_status"`
	LastVulnScanAt     string `json:"last_vuln_scan_at,omitempty"`
}

type DiscoveryServiceDetailResponse struct {
	ID                       uint                           `json:"id"`
	MachineID                string                         `json:"machine_id"`
	Hostname                 string                         `json:"hostname"`
	ServiceName              string                         `json:"service_name"`
	Port                     *int                           `json:"port,omitempty"`
	Version                  string                         `json:"version,omitempty"`
	Category                 string                         `json:"category"`
	IsSupported              bool                           `json:"is_supported"`
	LoggingMode              string                         `json:"logging_mode"`
	UsesJournald             bool                           `json:"uses_journald"`
	UsesSyslog               bool                           `json:"uses_syslog"`
	AccessLogPath            string                         `json:"access_log_path"`
	ErrorLogPath             string                         `json:"error_log_path"`
	ConfirmedLogPath         string                         `json:"confirmed_log_path,omitempty"`
	LogPathSource            string                         `json:"log_path_source"`
	EffectiveLogPaths        []string                       `json:"effective_log_paths"`
	SuggestedLogPath         string                         `json:"suggested_log_path,omitempty"`
	UserMetricsEndpoint      string                         `json:"user_metrics_endpoint,omitempty"`
	SuggestedMetricsEndpoint string                         `json:"suggested_metrics_endpoint,omitempty"`
	DetectedLogPaths         []string                       `json:"detected_log_paths"`
	LogCollectionOn          bool                           `json:"log_collection_on"`
	MetricCollectionOn       bool                           `json:"metric_collection_on"`
	VulnScanSupported        bool                           `json:"vuln_scan_supported"`
	VulnScanStatus           string                         `json:"vuln_scan_status"`
	LastVulnScanAt           string                         `json:"last_vuln_scan_at,omitempty"`
	VulnStatusMessage        string                         `json:"vuln_status_message"`
	Vulnerabilities          []ServiceVulnerabilityItem     `json:"vulnerabilities"`
	Catalog                  *catalog.ServiceDef            `json:"catalog,omitempty"`
	NginxVirtualHosts        []WebServerVirtualHostResponse `json:"nginx_virtual_hosts,omitempty"`
	ApacheVirtualHosts       []WebServerVirtualHostResponse `json:"apache_virtual_hosts,omitempty"`
}

// WebServerVirtualHostResponse represents a merged virtual host (one per domain)
// in API responses. One DB row per log_kind is merged into a single object
// with both access_log and error_log fields.
type WebServerVirtualHostResponse struct {
	ID          uint   `json:"id"`
	Domain      string `json:"domain"`
	AccessLog   string `json:"access_log,omitempty"`
	ErrorLog    string `json:"error_log,omitempty"`
	Enabled     bool   `json:"enabled"`
	Source      string `json:"source"` // "detected" or "user"
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`

	// Internal helper used during grouping in groupWebServerVirtualHosts(); not serialized.
	UpdatedAt_t time.Time `json:"-"`
}

type ServiceVulnerabilityItem struct {
	CVEID          string  `json:"cve_id"`
	CVSSScore      float32 `json:"cvss_score"`
	Severity       string  `json:"severity"`
	AlertLevel     string  `json:"alert_level"`
	Description    string  `json:"description"`
	FixedInVersion string  `json:"fixed_in_version"`
}

type UpdateCollectionRequest struct {
	LogCollectionOn    bool `json:"log_collection_on"`
	MetricCollectionOn bool `json:"metric_collection_on"`
}

type UpdateServiceConfigRequest struct {
	ConfirmedLogPath    string                        `json:"confirmed_log_path"`
	UserMetricsEndpoint string                        `json:"user_metrics_endpoint"`
	NginxVirtualHosts   []WebServerVirtualHostRequest `json:"nginx_virtual_hosts,omitempty"`
	ApacheVirtualHosts  []WebServerVirtualHostRequest `json:"apache_virtual_hosts,omitempty"`
}

// WebServerVirtualHostRequest is used when saving the virtual hosts table from the UI.
// One entry per domain row; the backend splits it into separate DB rows per log_kind.
type WebServerVirtualHostRequest struct {
	ID          uint   `json:"id,omitempty"`
	Domain      string `json:"domain" validate:"required"`
	AccessLog   string `json:"access_log"`
	ErrorLog    string `json:"error_log"`
	Enabled     bool   `json:"enabled"`
}
