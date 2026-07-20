package domain

import "time"

type NotificationChannel struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TenantID   uint      `gorm:"not null;index" json:"-"`
	Type       string    `gorm:"type:varchar(20);not null" json:"type"`
	Target     string    `gorm:"type:varchar(255);not null" json:"target"`
	IsVerified bool      `gorm:"type:boolean;default:false" json:"is_verified"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}
