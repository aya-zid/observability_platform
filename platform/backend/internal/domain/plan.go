package domain

import "time"

type Plan struct {
	ID                   uint    `gorm:"primaryKey"`
	Name                 string  `gorm:"type:varchar(50);not null;uniqueIndex"`
	MaxHosts             int     `gorm:"not null"`
	MaxServicesPerHost   int     `gorm:"not null;default:20"`
	LogRetentionDays     int     `gorm:"not null"`
	MetricsRetentionDays int     `gorm:"not null"`
	MaxLogBytesPerDay    int64   `gorm:"not null"`
	PricePerMonth        float64 `gorm:"type:numeric(10,2)"`
	IsActive             bool    `gorm:"type:boolean;default:true"`
}

type TenantSubscription struct {
	ID               uint       `gorm:"primaryKey"`
	TenantID         uint       `gorm:"not null;uniqueIndex"`
	Tenant           Tenant     `gorm:"foreignKey:TenantID"`
	PlanID           uint       `gorm:"not null"`
	Plan             Plan       `gorm:"foreignKey:PlanID"`
	Status           string     `gorm:"type:varchar(20);not null;default:'active'"`
	StartedAt        time.Time  `gorm:"autoCreateTime"`
	CurrentPeriodEnd *time.Time
}
