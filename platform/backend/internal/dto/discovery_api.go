package dto

import "gitlab.medianet.tn/pfe/2026-aya-zid/observability-platform/backend/internal/catalog"

type DiscoveryHostListItem struct {
	MachineID    string `json:"machine_id"`
	Hostname     string `json:"hostname"`
	OSName       string `json:"os_name"`
	Arch         string `json:"arch"`
	AgentVersion string `json:"agent_version"`
	LastSeenAt   string `json:"last_seen_at"`
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
	MetricsMode        string `json:"metrics_mode"`
	LogCollectionOn    bool   `json:"log_collection_on"`
	MetricCollectionOn bool   `json:"metric_collection_on"`
	IntegrationStatus  string `json:"integration_status"`
	LoggingMode        string `json:"logging_mode"`
	VulnScanSupported  bool   `json:"vuln_scan_supported"`
	VulnScanStatus     string `json:"vuln_scan_status"`
	LastVulnScanAt     string `json:"last_vuln_scan_at,omitempty"`
}

type DiscoveryServiceDetailResponse struct {
	ID                  uint                     `json:"id"`
	MachineID           string                   `json:"machine_id"`
	Hostname            string                   `json:"hostname"`
	ServiceName         string                   `json:"service_name"`
	Port                *int                     `json:"port,omitempty"`
	Version             string                   `json:"version,omitempty"`
	Category            string                   `json:"category"`
	IsSupported         bool                     `json:"is_supported"`
	LoggingMode         string                   `json:"logging_mode"`
	UsesJournald        bool                     `json:"uses_journald"`
	UsesSyslog          bool                     `json:"uses_syslog"`
	AccessLogPath       string                   `json:"access_log_path"`
	ErrorLogPath        string                   `json:"error_log_path"`
	ConfirmedLogPath    string                   `json:"confirmed_log_path,omitempty"`
	LogPathSource       string                   `json:"log_path_source"`
	EffectiveLogPaths   []string                 `json:"effective_log_paths"`
	SuggestedLogPath    string                   `json:"suggested_log_path,omitempty"`
	UserMetricsEndpoint string                   `json:"user_metrics_endpoint,omitempty"`
	SuggestedMetricsEndpoint string              `json:"suggested_metrics_endpoint,omitempty"`
	MetricsMode         string                   `json:"metrics_mode"`
	IsJavaApp           bool                     `json:"is_java_app"`
	JMXExporterPort     *int                     `json:"jmx_exporter_port,omitempty"`
	DetectedLogPaths    []string                 `json:"detected_log_paths"`
	LogCollectionOn     bool                     `json:"log_collection_on"`
	MetricCollectionOn  bool                     `json:"metric_collection_on"`
	IntegrationStatus   string                   `json:"integration_status"`
	VulnScanSupported   bool                     `json:"vuln_scan_supported"`
	VulnScanStatus      string                   `json:"vuln_scan_status"`
	LastVulnScanAt      string                   `json:"last_vuln_scan_at,omitempty"`
	VulnStatusMessage   string                   `json:"vuln_status_message"`
	Vulnerabilities     []ServiceVulnerabilityItem `json:"vulnerabilities"`
	Catalog             *catalog.ServiceDef      `json:"catalog,omitempty"`
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
	ConfirmedLogPath   string `json:"confirmed_log_path"`
	UserMetricsEndpoint string `json:"user_metrics_endpoint"`
	MetricsMode         string `json:"metrics_mode"`
	IsJavaApp           bool   `json:"is_java_app"`
	JMXExporterPort     *int   `json:"jmx_exporter_port"`
}
