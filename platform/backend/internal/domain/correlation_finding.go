package domain

import "time"

type CorrelationFinding struct {
	ID        uint   `gorm:"primaryKey"`
	TenantID  uint   `gorm:"not null;index"`
	MachineID string `gorm:"type:varchar(255);not null;index"`
	RuleID    string `gorm:"type:varchar(64);not null;index"`
	ServiceName *string `gorm:"type:varchar(120);index"`
	Severity  string `gorm:"type:varchar(16);not null"`
	Title     string `gorm:"type:varchar(255);not null"`
	Message   string `gorm:"type:text;not null"`
	Evidence  string `gorm:"type:jsonb"`
	RecommendedAction *string `gorm:"type:text"`
	DashboardLink *string `gorm:"type:text"`
	Hostname  string `gorm:"type:varchar(255);not null;default:''"`
	FiredAt   time.Time `gorm:"not null;index"`
	ResolvedAt *time.Time `gorm:"index"`

	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}
