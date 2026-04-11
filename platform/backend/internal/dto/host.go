package dto

type HostRegisterRequest struct {
	MachineID    string `json:"machine_id" validate:"required,min=4,max=255"`
	Hostname     string `json:"hostname" validate:"required,min=1,max=255"`
	OSName       string `json:"os_name,omitempty" validate:"omitempty,max=128"`
	OSVersion    string `json:"os_version,omitempty" validate:"omitempty,max=128"`
	Arch         string `json:"arch,omitempty" validate:"omitempty,max=64"`
	AgentVersion string `json:"agent_version,omitempty" validate:"omitempty,max=64"`
}

type HostHeartbeatRequest struct {
	MachineID    string `json:"machine_id" validate:"required,min=4,max=255"`
	Hostname     string `json:"hostname,omitempty" validate:"omitempty,max=255"`
	AgentVersion string `json:"agent_version,omitempty" validate:"omitempty,max=64"`
}

