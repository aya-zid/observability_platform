package domain

import "time"

type AlertRule struct {
	ID       uint `gorm:"primaryKey"`
	TenantID uint `gorm:"not null;index"`

	// Template info (embedded from Go code constants — not a FK to a templates table)
	TemplateID      string `gorm:"type:varchar(50);not null"`
	ServiceType     string `gorm:"type:varchar(30);not null;index"`
	TemplateName    string `gorm:"type:varchar(255);not null"`
	Description     string `gorm:"type:text"`
	Unit            string `gorm:"type:varchar(20)"`
	ComparisonOp    string `gorm:"type:varchar(5);default:'>'"`
	DefaultSeverity string `gorm:"type:varchar(20);default:'warning'"`

	ForDuration string `gorm:"type:varchar(10);default:'5m'"`
	IsEnabled   bool   `gorm:"type:boolean;default:true"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`

	Scopes     []AlertRuleScope    `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE"`
	Thresholds []AlertThreshold    `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE"`
}

type AlertRuleScope struct {
	ID        uint   `gorm:"primaryKey"`
	RuleID    uint   `gorm:"not null;index"`
	ScopeType string `gorm:"type:varchar(30);not null"` // host | service | all_hosts | all_services_of_type
	ScopeID   *uint  // nullable for "all" scopes
}

type AlertThreshold struct {
	ID        uint    `gorm:"primaryKey"`
	RuleID    uint    `gorm:"not null;index"`
	Label     string  `gorm:"type:varchar(255)"`
	Threshold float64 `gorm:"not null"`
	Severity  string  `gorm:"type:varchar(20);not null;default:'warning'"`
	ForDuration string `gorm:"type:varchar(10);default:'5m'"`
}

type AlertEvent struct {
	ID            uint      `gorm:"primaryKey"`
	RuleID        uint      `gorm:"not null;index"`
	Rule          AlertRule     `gorm:"foreignKey:RuleID;constraint:OnDelete:CASCADE"`
	ThresholdID   *uint         `gorm:"index"`
	Threshold     *AlertThreshold `gorm:"foreignKey:ThresholdID"`
	TenantID      uint      `gorm:"not null;index"`
	HostMachineID string    `gorm:"type:varchar(255)"`
	ServiceID     *uint
	Status        string    `gorm:"type:varchar(20);not null"`
	IsNotified    bool      `gorm:"type:boolean;default:false"`
	TriggeredAt   time.Time `gorm:"not null"`
	ResolvedAt    *time.Time
	AcknowledgedBy *uint
	CreatedAt     time.Time `gorm:"autoCreateTime"`
}
