package domain

import "time"

type Invitation struct {
	ID        uint      `gorm:"primaryKey"`
	TenantID  uint      `gorm:"not null;index"`
	Tenant    Tenant    `gorm:"foreignKey:TenantID"`
	Email     string    `gorm:"type:varchar(255);not null"`
	Role      string    `gorm:"type:varchar(50);not null;default:'member'"`
	Token     string    `gorm:"type:varchar(255);uniqueIndex;not null"`
	InvitedBy uint      `gorm:"not null"`
	Status    string    `gorm:"type:varchar(20);not null;default:'pending'"`
	ExpiresAt time.Time `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
