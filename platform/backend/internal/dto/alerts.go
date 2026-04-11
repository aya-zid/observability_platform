package dto

type VulnerabilityAlertListItem struct {
	ID             uint    `json:"id"`
	HostID         uint    `json:"host_id"`
	Hostname       string  `json:"hostname"`
	MachineID      string  `json:"machine_id"`
	ServiceName    string  `json:"service_name"`
	ServiceVersion string  `json:"service_version"`
	CVEID          string  `json:"cve_id"`
	CVSSScore      float32 `json:"cvss_score"`
	Severity       string  `json:"severity"`
	AlertLevel     string  `json:"alert_level"`
	Description    string  `json:"description"`
	FixedInVersion string  `json:"fixed_in_version"`
}
