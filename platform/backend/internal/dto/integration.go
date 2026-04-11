package dto

type IntegrationInstallResponse struct {
	ServiceName   string `json:"service_name"`
	ScriptURL     string `json:"script_url"`
	InstallCommand string `json:"install_command"`
}

type IntegrationStatusIngestRequest struct {
	MachineID          string `json:"machine_id" validate:"required,min=4,max=255"`
	ServiceName        string `json:"service_name" validate:"required,min=2,max=120"`
	LogCollectionOn    bool   `json:"log_collection_on"`
	MetricCollectionOn bool   `json:"metric_collection_on"`
	IntegrationStatus  string `json:"integration_status,omitempty" validate:"omitempty,oneof=not_installed pending active error"`
}

