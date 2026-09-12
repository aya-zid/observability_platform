package domain

import "time"

type Plan struct {
	ID                   uint    `gorm:"primaryKey"`
	Name                 string  `gorm:"type:varchar(50);not null;uniqueIndex"`
	MaxHosts             int     `gorm:"not null"`
	MaxServicesPerHost   int     `gorm:"not null;default:20"`
	LogRetentionDays     int     `gorm:"not null"`
	MetricsRetentionDays int     `gorm:"not null"`
	PricePerMonth        float64 `gorm:"type:numeric(10,2)"`
	IsActive             bool    `gorm:"type:boolean;default:true"`
	AIReportsEnabled     bool    `gorm:"type:boolean;not null;default:false"`
	Description          string  `gorm:"type:text"`
	DurationMonths       int     `gorm:"not null;default:1"`
	IsTrial              bool    `gorm:"type:boolean;not null;default:false"`
}

type TenantSubscription struct {
	ID               uint      `gorm:"primaryKey"`
	TenantID         uint      `gorm:"not null;uniqueIndex"`
	Tenant           Tenant    `gorm:"foreignKey:TenantID"`
	PlanID           uint      `gorm:"not null"`
	Plan             Plan      `gorm:"foreignKey:PlanID"`
	Status           string    `gorm:"type:varchar(20);not null;default:'active'"`
	StartedAt        time.Time `gorm:"autoCreateTime"`
	CurrentPeriodEnd *time.Time
	Notes            string `gorm:"type:text"`
}

// SubscriptionRequest represents a tenant's request to upgrade/change their
// subscription plan. The admin reviews and approves or rejects the request.
type SubscriptionRequest struct {
	ID         uint      `gorm:"primaryKey"`
	TenantID   uint      `gorm:"not null;index"`
	Tenant     Tenant    `gorm:"foreignKey:TenantID"`
	PlanID     uint      `gorm:"not null"`
	Plan       Plan      `gorm:"foreignKey:PlanID"`
	Status     string    `gorm:"type:varchar(20);not null;default:'pending'"` // pending, approved, rejected
	Notes      string    `gorm:"type:text"`                                   // tenant's message/reason
	AdminNotes string    `gorm:"type:text"`                                   // admin's response
	ReviewedAt *time.Time
	ReviewedBy *uint
	CreatedAt  time.Time `gorm:"autoCreateTime"`
}
