package domain

import "time"

type AlertRule struct {
	ID       uint `gorm:"primaryKey" json:"id"`
	TenantID uint `gorm:"not null;index" json:"tenant_id"`

	// Template info (embedded from Go code constants — not a FK to a templates table)
	TemplateID      string `gorm:"type:varchar(50);not null" json:"template_id"`
	ServiceType     string `gorm:"type:varchar(30);not null;index" json:"service_type"`
	TemplateName    string `gorm:"type:varchar(255);not null" json:"template_name"`
	Description     string `gorm:"type:text" json:"description"`
	Explanation     string `gorm:"type:text" json:"explanation"`
	Unit            string `gorm:"type:varchar(20)" json:"unit"`
	ComparisonOp    string `gorm:"type:varchar(5);default:'>'" json:"comparison_op"`
	DefaultSeverity string `gorm:"type:varchar(20);default:'warning'" json:"default_severity"`

	ForDuration string `gorm:"type:varchar(10);default:'5m'" json:"for_duration"`
	IsEnabled   bool   `gorm:"type:boolean;default:true" json:"is_enabled"`

	// Domain restricts this rule to a specific virtual host domain
	// (used by nginx/apache rules that target a single vhost).
	// Empty means all domains / not applicable.
	Domain string `gorm:"type:varchar(255)" json:"domain"`
	// Pool restricts this rule to a specific PHP-FPM pool.
	// Empty means all pools / not applicable.
	Pool string `gorm:"type:varchar(255)" json:"pool"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Scopes     []AlertRuleScope `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE" json:"scopes"`
	Thresholds []AlertThreshold `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE" json:"thresholds"`
}

type AlertRuleScope struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	RuleID    uint   `gorm:"not null;index" json:"rule_id"`
	ScopeType string `gorm:"type:varchar(30);not null" json:"scope_type"` // host | service | all_hosts | all_services_of_type
	ScopeID   *uint  `json:"scope_id"`                                    // nullable for "all" scopes
}

type AlertThreshold struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	RuleID      uint    `gorm:"not null;index" json:"rule_id"`
	Label       string  `gorm:"type:varchar(255)" json:"label"`
	Threshold   float64 `gorm:"not null" json:"threshold"`
	Severity    string  `gorm:"type:varchar(20);not null;default:'warning'" json:"severity"`
	ForDuration string  `gorm:"type:varchar(10);default:'5m'" json:"for_duration"`
}

type AlertEvent struct {
	ID             uint            `gorm:"primaryKey" json:"id"`
	RuleID         uint            `gorm:"not null;index" json:"rule_id"`
	Rule           *AlertRule      `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE" json:"rule"`
	ThresholdID    *uint           `gorm:"index" json:"threshold_id"`
	Threshold      *AlertThreshold `gorm:"foreignKey:ThresholdID" json:"threshold"`
	TenantID       uint            `gorm:"not null;index" json:"tenant_id"`
	HostMachineID  string          `gorm:"type:varchar(255)" json:"host_machine_id"`
	ServiceID      *uint           `json:"service_id"`
	Status         string          `gorm:"type:varchar(20);not null" json:"status"`
	IsNotified     bool            `gorm:"type:boolean;default:false" json:"is_notified"`
	TriggeredAt    time.Time       `gorm:"not null" json:"triggered_at"`
	ResolvedAt     *time.Time      `json:"resolved_at"`
	AcknowledgedBy *uint           `json:"acknowledged_by"`

	// ActualValue stores the metric value that triggered this event
	ActualValue *float64 `gorm:"type:decimal(18,4)" json:"actual_value"`
	// Domain stores the virtual-host domain this event is associated with
	// (empty for host-level rules or rules that target all domains).
	Domain string `gorm:"type:varchar(255)" json:"domain"`
	// Pool stores the PHP-FPM pool this event is associated with
	// (empty for host-level rules or rules that target all pools).
	Pool string `gorm:"type:varchar(255)" json:"pool"`
	// ContainerName stores the container this event is associated with
	// (empty for host/service/domain/pool rules).
	ContainerName string `gorm:"type:varchar(255);index" json:"container_name"`
	// MetricString is a human-readable description of the metric (host, service, domain, etc.)
	MetricString string `gorm:"type:text" json:"metric_string"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
