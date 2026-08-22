package dto

type AdminSignupRequest struct {
	FullName string `json:"full_name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type AdminLoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type AdminChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

type AdminAuthResponse struct {
	OK   bool   `json:"ok"`
	Role string `json:"role"`
}

type AdminProfileResponse struct {
	AdminID uint   `json:"admin_id"`
	Email   string `json:"email"`
	FullName string `json:"full_name"`
	Role    string `json:"role"`
}

type AdminDashboardStatsResponse struct {
	TotalTenants     int64            `json:"total_tenants"`
	ActiveTenants    int64            `json:"active_tenants"`
	BannedTenants    int64            `json:"banned_tenants"`
	TotalHosts       int64            `json:"total_hosts"`
	TotalIngestToday int64            `json:"total_ingest_today"`
	RevenueEstimate  float64          `json:"revenue_estimate"`
	PlanDistribution []PlanDistribution `json:"plan_distribution"`
	IngestionTrend   []DailyUsage     `json:"ingestion_trend"`
}

type PlanDistribution struct {
	PlanName string `json:"plan_name"`
	Count    int64  `json:"count"`
}

type AdminTenantListItem struct {
	ID                uint             `json:"id"`
	CompanyName       string           `json:"company_name"`
	ContactEmail      string           `json:"contact_email"`
	IsActive          bool             `json:"is_active"`
	SubscriptionStatus string          `json:"subscription_status"`
	HostsUsed         int              `json:"hosts_used"`
	PlanName          string           `json:"plan_name"`
	PlanPrice         float64          `json:"plan_price"`
	CurrentPeriodEnd  *string          `json:"current_period_end"`
	BannedAt          *string          `json:"banned_at"`
	BanReason         string           `json:"ban_reason"`
	CreatedAt         string           `json:"created_at"`
}

type AdminTenantDetailResponse struct {
	ID                uint             `json:"id"`
	CompanyName       string           `json:"company_name"`
	ContactEmail      string           `json:"contact_email"`
	ContactPhone      string           `json:"contact_phone"`
	IsActive          bool             `json:"is_active"`
	SubscriptionStatus string          `json:"subscription_status"`
	Timezone          string           `json:"timezone"`
	HostsUsed         int              `json:"hosts_used"`
	PlanName          string           `json:"plan_name"`
	PlanPrice         float64          `json:"plan_price"`
	LogRetentionDays  int              `json:"log_retention_days"`
	MetricsRetentionDays int           `json:"metrics_retention_days"`
	MaxHosts          int              `json:"max_hosts"`
	MaxLogBytesPerDay int64            `json:"max_log_bytes_per_day"`
	CurrentPeriodEnd  *string          `json:"current_period_end"`
	StartedAt         string           `json:"started_at"`
	BannedAt          *string          `json:"banned_at"`
	BanReason         string           `json:"ban_reason"`
	AIReportsEnabled  bool             `json:"ai_reports_enabled"`
	UsersCount        int              `json:"users_count"`
	CreatedAt         string           `json:"created_at"`
}

type AdminBanTenantRequest struct {
	Reason string `json:"reason"`
}

type AdminCreatePackRequest struct {
	Name                 string  `json:"name" validate:"required,min=2,max=50"`
	Description          string  `json:"description"`
	PricePerMonth        float64 `json:"price_per_month" validate:"min=0"`
	MaxHosts             int     `json:"max_hosts" validate:"required,min=1"`
	MaxServicesPerHost   int     `json:"max_services_per_host" validate:"required,min=1"`
	LogRetentionDays     int     `json:"log_retention_days" validate:"required,min=1"`
	MetricsRetentionDays int     `json:"metrics_retention_days" validate:"required,min=1"`
	MaxLogBytesPerDay    int64   `json:"max_log_bytes_per_day" validate:"required,min=1"`
	AIReportsEnabled     bool    `json:"ai_reports_enabled"`
	DurationMonths       int     `json:"duration_months" validate:"required,min=0"`
	IsActive             bool    `json:"is_active"`
	IsTrial              bool    `json:"is_trial"`
}

type AdminUpdatePackRequest struct {
	Name                 string  `json:"name" validate:"required,min=2,max=50"`
	Description          string  `json:"description"`
	PricePerMonth        float64 `json:"price_per_month" validate:"min=0"`
	MaxHosts             int     `json:"max_hosts" validate:"required,min=1"`
	MaxServicesPerHost   int     `json:"max_services_per_host" validate:"required,min=1"`
	LogRetentionDays     int     `json:"log_retention_days" validate:"required,min=1"`
	MetricsRetentionDays int     `json:"metrics_retention_days" validate:"required,min=1"`
	MaxLogBytesPerDay    int64   `json:"max_log_bytes_per_day" validate:"required,min=1"`
	AIReportsEnabled     bool    `json:"ai_reports_enabled"`
	DurationMonths       int     `json:"duration_months" validate:"required,min=0"`
	IsActive             bool    `json:"is_active"`
	IsTrial              bool    `json:"is_trial"`
}

type AdminPackResponse struct {
	ID                   uint    `json:"id"`
	Name                 string  `json:"name"`
	Description          string  `json:"description"`
	PricePerMonth        float64 `json:"price_per_month"`
	MaxHosts             int     `json:"max_hosts"`
	MaxServicesPerHost   int     `json:"max_services_per_host"`
	LogRetentionDays     int     `json:"log_retention_days"`
	MetricsRetentionDays int     `json:"metrics_retention_days"`
	MaxLogBytesPerDay    int64   `json:"max_log_bytes_per_day"`
	AIReportsEnabled     bool    `json:"ai_reports_enabled"`
	DurationMonths       int     `json:"duration_months"`
	IsActive             bool    `json:"is_active"`
	IsTrial              bool    `json:"is_trial"`
	ActiveSubscriptions  int64   `json:"active_subscriptions"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

type AdminAssignPackRequest struct {
	PackID uint `json:"pack_id" validate:"required"`
}

type AdminRenewSubscriptionRequest struct {
	DurationMonths int `json:"duration_months" validate:"required,min=1"`
}
