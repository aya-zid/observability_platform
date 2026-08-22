package domain

import "time"

type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
)

type User struct {
    ID           uint      `gorm:"primaryKey"`
    TenantID     uint      `gorm:"index"`
    Tenant       Tenant    `gorm:"foreignKey:TenantID"`

    FullName     string    `gorm:"type:varchar(255)"`
    Email        string    `gorm:"type:varchar(255);unique;not null"`
    PasswordHash string    `gorm:"type:varchar(255);not null"`
    Role         string    `gorm:"type:varchar(50);default:'admin'"`
    Status       string    `gorm:"type:varchar(20);not null;default:'active'"`
    PasswordChangedAt *time.Time `gorm:"column:password_changed_at"`

    CreatedAt    time.Time `gorm:"autoCreateTime"`
    UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}