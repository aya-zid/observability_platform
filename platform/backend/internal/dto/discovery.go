package dto

// WebServerVirtualHostDiscovered represents a detected web-server virtual host during discovery
// Used to map domains to their access/error log files
type WebServerVirtualHostDiscovered struct {
	Domain    string `json:"domain"`               // e.g., "shop.example.com" or "(default)"
	AccessLog string `json:"access_log,omitempty"` // e.g., "/var/log/nginx/shop.access.log"
	ErrorLog  string `json:"error_log,omitempty"`  // e.g., "/var/log/nginx/shop.error.log"
	IsDefault bool   `json:"is_default"`           // true if this is the default/catch-all server block
}

type InstallDiscoveryService struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	LogPaths string `json:"log_paths"` // Legacy: flat comma-separated paths

	// Web-server virtual hosts (optional, for backward compatibility)
	VirtualHosts []WebServerVirtualHostDiscovered `json:"virtual_hosts,omitempty"`

	Notes   string `json:"notes"`
	Port    *int   `json:"port,omitempty"`
	Version string `json:"version,omitempty"`
}

type InstallDiscoveryReportRequest struct {
	Host         string                    `json:"host" validate:"required,min=1,max=255"`
	MachineID    string                    `json:"machine_id" validate:"required,min=4,max=255"`
	OS           string                    `json:"os,omitempty" validate:"omitempty,max=128"`
	Arch         string                    `json:"arch,omitempty" validate:"omitempty,max=64"`
	AgentVersion string                    `json:"agent_version,omitempty" validate:"omitempty,max=64"`
	Services     []InstallDiscoveryService `json:"services"`
}
