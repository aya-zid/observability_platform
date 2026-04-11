package dto

type InstallDiscoveryService struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	LogPaths string `json:"log_paths"`
	Notes    string `json:"notes"`
	Port     *int   `json:"port,omitempty"`
	Version  string `json:"version,omitempty"`
}

type InstallDiscoveryReportRequest struct {
	Host         string                   `json:"host" validate:"required,min=1,max=255"`
	MachineID    string                   `json:"machine_id" validate:"required,min=4,max=255"`
	OS           string                   `json:"os,omitempty" validate:"omitempty,max=128"`
	Arch         string                   `json:"arch,omitempty" validate:"omitempty,max=64"`
	AgentVersion string                   `json:"agent_version,omitempty" validate:"omitempty,max=64"`
	Services     []InstallDiscoveryService `json:"services"`
}
