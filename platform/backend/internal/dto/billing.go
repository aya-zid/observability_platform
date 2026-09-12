package dto

type PendingRequestInfo struct {
	ID        uint    `json:"id"`
	PlanID    uint    `json:"plan_id"`
	PlanName  string  `json:"plan_name"`
	PlanPrice float64 `json:"plan_price"`
	Status    string  `json:"status"`
	Notes     string  `json:"notes"`
	CreatedAt string  `json:"created_at"`
}

type BillingOverview struct {
	Plan           PlanInfo            `json:"plan"`
	Usage          UsageInfo           `json:"usage"`
	AvailablePlans []PlanInfo          `json:"available_plans"`
	PendingRequest *PendingRequestInfo `json:"pending_request,omitempty"`
}

type PlanInfo struct {
	ID                   uint    `json:"id"`
	Name                 string  `json:"name"`
	MaxHosts             int     `json:"max_hosts"`
	MaxServicesPerHost   int     `json:"max_services_per_host"`
	LogRetentionDays     int     `json:"log_retention_days"`
	MetricsRetentionDays int     `json:"metrics_retention_days"`
	PricePerMonth        float64 `json:"price_per_month"`
}

type UsageInfo struct {
	HostsUsed        int   `json:"hosts_used"`
	HostsLimit       int   `json:"hosts_limit"`
	LogBytesToday    int64 `json:"log_bytes_today"`
	LogRetentionDays int   `json:"log_retention_days"`
}

type ChangePlanRequest struct {
	PlanID uint `json:"plan_id" validate:"required"`
}

type RequestPlanRequest struct {
	PlanID uint   `json:"plan_id" validate:"required"`
	Notes  string `json:"notes"`
}

type DailyUsage struct {
	Date     string `json:"date"`
	LogBytes int64  `json:"log_bytes"`
	Metrics  int64  `json:"metric_samples"`
}
