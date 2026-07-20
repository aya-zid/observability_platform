package dto

type BillingOverview struct {
	Plan           PlanInfo   `json:"plan"`
	Usage          UsageInfo  `json:"usage"`
	AvailablePlans []PlanInfo `json:"available_plans"`
}

type PlanInfo struct {
	ID                   uint    `json:"id"`
	Name                 string  `json:"name"`
	MaxHosts             int     `json:"max_hosts"`
	MaxServicesPerHost   int     `json:"max_services_per_host"`
	LogRetentionDays     int     `json:"log_retention_days"`
	MetricsRetentionDays int     `json:"metrics_retention_days"`
	MaxLogBytesPerDay    int64   `json:"max_log_bytes_per_day"`
	PricePerMonth        float64 `json:"price_per_month"`
}

type UsageInfo struct {
	HostsUsed        int   `json:"hosts_used"`
	HostsLimit       int   `json:"hosts_limit"`
	LogBytesToday    int64 `json:"log_bytes_today"`
	LogBytesLimit    int64 `json:"log_bytes_limit"`
	LogRetentionDays int   `json:"log_retention_days"`
}

type ChangePlanRequest struct {
	PlanID uint `json:"plan_id" validate:"required"`
}

type DailyUsage struct {
	Date     string `json:"date"`
	LogBytes int64  `json:"log_bytes"`
	Metrics  int64  `json:"metric_samples"`
}
