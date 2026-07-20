package domain

import "time"

type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusPastDue   SubscriptionStatus = "past_due"
	SubscriptionStatusCanceled  SubscriptionStatus = "canceled"
	SubscriptionStatusSuspended SubscriptionStatus = "suspended"
)

type Tenant struct {
    ID           uint      `gorm:"primaryKey"`
    UUID         string    `gorm:"type:uuid;uniqueIndex"`
    CompanyName  string    `gorm:"type:varchar(255);not null"`
    ContactEmail string    `gorm:"type:varchar(255);unique;not null"`
    ContactPhone string    `gorm:"type:varchar(50)"`
    IsActive     bool      `gorm:"type:boolean;not null;default:true"`
    SubscriptionStatus SubscriptionStatus `gorm:"type:varchar(32);not null;default:'active'"`
    APIKeyEnabled bool     `gorm:"type:boolean;not null;default:true"`
    // APIKey is the tenant API key used by agents to authenticate (Basic auth).
    // NOTE: This is stored in clear text by request (not recommended for production).
    APIKey       string    `gorm:"column:api_key;type:varchar(255);unique;not null"`
    APIKeyLastUsedAt *time.Time `gorm:"column:api_key_last_used_at"`
    Timezone     string    `gorm:"type:varchar(50);default:'UTC'"`

    // Has many Users (bidirectional)
    Users        []User    `gorm:"foreignKey:TenantID"`

    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}
